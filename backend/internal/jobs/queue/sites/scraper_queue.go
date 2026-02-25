package sitequeue

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
	scrapesiteKeyPrefix = "scrapesite:"

	legacyScrapesiteQueueKey   = "scrapesite_queue"
	scrapesiteQueueHeadKey     = "scrapesite_queue_heads"
	scrapesiteQueueShardPrefix = "scrapesite_queue:"
	defaultScrapeQueueShards   = 8
	maxScrapeQueueShards       = 64
	minDequeueSleep            = 10 * time.Millisecond
	idleQueueSleep             = 250 * time.Millisecond
	maxDequeueSleep            = 2 * time.Second
	processingLease            = 5 * time.Minute
	scrapeRescheduleBatch      = 500
)

//go:embed pop.lua
var luaScrapePopScript string

type RedisScrapeSiteQueue struct {
	client         *redis.Client
	ctx            context.Context
	popScript      *redis.Script
	queueShardKeys []string
	popQueueKeys   []string
}

type scrapePopResult struct {
	Found       bool
	SiteJSON    string
	ScoreMs     int64
	NextReadyMs int64
}

var PublicScrapeSiteQueue RedisScrapeSiteQueue

func init() {
	client, err := support.GetRedisClient()
	if err != nil {
		log.Fatal("Could not connect to redis for scrape site queue", "error", err)
	}
	PublicScrapeSiteQueue = *NewRedisScrapeSiteQueue(client)

	go func() {
		updates := config.ScrapeIntervalUpdates()
		for interval := range updates {
			if err := PublicScrapeSiteQueue.Reschedule(interval); err != nil {
				log.Error("Failed to reschedule scrape queue after interval update", "error", err)
			}
		}
	}()
}

func NewRedisScrapeSiteQueue(client *redis.Client) *RedisScrapeSiteQueue {
	shards := support.GetEnvInt("SCRAPE_QUEUE_SHARDS", defaultScrapeQueueShards)
	if shards <= 0 {
		shards = defaultScrapeQueueShards
	}
	if shards > maxScrapeQueueShards {
		shards = maxScrapeQueueShards
	}

	shardKeys := buildScrapeQueueShardKeys(shards)
	popKeys := buildScrapePopQueueKeys(shardKeys)

	queue := &RedisScrapeSiteQueue{
		client:         client,
		ctx:            context.Background(),
		popScript:      redis.NewScript(luaScrapePopScript),
		queueShardKeys: shardKeys,
		popQueueKeys:   popKeys,
	}
	if err := queue.refreshQueueHeads(); err != nil {
		log.Warn("scrape queue head refresh failed", "error", err)
	}
	return queue
}

func buildScrapeQueueShardKeys(shards int) []string {
	keys := make([]string, shards)
	for i := 0; i < shards; i++ {
		keys[i] = fmt.Sprintf("%s%d", scrapesiteQueueShardPrefix, i)
	}
	return keys
}

func buildScrapePopQueueKeys(shardKeys []string) []string {
	keys := make([]string, 0, len(shardKeys)+1)
	keys = append(keys, legacyScrapesiteQueueKey)
	keys = append(keys, shardKeys...)
	return keys
}

func (rssq *RedisScrapeSiteQueue) queueKeyForMember(member string) string {
	if len(rssq.queueShardKeys) == 0 {
		return legacyScrapesiteQueueKey
	}
	idx := scrapeQueueShardIndex(member, len(rssq.queueShardKeys))
	return rssq.queueShardKeys[idx]
}

func scrapeQueueShardIndex(member string, shards int) int {
	if shards <= 1 {
		return 0
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(member))
	return int(hasher.Sum32() % uint32(shards))
}

func (rssq *RedisScrapeSiteQueue) refreshQueueHeads() error {
	if rssq == nil {
		return errors.New("redis scrape queue is nil")
	}

	pipe := rssq.client.Pipeline()
	pipe.Del(rssq.ctx, scrapesiteQueueHeadKey)

	for _, key := range rssq.popQueueKeys {
		entries, err := rssq.client.ZRangeWithScores(rssq.ctx, key, 0, 0).Result()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			continue
		}
		pipe.ZAdd(rssq.ctx, scrapesiteQueueHeadKey, redis.Z{
			Score:  entries[0].Score,
			Member: key,
		})
	}

	if _, err := pipe.Exec(rssq.ctx); err != nil {
		return err
	}
	return nil
}

func (rssq *RedisScrapeSiteQueue) AddToQueue(sites []domain.ScrapeSite) error {
	filtered := make([]domain.ScrapeSite, 0, len(sites))
	for _, site := range sites {
		if site.URL == "" {
			continue
		}
		if config.IsWebsiteBlocked(site.URL) {
			log.Info("Skipping blocked scrape site when queuing", "url", site.URL)
			continue
		}
		filtered = append(filtered, site)
	}

	if len(filtered) == 0 {
		return nil
	}

	pipe := rssq.client.Pipeline()
	interval := config.GetTimeBetweenScrapes()
	now := time.Now()
	sitesLenDuration := time.Duration(len(filtered))
	batchSize := 50

	for i, site := range filtered {
		offset := (interval * time.Duration(i)) / sitesLenDuration
		nextCheck := now.Add(offset)
		siteKey := scrapesiteKeyPrefix + site.URL
		queueKey := rssq.queueKeyForMember(site.URL)

		proxyJSON, err := json.Marshal(site)
		if err != nil {
			return fmt.Errorf("failed to marshal site: %w", err)
		}

		pipe.Set(rssq.ctx, siteKey, proxyJSON, 0)
		pipe.ZAdd(rssq.ctx, queueKey, redis.Z{
			Score:  float64(nextCheck.UnixMilli()),
			Member: site.URL,
		})
		if queueKey != legacyScrapesiteQueueKey {
			pipe.ZRem(rssq.ctx, legacyScrapesiteQueueKey, site.URL)
		}
		pipe.ZAddArgs(rssq.ctx, scrapesiteQueueHeadKey, redis.ZAddArgs{
			LT: true,
			Members: []redis.Z{{
				Score:  float64(nextCheck.UnixMilli()),
				Member: queueKey,
			}},
		})

		// Execute in batches to prevent oversized pipelines
		if i%batchSize == 0 && i > 0 {
			if _, err := pipe.Exec(rssq.ctx); err != nil {
				return fmt.Errorf("batch pipeline failed: %w", err)
			}
			pipe = rssq.client.Pipeline()
		}
	}

	if _, err := pipe.Exec(rssq.ctx); err != nil {
		return fmt.Errorf("final pipeline exec failed: %w", err)
	}

	return nil
}

func (rssq *RedisScrapeSiteQueue) RemoveFromQueue(sites []domain.ScrapeSite) error {
	if rssq == nil {
		return errors.New("redis scrape queue is nil")
	}
	if len(sites) == 0 {
		return nil
	}

	const batchSize = 250
	pipe := rssq.client.Pipeline()
	opCount := 0

	flush := func() error {
		if opCount == 0 {
			return nil
		}
		if _, err := pipe.Exec(rssq.ctx); err != nil {
			return fmt.Errorf("remove pipeline exec failed: %w", err)
		}
		pipe = rssq.client.Pipeline()
		opCount = 0
		return nil
	}

	for _, site := range sites {
		if site.URL == "" {
			continue
		}

		key := scrapesiteKeyPrefix + site.URL
		queueKey := rssq.queueKeyForMember(site.URL)

		pipe.Del(rssq.ctx, key)
		opCount++
		pipe.ZRem(rssq.ctx, queueKey, site.URL)
		opCount++
		if queueKey != legacyScrapesiteQueueKey {
			pipe.ZRem(rssq.ctx, legacyScrapesiteQueueKey, site.URL)
		}
		opCount++

		if opCount >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	return flush()
}

func (rssq *RedisScrapeSiteQueue) GetNextScrapeSite() (domain.ScrapeSite, time.Time, error) {
	return rssq.GetNextScrapeSiteContext(rssq.ctx)
}

func (rssq *RedisScrapeSiteQueue) GetNextScrapeSiteContext(ctx context.Context) (domain.ScrapeSite, time.Time, error) {
	if ctx == nil {
		ctx = rssq.ctx
	}

	for {
		select {
		case <-ctx.Done():
			return domain.ScrapeSite{}, time.Time{}, ctx.Err()
		default:
		}

		currentTimeMs := time.Now().UnixMilli()
		result, err := rssq.popScript.Run(
			ctx,
			rssq.client,
			[]string{scrapesiteQueueHeadKey},
			currentTimeMs,
			int64(processingLease/time.Millisecond),
			scrapesiteKeyPrefix,
		).Result()

		if err != nil {
			return domain.ScrapeSite{}, time.Time{}, fmt.Errorf("lua script failed: %w", err)
		}

		popResult, err := parseScrapePopResult(result)
		if err != nil {
			return domain.ScrapeSite{}, time.Time{}, fmt.Errorf("invalid lua response: %w", err)
		}
		if !popResult.Found {
			waitDuration := dequeueWaitDuration(popResult.NextReadyMs, currentTimeMs)
			if err := waitForNextScrapePoll(ctx, waitDuration); err != nil {
				return domain.ScrapeSite{}, time.Time{}, err
			}
			continue
		}

		var site domain.ScrapeSite
		if err := json.Unmarshal([]byte(popResult.SiteJSON), &site); err != nil {
			return domain.ScrapeSite{}, time.Time{}, fmt.Errorf("failed to unmarshal scrapesite: %w", err)
		}

		return site, time.UnixMilli(popResult.ScoreMs), nil
	}
}

func parseScrapePopResult(result interface{}) (scrapePopResult, error) {
	resSlice, ok := result.([]interface{})
	if !ok {
		return scrapePopResult{}, fmt.Errorf("unexpected lua result type %T", result)
	}
	if len(resSlice) != 5 {
		return scrapePopResult{}, fmt.Errorf("unexpected lua result length %d", len(resSlice))
	}

	foundFlag, err := coerceLuaInt64(resSlice[0])
	if err != nil {
		return scrapePopResult{}, fmt.Errorf("invalid found flag: %w", err)
	}

	if foundFlag == 0 {
		nextReadyMs, err := coerceLuaInt64(resSlice[3])
		if err != nil {
			return scrapePopResult{}, fmt.Errorf("invalid next-ready score: %w", err)
		}
		return scrapePopResult{
			Found:       false,
			NextReadyMs: nextReadyMs,
		}, nil
	}

	siteJSON, err := coerceLuaString(resSlice[2])
	if err != nil {
		return scrapePopResult{}, fmt.Errorf("invalid site payload: %w", err)
	}

	score, err := coerceLuaInt64(resSlice[3])
	if err != nil {
		return scrapePopResult{}, fmt.Errorf("invalid score: %w", err)
	}

	return scrapePopResult{
		Found:       true,
		SiteJSON:    siteJSON,
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

func waitForNextScrapePoll(ctx context.Context, duration time.Duration) error {
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

func (rssq *RedisScrapeSiteQueue) RequeueScrapeSite(site domain.ScrapeSite, lastCheckTime time.Time) error {
	interval := config.GetTimeBetweenScrapes()
	base := lastCheckTime
	if now := time.Now(); now.After(base) {
		base = now
	}
	nextCheck := base.Add(interval)
	siteKey := scrapesiteKeyPrefix + site.URL
	queueKey := rssq.queueKeyForMember(site.URL)

	proxyJSON, err := json.Marshal(site)
	if err != nil {
		return fmt.Errorf("failed to marshal proxy: %w", err)
	}

	pipe := rssq.client.Pipeline()
	pipe.Set(rssq.ctx, siteKey, proxyJSON, 0)
	if queueKey != legacyScrapesiteQueueKey {
		pipe.ZRem(rssq.ctx, legacyScrapesiteQueueKey, site.URL)
	}
	pipe.ZAdd(rssq.ctx, queueKey, redis.Z{
		Score:  float64(nextCheck.UnixMilli()),
		Member: site.URL,
	})
	pipe.ZAddArgs(rssq.ctx, scrapesiteQueueHeadKey, redis.ZAddArgs{
		LT: true,
		Members: []redis.Z{{
			Score:  float64(nextCheck.UnixMilli()),
			Member: queueKey,
		}},
	})

	_, err = pipe.Exec(rssq.ctx)
	return err
}

func (rssq *RedisScrapeSiteQueue) GetScrapeSiteCount() (int64, error) {
	var total int64
	for _, key := range rssq.popQueueKeys {
		count, err := rssq.client.ZCard(rssq.ctx, key).Result()
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (rssq *RedisScrapeSiteQueue) GetActiveInstances() (int, error) {
	return runtime.CountActiveInstances(rssq.ctx, rssq.client)
}

func (rssq *RedisScrapeSiteQueue) Close() error {
	return support.CloseRedisClient()
}

func (rssq *RedisScrapeSiteQueue) Reschedule(interval time.Duration) error {
	if rssq == nil {
		return errors.New("redis scrape queue is nil")
	}

	if interval <= 0 {
		interval = time.Second
	}

	type queueCount struct {
		key   string
		count int64
	}

	counts := make([]queueCount, 0, len(rssq.popQueueKeys))
	var total int64
	for _, key := range rssq.popQueueKeys {
		count, err := rssq.client.ZCard(rssq.ctx, key).Result()
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
	const fetchBatch int64 = scrapeRescheduleBatch
	const updateBatch = 250

	pipe := rssq.client.Pipeline()
	opCount := 0

	flush := func() error {
		if opCount == 0 {
			return nil
		}
		if _, err := pipe.Exec(rssq.ctx); err != nil {
			return fmt.Errorf("reschedule: pipeline exec failed: %w", err)
		}
		pipe = rssq.client.Pipeline()
		opCount = 0
		return nil
	}

	for _, queue := range counts {
		for start := int64(0); start < queue.count; start += fetchBatch {
			end := start + fetchBatch - 1
			if end >= queue.count {
				end = queue.count - 1
			}

			members, err := rssq.client.ZRange(rssq.ctx, queue.key, start, end).Result()
			if err != nil {
				return fmt.Errorf("reschedule: failed to fetch members: %w", err)
			}

			for _, member := range members {
				offset := (interval * time.Duration(globalIndex)) / totalDuration
				nextCheck := now.Add(offset).UnixMilli()
				targetQueue := rssq.queueKeyForMember(member)

				if queue.key != targetQueue {
					pipe.ZRem(rssq.ctx, queue.key, member)
					opCount++
				}

				pipe.ZAdd(rssq.ctx, targetQueue, redis.Z{
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
	if err := rssq.refreshQueueHeads(); err != nil {
		return fmt.Errorf("reschedule: failed to refresh queue heads: %w", err)
	}

	log.Debug("scrape queue rescheduled", "entries", total, "interval", interval)
	return nil
}
