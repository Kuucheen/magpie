package checker

import (
	"math/bits"
	"net/http"
	"strings"
	"sync"
	"time"

	"magpie/internal/config"
	"magpie/internal/domain"
	"magpie/internal/support"
)

const (
	checkerHTTPClientCacheTTL             = 5 * time.Minute
	checkerHTTPClientCacheCleanupInterval = 1 * time.Minute
	checkerHTTPClientCacheMinEntries      = 2048
	checkerHTTPClientCacheMaxCap          = 16384
	checkerHTTPClientCacheDefaultEntries  = 12288
)

type checkerHTTPClientCacheKey struct {
	proxyAddr         string
	proxyUsername     string
	proxyPassword     string
	judgeURL          string
	judgeHostname     string
	judgeIP           string
	protocol          string
	transportProtocol string
}

type cachedCheckerHTTPClient struct {
	client   *http.Client
	closeFn  func()
	lastUsed time.Time
}

var (
	checkerHTTPClientCacheMu sync.Mutex
	checkerHTTPClientCache   = make(map[checkerHTTPClientCacheKey]*cachedCheckerHTTPClient)
	nextCheckerCacheCleanup  = time.Now().Add(checkerHTTPClientCacheCleanupInterval)

	checkerTransportFactory = support.CreateTransport
)

func getCheckerHTTPClient(proxyToCheck domain.Proxy, judge *domain.Judge, protocol string, transportProtocol string) (*http.Client, error) {
	if transportProtocol == "" {
		transportProtocol = support.TransportTCP
	}

	key := checkerHTTPClientCacheKey{
		proxyAddr:         proxyToCheck.GetFullProxy(),
		proxyUsername:     proxyToCheck.Username,
		proxyPassword:     proxyToCheck.Password,
		judgeURL:          judge.FullString,
		judgeHostname:     judge.GetHostname(),
		judgeIP:           judge.GetIp(),
		protocol:          strings.ToLower(strings.TrimSpace(protocol)),
		transportProtocol: support.NormalizeTransportProtocol(transportProtocol),
	}

	now := time.Now()

	checkerHTTPClientCacheMu.Lock()
	closeFns := runCheckerHTTPClientCacheMaintenanceLocked(now)
	if entry, ok := checkerHTTPClientCache[key]; ok {
		entry.lastUsed = now
		client := entry.client
		checkerHTTPClientCacheMu.Unlock()
		closeCheckerClients(closeFns)
		return client, nil
	}
	checkerHTTPClientCacheMu.Unlock()
	closeCheckerClients(closeFns)

	transport, closeFn, err := checkerTransportFactory(proxyToCheck, judge, protocol, transportProtocol)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Transport: transport}

	checkerHTTPClientCacheMu.Lock()
	closeFns = runCheckerHTTPClientCacheMaintenanceLocked(now)
	if existing, ok := checkerHTTPClientCache[key]; ok {
		existing.lastUsed = now
		client = existing.client
		checkerHTTPClientCacheMu.Unlock()

		if closeFn != nil {
			closeFn()
		}
		closeCheckerClients(closeFns)
		return client, nil
	}

	checkerHTTPClientCache[key] = &cachedCheckerHTTPClient{
		client:   client,
		closeFn:  closeFn,
		lastUsed: now,
	}
	checkerHTTPClientCacheMu.Unlock()
	closeCheckerClients(closeFns)

	return client, nil
}

func runCheckerHTTPClientCacheMaintenanceLocked(now time.Time) []func() {
	var closeFns []func()

	if now.After(nextCheckerCacheCleanup) {
		for key, entry := range checkerHTTPClientCache {
			if now.Sub(entry.lastUsed) <= checkerHTTPClientCacheTTL {
				continue
			}
			delete(checkerHTTPClientCache, key)
			if entry.closeFn != nil {
				closeFns = append(closeFns, entry.closeFn)
			}
		}
		nextCheckerCacheCleanup = now.Add(checkerHTTPClientCacheCleanupInterval)
	}

	maxEntries := checkerHTTPClientCacheMaxEntries()
	for len(checkerHTTPClientCache) >= maxEntries {
		evictedKey, ok := oldestCheckerHTTPClientCacheKeyLocked()
		if !ok {
			break
		}
		entry := checkerHTTPClientCache[evictedKey]
		delete(checkerHTTPClientCache, evictedKey)
		if entry != nil && entry.closeFn != nil {
			closeFns = append(closeFns, entry.closeFn)
		}
	}

	return closeFns
}

func checkerHTTPClientCacheMaxEntries() int {
	threads := currentThreads.Load()
	if threads == 0 {
		threads = config.GetConfig().Checker.Threads
	}
	if threads == 0 {
		return checkerHTTPClientCacheDefaultEntries
	}
	return checkerHTTPClientCacheEntriesForThreads(threads)
}

func checkerHTTPClientCacheEntriesForThreads(threads uint32) int {
	target := nextPow2(uint64(threads) * 3)
	if target < checkerHTTPClientCacheMinEntries {
		target = checkerHTTPClientCacheMinEntries
	}
	if target > checkerHTTPClientCacheMaxCap {
		target = checkerHTTPClientCacheMaxCap
	}
	return int(target)
}

func nextPow2(value uint64) uint64 {
	if value <= 1 {
		return 1
	}
	return uint64(1) << bits.Len64(value-1)
}

func oldestCheckerHTTPClientCacheKeyLocked() (checkerHTTPClientCacheKey, bool) {
	var (
		oldestKey checkerHTTPClientCacheKey
		oldest    time.Time
		found     bool
	)

	for key, entry := range checkerHTTPClientCache {
		if entry == nil {
			continue
		}
		if !found || entry.lastUsed.Before(oldest) {
			oldestKey = key
			oldest = entry.lastUsed
			found = true
		}
	}

	return oldestKey, found
}

func closeCheckerClients(closeFns []func()) {
	for _, closeFn := range closeFns {
		if closeFn != nil {
			closeFn()
		}
	}
}

func resetCheckerHTTPClientCacheForTests() {
	checkerHTTPClientCacheMu.Lock()
	closeFns := make([]func(), 0, len(checkerHTTPClientCache))
	for _, entry := range checkerHTTPClientCache {
		if entry != nil && entry.closeFn != nil {
			closeFns = append(closeFns, entry.closeFn)
		}
	}
	checkerHTTPClientCache = make(map[checkerHTTPClientCacheKey]*cachedCheckerHTTPClient)
	nextCheckerCacheCleanup = time.Now().Add(checkerHTTPClientCacheCleanupInterval)
	checkerHTTPClientCacheMu.Unlock()

	closeCheckerClients(closeFns)
}
