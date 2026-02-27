package scraper

import (
	"context"
	"errors"
	"fmt"
	"magpie/internal/blacklist"
	"magpie/internal/config"
	"magpie/internal/database"
	"magpie/internal/domain"
	proxyqueue "magpie/internal/jobs/queue/proxy"
	sitequeue "magpie/internal/jobs/queue/sites"
	"magpie/internal/support"
	"math"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

/* ─────────────────────────────  thread control  ─────────────────────────── */

var (
	currentThreads atomic.Uint32
	stopThread     = make(chan struct{}) // signals a worker to exit
)

const maxScraperPages = 2000

const (
	browserEnsureWaitTimeout     = 2 * time.Second
	browserEnsureWaitPoll        = 100 * time.Millisecond
	browserRestartInitialBackoff = 500 * time.Millisecond
	browserRestartMaxBackoff     = 15 * time.Second
	defaultPostProcessWorkers    = 8
	defaultPostProcessQueueSize  = 256
	maxPostProcessWorkers        = 64
	maxPostProcessQueueSize      = 4096
	envPostProcessWorkers        = "SCRAPER_POST_PROCESS_WORKERS"
	envPostProcessQueueSize      = "SCRAPER_POST_PROCESS_QUEUE_CAPACITY"
)

/* ─────────────────────────────  browser & page pool  ───────────────────── */

var (
	browser      *rod.Browser
	pagePool     chan *rod.Page
	currentPages atomic.Int32

	postProcessQueue chan scrapedHTMLJob

	stopPage     = make(chan struct{}) // signals that a page should be closed
	browserAlive atomic.Bool
	restartCh    = make(chan struct{}, 1) // coalesced restart signal
)

type scrapedHTMLJob struct {
	site domain.ScrapeSite
	html string
}

/* ─────────────────────────────  init  ───────────────────────────────────── */

func init() {
	pagePool = make(chan *rod.Page, maxScraperPages)
	postProcessQueue = make(chan scrapedHTMLJob, resolvePostProcessQueueSize())

	go BrowserWatchdog() // listen for restart requests
	requestRestartBrowser()
	go ManagePagePool() // keep pool aligned with demand
	startScrapedHTMLWorkers(resolvePostProcessWorkers())
}

/* ─────────────────────────────  dispatcher  ─────────────────────────────── */

func ThreadDispatcher(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cfg := config.GetConfig()

		var target uint32
		if cfg.Scraper.DynamicThreads {
			target = autoThreadCount(cfg)
		} else {
			target = cfg.Scraper.Threads
		}

		for currentThreads.Load() < target {
			go scrapeWorker(ctx)
			currentThreads.Add(1)
		}
		for currentThreads.Load() > target {
			stopThread <- struct{}{}
			currentThreads.Add(^uint32(0)) // decrement
		}

		log.Debug("Scraper threads", "active", currentThreads.Load())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

/* ─────────────────────────────  worker  ─────────────────────────────────── */

func scrapeWorker(parent context.Context) {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})

	go func() {
		defer close(done)
		select {
		case <-stopThread:
			cancel()
		case <-ctx.Done():
		}
	}()

	defer func() {
		cancel()
		<-done
	}()

	for {
		site, due, err := sitequeue.PublicScrapeSiteQueue.GetNextScrapeSiteContext(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Error("pop scrape site", "err", err)
			time.Sleep(2 * time.Second)
			continue
		}

		cfg := config.GetConfig()
		timeout := time.Duration(cfg.Scraper.Timeout) * time.Millisecond

		skipScrape := false
		if config.IsWebsiteBlocked(site.URL) {
			log.Info("Skipping blocked scrape site", "url", site.URL)
			_ = sitequeue.PublicScrapeSiteQueue.RemoveFromQueue([]domain.ScrapeSite{site})
			continue
		}
		if cfg.Scraper.RespectRobots {
			result, robotsErr := CheckRobotsAllowance(site.URL, timeout)
			if robotsErr != nil {
				log.Warn("robots.txt check failed", "url", site.URL, "err", robotsErr)
			}
			if result.RobotsFound && !result.Allowed {
				log.Info("robots.txt disallows scraping; skipping", "url", site.URL)
				skipScrape = true
			}
		}

		var html string
		var scrapeErr error

		if !skipScrape {
			for attempts := 0; attempts < 3; attempts++ {
				html, scrapeErr = ScraperRequest(site.URL, timeout)
				if isConnClosed(scrapeErr) {
					// Treat DevTools socket loss as transient infra failure, not site failure.
					browserAlive.Store(false)
					requestRestartBrowser()
					time.Sleep(1 * time.Second)
					continue
				}
				if scrapeErr == nil || !strings.Contains(scrapeErr.Error(), "timeout waiting for available page") {
					break
				}
				log.Debug("retrying after page timeout", "url", site.URL, "attempt", attempts+1)
				time.Sleep(1 * time.Second)
			}

			if scrapeErr != nil {
				log.Warn("scrape failed", "url", site.URL, "err", scrapeErr)
			} else {
				if err := enqueueScrapedHTML(ctx, site, html); err != nil {
					log.Warn("scraped html enqueue interrupted", "url", site.URL, "err", err)
				}
			}
		}

		hasUsers, err := database.ScrapeSiteHasUsers(site.ID)
		if err != nil {
			log.Error("verify scrape site ownership", "site_id", site.ID, "url", site.URL, "err", err)
			if err := sitequeue.PublicScrapeSiteQueue.RequeueScrapeSite(site, due); err != nil {
				log.Error("requeue site", "err", err)
			}
			continue
		}

		if !hasUsers {
			log.Debug("scrape site no longer in use; skipping requeue", "site_id", site.ID, "url", site.URL)
			if err := sitequeue.PublicScrapeSiteQueue.RemoveFromQueue([]domain.ScrapeSite{site}); err != nil {
				log.Error("failed to remove scrape site without owners from queue", "site_id", site.ID, "url", site.URL, "err", err)
			}
			continue
		}

		if err := sitequeue.PublicScrapeSiteQueue.RequeueScrapeSite(site, due); err != nil {
			log.Error("requeue site", "err", err)
		}
	}
}

/* ─────────────────────────────  auto-sizing  ────────────────────────────── */

func autoThreadCount(cfg config.Config) uint32 {
	totalSites, err := sitequeue.PublicScrapeSiteQueue.GetScrapeSiteCount()
	if err != nil {
		log.Error("count sites", "err", err)
		return 1
	}

	instances, err := sitequeue.PublicScrapeSiteQueue.GetActiveInstances()
	if err != nil || instances == 0 {
		instances = 1
	}

	perInstance := (totalSites + int64(instances) - 1) / int64(instances)

	period := config.CalculateMillisecondsOfCheckingPeriod(cfg.Scraper.ScraperTimer)
	if period == 0 {
		log.Warn("scraper period 0 → forcing 1 day")
		period = 86_400_000
	}

	numerator := uint64(perInstance) * uint64(cfg.Scraper.Timeout) * uint64(cfg.Scraper.Retries+1)
	threads := (numerator + period - 1) / period
	maxThreads := cfg.Scraper.MaxThreads
	if maxThreads == 0 {
		if cfg.Scraper.Threads > 0 {
			maxThreads = cfg.Scraper.Threads
		} else {
			maxThreads = 250
		}
	}

	if threads == 0 && perInstance > 0 {
		threads = 1
	}
	if threads > uint64(maxThreads) {
		threads = uint64(maxThreads)
	}
	if threads > math.MaxUint32 {
		threads = math.MaxUint32
	}
	return uint32(threads)
}

/* ─────────────────────────────  page-pool mgmt  ─────────────────────────── */

func ManagePagePool() {
	for {
		cfg := config.GetConfig()
		targetPages := calcRequiredPages(cfg)

		for currentPages.Load() < targetPages {
			if err := addPage(); err != nil {
				if isConnClosed(err) {
					browserAlive.Store(false)
					requestRestartBrowser()
				}
				log.Error("add page", "err", err)
				time.Sleep(1 * time.Second)
				continue
			}
		}

		for currentPages.Load() > targetPages {
			select {
			case p := <-pagePool:
				_ = safeClosePage(p)
				currentPages.Add(-1)
			default:
				stopPage <- struct{}{}
			}
		}

		time.Sleep(15 * time.Second)
	}
}

func calcRequiredPages(cfg config.Config) int32 {
	count := uint64(1)
	if n, err := sitequeue.PublicScrapeSiteQueue.GetScrapeSiteCount(); err == nil {
		count = uint64(n)
	}

	interval := config.CalculateMillisecondsOfCheckingPeriod(cfg.Scraper.ScraperTimer)
	if interval == 0 {
		interval = 86_400_000
	}
	avg := uint64(cfg.Scraper.Timeout * (cfg.Scraper.Retries + 1)) // ms

	required := (count * avg) / uint64(interval)
	if required < 1 && count > 0 {
		required = 1
	}
	if required > maxScraperPages {
		required = maxScraperPages
	}
	return int32(required)
}

func addPage() error {
	if err := ensureBrowser(); err != nil {
		return err
	}
	p, err := stealth.Page(browser)
	if err != nil {
		if isConnClosed(err) {
			browserAlive.Store(false)
			requestRestartBrowser()
		}
		return fmt.Errorf("stealth page: %w", err)
	}
	select {
	case pagePool <- p:
		currentPages.Add(1)
		return nil
	default:
		_ = safeClosePage(p)
		return fmt.Errorf("pool full")
	}
}

func recyclePage(p *rod.Page) {
	select {
	case <-stopPage:
		_ = safeClosePage(p)
		currentPages.Add(-1)
		return
	default:
	}

	if err := resetPage(p); err != nil {
		log.Debug("page reset failed, replacing", "err", err)
		_ = safeClosePage(p)
		currentPages.Add(-1)
		if isConnClosed(err) {
			browserAlive.Store(false)
			requestRestartBrowser()
		}
		go func() {
			if err := addPage(); err != nil {
				log.Error("add replacement page", "err", err)
			}
		}()
		return
	}

	select {
	case pagePool <- p:
		// recycled
	default:
		_ = safeClosePage(p)
		currentPages.Add(-1)
	}
}

/* ─────────────────────────────  browser lifecycle  ──────────────────────── */

func BrowserWatchdog() {
	for range restartCh {
		browserAlive.Store(false)

		// drain page pool; old pages are tied to dead DevTools socket
		for {
			select {
			case p := <-pagePool:
				_ = safeClosePage(p)
				currentPages.Add(-1)
			default:
				goto drained
			}
		}
	drained:

		restartBrowserWithRetry()

		// repopulate opportunistically to previous target
		go func(target int32) {
			for currentPages.Load() < target {
				if err := addPage(); err != nil {
					time.Sleep(300 * time.Millisecond)
					continue
				}
			}
		}(currentPages.Load() + 0) // snapshot
	}
}

func requestRestartBrowser() {
	select {
	case restartCh <- struct{}{}:
	default:
	}
}

func ensureBrowser() error {
	if browserAlive.Load() {
		return nil
	}
	requestRestartBrowser()
	deadline := time.Now().Add(browserEnsureWaitTimeout)
	for !browserAlive.Load() && time.Now().Before(deadline) {
		time.Sleep(browserEnsureWaitPoll)
	}
	if !browserAlive.Load() {
		return fmt.Errorf("browser not available")
	}
	return nil
}

func restartBrowserWithRetry() {
	backoff := browserRestartInitialBackoff

	for {
		if err := restartBrowser(); err == nil {
			return
		} else {
			log.Error("browser restart failed; running in degraded mode until retry succeeds", "err", err, "retry_in", backoff)
		}

		time.Sleep(backoff)
		backoff *= 2
		if backoff > browserRestartMaxBackoff {
			backoff = browserRestartMaxBackoff
		}
	}
}

func restartBrowser() error {
	browserAlive.Store(false)

	// Close old quietly
	if browser != nil {
		_ = rod.Try(func() { browser.MustClose() })
		browser = nil
	}

	// Launch Chrome
	url, err := launcher.New().
		// Sleep/resume can confuse leakless in dev; keep it off on laptops
		Leakless(true).
		Headless(true).
		// Flags that reduce background throttling after resume
		Set("disable-background-timer-throttling").
		Set("disable-backgrounding-occluded-windows").
		Set("disable-renderer-backgrounding").
		Launch()
	if err != nil {
		return fmt.Errorf("browser launch failed: %w", err)
	}

	b := rod.New().ControlURL(url)
	// connect with simple backoff
	var connectErr error
	for i := 0; i < 10; i++ {
		if connectErr = b.Connect(); connectErr == nil {
			break
		}
		time.Sleep(time.Duration(250*(i+1)) * time.Millisecond)
	}
	if connectErr != nil {
		_ = rod.Try(func() { b.MustClose() })
		return fmt.Errorf("browser connect failed: %w", connectErr)
	}

	browser = b
	if err := (proto.BrowserSetDownloadBehavior{
		Behavior:         proto.BrowserSetDownloadBehaviorBehaviorDeny,
		BrowserContextID: browser.BrowserContextID,
	}).Call(browser); err != nil {
		log.Warn("disable browser downloads failed", "err", err)
	}
	browserAlive.Store(true)
	return nil
}

/* ─────────────────────────────  helpers  ────────────────────────────────── */

func safeClosePage(p *rod.Page) error {
	return rod.Try(func() { p.MustClose() })
}

func startScrapedHTMLWorkers(count int) {
	for i := 0; i < count; i++ {
		go func() {
			for job := range postProcessQueue {
				handleScrapedHTML(job.site, job.html)
			}
		}()
	}
}

func enqueueScrapedHTML(ctx context.Context, site domain.ScrapeSite, html string) error {
	job := scrapedHTMLJob{
		site: site,
		html: html,
	}

	select {
	case postProcessQueue <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func resolvePostProcessWorkers() int {
	workers := support.GetEnvInt(envPostProcessWorkers, defaultPostProcessWorkers)
	if workers < 1 {
		return 1
	}
	if workers > maxPostProcessWorkers {
		return maxPostProcessWorkers
	}
	return workers
}

func resolvePostProcessQueueSize() int {
	size := support.GetEnvInt(envPostProcessQueueSize, defaultPostProcessQueueSize)
	if size < 1 {
		return 1
	}
	if size > maxPostProcessQueueSize {
		return maxPostProcessQueueSize
	}
	return size
}

func isConnClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "websocket: close") ||
		strings.Contains(s, "read tcp") ||
		strings.Contains(s, "write tcp")
}

/* ─────────────────────────────  downstream handlers  ────────────────────── */

func handleScrapedHTML(site domain.ScrapeSite, rawHTML string) {
	proxyList := support.GetProxiesOfHTML(rawHTML)
	parsedProxies := support.ParseTextToProxiesStrictAuth(strings.Join(proxyList, "\n"))

	parsedProxies, blocked := blacklist.FilterProxies(parsedProxies)
	if len(blocked) > 0 {
		log.Info("Skipped blacklisted scraped proxies", "count", len(blocked), "url", site.URL)
	}

	proxies, err := database.InsertAndGetProxiesWithUser(parsedProxies, support.GetUserIdsFromList(site.Users)...)
	if err != nil {
		log.Error("insert proxies from scraping failed", "err", err)
	} else {
		proxiesToEnrich := database.FilterProxiesMissingGeo(proxies)
		if len(proxiesToEnrich) > 0 {
			database.AsyncEnrichProxyMetadata(proxiesToEnrich)
		}
	}

	err = database.AssociateProxiesToScrapeSite(site.ID, proxies)
	if err != nil {
		log.Warn("associate proxies to ScrapeSite failed", "err", err)
	}

	err = proxyqueue.PublicProxyQueue.AddToQueue(proxies)
	if err != nil {
		log.Error("adding scraped proxies to queue failed", "err", err)
	}

	log.Info(fmt.Sprintf("Found %d unique proxies that users don't have", len(proxies)), "url", site.URL)
}
