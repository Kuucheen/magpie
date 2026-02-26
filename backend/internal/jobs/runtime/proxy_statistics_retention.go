package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"magpie/internal/database"
	"magpie/internal/support"

	"github.com/charmbracelet/log"
)

const (
	proxyStatisticsRetentionLockKey = "magpie:leader:proxy_statistics_retention"

	envProxyStatisticsRetentionInterval          = "PROXY_STATISTICS_RETENTION_INTERVAL"
	envProxyStatisticsRetentionIntervalMins      = "PROXY_STATISTICS_RETENTION_INTERVAL_MINUTES"
	envProxyStatisticsRetentionDays              = "PROXY_STATISTICS_RETENTION_DAYS"
	envProxyStatisticsResponseRetentionDays      = "PROXY_STATISTICS_RESPONSE_RETENTION_DAYS"
	envProxyStatisticsRetentionBatchSize         = "PROXY_STATISTICS_RETENTION_BATCH_SIZE"
	envProxyStatisticsRetentionMaxRunBatches     = "PROXY_STATISTICS_RETENTION_MAX_BATCHES"
	envProxyStatisticsRetentionWorkers           = "PROXY_STATISTICS_RETENTION_WORKERS"
	envProxyStatisticsRetentionMaxRun            = "PROXY_STATISTICS_RETENTION_MAX_RUN_DURATION"
	envProxyStatisticsRetentionDropPartitions    = "PROXY_STATISTICS_RETENTION_DROP_PARTITIONS"
	envProxyStatisticsRetentionMaxPartitionDrops = "PROXY_STATISTICS_RETENTION_MAX_PARTITION_DROPS"

	defaultProxyStatisticsRetentionIntervalMins      = 5
	defaultProxyStatisticsRetentionDays              = 30
	defaultProxyStatisticsResponseRetentionDays      = 7
	defaultProxyStatisticsRetentionBatchSize         = 50_000
	defaultProxyStatisticsRetentionMaxBatches        = 240
	defaultProxyStatisticsRetentionWorkers           = 4
	defaultProxyStatisticsRetentionMaxRun            = 4 * time.Minute
	defaultProxyStatisticsRetentionDropPartitions    = true
	defaultProxyStatisticsRetentionMaxPartitionDrops = 2
	proxyStatisticsRetentionDBTimeout                = 45 * time.Second
)

type proxyStatisticsRetentionConfig struct {
	interval              time.Duration
	statRetentionDays     int
	responseRetentionDays int
	batchSize             int
	maxBatches            int
	workers               int
	maxRunDuration        time.Duration
	dropPartitions        bool
	maxPartitionDrops     int
}

func StartProxyStatisticsRetentionRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := resolveProxyStatisticsRetentionConfig()
	if cfg.statRetentionDays <= 0 && cfg.responseRetentionDays <= 0 {
		log.Info("Proxy statistics retention disabled")
		return
	}

	err := support.RunWithLeader(ctx, proxyStatisticsRetentionLockKey, support.DefaultLeadershipTTL, func(leaderCtx context.Context) {
		runProxyStatisticsRetentionLoop(leaderCtx, cfg)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Proxy statistics retention routine stopped", "error", err)
	}
}

func runProxyStatisticsRetentionLoop(ctx context.Context, cfg proxyStatisticsRetentionConfig) {
	runProxyStatisticsRetentionOnce(ctx, cfg)

	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runProxyStatisticsRetentionOnce(ctx, cfg)
		}
	}
}

func runProxyStatisticsRetentionOnce(ctx context.Context, cfg proxyStatisticsRetentionConfig) {
	now := time.Now().UTC()
	start := time.Now()
	deadline := start.Add(cfg.maxRunDuration)

	var totalPruned int64
	var totalDeleted int64
	var totalPartitionsDropped int
	batchesRun := 0

	for batchesRun < cfg.maxBatches {
		if cfg.maxRunDuration > 0 && time.Now().After(deadline) {
			break
		}

		toRun := cfg.workers
		remaining := cfg.maxBatches - batchesRun
		if toRun > remaining {
			toRun = remaining
		}
		if toRun <= 0 {
			break
		}

		progress := false

		if cfg.dropPartitions && cfg.statRetentionDays > 0 {
			dropBefore := now.Add(-time.Duration(cfg.statRetentionDays) * 24 * time.Hour)
			opCtx, cancel := context.WithTimeout(ctx, proxyStatisticsRetentionDBTimeout)
			dropped, err := database.DropProxyStatisticsPartitionsOlderThan(opCtx, dropBefore, cfg.maxPartitionDrops)
			cancel()
			if err != nil {
				log.Error("Failed to drop old proxy statistics partitions", "error", err)
				return
			}
			totalPartitionsDropped += dropped
			if dropped > 0 {
				progress = true
			}
		}

		results := make(chan struct {
			pruned  int64
			deleted int64
			err     error
		}, toRun)

		var wg sync.WaitGroup
		for i := 0; i < toRun; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pruned, deleted, err := runProxyStatisticsRetentionBatch(ctx, cfg, now)
				results <- struct {
					pruned  int64
					deleted int64
					err     error
				}{pruned: pruned, deleted: deleted, err: err}
			}()
		}
		wg.Wait()
		close(results)

		for result := range results {
			if result.err != nil {
				log.Error("Failed to apply proxy statistics retention", "error", result.err)
				return
			}
			totalPruned += result.pruned
			totalDeleted += result.deleted
			if result.pruned > 0 || result.deleted > 0 {
				progress = true
			}
		}

		batchesRun += toRun
		if !progress {
			break
		}
	}

	if totalPruned == 0 && totalDeleted == 0 && totalPartitionsDropped == 0 {
		return
	}

	log.Info(
		"Proxy statistics retention applied",
		"response_bodies_pruned", totalPruned,
		"statistics_deleted", totalDeleted,
		"partitions_dropped", totalPartitionsDropped,
		"batches_run", batchesRun,
		"duration", time.Since(start),
	)
}

func runProxyStatisticsRetentionBatch(ctx context.Context, cfg proxyStatisticsRetentionConfig, now time.Time) (int64, int64, error) {
	var pruned int64
	if cfg.responseRetentionDays > 0 {
		pruneBefore := now.Add(-time.Duration(cfg.responseRetentionDays) * 24 * time.Hour)
		opCtx, cancel := context.WithTimeout(ctx, proxyStatisticsRetentionDBTimeout)
		count, err := database.PruneProxyStatisticResponseBodies(opCtx, pruneBefore, cfg.batchSize)
		cancel()
		if err != nil {
			return 0, 0, err
		}
		pruned = count
	}

	var deleted int64
	if cfg.statRetentionDays > 0 {
		deleteBefore := now.Add(-time.Duration(cfg.statRetentionDays) * 24 * time.Hour)
		opCtx, cancel := context.WithTimeout(ctx, proxyStatisticsRetentionDBTimeout)
		count, err := database.DeleteOldProxyStatistics(opCtx, deleteBefore, cfg.batchSize)
		cancel()
		if err != nil {
			return pruned, 0, err
		}
		deleted = count
	}

	return pruned, deleted, nil
}

func resolveProxyStatisticsRetentionConfig() proxyStatisticsRetentionConfig {
	interval := resolveProxyStatisticsRetentionInterval()
	if interval <= 0 {
		interval = time.Duration(defaultProxyStatisticsRetentionIntervalMins) * time.Minute
	}

	statRetentionDays := support.GetEnvInt(envProxyStatisticsRetentionDays, defaultProxyStatisticsRetentionDays)
	if statRetentionDays < 0 {
		statRetentionDays = 0
	}

	responseRetentionDays := support.GetEnvInt(envProxyStatisticsResponseRetentionDays, defaultProxyStatisticsResponseRetentionDays)
	if responseRetentionDays < 0 {
		responseRetentionDays = 0
	}

	batchSize := support.GetEnvInt(envProxyStatisticsRetentionBatchSize, defaultProxyStatisticsRetentionBatchSize)
	if batchSize <= 0 {
		batchSize = defaultProxyStatisticsRetentionBatchSize
	}

	maxBatches := support.GetEnvInt(envProxyStatisticsRetentionMaxRunBatches, defaultProxyStatisticsRetentionMaxBatches)
	if maxBatches <= 0 {
		maxBatches = defaultProxyStatisticsRetentionMaxBatches
	}

	workers := support.GetEnvInt(envProxyStatisticsRetentionWorkers, defaultProxyStatisticsRetentionWorkers)
	if workers <= 0 {
		workers = defaultProxyStatisticsRetentionWorkers
	}
	if workers > maxBatches {
		workers = maxBatches
	}

	maxRunDuration := defaultProxyStatisticsRetentionMaxRun
	if raw := support.GetEnv(envProxyStatisticsRetentionMaxRun, ""); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			maxRunDuration = parsed
		} else {
			log.Warn("Invalid PROXY_STATISTICS_RETENTION_MAX_RUN_DURATION value, using default", "value", raw)
		}
	}

	maxPartitionDrops := support.GetEnvInt(envProxyStatisticsRetentionMaxPartitionDrops, defaultProxyStatisticsRetentionMaxPartitionDrops)
	if maxPartitionDrops <= 0 {
		maxPartitionDrops = defaultProxyStatisticsRetentionMaxPartitionDrops
	}

	return proxyStatisticsRetentionConfig{
		interval:              interval,
		statRetentionDays:     statRetentionDays,
		responseRetentionDays: responseRetentionDays,
		batchSize:             batchSize,
		maxBatches:            maxBatches,
		workers:               workers,
		maxRunDuration:        maxRunDuration,
		dropPartitions:        support.GetEnvBool(envProxyStatisticsRetentionDropPartitions, defaultProxyStatisticsRetentionDropPartitions),
		maxPartitionDrops:     maxPartitionDrops,
	}
}

func resolveProxyStatisticsRetentionInterval() time.Duration {
	if raw := support.GetEnv(envProxyStatisticsRetentionInterval, ""); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
		log.Warn("Invalid PROXY_STATISTICS_RETENTION_INTERVAL value, falling back to minute env", "value", raw)
	}

	minutes := support.GetEnvInt(envProxyStatisticsRetentionIntervalMins, defaultProxyStatisticsRetentionIntervalMins)
	if minutes <= 0 {
		minutes = defaultProxyStatisticsRetentionIntervalMins
	}

	return time.Duration(minutes) * time.Minute
}
