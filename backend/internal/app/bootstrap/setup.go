package bootstrap

import (
	"context"
	"time"

	"github.com/charmbracelet/log"

	"magpie/internal/blacklist"
	"magpie/internal/config"
	"magpie/internal/database"
	"magpie/internal/domain"
	"magpie/internal/geolite"
	"magpie/internal/jobs/checker"
	"magpie/internal/jobs/checker/judges"
	maintenance "magpie/internal/jobs/maintenance"
	proxyqueue "magpie/internal/jobs/queue/proxy"
	sitequeue "magpie/internal/jobs/queue/sites"
	jobruntime "magpie/internal/jobs/runtime"
	"magpie/internal/jobs/scraper"
	"magpie/internal/rotatingproxy"
	"magpie/internal/support"
)

func Setup() {
	config.ReadSettings()

	if redisClient, err := support.GetRedisClient(); err != nil {
		log.Warn("Redis synchronization disabled", "error", err)
	} else {
		config.EnableRedisSynchronization(context.Background(), redisClient)
		judges.EnableRedisSynchronization(context.Background(), redisClient)
		geolite.EnableRedisDistribution(context.Background(), redisClient)
	}

	if _, err := database.SetupDB(); err != nil {
		log.Fatalf("failed to set up database: %v", err)
	}
	config.SetBetweenTime()

	if err := blacklist.Initialize(context.Background()); err != nil {
		log.Warn("Blacklist initialisation failed", "error", err)
	}

	if redisClient, err := support.GetRedisClient(); err != nil {
		log.Warn("Blacklist synchronization disabled", "error", err)
	} else {
		blacklist.EnableRedisSynchronization(context.Background(), redisClient)
	}

	judgeSetup()

	cleanedRelations, orphanedProxies, cleanupErr := database.CleanupAutoRemovalViolations(context.Background())
	if cleanupErr != nil {
		log.Error("auto-remove cleanup failed", "error", cleanupErr)
	} else if cleanedRelations > 0 {
		log.Info(
			"Auto-remove cleanup completed",
			"relations_removed", cleanedRelations,
			"orphaned_proxies", len(orphanedProxies),
		)
		if len(orphanedProxies) > 0 {
			if err := proxyqueue.PublicProxyQueue.RemoveFromQueue(orphanedProxies); err != nil {
				log.Warn("failed to purge orphaned proxies from queue", "error", err)
			}
		}
	}

	limitCleanedRelations, limitOrphanedProxies, limitCleanupErr := database.CleanupProxyLimitViolations(context.Background())
	if limitCleanupErr != nil {
		log.Error("proxy-limit cleanup failed", "error", limitCleanupErr)
	} else if limitCleanedRelations > 0 {
		log.Info(
			"Proxy-limit cleanup completed",
			"relations_removed", limitCleanedRelations,
			"orphaned_proxies", len(limitOrphanedProxies),
		)
		if len(limitOrphanedProxies) > 0 {
			if err := proxyqueue.PublicProxyQueue.RemoveFromQueue(limitOrphanedProxies); err != nil {
				log.Warn("failed to purge proxy-limit orphans from queue", "error", err)
			}
		}
	}

	go func() {
		cfg := config.GetConfig()

		if config.GetCurrentIp() == "" && cfg.Checker.IpLookup == "" {
			return
		}

		for config.GetCurrentIp() == "" {
			html, err := checker.DefaultRequest(cfg.Checker.IpLookup)
			if err != nil {
				log.Error("Error checking IP address:", err)
				continue
			}

			currentIp := support.FindIP(html)
			config.SetCurrentIp(currentIp)
			log.Infof("Found IP! Current IP: %s", currentIp)

			time.Sleep(3 * time.Second)
		}

	}()

	proxies, err := database.GetAllProxies()
	if err != nil {
		log.Error("Error getting all proxies:", "error", err)
	} else {
		missingGeo := database.FilterProxiesMissingGeo(proxies)
		if len(missingGeo) > 0 {
			database.AsyncEnrichProxyMetadata(missingGeo)
		}
		proxyqueue.PublicProxyQueue.AddToQueue(proxies)
		log.Infof("Added %d proxies to queue", len(proxies))
	}

	scrapeSites, err := database.GetAllScrapeSites()
	if err != nil {
		log.Error("Error getting all scrape sites:", "error", err)
	} else {
		filtered := make([]domain.ScrapeSite, 0, len(scrapeSites))
		blocked := 0
		for _, site := range scrapeSites {
			if config.IsWebsiteBlocked(site.URL) {
				blocked++
				continue
			}
			filtered = append(filtered, site)
		}

		if blocked > 0 {
			log.Info("Skipped blocked scrape sites", "count", blocked)
		}

		if len(filtered) > 0 {
			sitequeue.PublicScrapeSiteQueue.AddToQueue(filtered)
			log.Infof("Added %d scrape sites to queue", len(filtered))
		}
	}

	rotatingproxy.GlobalManager.StartAll()
	syncIntervalSeconds := support.GetEnvInt("ROTATING_PROXY_SYNC_INTERVAL_SECONDS", 10)
	if syncIntervalSeconds <= 0 {
		syncIntervalSeconds = 10
	}
	go rotatingproxy.GlobalManager.StartSyncLoop(context.Background(), time.Duration(syncIntervalSeconds)*time.Second)

	// Routines

	go judges.StartJudgeRoutine()
	go jobruntime.StartProxyStatisticsRoutine(context.Background())
	go jobruntime.StartProxyHistoryRoutine(context.Background())
	go jobruntime.StartProxySnapshotRoutine(context.Background())
	go jobruntime.StartProxyGeoRefreshRoutine(context.Background())
	go maintenance.StartOrphanCleanupRoutine(context.Background())
	go jobruntime.StartGeoLiteUpdateRoutine(context.Background())
	go blacklist.StartRefreshRoutine(context.Background())
	go checker.ThreadDispatcher()
	go scraper.ThreadDispatcher()
}

func judgeSetup() {
	addJudgeRelationsToCache()
	AddDefaultJudgesToUsers()
}
