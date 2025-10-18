package scraper

import (
	"context"
	"errors"
	"fmt"
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

/* ─────────────────────────────  browser & page pool  ───────────────────── */

var (
	browser      *rod.Browser
	pagePool     chan *rod.Page
	currentPages atomic.Int32

	stopPage     = make(chan struct{}) // signals that a page should be closed
	browserAlive atomic.Bool
	restartCh    = make(chan struct{}, 1) // coalesced restart signal
)

/* ─────────────────────────────  init  ───────────────────────────────────── */

func init() {
	pagePool = make(chan *rod.Page, 2000)
	mustRestartBrowser() // initial bring-up
	go BrowserWatchdog() // listen for restart requests
	go ManagePagePool()  // keep pool aligned with demand
}

/* ─────────────────────────────  dispatcher  ─────────────────────────────── */

func ThreadDispatcher() {
	for {
		cfg := config.GetConfig()

		var target uint32
		if cfg.Scraper.DynamicThreads {
			target = autoThreadCount(cfg)
		} else {
			target = cfg.Scraper.Threads
		}

		for currentThreads.Load() < target {
			go scrapeWorker()
			currentThreads.Add(1)
		}
		for currentThreads.Load() > target {
			stopThread <- struct{}{}
			currentThreads.Add(^uint32(0)) // decrement
		}

		log.Debug("Scraper threads", "active", currentThreads.Load())
		time.Sleep(15 * time.Second)
	}
}

/* ─────────────────────────────  worker  ─────────────────────────────────── */

func scrapeWorker() {
	ctx, cancel := context.WithCancel(context.Background())
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

		var html string
		var scrapeErr error

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
			go handleScrapedHTML(site, html)
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

	switch {
	case threads == 0 && perInstance > 0:
		threads = 1
	case threads > math.MaxUint32:
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
	if required > 2000 {
		required = 2000
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

		mustRestartBrowser()

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
	// small wait window to let watchdog spin up
	select {
	case <-time.After(2 * time.Second):
		// continue; watchdog might already have done the work
	default:
	}
	if !browserAlive.Load() {
		return fmt.Errorf("browser not available")
	}
	return nil
}

func mustRestartBrowser() {
	// Close old quietly
	if browser != nil {
		_ = rod.Try(func() { browser.MustClose() })
	}

	// Launch Chrome
	url := launcher.New().
		// Sleep/resume can confuse leakless in dev; keep it off on laptops
		Leakless(false).
		Headless(true).
		// Flags that reduce background throttling after resume
		Set("disable-background-timer-throttling").
		Set("disable-backgrounding-occluded-windows").
		Set("disable-renderer-backgrounding").
		MustLaunch()

	b := rod.New().ControlURL(url)
	// connect with simple backoff
	var err error
	for i := 0; i < 10; i++ {
		if err = b.Connect(); err == nil {
			break
		}
		time.Sleep(time.Duration(250*(i+1)) * time.Millisecond)
	}
	if err != nil {
		panic(fmt.Errorf("browser connect failed: %w", err))
	}

	browser = b
	if err := (proto.BrowserSetDownloadBehavior{
		Behavior:         proto.BrowserSetDownloadBehaviorBehaviorDeny,
		BrowserContextID: browser.BrowserContextID,
	}).Call(browser); err != nil {
		log.Warn("disable browser downloads failed", "err", err)
	}
	browserAlive.Store(true)
}

/* ─────────────────────────────  helpers  ────────────────────────────────── */

func safeClosePage(p *rod.Page) error {
	return rod.Try(func() { p.MustClose() })
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
	parsedProxies := support.ParseTextToProxies(strings.Join(proxyList, "\n"))

	proxies, err := database.InsertAndGetProxies(parsedProxies, support.GetUserIdsFromList(site.Users)...)
	if err != nil {
		log.Error("insert proxies from scraping failed", "err", err)
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
