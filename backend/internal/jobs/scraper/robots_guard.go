package scraper

import (
	"context"
	"fmt"
	"magpie/internal/config"
	"magpie/internal/support"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

type robotsCacheEntry struct {
	data    *robotstxt.RobotsData
	fetched time.Time
}

const (
	robotsCacheTTL = time.Hour
)

var robotsCache = struct {
	mu      sync.Mutex
	entries map[string]robotsCacheEntry
}{
	entries: make(map[string]robotsCacheEntry),
}

type RobotsCheckResult struct {
	Allowed     bool
	RobotsFound bool
}

func CheckRobotsAllowance(targetURL string, timeout time.Duration) (RobotsCheckResult, error) {
	if config.IsWebsiteBlocked(targetURL) {
		return RobotsCheckResult{Allowed: false}, fmt.Errorf("website is blocked: %s", targetURL)
	}

	validationCtx, cancel := context.WithTimeout(context.Background(), effectiveRobotsTimeout(timeout))
	defer cancel()
	if _, err := support.ValidateOutboundHTTPURLContext(validationCtx, targetURL); err != nil {
		return RobotsCheckResult{Allowed: false}, fmt.Errorf("unsafe robots target: %w", err)
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return RobotsCheckResult{Allowed: true}, fmt.Errorf("parse robots target: %w", err)
	}
	if parsed.Host == "" {
		return RobotsCheckResult{Allowed: true}, fmt.Errorf("parse robots target: missing host in %q", targetURL)
	}

	robotsTimeout := effectiveRobotsTimeout(timeout)

	entry, fetchErr := loadRobotsEntry(parsed, robotsTimeout)
	if fetchErr != nil {
		return RobotsCheckResult{Allowed: true}, fetchErr
	}
	if entry.data == nil {
		return RobotsCheckResult{Allowed: true, RobotsFound: false}, nil
	}

	group := entry.data.FindGroup(scraperUserAgent)
	if group == nil {
		group = entry.data.FindGroup("*")
	}
	if group == nil {
		return RobotsCheckResult{Allowed: true, RobotsFound: true}, nil
	}

	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}

	return RobotsCheckResult{
		Allowed:     group.Test(path),
		RobotsFound: true,
	}, nil
}

func loadRobotsEntry(parsed *url.URL, timeout time.Duration) (robotsCacheEntry, error) {
	key := robotsCacheKey(parsed)

	if entry, ok := getCachedRobotsEntry(key); ok {
		return entry, nil
	}

	entry, err := fetchRobotsEntry(parsed, timeout)
	if err != nil {
		return entry, err
	}

	entry.fetched = time.Now()

	robotsCache.mu.Lock()
	robotsCache.entries[key] = entry
	robotsCache.mu.Unlock()

	return entry, nil
}

func getCachedRobotsEntry(key string) (robotsCacheEntry, bool) {
	robotsCache.mu.Lock()
	defer robotsCache.mu.Unlock()

	entry, ok := robotsCache.entries[key]
	if !ok {
		return robotsCacheEntry{}, false
	}

	if time.Since(entry.fetched) > robotsCacheTTL {
		delete(robotsCache.entries, key)
		return robotsCacheEntry{}, false
	}

	return entry, true
}

func fetchRobotsEntry(parsed *url.URL, timeout time.Duration) (robotsCacheEntry, error) {
	robotsURL := robotsURLFor(parsed)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return robotsCacheEntry{}, err
	}
	req.Header.Set("User-Agent", scraperUserAgent)

	client := support.NewRestrictedOutboundHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return robotsCacheEntry{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return robotsCacheEntry{}, nil
	}

	data, err := robotstxt.FromResponse(resp)
	if err != nil {
		return robotsCacheEntry{}, err
	}

	return robotsCacheEntry{data: data}, nil
}

func robotsCacheKey(parsed *url.URL) string {
	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, parsed.Host)
}

func robotsURLFor(parsed *url.URL) string {
	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/robots.txt", scheme, parsed.Host)
}

func effectiveRobotsTimeout(timeout time.Duration) time.Duration {
	robotsTimeout := timeout
	if robotsTimeout <= 0 {
		robotsTimeout = 10 * time.Second
	}
	return robotsTimeout
}
