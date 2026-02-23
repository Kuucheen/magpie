package runtime

import (
	"context"
	"errors"
	"time"

	"magpie/internal/database"
	"magpie/internal/support"

	"github.com/charmbracelet/log"
)

const (
	proxyStatisticsRetentionLockKey = "magpie:leader:proxy_statistics_retention"

	envProxyStatisticsRetentionInterval      = "PROXY_STATISTICS_RETENTION_INTERVAL"
	envProxyStatisticsRetentionIntervalMins  = "PROXY_STATISTICS_RETENTION_INTERVAL_MINUTES"
	envProxyStatisticsRetentionDays          = "PROXY_STATISTICS_RETENTION_DAYS"
	envProxyStatisticsResponseRetentionDays  = "PROXY_STATISTICS_RESPONSE_RETENTION_DAYS"
	envProxyStatisticsRetentionBatchSize     = "PROXY_STATISTICS_RETENTION_BATCH_SIZE"
	envProxyStatisticsRetentionMaxRunBatches = "PROXY_STATISTICS_RETENTION_MAX_BATCHES"

	defaultProxyStatisticsRetentionIntervalMins = 60
	defaultProxyStatisticsRetentionDays         = 30
	defaultProxyStatisticsResponseRetentionDays = 7
	defaultProxyStatisticsRetentionBatchSize    = 5000
	defaultProxyStatisticsRetentionMaxBatches   = 12
	proxyStatisticsRetentionDBTimeout           = 45 * time.Second
)

type proxyStatisticsRetentionConfig struct {
	interval              time.Duration
	statRetentionDays     int
	responseRetentionDays int
	batchSize             int
	maxBatches            int
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

	var totalPruned int64
	var totalDeleted int64

	for batch := 0; batch < cfg.maxBatches; batch++ {
		batchPruned, batchDeleted, err := runProxyStatisticsRetentionBatch(ctx, cfg, now)
		if err != nil {
			log.Error("Failed to apply proxy statistics retention", "error", err)
			return
		}

		totalPruned += batchPruned
		totalDeleted += batchDeleted

		if batchPruned == 0 && batchDeleted == 0 {
			break
		}
	}

	if totalPruned == 0 && totalDeleted == 0 {
		return
	}

	log.Info(
		"Proxy statistics retention applied",
		"response_bodies_pruned", totalPruned,
		"statistics_deleted", totalDeleted,
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

	return proxyStatisticsRetentionConfig{
		interval:              interval,
		statRetentionDays:     statRetentionDays,
		responseRetentionDays: responseRetentionDays,
		batchSize:             batchSize,
		maxBatches:            maxBatches,
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
