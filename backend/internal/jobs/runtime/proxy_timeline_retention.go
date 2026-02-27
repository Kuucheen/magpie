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
	proxyTimelineRetentionLockKey = "magpie:leader:proxy_timeline_retention"

	envProxyTimelineRetentionInterval     = "PROXY_TIMELINE_RETENTION_INTERVAL"
	envProxyTimelineRetentionIntervalMins = "PROXY_TIMELINE_RETENTION_INTERVAL_MINUTES"
	envProxyTimelineRetentionMaxRun       = "PROXY_TIMELINE_RETENTION_MAX_RUN_DURATION"
	envProxyTimelineRetentionBatchSize    = "PROXY_TIMELINE_RETENTION_BATCH_SIZE"
	envProxyTimelineRetentionMaxBatches   = "PROXY_TIMELINE_RETENTION_MAX_BATCHES"

	envProxySnapshotRetentionDays = "PROXY_SNAPSHOT_RETENTION_DAYS"
	envProxyHistoryRetentionDays  = "PROXY_HISTORY_RETENTION_DAYS"

	defaultProxyTimelineRetentionIntervalMins = 10
	defaultProxyTimelineRetentionMaxRun       = 90 * time.Second
	defaultProxyTimelineRetentionBatchSize    = 50_000
	defaultProxyTimelineRetentionMaxBatches   = 120
	defaultProxySnapshotRetentionDays         = 30
	defaultProxyHistoryRetentionDays          = 90
	proxyTimelineRetentionDBTimeout           = 30 * time.Second
)

type proxyTimelineRetentionConfig struct {
	interval              time.Duration
	maxRunDuration        time.Duration
	batchSize             int
	maxBatches            int
	snapshotRetentionDays int
	historyRetentionDays  int
}

func StartProxyTimelineRetentionRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := resolveProxyTimelineRetentionConfig()
	if cfg.snapshotRetentionDays <= 0 && cfg.historyRetentionDays <= 0 {
		log.Info("Proxy timeline retention disabled")
		return
	}

	err := support.RunWithLeader(ctx, proxyTimelineRetentionLockKey, support.DefaultLeadershipTTL, func(leaderCtx context.Context) {
		runProxyTimelineRetentionLoop(leaderCtx, cfg)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Proxy timeline retention routine stopped", "error", err)
	}
}

func runProxyTimelineRetentionLoop(ctx context.Context, cfg proxyTimelineRetentionConfig) {
	runProxyTimelineRetentionOnce(ctx, cfg)

	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runProxyTimelineRetentionOnce(ctx, cfg)
		}
	}
}

func runProxyTimelineRetentionOnce(ctx context.Context, cfg proxyTimelineRetentionConfig) {
	start := time.Now()
	now := start.UTC()
	deadline := start.Add(cfg.maxRunDuration)
	totalSnapshotsDeleted := int64(0)
	totalHistoryDeleted := int64(0)
	batchesRun := 0

	for batchesRun < cfg.maxBatches {
		if cfg.maxRunDuration > 0 && time.Now().After(deadline) {
			break
		}

		deletedSnapshots, deletedHistory, err := runProxyTimelineRetentionBatch(ctx, cfg, now)
		if err != nil {
			log.Error("Failed to apply proxy timeline retention", "error", err)
			return
		}

		totalSnapshotsDeleted += deletedSnapshots
		totalHistoryDeleted += deletedHistory
		batchesRun++

		if deletedSnapshots == 0 && deletedHistory == 0 {
			break
		}
	}

	if totalSnapshotsDeleted == 0 && totalHistoryDeleted == 0 {
		return
	}

	log.Info(
		"Proxy timeline retention applied",
		"snapshots_deleted", totalSnapshotsDeleted,
		"history_deleted", totalHistoryDeleted,
		"batches_run", batchesRun,
		"duration", time.Since(start),
	)
}

func runProxyTimelineRetentionBatch(ctx context.Context, cfg proxyTimelineRetentionConfig, now time.Time) (int64, int64, error) {
	var deletedSnapshots int64
	if cfg.snapshotRetentionDays > 0 {
		deleteBefore := now.Add(-time.Duration(cfg.snapshotRetentionDays) * 24 * time.Hour)
		opCtx, cancel := context.WithTimeout(ctx, proxyTimelineRetentionDBTimeout)
		count, err := database.DeleteOldProxySnapshots(opCtx, deleteBefore, cfg.batchSize)
		cancel()
		if err != nil {
			return 0, 0, err
		}
		deletedSnapshots = count
	}

	var deletedHistory int64
	if cfg.historyRetentionDays > 0 {
		deleteBefore := now.Add(-time.Duration(cfg.historyRetentionDays) * 24 * time.Hour)
		opCtx, cancel := context.WithTimeout(ctx, proxyTimelineRetentionDBTimeout)
		count, err := database.DeleteOldProxyHistory(opCtx, deleteBefore, cfg.batchSize)
		cancel()
		if err != nil {
			return deletedSnapshots, 0, err
		}
		deletedHistory = count
	}

	return deletedSnapshots, deletedHistory, nil
}

func resolveProxyTimelineRetentionConfig() proxyTimelineRetentionConfig {
	interval := resolveProxyTimelineRetentionInterval()
	if interval <= 0 {
		interval = time.Duration(defaultProxyTimelineRetentionIntervalMins) * time.Minute
	}

	maxRunDuration := defaultProxyTimelineRetentionMaxRun
	if raw := support.GetEnv(envProxyTimelineRetentionMaxRun, ""); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			maxRunDuration = parsed
		} else {
			log.Warn("Invalid PROXY_TIMELINE_RETENTION_MAX_RUN_DURATION value, using default", "value", raw)
		}
	}

	batchSize := support.GetEnvInt(envProxyTimelineRetentionBatchSize, defaultProxyTimelineRetentionBatchSize)
	if batchSize <= 0 {
		batchSize = defaultProxyTimelineRetentionBatchSize
	}

	maxBatches := support.GetEnvInt(envProxyTimelineRetentionMaxBatches, defaultProxyTimelineRetentionMaxBatches)
	if maxBatches <= 0 {
		maxBatches = defaultProxyTimelineRetentionMaxBatches
	}

	snapshotRetentionDays := support.GetEnvInt(envProxySnapshotRetentionDays, defaultProxySnapshotRetentionDays)
	if snapshotRetentionDays < 0 {
		snapshotRetentionDays = 0
	}

	historyRetentionDays := support.GetEnvInt(envProxyHistoryRetentionDays, defaultProxyHistoryRetentionDays)
	if historyRetentionDays < 0 {
		historyRetentionDays = 0
	}

	return proxyTimelineRetentionConfig{
		interval:              interval,
		maxRunDuration:        maxRunDuration,
		batchSize:             batchSize,
		maxBatches:            maxBatches,
		snapshotRetentionDays: snapshotRetentionDays,
		historyRetentionDays:  historyRetentionDays,
	}
}

func resolveProxyTimelineRetentionInterval() time.Duration {
	if raw := support.GetEnv(envProxyTimelineRetentionInterval, ""); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
		log.Warn("Invalid PROXY_TIMELINE_RETENTION_INTERVAL value, falling back to minute env", "value", raw)
	}

	minutes := support.GetEnvInt(envProxyTimelineRetentionIntervalMins, defaultProxyTimelineRetentionIntervalMins)
	if minutes <= 0 {
		minutes = defaultProxyTimelineRetentionIntervalMins
	}

	return time.Duration(minutes) * time.Minute
}
