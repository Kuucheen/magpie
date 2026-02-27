package runtime

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"magpie/internal/database"
	"magpie/internal/domain"
	"magpie/internal/support"

	"github.com/charmbracelet/log"
)

const (
	statisticsFlushInterval           = 15 * time.Second
	statisticsBatchThreshold          = 5000
	statisticsQueueCapacity           = 20_000
	statisticsDirtyProxyQueueCapacity = 1024
	statisticsSaturationLogEvery      = 15 * time.Second
	statisticsInsertTimeout           = 30 * time.Second
	reputationRecalcTimeout           = 10 * time.Second
	reputationRecalcInterval          = 1 * time.Minute
	reputationRecalcBatch             = 5000
	reputationRecalcPerTick           = 4
	defaultStatisticsIngestWorkers    = 4
	maxStatisticsIngestWorkers        = 32
	envProxyStatisticsIngestWorkers   = "PROXY_STATISTICS_INGEST_WORKERS"
)

var (
	proxyStatisticQueue             = make(chan domain.ProxyStatistic, statisticsQueueCapacity)
	proxyStatisticDroppedCount      atomic.Uint64
	proxyStatisticEvictedCount      atomic.Uint64
	proxyStatisticLastSaturationLog atomic.Int64
)

func AddProxyStatistic(proxyStatistic domain.ProxyStatistic) {
	if enqueueProxyStatistic(proxyStatistic) {
		return
	}

	if evictOldestProxyStatistic() && enqueueProxyStatistic(proxyStatistic) {
		return
	}

	dropped := proxyStatisticDroppedCount.Add(1)
	recordProxyStatisticSaturation(dropped, proxyStatisticEvictedCount.Load())
}

func enqueueProxyStatistic(proxyStatistic domain.ProxyStatistic) bool {
	select {
	case proxyStatisticQueue <- proxyStatistic:
		return true
	default:
		return false
	}
}

func evictOldestProxyStatistic() bool {
	select {
	case <-proxyStatisticQueue:
		proxyStatisticEvictedCount.Add(1)
		return true
	default:
		return false
	}
}

func recordProxyStatisticSaturation(totalDropped, totalEvicted uint64) {
	nowUnix := time.Now().Unix()
	last := proxyStatisticLastSaturationLog.Load()
	if nowUnix-last < int64(statisticsSaturationLogEvery/time.Second) {
		return
	}
	if !proxyStatisticLastSaturationLog.CompareAndSwap(last, nowUnix) {
		return
	}

	log.Warn(
		"Proxy statistics queue saturated; evicting oldest statistics to preserve throughput",
		"dropped_total", totalDropped,
		"evicted_total", totalEvicted,
		"queue_len", len(proxyStatisticQueue),
		"queue_capacity", cap(proxyStatisticQueue),
	)
}

func StartProxyStatisticsRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	workerCount := resolveStatisticsIngestWorkers()
	dirtyProxyIDsQueue := make(chan []uint64, statisticsDirtyProxyQueueCapacity)

	var reputationWG sync.WaitGroup
	reputationWG.Add(1)
	go func() {
		defer reputationWG.Done()
		runProxyReputationCoordinator(ctx, dirtyProxyIDsQueue)
	}()

	var workerWG sync.WaitGroup
	for i := 0; i < workerCount; i++ {
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

func runProxyStatisticsWorker(ctx context.Context, dirtyProxyIDsQueue chan<- []uint64) {
	var buffer []domain.ProxyStatistic
	flushTimer := time.NewTimer(statisticsFlushInterval)
	defer flushTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			drainProxyStatisticQueue(&buffer)
			publishDirtyProxyIDs(dirtyProxyIDsQueue, flushProxyStatistics(&buffer))
			return
		case stat := <-proxyStatisticQueue:
			buffer = append(buffer, stat)
			if len(buffer) >= statisticsBatchThreshold {
				publishDirtyProxyIDs(dirtyProxyIDsQueue, flushProxyStatistics(&buffer))
				resetTimer(flushTimer)
			}
		case <-flushTimer.C:
			publishDirtyProxyIDs(dirtyProxyIDsQueue, flushProxyStatistics(&buffer))
			flushTimer.Reset(statisticsFlushInterval)
		}
	}
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

func flushProxyStatistics(buffer *[]domain.ProxyStatistic) []uint64 {
	if len(*buffer) == 0 {
		return nil
	}

	toInsert := *buffer
	*buffer = nil

	start := time.Now()
	dbCtx, cancel := context.WithTimeout(context.Background(), statisticsInsertTimeout)
	defer cancel()

	preparedStats, proxyIDs, err := prepareProxyStatistics(dbCtx, toInsert)
	if err != nil {
		log.Error("Failed to prepare proxy statistics", "error", err)
		return nil
	}
	if len(preparedStats) == 0 {
		return nil
	}

	batchSize := database.CalculateProxyStatisticBatchSize(len(preparedStats))
	if err := database.InsertProxyStatistics(dbCtx, preparedStats, batchSize); err != nil {
		log.Error("Failed to insert proxy statistics", "error", err, "count", len(preparedStats))
		return nil
	}

	log.Debug("Inserted proxy statistics", "seconds", time.Since(start).Seconds(), "count", len(preparedStats))
	return proxyIDs
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
