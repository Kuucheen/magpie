package proxyqueue

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"strconv"
	"strings"
	"time"

	"magpie/internal/config"
	"magpie/internal/domain"
	"magpie/internal/jobs/runtime"
	"magpie/internal/support"

	"github.com/charmbracelet/log"
	"github.com/redis/go-redis/v9"
)

const (
	proxyKeyPrefix = "proxy:"

	legacyQueueKey          = "proxy_queue"
	proxyQueueHeadKey       = "proxy_queue_heads"
	queueShardKeyPrefix     = "proxy_queue:"
	defaultQueueShards      = 16
	maxQueueShards          = 128
	minDequeueSleep         = 10 * time.Millisecond
	idleQueueSleep          = 250 * time.Millisecond
	maxDequeueSleep         = 2 * time.Second
	processingLease         = 5 * time.Minute
	queueRescheduleLockKey  = "magpie:leader:proxy_queue_reschedule"
	queueRescheduleStateKey = "magpie:queue:proxy:interval_ms"
)

//go:embed pop.lua
var luaPopScript string

type RedisProxyQueue struct {
	client         *redis.Client
	ctx            context.Context
	popScript      *redis.Script
	queueShardKeys []string
	popQueueKeys   []string
}

type proxyPopResult struct {
	Found       bool
	ProxyJSON   string
	ScoreMs     int64
	NextReadyMs int64
}

type queuedProxyUser struct {
	ID uint `json:"ID"`
}

type queuedProxy struct {
	ID       uint64            `json:"ID"`
	IP       string            `json:"IP"`
	Port     uint16            `json:"Port"`
	Username string            `json:"Username"`
	Password string            `json:"Password"`
	Hash     []byte            `json:"Hash"`
	UserIDs  []uint            `json:"UserIDs,omitempty"`
	Users    []queuedProxyUser `json:"Users,omitempty"` // Legacy payload compatibility
}

var PublicProxyQueue RedisProxyQueue
var runLeaderTaskOnce = support.RunLeaderTaskOnce

func init() {
	client, err := support.GetRedisClient()
	if err != nil {
		log.Warn("Redis unavailable during proxy queue init; continuing in degraded mode", "error", err)
	}
	PublicProxyQueue = *NewRedisProxyQueue(client)

	go func() {
		updates := config.CheckIntervalUpdates()
		for interval := range updates {
			err := applyIntervalUpdateAsLeader(
				queueRescheduleLockKey,
				interval,
				PublicProxyQueue.Reschedule,
			)
			if err != nil {
				log.Error("Failed to reschedule proxy queue after interval update", "error", err)
			}
		}
	}()
}

func applyIntervalUpdateAsLeader(lockKey string, interval time.Duration, reschedule func(time.Duration) error) error {
	return applyIntervalUpdateAsLeaderWithRunner(runLeaderTaskOnce, lockKey, interval, reschedule)
}

func applyIntervalUpdateAsLeaderWithRunner(
	runner func(context.Context, string, time.Duration, func(context.Context) error) error,
	lockKey string,
	interval time.Duration,
	reschedule func(time.Duration) error,
) error {
	if runner == nil {
		return errors.New("leader runner is nil")
	}
	if reschedule == nil {
		return errors.New("reschedule function is nil")
	}

	err := runner(context.Background(), lockKey, support.DefaultLeadershipTTL, func(context.Context) error {
		return reschedule(interval)
	})
	if errors.Is(err, support.ErrLeaderLockNotAcquired) {
		return nil
	}
	return err
}

func NewRedisProxyQueue(client *redis.Client) *RedisProxyQueue {
	shards := support.GetEnvInt("PROXY_QUEUE_SHARDS", defaultQueueShards)
	if shards <= 0 {
		shards = defaultQueueShards
	}
	if shards > maxQueueShards {
		shards = maxQueueShards
	}

	shardKeys := buildQueueShardKeys(shards)
	popKeys := buildPopQueueKeys(shardKeys)

	queue := &RedisProxyQueue{
		client:         client,
		ctx:            context.Background(),
		popScript:      redis.NewScript(luaPopScript),
		queueShardKeys: shardKeys,
		popQueueKeys:   popKeys,
	}
	if client != nil {
		if err := queue.refreshQueueHeads(); err != nil {
			log.Warn("proxy queue head refresh failed", "error", err)
		}
	}
	return queue
}

func (rpq *RedisProxyQueue) clientOrErr() (*redis.Client, error) {
	if rpq == nil {
		return nil, errors.New("redis proxy queue is nil")
	}
	if rpq.client != nil {
		return rpq.client, nil
	}

	client, err := support.GetRedisClient()
	if err != nil {
		return nil, fmt.Errorf("redis proxy queue unavailable: %w", err)
	}
	return client, nil
}

func (rpq *RedisProxyQueue) baseContext() context.Context {
	if rpq == nil || rpq.ctx == nil {
		return context.Background()
	}
	return rpq.ctx
}

func (rpq *RedisProxyQueue) popKeys() []string {
	if len(rpq.popQueueKeys) == 0 {
		return []string{legacyQueueKey}
	}
	return rpq.popQueueKeys
}

func (rpq *RedisProxyQueue) refreshQueueHeads() error {
	client, err := rpq.clientOrErr()
	if err != nil {
		return err
	}
	ctx := rpq.baseContext()

	pipe := client.Pipeline()
	pipe.Del(ctx, proxyQueueHeadKey)

	for _, key := range rpq.popKeys() {
		entries, err := client.ZRangeWithScores(ctx, key, 0, 0).Result()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			continue
		}
		pipe.ZAdd(ctx, proxyQueueHeadKey, redis.Z{
			Score:  entries[0].Score,
			Member: key,
		})
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	return nil
}

func buildQueueShardKeys(shards int) []string {
	keys := make([]string, shards)
	for i := 0; i < shards; i++ {
		keys[i] = fmt.Sprintf("%s%d", queueShardKeyPrefix, i)
	}
	return keys
}

func buildPopQueueKeys(shardKeys []string) []string {
	keys := make([]string, 0, len(shardKeys)+1)
	keys = append(keys, legacyQueueKey)
	keys = append(keys, shardKeys...)
	return keys
}

func (rpq *RedisProxyQueue) queueKeyForMember(member string) string {
	if rpq == nil || len(rpq.queueShardKeys) == 0 {
		return legacyQueueKey
	}
	idx := proxyQueueShardIndex(member, len(rpq.queueShardKeys))
	return rpq.queueShardKeys[idx]
}

func proxyQueueShardIndex(member string, shards int) int {
	if shards <= 1 {
		return 0
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(member))
	return int(hasher.Sum32() % uint32(shards))
}

func (rpq *RedisProxyQueue) AddToQueue(proxies []domain.Proxy) error {
	if len(proxies) == 0 {
		return nil
	}

	client, err := rpq.clientOrErr()
	if err != nil {
		return err
	}
	ctx := rpq.baseContext()

	pipe := client.Pipeline()
	interval := config.GetTimeBetweenChecks()
	now := time.Now()
	proxyLenDuration := time.Duration(len(proxies))
	batchSize := 500 // Adjust based on your Redis server capabilities

	for i, proxy := range proxies {
		offset := (interval * time.Duration(i)) / proxyLenDuration
		nextCheck := now.Add(offset)
		hashKey := string(proxy.Hash)
		proxyKey := proxyKeyPrefix + hashKey
		queueKey := rpq.queueKeyForMember(hashKey)

		proxyJSON, err := marshalQueuedProxy(proxy)
		if err != nil {
			return fmt.Errorf("failed to marshal proxy: %w", err)
		}

		pipe.Set(ctx, proxyKey, proxyJSON, 0)
		pipe.ZAddArgs(ctx, queueKey, redis.ZAddArgs{
			NX: true,
			Members: []redis.Z{{
				Score:  float64(nextCheck.UnixMilli()),
				Member: hashKey,
			}},
		})
		if queueKey != legacyQueueKey {
			pipe.ZRem(ctx, legacyQueueKey, hashKey)
		}
		pipe.ZAddArgs(ctx, proxyQueueHeadKey, redis.ZAddArgs{
			LT: true,
			Members: []redis.Z{{
				Score:  float64(nextCheck.UnixMilli()),
				Member: queueKey,
			}},
		})

		// Execute in batches to prevent oversized pipelines
		if i%batchSize == 0 && i > 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				return fmt.Errorf("batch pipeline failed: %w", err)
			}
			pipe = client.Pipeline()
		}
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("final pipeline exec failed: %w", err)
	}

	return nil
}

func (rpq *RedisProxyQueue) RemoveFromQueue(proxies []domain.Proxy) error {
	if rpq == nil {
		return errors.New("redis proxy queue is nil")
	}
	if len(proxies) == 0 {
		return nil
	}

	client, err := rpq.clientOrErr()
	if err != nil {
		return err
	}
	ctx := rpq.baseContext()

	const batchSize = 500
	pipe := client.Pipeline()
	opCount := 0

	flush := func() error {
		if opCount == 0 {
			return nil
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("remove pipeline exec failed: %w", err)
		}
		pipe = client.Pipeline()
		opCount = 0
		return nil
	}

	for _, proxy := range proxies {
		if len(proxy.Hash) == 0 {
			continue
		}

		hashKey := string(proxy.Hash)
		proxyKey := proxyKeyPrefix + hashKey
		queueKey := rpq.queueKeyForMember(hashKey)

		pipe.Del(ctx, proxyKey)
		opCount++
		pipe.ZRem(ctx, queueKey, hashKey)
		opCount++
		if queueKey != legacyQueueKey {
			pipe.ZRem(ctx, legacyQueueKey, hashKey)
			opCount++
		}

		if opCount >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	return flush()
}

func (rpq *RedisProxyQueue) GetNextProxy() (domain.Proxy, time.Time, error) {
	return rpq.GetNextProxyContext(rpq.baseContext())
}

func (rpq *RedisProxyQueue) GetNextProxyContext(ctx context.Context) (domain.Proxy, time.Time, error) {
	if ctx == nil {
		ctx = rpq.baseContext()
	}
	client, err := rpq.clientOrErr()
	if err != nil {
		return domain.Proxy{}, time.Time{}, err
	}
	popScript := rpq.popScript
	if popScript == nil {
		popScript = redis.NewScript(luaPopScript)
	}

	for {
		select {
		case <-ctx.Done():
			return domain.Proxy{}, time.Time{}, ctx.Err()
		default:
		}

		currentTimeMs := time.Now().UnixMilli()
		result, err := popScript.Run(
			ctx,
			client,
			[]string{proxyQueueHeadKey},
			currentTimeMs,
			int64(processingLease/time.Millisecond),
			proxyKeyPrefix,
		).Result()

		if err != nil {
			return domain.Proxy{}, time.Time{}, fmt.Errorf("lua script failed: %w", err)
		}

		popResult, err := parseProxyPopResult(result)
		if err != nil {
			return domain.Proxy{}, time.Time{}, fmt.Errorf("invalid lua response: %w", err)
		}
		if !popResult.Found {
			waitDuration := dequeueWaitDuration(popResult.NextReadyMs, currentTimeMs)
			if err := waitForNextProxyPoll(ctx, waitDuration); err != nil {
				return domain.Proxy{}, time.Time{}, err
			}
			continue
		}

		var payload queuedProxy
		if err := json.Unmarshal([]byte(popResult.ProxyJSON), &payload); err != nil {
			return domain.Proxy{}, time.Time{}, fmt.Errorf("failed to unmarshal proxy: %w", err)
		}
		proxy := payload.toDomainProxy()

		return proxy, time.UnixMilli(popResult.ScoreMs), nil
	}
}

func dequeueWaitDuration(nextReadyMs int64, currentMs int64) time.Duration {
	if nextReadyMs <= 0 {
		return idleQueueSleep
	}

	waitMs := nextReadyMs - currentMs
	if waitMs <= 0 {
		return minDequeueSleep
	}

	wait := time.Duration(waitMs) * time.Millisecond
	if wait < minDequeueSleep {
		return minDequeueSleep
	}
	if wait > maxDequeueSleep {
		return maxDequeueSleep
	}
	return wait
}

func waitForNextProxyPoll(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		duration = minDequeueSleep
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func coerceLuaString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("expected string/[]byte, got %T", value)
	}
}

func coerceLuaInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case uint64:
		if v > math.MaxInt64 {
			return 0, fmt.Errorf("uint64 value out of range: %d", v)
		}
		return int64(v), nil
	case float64:
		if math.Trunc(v) != v {
			return 0, fmt.Errorf("non-integer float64 value %f", v)
		}
		return int64(v), nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	case []byte:
		parsed, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("expected numeric lua response, got %T", value)
	}
}

func (rpq *RedisProxyQueue) RequeueProxy(proxy domain.Proxy, lastCheckTime time.Time) error {
	client, err := rpq.clientOrErr()
	if err != nil {
		return err
	}
	ctx := rpq.baseContext()

	interval := rpq.getEffectiveCheckInterval()
	base := lastCheckTime
	// Clamp to now so overdue proxies don't keep hogging the queue.
	if now := time.Now(); now.After(base) {
		base = now
	}
	nextCheck := base.Add(interval)
	hashKey := string(proxy.Hash)
	proxyKey := proxyKeyPrefix + hashKey
	queueKey := rpq.queueKeyForMember(hashKey)

	proxyJSON, err := marshalQueuedProxy(proxy)
	if err != nil {
		return fmt.Errorf("failed to marshal proxy: %w", err)
	}

	pipe := client.Pipeline()
	pipe.Set(ctx, proxyKey, proxyJSON, 0)
	if queueKey != legacyQueueKey {
		pipe.ZRem(ctx, legacyQueueKey, hashKey)
	}
	pipe.ZAdd(ctx, queueKey, redis.Z{
		Score:  float64(nextCheck.UnixMilli()),
		Member: hashKey,
	})
	pipe.ZAddArgs(ctx, proxyQueueHeadKey, redis.ZAddArgs{
		LT: true,
		Members: []redis.Z{{
			Score:  float64(nextCheck.UnixMilli()),
			Member: queueKey,
		}},
	})

	_, err = pipe.Exec(ctx)
	return err
}

func (rpq *RedisProxyQueue) getEffectiveCheckInterval() time.Duration {
	fallback := config.GetTimeBetweenChecks()
	client, err := rpq.clientOrErr()
	if err != nil {
		return fallback
	}
	ctx := rpq.baseContext()

	raw, err := client.Get(ctx, queueRescheduleStateKey).Result()
	if err != nil {
		return fallback
	}
	return parseIntervalStateMillis(raw, fallback)
}

func parseIntervalStateMillis(raw string, fallback time.Duration) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}

	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || ms <= 0 {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}

func marshalQueuedProxy(proxy domain.Proxy) ([]byte, error) {
	return json.Marshal(newQueuedProxy(proxy))
}

func newQueuedProxy(proxy domain.Proxy) queuedProxy {
	return queuedProxy{
		ID:       proxy.ID,
		IP:       proxy.GetIp(),
		Port:     proxy.Port,
		Username: proxy.Username,
		Password: proxy.Password,
		Hash:     proxy.Hash,
		UserIDs:  collectQueuedUserIDs(proxy.Users),
	}
}

func (qp queuedProxy) toDomainProxy() domain.Proxy {
	userIDs := qp.UserIDs
	if len(userIDs) == 0 && len(qp.Users) > 0 {
		userIDs = make([]uint, 0, len(qp.Users))
		for _, user := range qp.Users {
			if user.ID == 0 {
				continue
			}
			userIDs = append(userIDs, user.ID)
		}
	}

	users := make([]domain.User, 0, len(userIDs))
	seen := make(map[uint]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID == 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		users = append(users, domain.User{ID: userID})
	}

	return domain.Proxy{
		ID:       qp.ID,
		IP:       qp.IP,
		Port:     qp.Port,
		Username: qp.Username,
		Password: qp.Password,
		Hash:     qp.Hash,
		Users:    users,
	}
}

func collectQueuedUserIDs(users []domain.User) []uint {
	if len(users) == 0 {
		return nil
	}

	out := make([]uint, 0, len(users))
	seen := make(map[uint]struct{}, len(users))
	for _, user := range users {
		if user.ID == 0 {
			continue
		}
		if _, ok := seen[user.ID]; ok {
			continue
		}
		seen[user.ID] = struct{}{}
		out = append(out, user.ID)
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func (rpq *RedisProxyQueue) GetProxyCount() (int64, error) {
	client, err := rpq.clientOrErr()
	if err != nil {
		return 0, err
	}
	ctx := rpq.baseContext()

	var total int64
	for _, key := range rpq.popKeys() {
		count, err := client.ZCard(ctx, key).Result()
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (rpq *RedisProxyQueue) GetActiveInstances() (int, error) {
	client, err := rpq.clientOrErr()
	if err != nil {
		return 0, err
	}
	return runtime.CountActiveInstances(rpq.baseContext(), client)
}

func (rpq *RedisProxyQueue) RequeueAll() (int64, error) {
	if rpq == nil {
		return 0, errors.New("redis proxy queue is nil")
	}

	client, err := rpq.clientOrErr()
	if err != nil {
		return 0, err
	}
	ctx := rpq.baseContext()
	interval := rpq.getEffectiveCheckInterval()
	if interval <= 0 {
		interval = time.Second
	}

	const batchSize int64 = 500

	var total int64
	now := time.Now()

	for _, key := range rpq.popKeys() {
		count, err := client.ZCard(ctx, key).Result()
		if err != nil {
			return total, fmt.Errorf("requeue all: count queue %s: %w", key, err)
		}
		if count == 0 {
			continue
		}

		total += count
		members, err := client.ZRange(ctx, key, 0, count-1).Result()
		if err != nil {
			return total, fmt.Errorf("requeue all: list queue %s: %w", key, err)
		}

		for start := 0; start < len(members); start += int(batchSize) {
			end := start + int(batchSize)
			if end > len(members) {
				end = len(members)
			}

			pipe := client.Pipeline()
			for index, member := range members[start:end] {
				position := int64(start + index)
				offset := (interval * time.Duration(position)) / time.Duration(count)
				nextCheck := now.Add(offset)
				pipe.ZAddXX(ctx, key, redis.Z{
					Score:  float64(nextCheck.UnixMilli()),
					Member: member,
				})
			}

			if _, err := pipe.Exec(ctx); err != nil {
				return total, fmt.Errorf("requeue all: update queue %s: %w", key, err)
			}
		}
	}

	if err := rpq.refreshQueueHeads(); err != nil {
		return total, fmt.Errorf("requeue all: refresh queue heads: %w", err)
	}

	return total, nil
}

func (rpq *RedisProxyQueue) Close() error {
	return support.CloseRedisClient()
}

func (rpq *RedisProxyQueue) Reschedule(interval time.Duration) error {
	if rpq == nil {
		return errors.New("redis proxy queue is nil")
	}

	client, err := rpq.clientOrErr()
	if err != nil {
		return err
	}
	ctx := rpq.baseContext()

	if interval <= 0 {
		interval = time.Second
	}

	if err := client.Set(ctx, queueRescheduleStateKey, strconv.FormatInt(interval.Milliseconds(), 10), 0).Err(); err != nil {
		return fmt.Errorf("reschedule: failed to persist interval state: %w", err)
	}

	// Existing queue members keep their current due times and converge naturally
	// to the new interval as they are popped and requeued.
	if err := rpq.refreshQueueHeads(); err != nil {
		return fmt.Errorf("reschedule: failed to refresh queue heads: %w", err)
	}

	log.Debug("proxy queue interval updated; existing entries converge lazily", "interval", interval)
	return nil
}
func parseProxyPopResult(result interface{}) (proxyPopResult, error) {
	resSlice, ok := result.([]interface{})
	if !ok {
		return proxyPopResult{}, fmt.Errorf("unexpected lua result type %T", result)
	}
	if len(resSlice) != 5 {
		return proxyPopResult{}, fmt.Errorf("unexpected lua result length %d", len(resSlice))
	}

	foundFlag, err := coerceLuaInt64(resSlice[0])
	if err != nil {
		return proxyPopResult{}, fmt.Errorf("invalid found flag: %w", err)
	}

	if foundFlag == 0 {
		nextReadyMs, err := coerceLuaInt64(resSlice[3])
		if err != nil {
			return proxyPopResult{}, fmt.Errorf("invalid next-ready score: %w", err)
		}
		return proxyPopResult{
			Found:       false,
			NextReadyMs: nextReadyMs,
		}, nil
	}

	proxyJSON, err := coerceLuaString(resSlice[2])
	if err != nil {
		return proxyPopResult{}, fmt.Errorf("invalid proxy payload: %w", err)
	}

	score, err := coerceLuaInt64(resSlice[3])
	if err != nil {
		return proxyPopResult{}, fmt.Errorf("invalid score: %w", err)
	}

	return proxyPopResult{
		Found:       true,
		ProxyJSON:   proxyJSON,
		ScoreMs:     score,
		NextReadyMs: -1,
	}, nil
}
