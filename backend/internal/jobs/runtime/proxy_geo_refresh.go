package runtime

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	"magpie/internal/config"
	"magpie/internal/database"
	"magpie/internal/support"
)

const (
	proxyGeoRefreshLockKey        = "magpie:leader:proxy_geo_refresh"
	proxyGeoRefreshFallbackTicker = 24 * time.Hour
)

var runProxyGeoRefreshLeaderTaskOnce = support.RunLeaderTaskOnce

func StartProxyGeoRefreshRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	var intervalValue atomic.Value
	initialInterval := config.GetProxyGeoRefreshInterval()
	if initialInterval <= 0 {
		initialInterval = proxyGeoRefreshFallbackTicker
	}
	intervalValue.Store(initialInterval)

	updateSignal := make(chan struct{}, 1)
	updates := config.ProxyGeoRefreshIntervalUpdates()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case newInterval := <-updates:
				if newInterval <= 0 {
					newInterval = proxyGeoRefreshFallbackTicker
				}
				intervalValue.Store(newInterval)
				select {
				case updateSignal <- struct{}{}:
				default:
				}
			}
		}
	}()

	err := support.RunWithLeader(ctx, proxyGeoRefreshLockKey, support.DefaultLeadershipTTL, func(leaderCtx context.Context) {
		runProxyGeoRefreshLoop(leaderCtx, &intervalValue, updateSignal)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Proxy geo refresh routine stopped", "error", err)
	}
}

func runProxyGeoRefreshLoop(ctx context.Context, intervalValue *atomic.Value, updateSignal <-chan struct{}) {
	currentInterval := intervalValue.Load().(time.Duration)
	if currentInterval <= 0 {
		currentInterval = proxyGeoRefreshFallbackTicker
	}

	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	refreshOnce(ctx, "startup")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshOnce(ctx, "scheduled")
		case <-updateSignal:
			newInterval := intervalValue.Load().(time.Duration)
			if newInterval <= 0 {
				newInterval = proxyGeoRefreshFallbackTicker
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

// RunProxyGeoRefresh triggers a single proxy geo refresh immediately while
// respecting the existing leader lock so only one instance updates the database.
func RunProxyGeoRefresh(ctx context.Context, reason string) {
	if ctx == nil {
		ctx = context.Background()
	}

	err := runProxyGeoRefreshLeaderTaskOnce(ctx, proxyGeoRefreshLockKey, support.DefaultLeadershipTTL, func(leaderCtx context.Context) error {
		refreshOnce(leaderCtx, reason)
		return nil
	})
	switch {
	case errors.Is(err, support.ErrLeaderLockNotAcquired):
		log.Debug("Proxy geo refresh skipped: leader lock not acquired", "reason", reason)
	case errors.Is(err, context.Canceled):
		log.Info("Proxy geo refresh canceled before start", "reason", reason)
	case err != nil:
		log.Error("Proxy geo refresh trigger failed", "reason", reason, "error", err)
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

func refreshOnce(ctx context.Context, reason string) {
	start := time.Now()

	scanned, updated, err := database.RunProxyGeoRefresh(ctx, 0)
	if err != nil {
		switch {
		case errors.Is(err, database.ErrProxyGeoRefreshDatabaseNotInitialized):
			log.Warn("Proxy geo refresh skipped: database not initialized", "reason", reason)
		case errors.Is(err, database.ErrProxyGeoRefreshGeoLiteUnavailable):
			log.Warn("Proxy geo refresh skipped: GeoLite databases unavailable", "reason", reason)
		case errors.Is(err, context.Canceled):
			log.Info("Proxy geo refresh canceled", "reason", reason, "duration", time.Since(start))
		default:
			log.Error("Proxy geo refresh failed", "reason", reason, "error", err)
		}
		return
	}

	log.Info("Proxy geo refresh completed", "reason", reason, "scanned", scanned, "updated", updated, "duration", time.Since(start))
}
