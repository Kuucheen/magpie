package runtime

import (
	"context"
	"time"

	"magpie/internal/database"
	"magpie/internal/domain"

	"github.com/charmbracelet/log"
)

const (
	statisticsFlushInterval  = 15 * time.Second
	statisticsBatchThreshold = 5000
	statisticsQueueCapacity  = 20_000
	statisticsInsertTimeout  = 30 * time.Second
	reputationRecalcTimeout  = 10 * time.Second
	reputationRecalcInterval = 1 * time.Minute
	reputationRecalcBatch    = 5000
	reputationRecalcPerTick  = 4
)

var (
	proxyStatisticQueue = make(chan domain.ProxyStatistic, statisticsQueueCapacity)
)

func AddProxyStatistic(proxyStatistic domain.ProxyStatistic) {
	proxyStatisticQueue <- proxyStatistic
}

func StartProxyStatisticsRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	var buffer []domain.ProxyStatistic
	dirtyProxyIDs := make(map[uint64]struct{})
	flushTimer := time.NewTimer(statisticsFlushInterval)
	defer flushTimer.Stop()
	reputationTimer := time.NewTimer(reputationRecalcInterval)
	defer reputationTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			drainProxyStatisticQueue(&buffer)
			mergeProxyIDs(dirtyProxyIDs, flushProxyStatistics(&buffer))
			flushDirtyProxyReputations(dirtyProxyIDs, true)
			return
		case stat := <-proxyStatisticQueue:
			buffer = append(buffer, stat)
			if len(buffer) >= statisticsBatchThreshold {
				mergeProxyIDs(dirtyProxyIDs, flushProxyStatistics(&buffer))
				resetTimer(flushTimer)
			}
		case <-flushTimer.C:
			mergeProxyIDs(dirtyProxyIDs, flushProxyStatistics(&buffer))
			flushTimer.Reset(statisticsFlushInterval)
		case <-reputationTimer.C:
			flushDirtyProxyReputations(dirtyProxyIDs, false)
			reputationTimer.Reset(reputationRecalcInterval)
		}
	}
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
