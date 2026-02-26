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

	legacyQueueKey       = "proxy_queue"
	proxyQueueHeadKey    = "proxy_queue_heads"
	queueShardKeyPrefix  = "proxy_queue:"
	defaultQueueShards   = 16
	maxQueueShards       = 128
	minDequeueSleep      = 10 * time.Millisecond
	idleQueueSleep       = 250 * time.Millisecond
	maxDequeueSleep      = 2 * time.Second
	processingLease      = 5 * time.Minute
	queueRescheduleBatch = 1000
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

func init() {
	client, err := support.GetRedisClient()
	if err != nil {
		log.Fatal("Could not connect to redis for proxy queue", "error", err)
	}
	PublicProxyQueue = *NewRedisProxyQueue(client)

	go func() {
		updates := config.CheckIntervalUpdates()
		for interval := range updates {
			if err := PublicProxyQueue.Reschedule(interval); err != nil {
				log.Error("Failed to reschedule proxy queue after interval update", "error", err)
			}
		}
	}()
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
	if err := queue.refreshQueueHeads(); err != nil {
		log.Warn("proxy queue head refresh failed", "error", err)
	}
	return queue
}

func (rpq *RedisProxyQueue) refreshQueueHeads() error {
	if rpq == nil {
		return errors.New("redis proxy queue is nil")
	}

	pipe := rpq.client.Pipeline()
	pipe.Del(rpq.ctx, proxyQueueHeadKey)

	for _, key := range rpq.popQueueKeys {
		entries, err := rpq.client.ZRangeWithScores(rpq.ctx, key, 0, 0).Result()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			continue
		}
		pipe.ZAdd(rpq.ctx, proxyQueueHeadKey, redis.Z{
			Score:  entries[0].Score,
			Member: key,
		})
	}

	if _, err := pipe.Exec(rpq.ctx); err != nil {
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
	if len(rpq.queueShardKeys) == 0 {
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

	pipe := rpq.client.Pipeline()
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

		pipe.Set(rpq.ctx, proxyKey, proxyJSON, 0)
		pipe.ZAddArgs(rpq.ctx, queueKey, redis.ZAddArgs{
			NX: true,
			Members: []redis.Z{{
				Score:  float64(nextCheck.UnixMilli()),
				Member: hashKey,
			}},
		})
		if queueKey != legacyQueueKey {
			pipe.ZRem(rpq.ctx, legacyQueueKey, hashKey)
		}
		pipe.ZAddArgs(rpq.ctx, proxyQueueHeadKey, redis.ZAddArgs{
			LT: true,
			Members: []redis.Z{{
				Score:  float64(nextCheck.UnixMilli()),
				Member: queueKey,
			}},
		})

		// Execute in batches to prevent oversized pipelines
		if i%batchSize == 0 && i > 0 {
			if _, err := pipe.Exec(rpq.ctx); err != nil {
				return fmt.Errorf("batch pipeline failed: %w", err)
			}
			pipe = rpq.client.Pipeline()
		}
	}

	if _, err := pipe.Exec(rpq.ctx); err != nil {
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

	const batchSize = 500
	pipe := rpq.client.Pipeline()
	opCount := 0

	flush := func() error {
		if opCount == 0 {
			return nil
		}
		if _, err := pipe.Exec(rpq.ctx); err != nil {
			return fmt.Errorf("remove pipeline exec failed: %w", err)
		}
		pipe = rpq.client.Pipeline()
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

		pipe.Del(rpq.ctx, proxyKey)
		opCount++
		pipe.ZRem(rpq.ctx, queueKey, hashKey)
		opCount++
		if queueKey != legacyQueueKey {
			pipe.ZRem(rpq.ctx, legacyQueueKey, hashKey)
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
	return rpq.GetNextProxyContext(rpq.ctx)
}

func (rpq *RedisProxyQueue) GetNextProxyContext(ctx context.Context) (domain.Proxy, time.Time, error) {
	if ctx == nil {
		ctx = rpq.ctx
	}

	for {
		select {
		case <-ctx.Done():
			return domain.Proxy{}, time.Time{}, ctx.Err()
		default:
		}

		currentTimeMs := time.Now().UnixMilli()
		result, err := rpq.popScript.Run(
			ctx,
			rpq.client,
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
	interval := config.GetTimeBetweenChecks()
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

	pipe := rpq.client.Pipeline()
	pipe.Set(rpq.ctx, proxyKey, proxyJSON, 0)
	if queueKey != legacyQueueKey {
		pipe.ZRem(rpq.ctx, legacyQueueKey, hashKey)
	}
	pipe.ZAdd(rpq.ctx, queueKey, redis.Z{
		Score:  float64(nextCheck.UnixMilli()),
		Member: hashKey,
	})
	pipe.ZAddArgs(rpq.ctx, proxyQueueHeadKey, redis.ZAddArgs{
		LT: true,
		Members: []redis.Z{{
			Score:  float64(nextCheck.UnixMilli()),
			Member: queueKey,
		}},
	})

	_, err = pipe.Exec(rpq.ctx)
	return err
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
	var total int64
	for _, key := range rpq.popQueueKeys {
		count, err := rpq.client.ZCard(rpq.ctx, key).Result()
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (rpq *RedisProxyQueue) GetActiveInstances() (int, error) {
	return runtime.CountActiveInstances(rpq.ctx, rpq.client)
}

func (rpq *RedisProxyQueue) Close() error {
	return support.CloseRedisClient()
}

func (rpq *RedisProxyQueue) Reschedule(interval time.Duration) error {
	if rpq == nil {
		return errors.New("redis proxy queue is nil")
	}

	if interval <= 0 {
		interval = time.Second
	}

	type queueCount struct {
		key   string
		count int64
	}

	counts := make([]queueCount, 0, len(rpq.popQueueKeys))
	var total int64
	for _, key := range rpq.popQueueKeys {
		count, err := rpq.client.ZCard(rpq.ctx, key).Result()
		if err != nil {
			return fmt.Errorf("reschedule: failed to count queue entries: %w", err)
		}
		if count == 0 {
			continue
		}
		counts = append(counts, queueCount{key: key, count: count})
		total += count
	}

	if total == 0 {
		return nil
	}

	now := time.Now()
	totalDuration := time.Duration(total)
	var globalIndex int64
	const fetchBatch int64 = queueRescheduleBatch
	const updateBatch = 500

	pipe := rpq.client.Pipeline()
	opCount := 0

	flush := func() error {
		if opCount == 0 {
			return nil
		}
		if _, err := pipe.Exec(rpq.ctx); err != nil {
			return fmt.Errorf("reschedule: pipeline exec failed: %w", err)
		}
		pipe = rpq.client.Pipeline()
		opCount = 0
		return nil
	}

	for _, queue := range counts {
		for start := int64(0); start < queue.count; start += fetchBatch {
			end := start + fetchBatch - 1
			if end >= queue.count {
				end = queue.count - 1
			}

			members, err := rpq.client.ZRange(rpq.ctx, queue.key, start, end).Result()
			if err != nil {
				return fmt.Errorf("reschedule: failed to fetch members: %w", err)
			}

			for _, member := range members {
				offset := (interval * time.Duration(globalIndex)) / totalDuration
				nextCheck := now.Add(offset).UnixMilli()
				targetQueue := rpq.queueKeyForMember(member)

				if queue.key != targetQueue {
					pipe.ZRem(rpq.ctx, queue.key, member)
					opCount++
				}

				pipe.ZAdd(rpq.ctx, targetQueue, redis.Z{
					Score:  float64(nextCheck),
					Member: member,
				})
				opCount++
				globalIndex++

				if opCount != 0 && opCount%updateBatch == 0 {
					if err := flush(); err != nil {
						return err
					}
				}
			}
		}
	}

	if err := flush(); err != nil {
		return err
	}
	if err := rpq.refreshQueueHeads(); err != nil {
		return fmt.Errorf("reschedule: failed to refresh queue heads: %w", err)
	}

	log.Debug("proxy queue rescheduled", "entries", total, "interval", interval)
	return nil
}
