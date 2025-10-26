package maintenance

import (
	"context"
	"time"

	"github.com/charmbracelet/log"

	"magpie/internal/database"
	"magpie/internal/support"
)

const (
	envCleanupInterval        = "PROXY_ORPHAN_CLEAN_INTERVAL"
	envCleanupIntervalMinutes = "PROXY_ORPHAN_CLEAN_INTERVAL_MINUTES"

	defaultCleanupMinutes = 60
)

func StartOrphanCleanupRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	interval := resolveCleanupInterval()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runOrphanCleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOrphanCleanup(ctx)
		}
	}
}

func resolveCleanupInterval() time.Duration {
	if raw := support.GetEnv(envCleanupInterval, ""); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
		log.Warn("Invalid PROXY_ORPHAN_CLEAN_INTERVAL value, falling back to minutes env", "value", raw)
	}

	minutes := support.GetEnvInt(envCleanupIntervalMinutes, defaultCleanupMinutes)
	if minutes <= 0 {
		minutes = defaultCleanupMinutes
	}

	return time.Duration(minutes) * time.Minute
}

func runOrphanCleanup(ctx context.Context) {
	start := time.Now()

	var proxyRemoved, siteRemoved int64

	if removed, err := database.DeleteOrphanProxies(ctx); err != nil {
		log.Error("Failed to cleanup orphan proxies", "error", err)
	} else {
		proxyRemoved = removed
	}

	if removed, err := database.DeleteOrphanScrapeSites(ctx); err != nil {
		log.Error("Failed to cleanup orphan scrape sites", "error", err)
	} else {
		siteRemoved = removed
	}

	if proxyRemoved == 0 && siteRemoved == 0 {
		return
	}

	log.Info(
		"Orphan cleanup completed",
		"proxies_removed", proxyRemoved,
		"scrape_sites_removed", siteRemoved,
		"duration", time.Since(start),
	)
}
