package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"magpie/internal/database"
	"magpie/internal/domain"
	"magpie/internal/instanceid"
	"magpie/internal/support"

	"github.com/charmbracelet/log"
	"github.com/redis/go-redis/v9"
)

const (
	statisticsFlushInterval           = 15 * time.Second
	statisticsBatchThreshold          = 5000
	statisticsQueueCapacity           = 20_000
	statisticsDirtyProxyQueueCapacity = 1024
	statisticsBackpressureLogEvery    = 15 * time.Second
	statisticsInsertTimeout           = 30 * time.Second
	reputationRecalcTimeout           = 10 * time.Second
	reputationRecalcInterval          = 1 * time.Minute
	reputationRecalcBatch             = 5000
	reputationRecalcPerTick           = 4
	statisticsProducerRetryDelay      = 250 * time.Millisecond
	defaultStatisticsIngestWorkers    = 4
	maxStatisticsIngestWorkers        = 32
	envProxyStatisticsIngestWorkers   = "PROXY_STATISTICS_INGEST_WORKERS"

	envProxyStatisticsRedisStreamEnabled        = "PROXY_STATISTICS_REDIS_STREAM_ENABLED"
	envProxyStatisticsRedisStreamKey            = "PROXY_STATISTICS_REDIS_STREAM_KEY"
	envProxyStatisticsRedisStreamGroup          = "PROXY_STATISTICS_REDIS_STREAM_GROUP"
	envProxyStatisticsRedisStreamMaxLen         = "PROXY_STATISTICS_REDIS_STREAM_MAXLEN"
	envProxyStatisticsRedisStreamOverloadPolicy = "PROXY_STATISTICS_REDIS_STREAM_OVERLOAD_POLICY"
	envProxyStatisticsTenantOverloadPolicies    = "PROXY_STATISTICS_TENANT_OVERLOAD_POLICIES"

	defaultProxyStatisticsRedisStreamKey       = "magpie:proxy_statistics:stream"
	defaultProxyStatisticsRedisStreamGroup     = "magpie:proxy_statistics:consumers"
	defaultProxyStatisticsRedisStreamMaxLen    = 1_000_000
	defaultProxyStatisticsStreamOverloadPolicy = statisticsOverloadPolicyBlock

	statisticsStreamAddTimeout   = 150 * time.Millisecond
	statisticsStreamReadBlock    = 2 * time.Second
	statisticsStreamErrorDelay   = 1 * time.Second
	statisticsStreamAckBatchCap  = 1000
	statisticsStreamClaimEvery   = 15 * time.Second
	statisticsStreamClaimMinIdle = 1 * time.Minute
	statisticsStreamClaimBatch   = 500
	statisticsStreamClaimStart   = "0-0"

	statisticsOverloadPolicyBlock       = "block"
	statisticsOverloadPolicyEvictOldest = "evict_oldest"
	statisticsOverloadPolicyDropNew     = "drop_new"
)

var (
	proxyStatisticQueue               = make(chan domain.ProxyStatistic, statisticsQueueCapacity)
	proxyStatisticDroppedCount        atomic.Uint64
	proxyStatisticLastBackpressureLog atomic.Int64

	proxyStatisticStreamInitMu sync.Mutex
	proxyStatisticStreamClient *redis.Client
	proxyStatisticStreamCfg    proxyStatisticStreamConfig
	proxyStatisticStreamReady  atomic.Bool
)

type proxyStatisticStreamConfig struct {
	enabled                bool
	streamKey              string
	groupName              string
	maxLen                 int64
	overloadPolicy         string
	tenantOverloadPolicies map[uint]string
}

func AddProxyStatistic(proxyStatistic domain.ProxyStatistic) {
	AddProxyStatisticForUsers(proxyStatistic, nil)
}

func AddProxyStatisticForUsers(proxyStatistic domain.ProxyStatistic, userIDs []uint) {
	policy := resolveStatisticsOverloadPolicy(userIDs)

	if proxyStatisticStreamReady.Load() {
		for {
			enqueued, handled := enqueueProxyStatisticToStream(proxyStatistic, policy)
			if enqueued || handled {
				return
			}
			if policy != statisticsOverloadPolicyBlock {
				break
			}
			recordProxyStatisticBackpressure(policy)
			time.Sleep(statisticsProducerRetryDelay)
		}
	}

	if enqueueProxyStatistic(proxyStatistic, policy) {
		return
	}

	dropped := proxyStatisticDroppedCount.Add(1)
	recordProxyStatisticDrop(dropped, policy)
}

func enqueueProxyStatisticToStream(proxyStatistic domain.ProxyStatistic, policy string) (bool, bool) {
	client, cfg := proxyStatisticStreamClient, proxyStatisticStreamCfg
	if client == nil || !cfg.enabled {
		return false, false
	}

	if (policy == statisticsOverloadPolicyDropNew || policy == statisticsOverloadPolicyBlock) && cfg.maxLen > 0 {
		addCtx, cancel := context.WithTimeout(context.Background(), statisticsStreamAddTimeout)
		streamLen, err := client.XLen(addCtx, cfg.streamKey).Result()
		cancel()
		if err == nil && streamLen >= cfg.maxLen {
			if policy == statisticsOverloadPolicyDropNew {
				dropped := proxyStatisticDroppedCount.Add(1)
				recordProxyStatisticDrop(dropped, policy)
				return false, true
			}
			recordProxyStatisticBackpressure(policy)
			return false, false
		}
	}

	encoded, err := json.Marshal(proxyStatistic)
	if err != nil {
		log.Error("Failed to encode proxy statistic for stream ingest", "error", err)
		return false, false
	}

	addArgs := &redis.XAddArgs{
		Stream: cfg.streamKey,
		Values: map[string]any{"stat": string(encoded)},
	}
	if policy == statisticsOverloadPolicyEvictOldest && cfg.maxLen > 0 {
		addArgs.MaxLen = cfg.maxLen
		addArgs.Approx = true
	}

	addCtx, cancel := context.WithTimeout(context.Background(), statisticsStreamAddTimeout)
	defer cancel()
	if _, err := client.XAdd(addCtx, addArgs).Result(); err != nil {
		log.Warn("Failed to enqueue proxy statistic to Redis stream", "error", err, "policy", policy)
		return false, false
	}

	return true, true
}

func enqueueProxyStatistic(proxyStatistic domain.ProxyStatistic, policy string) bool {
	if policy == statisticsOverloadPolicyDropNew {
		select {
		case proxyStatisticQueue <- proxyStatistic:
			return true
		default:
			return false
		}
	}

	select {
	case proxyStatisticQueue <- proxyStatistic:
		return true
	default:
		recordProxyStatisticBackpressure(policy)
		proxyStatisticQueue <- proxyStatistic
		return true
	}
}

func recordProxyStatisticBackpressure(policy string) {
	nowUnix := time.Now().Unix()
	last := proxyStatisticLastBackpressureLog.Load()
	if nowUnix-last < int64(statisticsBackpressureLogEvery/time.Second) {
		return
	}
	if !proxyStatisticLastBackpressureLog.CompareAndSwap(last, nowUnix) {
		return
	}

	log.Warn(
		"Proxy statistics ingestion applying backpressure",
		"policy", policy,
		"queue_len", len(proxyStatisticQueue),
		"queue_capacity", cap(proxyStatisticQueue),
	)
}

func recordProxyStatisticDrop(totalDropped uint64, policy string) {
	log.Warn("Dropped proxy statistic due overload policy", "policy", policy, "dropped_total", totalDropped)
}

func StartProxyStatisticsRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	workerCount := resolveStatisticsIngestWorkers()
	initializeProxyStatisticStream(ctx)
	dirtyProxyIDsQueue := make(chan []uint64, statisticsDirtyProxyQueueCapacity)

	var reputationWG sync.WaitGroup
	reputationWG.Add(1)
	go func() {
		defer reputationWG.Done()
		runProxyReputationCoordinator(ctx, dirtyProxyIDsQueue)
	}()

	var workerWG sync.WaitGroup
	memoryWorkerCount := workerCount
	streamWorkerCount := 0
	if proxyStatisticStreamReady.Load() {
		streamWorkerCount = workerCount
		memoryWorkerCount = 1 // in-memory queue is fallback when stream publish fails
	}

	for i := 0; i < streamWorkerCount; i++ {
		consumerName := fmt.Sprintf("%s-%d", instanceid.Get(), i)
		workerWG.Add(1)
		go func(name string) {
			defer workerWG.Done()
			runProxyStatisticsStreamWorker(ctx, dirtyProxyIDsQueue, name)
		}(consumerName)
	}

	for i := 0; i < memoryWorkerCount; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			runProxyStatisticsWorker(ctx, dirtyProxyIDsQueue)
		}()
	}

	<-ctx.Done()
	workerWG.Wait()
	close(dirtyProxyIDsQueue)
	reputationWG.Wait()
}

func initializeProxyStatisticStream(ctx context.Context) {
	cfg := resolveProxyStatisticStreamConfig()
	proxyStatisticStreamCfg = cfg
	if !cfg.enabled {
		return
	}
	if proxyStatisticStreamReady.Load() {
		return
	}

	proxyStatisticStreamInitMu.Lock()
	defer proxyStatisticStreamInitMu.Unlock()
	if proxyStatisticStreamReady.Load() {
		return
	}

	client, err := support.GetRedisClient()
	if err != nil {
		log.Warn("Proxy statistics Redis stream unavailable; using in-memory backpressure path", "error", err)
		return
	}

	opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = client.XGroupCreateMkStream(opCtx, cfg.streamKey, cfg.groupName, "0").Err()
	if err != nil && !isBusyGroupError(err) {
		log.Warn("Proxy statistics Redis stream unavailable; failed to create consumer group", "error", err)
		return
	}

	proxyStatisticStreamClient = client
	proxyStatisticStreamReady.Store(true)
	log.Info(
		"Proxy statistics ingest using Redis stream",
		"stream", cfg.streamKey,
		"group", cfg.groupName,
		"default_overload_policy", cfg.overloadPolicy,
		"tenant_overload_policies", len(cfg.tenantOverloadPolicies),
		"max_len", cfg.maxLen,
	)
}

func resolveProxyStatisticStreamConfig() proxyStatisticStreamConfig {
	enabled := support.GetEnvBool(envProxyStatisticsRedisStreamEnabled, true)

	streamKey := strings.TrimSpace(support.GetEnv(envProxyStatisticsRedisStreamKey, defaultProxyStatisticsRedisStreamKey))
	if streamKey == "" {
		streamKey = defaultProxyStatisticsRedisStreamKey
	}

	groupName := strings.TrimSpace(support.GetEnv(envProxyStatisticsRedisStreamGroup, defaultProxyStatisticsRedisStreamGroup))
	if groupName == "" {
		groupName = defaultProxyStatisticsRedisStreamGroup
	}

	maxLen := int64(support.GetEnvInt(envProxyStatisticsRedisStreamMaxLen, defaultProxyStatisticsRedisStreamMaxLen))
	if maxLen < 1 {
		maxLen = defaultProxyStatisticsRedisStreamMaxLen
	}

	overloadPolicy := normalizeOverloadPolicy(support.GetEnv(envProxyStatisticsRedisStreamOverloadPolicy, defaultProxyStatisticsStreamOverloadPolicy))
	tenantPolicies := parseTenantOverloadPolicies(support.GetEnv(envProxyStatisticsTenantOverloadPolicies, ""))

	return proxyStatisticStreamConfig{
		enabled:                enabled,
		streamKey:              streamKey,
		groupName:              groupName,
		maxLen:                 maxLen,
		overloadPolicy:         overloadPolicy,
		tenantOverloadPolicies: tenantPolicies,
	}
}

func parseTenantOverloadPolicies(raw string) map[uint]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	entries := strings.Split(raw, ",")
	policies := make(map[uint]string, len(entries))
	for _, entry := range entries {
		parts := strings.SplitN(strings.TrimSpace(entry), "=", 2)
		if len(parts) != 2 {
			continue
		}
		tenantID, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil || tenantID == 0 {
			continue
		}
		policy := normalizeOverloadPolicy(parts[1])
		policies[uint(tenantID)] = policy
	}

	if len(policies) == 0 {
		return nil
	}
	return policies
}

func normalizeOverloadPolicy(raw string) string {
	policy := strings.ToLower(strings.TrimSpace(raw))
	switch policy {
	case statisticsOverloadPolicyDropNew, statisticsOverloadPolicyEvictOldest, statisticsOverloadPolicyBlock:
		return policy
	default:
		return defaultProxyStatisticsStreamOverloadPolicy
	}
}

func resolveStatisticsOverloadPolicy(userIDs []uint) string {
	cfg := proxyStatisticStreamCfg
	selected := normalizeOverloadPolicy(cfg.overloadPolicy)
	if len(userIDs) == 0 || len(cfg.tenantOverloadPolicies) == 0 {
		return selected
	}

	for _, userID := range userIDs {
		override, ok := cfg.tenantOverloadPolicies[userID]
		if !ok {
			continue
		}
		override = normalizeOverloadPolicy(override)
		if overloadPolicyPriority(override) > overloadPolicyPriority(selected) {
			selected = override
		}
	}

	return selected
}

func overloadPolicyPriority(policy string) int {
	switch policy {
	case statisticsOverloadPolicyBlock:
		return 3
	case statisticsOverloadPolicyDropNew:
		return 1
	case statisticsOverloadPolicyEvictOldest:
		return 0
	default:
		return 2
	}
}

func isBusyGroupError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToUpper(err.Error()), "BUSYGROUP")
}

func runProxyStatisticsWorker(ctx context.Context, dirtyProxyIDsQueue chan<- []uint64) {
	var buffer []domain.ProxyStatistic
	flushTimer := time.NewTimer(statisticsFlushInterval)
	defer flushTimer.Stop()

	for {
		if len(buffer) >= statisticsBatchThreshold {
			if !flushStatisticsBuffer(&buffer, dirtyProxyIDsQueue) {
				time.Sleep(statisticsStreamErrorDelay)
				continue
			}
			resetTimer(flushTimer)
		}

		select {
		case <-ctx.Done():
			drainProxyStatisticQueue(&buffer)
			_ = flushStatisticsBuffer(&buffer, dirtyProxyIDsQueue)
			return
		case stat := <-proxyStatisticQueue:
			buffer = append(buffer, stat)
		case <-flushTimer.C:
			_ = flushStatisticsBuffer(&buffer, dirtyProxyIDsQueue)
			flushTimer.Reset(statisticsFlushInterval)
		}
	}
}

func runProxyStatisticsStreamWorker(ctx context.Context, dirtyProxyIDsQueue chan<- []uint64, consumerName string) {
	client := proxyStatisticStreamClient
	cfg := proxyStatisticStreamCfg
	if client == nil || !cfg.enabled {
		return
	}

	var (
		buffer     []domain.ProxyStatistic
		messageIDs []string
		streamID   = "0" // consume pending entries for this consumer first, then switch to new entries.
		claimStart = statisticsStreamClaimStart
		lastClaim  time.Time
	)
	flushTimer := time.NewTimer(statisticsFlushInterval)
	defer flushTimer.Stop()

	for {
		if len(messageIDs) > 0 && len(buffer) == 0 {
			if !flushStreamStatisticsBuffer(ctx, &buffer, &messageIDs, dirtyProxyIDsQueue, client, cfg) {
				time.Sleep(statisticsStreamErrorDelay)
				continue
			}
		}

		if len(buffer) >= statisticsBatchThreshold {
			if !flushStreamStatisticsBuffer(ctx, &buffer, &messageIDs, dirtyProxyIDsQueue, client, cfg) {
				time.Sleep(statisticsStreamErrorDelay)
				continue
			}
			resetTimer(flushTimer)
		}

		if lastClaim.IsZero() || time.Since(lastClaim) >= statisticsStreamClaimEvery {
			claimed, nextStart, err := claimStaleProxyStatisticStreamMessages(ctx, client, cfg, consumerName, claimStart)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Error("Failed to claim stale proxy statistics stream messages", "error", err, "consumer", consumerName)
				time.Sleep(statisticsStreamErrorDelay)
				continue
			}
			lastClaim = time.Now()
			claimStart = nextStart
			if appendStreamMessagesToBuffer(ctx, claimed, &buffer, &messageIDs, client, cfg) > 0 {
				continue
			}
		}

		select {
		case <-ctx.Done():
			_ = flushStreamStatisticsBuffer(context.Background(), &buffer, &messageIDs, dirtyProxyIDsQueue, client, cfg)
			return
		case <-flushTimer.C:
			if flushStreamStatisticsBuffer(ctx, &buffer, &messageIDs, dirtyProxyIDsQueue, client, cfg) {
				flushTimer.Reset(statisticsFlushInterval)
			} else {
				time.Sleep(statisticsStreamErrorDelay)
				flushTimer.Reset(statisticsFlushInterval)
			}
		default:
		}

		streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    cfg.groupName,
			Consumer: consumerName,
			Streams:  []string{cfg.streamKey, streamID},
			Count:    int64(statisticsBatchThreshold),
			Block:    statisticsStreamReadBlock,
			NoAck:    false,
		}).Result()
		if err == redis.Nil {
			if streamID == "0" {
				streamID = ">"
			}
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error("Failed reading proxy statistics stream", "error", err, "consumer", consumerName)
			time.Sleep(statisticsStreamErrorDelay)
			continue
		}

		appendedMessages := 0
		for _, stream := range streams {
			appendedMessages += appendStreamMessagesToBuffer(ctx, stream.Messages, &buffer, &messageIDs, client, cfg)
		}
		if streamID == "0" && appendedMessages == 0 {
			streamID = ">"
		}
	}
}

func claimStaleProxyStatisticStreamMessages(ctx context.Context, client *redis.Client, cfg proxyStatisticStreamConfig, consumerName, start string) ([]redis.XMessage, string, error) {
	if strings.TrimSpace(start) == "" {
		start = statisticsStreamClaimStart
	}

	claimCtx, cancel := context.WithTimeout(ctx, statisticsStreamReadBlock)
	defer cancel()

	messages, nextStart, err := client.XAutoClaim(claimCtx, &redis.XAutoClaimArgs{
		Stream:   cfg.streamKey,
		Group:    cfg.groupName,
		Consumer: consumerName,
		MinIdle:  statisticsStreamClaimMinIdle,
		Start:    start,
		Count:    statisticsStreamClaimBatch,
	}).Result()
	if err != nil {
		return nil, start, err
	}
	if strings.TrimSpace(nextStart) == "" {
		nextStart = statisticsStreamClaimStart
	}
	return messages, nextStart, nil
}

func appendStreamMessagesToBuffer(ctx context.Context, messages []redis.XMessage, buffer *[]domain.ProxyStatistic, messageIDs *[]string, client *redis.Client, cfg proxyStatisticStreamConfig) int {
	if len(messages) == 0 {
		return 0
	}

	appended := 0
	for _, msg := range messages {
		stat, err := decodeProxyStatisticStreamMessage(msg)
		if err != nil {
			log.Warn("Dropping malformed proxy statistic stream message", "id", msg.ID, "error", err)
			_, _ = client.XAck(ctx, cfg.streamKey, cfg.groupName, msg.ID).Result()
			_, _ = client.XDel(ctx, cfg.streamKey, msg.ID).Result()
			continue
		}

		*buffer = append(*buffer, stat)
		*messageIDs = append(*messageIDs, msg.ID)
		appended++
	}

	return appended
}

func decodeProxyStatisticStreamMessage(msg redis.XMessage) (domain.ProxyStatistic, error) {
	raw, ok := msg.Values["stat"]
	if !ok {
		return domain.ProxyStatistic{}, fmt.Errorf("missing stat payload")
	}

	payload, ok := raw.(string)
	if !ok {
		payload = fmt.Sprint(raw)
	}
	if strings.TrimSpace(payload) == "" {
		return domain.ProxyStatistic{}, fmt.Errorf("empty stat payload")
	}

	var stat domain.ProxyStatistic
	if err := json.Unmarshal([]byte(payload), &stat); err != nil {
		return domain.ProxyStatistic{}, err
	}
	return stat, nil
}

func flushStreamStatisticsBuffer(ctx context.Context, buffer *[]domain.ProxyStatistic, messageIDs *[]string, dirtyProxyIDsQueue chan<- []uint64, client *redis.Client, cfg proxyStatisticStreamConfig) bool {
	if len(*buffer) > 0 && !flushStatisticsBuffer(buffer, dirtyProxyIDsQueue) {
		return false
	}

	ackedCount, err := ackAndDeleteStreamMessageIDs(ctx, *messageIDs, client, cfg)
	if ackedCount > 0 {
		*messageIDs = (*messageIDs)[ackedCount:]
	}
	if err != nil {
		log.Error("Failed to ack proxy statistic stream messages", "error", err, "remaining", len(*messageIDs))
		return false
	}
	return true
}

func ackAndDeleteStreamMessageIDs(ctx context.Context, messageIDs []string, client *redis.Client, cfg proxyStatisticStreamConfig) (int, error) {
	if len(messageIDs) == 0 {
		return 0, nil
	}

	ackedCount := 0
	for start := 0; start < len(messageIDs); start += statisticsStreamAckBatchCap {
		end := start + statisticsStreamAckBatchCap
		if end > len(messageIDs) {
			end = len(messageIDs)
		}
		ids := messageIDs[start:end]
		if len(ids) == 0 {
			continue
		}
		if _, err := client.XAck(ctx, cfg.streamKey, cfg.groupName, ids...).Result(); err != nil {
			return ackedCount, err
		}
		if _, err := client.XDel(ctx, cfg.streamKey, ids...).Result(); err != nil {
			log.Warn("Failed to trim acked proxy statistic stream messages", "error", err, "count", len(ids))
		}
		ackedCount += len(ids)
	}

	return ackedCount, nil
}

func flushStatisticsBuffer(buffer *[]domain.ProxyStatistic, dirtyProxyIDsQueue chan<- []uint64) bool {
	proxyIDs, flushed := flushProxyStatistics(buffer)
	if !flushed {
		return false
	}
	publishDirtyProxyIDs(dirtyProxyIDsQueue, proxyIDs)
	return true
}

func runProxyReputationCoordinator(ctx context.Context, dirtyProxyIDsQueue <-chan []uint64) {
	dirtyProxyIDs := make(map[uint64]struct{})
	reputationTimer := time.NewTimer(reputationRecalcInterval)
	defer reputationTimer.Stop()

	ctxDone := ctx.Done()
	for {
		select {
		case proxyIDs, ok := <-dirtyProxyIDsQueue:
			if !ok {
				flushDirtyProxyReputations(dirtyProxyIDs, true)
				return
			}
			mergeProxyIDs(dirtyProxyIDs, proxyIDs)
		case <-reputationTimer.C:
			flushDirtyProxyReputations(dirtyProxyIDs, false)
			reputationTimer.Reset(reputationRecalcInterval)
		case <-ctxDone:
			// Keep draining worker updates until the channel is closed.
			ctxDone = nil
		}
	}
}

func resolveStatisticsIngestWorkers() int {
	workers := support.GetEnvInt(envProxyStatisticsIngestWorkers, defaultStatisticsIngestWorkers)
	if workers <= 0 {
		return 1
	}
	if workers > maxStatisticsIngestWorkers {
		return maxStatisticsIngestWorkers
	}
	return workers
}

func publishDirtyProxyIDs(ch chan<- []uint64, proxyIDs []uint64) {
	if len(proxyIDs) == 0 {
		return
	}
	ch <- proxyIDs
}

func flushProxyStatistics(buffer *[]domain.ProxyStatistic) ([]uint64, bool) {
	if len(*buffer) == 0 {
		return nil, true
	}

	toInsert := *buffer

	start := time.Now()
	dbCtx, cancel := context.WithTimeout(context.Background(), statisticsInsertTimeout)
	defer cancel()

	preparedStats, proxyIDs, err := prepareProxyStatistics(dbCtx, toInsert)
	if err != nil {
		log.Error("Failed to prepare proxy statistics", "error", err)
		return nil, false
	}
	if len(preparedStats) == 0 {
		*buffer = nil
		return nil, true
	}

	batchSize := database.CalculateProxyStatisticBatchSize(len(preparedStats))
	if err := database.InsertProxyStatistics(dbCtx, preparedStats, batchSize); err != nil {
		log.Error("Failed to insert proxy statistics", "error", err, "count", len(preparedStats))
		return nil, false
	}

	*buffer = nil
	log.Debug("Inserted proxy statistics", "seconds", time.Since(start).Seconds(), "count", len(preparedStats))
	return proxyIDs, true
}

func drainProxyStatisticQueue(buffer *[]domain.ProxyStatistic) {
	for {
		select {
		case stat := <-proxyStatisticQueue:
			*buffer = append(*buffer, stat)
		default:
			return
		}
	}
}

func resetTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(statisticsFlushInterval)
}

func mergeProxyIDs(target map[uint64]struct{}, proxyIDs []uint64) {
	if len(proxyIDs) == 0 {
		return
	}
	for _, id := range proxyIDs {
		if id == 0 {
			continue
		}
		target[id] = struct{}{}
	}
}

func flushDirtyProxyReputations(dirty map[uint64]struct{}, drainAll bool) {
	if len(dirty) == 0 {
		return
	}

	remaining := reputationRecalcPerTick
	if drainAll {
		remaining = len(dirty)
	}

	for len(dirty) > 0 && remaining > 0 {
		proxyIDs := popProxyIDBatch(dirty, reputationRecalcBatch)
		if len(proxyIDs) == 0 {
			return
		}

		repCtx, cancel := context.WithTimeout(context.Background(), reputationRecalcTimeout)
		err := database.RecalculateProxyReputations(repCtx, proxyIDs)
		cancel()

		if err != nil {
			for _, id := range proxyIDs {
				dirty[id] = struct{}{}
			}
			log.Error("Failed to update proxy reputations", "error", err, "proxy_ids", proxyIDs)
			return
		}

		remaining--
	}
}

func popProxyIDBatch(dirty map[uint64]struct{}, limit int) []uint64 {
	if len(dirty) == 0 || limit <= 0 {
		return nil
	}

	if limit > len(dirty) {
		limit = len(dirty)
	}

	out := make([]uint64, 0, limit)
	for id := range dirty {
		out = append(out, id)
		delete(dirty, id)
		if len(out) >= limit {
			break
		}
	}

	return out
}

func collectProxyIDs(stats []domain.ProxyStatistic) []uint64 {
	if len(stats) == 0 {
		return nil
	}

	seen := make(map[uint64]struct{}, len(stats))
	for _, stat := range stats {
		if stat.ProxyID == 0 {
			continue
		}
		seen[stat.ProxyID] = struct{}{}
	}

	if len(seen) == 0 {
		return nil
	}

	ids := make([]uint64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}

	return ids
}

func prepareProxyStatistics(ctx context.Context, stats []domain.ProxyStatistic) ([]domain.ProxyStatistic, []uint64, error) {
	if len(stats) == 0 {
		return nil, nil, nil
	}

	proxyIDs := collectProxyIDs(stats)
	if len(proxyIDs) == 0 {
		return stats, nil, nil
	}

	existing, err := database.GetExistingProxyIDSet(ctx, proxyIDs)
	if err != nil {
		return nil, nil, err
	}

	if len(existing) == 0 {
		return nil, nil, nil
	}

	if len(existing) == len(proxyIDs) {
		return stats, proxyIDs, nil
	}

	filtered := make([]domain.ProxyStatistic, 0, len(stats))
	for _, stat := range stats {
		if _, ok := existing[stat.ProxyID]; ok {
			filtered = append(filtered, stat)
		}
	}

	if len(filtered) == 0 {
		return nil, nil, nil
	}

	validIDs := make([]uint64, 0, len(existing))
	for id := range existing {
		validIDs = append(validIDs, id)
	}

	dropped := len(stats) - len(filtered)
	if dropped > 0 {
		log.Info("Skipped proxy statistics for removed proxies", "dropped", dropped)
	}

	return filtered, validIDs, nil
}
