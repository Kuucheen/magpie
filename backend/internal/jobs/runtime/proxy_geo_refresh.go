package runtime

import (
	"context"
	"errors"
	"time"

	"github.com/charmbracelet/log"
	"magpie/internal/config"
	"magpie/internal/database"
)

func StartProxyGeoRefreshRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	intervalUpdates := config.ProxyGeoRefreshIntervalUpdates()
	currentInterval := <-intervalUpdates
	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	refreshOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshOnce(ctx)
		case newInterval := <-intervalUpdates:
			if newInterval <= 0 {
				continue
			}
			if newInterval == currentInterval {
				continue
			}
			drainTicker(ticker)
			currentInterval = newInterval
			ticker.Reset(currentInterval)
		}
	}
}

func drainTicker(ticker *time.Ticker) {
	for {
		select {
		case <-ticker.C:
		default:
			return
		}
	}
}

func refreshOnce(ctx context.Context) {
	start := time.Now()

	scanned, updated, err := database.RunProxyGeoRefresh(ctx, 0)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrProxyGeoRefreshDatabaseNotInitialized):
			log.Warn("Proxy geo refresh skipped: database not initialized")
		case errors.Is(err, database.ErrProxyGeoRefreshGeoLiteUnavailable):
			log.Warn("Proxy geo refresh skipped: GeoLite databases unavailable")
		case errors.Is(err, context.Canceled):
			log.Info("Proxy geo refresh canceled", "duration", time.Since(start))
		default:
			log.Error("Proxy geo refresh failed", "error", err)
		}
		return
	}

	log.Info("Proxy geo refresh completed", "scanned", scanned, "updated", updated, "duration", time.Since(start))
}
