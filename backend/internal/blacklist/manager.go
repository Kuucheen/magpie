package blacklist

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	"golang.org/x/sync/singleflight"

	"magpie/internal/config"
	"magpie/internal/database"
	"magpie/internal/domain"
	proxyqueue "magpie/internal/jobs/queue/proxy"
	"magpie/internal/support"
)

const (
	maxResponseBytes       = 10 << 20 // 10 MiB safety cap
	refreshLockKey         = "magpie:leader:blacklist_refresh"
	defaultRefreshInterval = 6 * time.Hour
)

var (
	cache       atomicMap
	rangeCache  atomicRangeList
	refreshOnce singleflight.Group
	httpClient  = &http.Client{Timeout: 30 * time.Second}
	ipRegex     = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}(?:/\d{1,2})?\b`)
)

type atomicMap struct {
	val atomic.Value
}

func (a *atomicMap) Load() map[string]struct{} {
	raw, ok := a.val.Load().(map[string]struct{})
	if !ok || raw == nil {
		empty := make(map[string]struct{})
		a.val.Store(empty)
		return empty
	}
	return raw
}

func (a *atomicMap) Store(m map[string]struct{}) {
	a.val.Store(m)
}

type atomicRangeList struct {
	val atomic.Value
}

func (a *atomicRangeList) Load() []domain.BlacklistedRange {
	raw, ok := a.val.Load().([]domain.BlacklistedRange)
	if !ok || raw == nil {
		empty := make([]domain.BlacklistedRange, 0)
		a.val.Store(empty)
		return empty
	}
	return raw
}

func (a *atomicRangeList) Store(r []domain.BlacklistedRange) {
	a.val.Store(r)
}

type RefreshOutcome struct {
	Sources          int
	TotalFromSources int
	NewIPs           int
	NewRanges        int
	TotalCachedIPs   int
	TotalRanges      int
	RelationsRemoved int64
	OrphanedProxies  []domain.Proxy
}

func init() {
	cache.Store(make(map[string]struct{}))
	rangeCache.Store(nil)
}

// Initialize hydrates the in-memory blacklist cache and backfills missing proxy hashes.
func Initialize(ctx context.Context) error {
	if hashUpdated, intUpdated, err := database.BackfillProxyIPMetadata(ctx); err != nil {
		return fmt.Errorf("backfill proxy ip metadata: %w", err)
	} else if hashUpdated > 0 || intUpdated > 0 {
		log.Info("Backfilled proxy IP metadata", "hash_count", hashUpdated, "int_count", intUpdated)
	}
	return LoadCache(ctx)
}

// LoadCache refreshes the in-memory blacklist IP set from the database.
func LoadCache(ctx context.Context) error {
	ips, err := database.ListBlacklistedIPs(ctx)
	if err != nil {
		return err
	}
	cache.Store(toSet(ips))
	ranges, err := database.ListBlacklistedRanges(ctx)
	if err != nil {
		return err
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].StartIP == ranges[j].StartIP {
			return ranges[i].EndIP < ranges[j].EndIP
		}
		return ranges[i].StartIP < ranges[j].StartIP
	})
	rangeCache.Store(ranges)
	return nil
}

func toSet(ips []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		m[ip] = struct{}{}
	}
	return m
}

func cloneSet(m map[string]struct{}) map[string]struct{} {
	cp := make(map[string]struct{}, len(m))
	for k := range m {
		cp[k] = struct{}{}
	}
	return cp
}

// FilterProxies separates allowed proxies from those using blacklisted IPs.
func FilterProxies(proxies []domain.Proxy) (allowed []domain.Proxy, blocked []domain.Proxy) {
	if len(proxies) == 0 {
		return nil, nil
	}

	set := cache.Load()
	ranges := rangeCache.Load()
	allowed = make([]domain.Proxy, 0, len(proxies))

	for _, proxy := range proxies {
		ip := normalizeIPv4(proxy.GetIp())
		if ip == "" {
			continue
		}
		if _, found := set[ip]; found {
			blocked = append(blocked, proxy)
			continue
		}
		if inRange(ip, ranges) {
			blocked = append(blocked, proxy)
			continue
		}
		allowed = append(allowed, proxy)
	}

	return allowed, blocked
}

// StartRefreshRoutine runs the blacklist refresh loop with dynamic rescheduling.
func StartRefreshRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	var intervalValue atomic.Value
	initial := config.GetBlacklistRefreshInterval()
	if initial <= 0 {
		initial = defaultRefreshInterval
	}
	intervalValue.Store(initial)

	updateSignal := make(chan struct{}, 1)
	updates := config.BlacklistIntervalUpdates()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case newInterval := <-updates:
				if newInterval <= 0 {
					newInterval = defaultRefreshInterval
				}
				intervalValue.Store(newInterval)
				select {
				case updateSignal <- struct{}{}:
				default:
				}
			}
		}
	}()

	err := support.RunWithLeader(ctx, refreshLockKey, support.DefaultLeadershipTTL, func(leaderCtx context.Context) {
		runRefreshLoop(leaderCtx, &intervalValue, updateSignal)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Blacklist refresh routine stopped", "error", err)
	}
}

// RunRefresh triggers a refresh immediately (outside of the scheduled loop).
func RunRefresh(ctx context.Context, reason string, force bool) {
	triggerRefresh(ctx, reason, force)
}

func runRefreshLoop(ctx context.Context, intervalValue *atomic.Value, updateSignal <-chan struct{}) {
	current := intervalValue.Load().(time.Duration)
	if current <= 0 {
		current = defaultRefreshInterval
	}

	ticker := time.NewTicker(current)
	defer ticker.Stop()

	triggerRefresh(ctx, "startup", true)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			triggerRefresh(ctx, "scheduled", false)
		case <-updateSignal:
			newInterval := intervalValue.Load().(time.Duration)
			if newInterval <= 0 {
				newInterval = defaultRefreshInterval
			}
			if newInterval == current {
				continue
			}
			drainTicker(ticker)
			current = newInterval
			ticker.Reset(current)
		}
	}
}

func triggerRefresh(ctx context.Context, reason string, force bool) {
	outcome, err := Refresh(ctx, reason, force)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("Blacklist refresh canceled", "reason", reason)
		} else {
			log.Error("Blacklist refresh failed", "reason", reason, "error", err)
		}
		return
	}
	if outcome == nil {
		return
	}

	if len(outcome.OrphanedProxies) > 0 {
		if err := proxyqueue.PublicProxyQueue.RemoveFromQueue(outcome.OrphanedProxies); err != nil {
			log.Warn("Failed to purge blacklisted proxies from queue", "error", err)
		}
	}

	log.Info("Blacklist refresh completed",
		"reason", reason,
		"sources", outcome.Sources,
		"new_ips", outcome.NewIPs,
		"cached_ips", outcome.TotalCachedIPs,
		"cached_ranges", outcome.TotalRanges,
		"relations_removed", outcome.RelationsRemoved,
	)
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

// Refresh downloads all configured blacklist sources, persists the IPs, refreshes the cache,
// and removes any newly blacklisted proxies from user inventories.
func Refresh(ctx context.Context, reason string, force bool) (*RefreshOutcome, error) {
	result, err, _ := refreshOnce.Do("refresh", func() (interface{}, error) {
		return doRefresh(ctx, reason, force)
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	outcome, _ := result.(*RefreshOutcome)
	return outcome, nil
}

func doRefresh(ctx context.Context, reason string, force bool) (*RefreshOutcome, error) {
	cfg := config.GetConfig()
	sources := append([]string(nil), cfg.BlacklistSources...)

	before := cloneSet(cache.Load())
	beforeRanges := rangeCache.Load()

	if len(sources) == 0 {
		if err := LoadCache(ctx); err != nil {
			return nil, err
		}
		return &RefreshOutcome{
			Sources:        0,
			NewIPs:         0,
			TotalCachedIPs: len(cache.Load()),
		}, nil
	}

	var (
		totalFromSources int
		totalRanges      int
		allIPs           []domain.BlacklistedIP
		allRanges        []domain.BlacklistedRange
	)

	for _, src := range sources {
		ips, ranges, fetchErr := fetchBlacklist(ctx, src)
		if fetchErr != nil {
			if errors.Is(fetchErr, context.Canceled) {
				return nil, fetchErr
			}
			log.Warn("Blacklist fetch failed", "source", src, "error", fetchErr)
			continue
		}

		totalFromSources += len(ips)
		totalRanges += len(ranges)

		for _, ip := range ips {
			allIPs = append(allIPs, domain.BlacklistedIP{IP: ip, Source: src})
		}
		for _, r := range ranges {
			r.Source = src
			allRanges = append(allRanges, r)
		}
	}

	if _, _, err := database.ReplaceBlacklistData(ctx, allIPs, allRanges); err != nil {
		return nil, err
	}

	if err := LoadCache(ctx); err != nil {
		return nil, err
	}

	current := cache.Load()
	currentRanges := rangeCache.Load()
	newIPs := diffSets(current, before)
	newRanges := diffRanges(currentRanges, beforeRanges)

	var (
		removed int64
		orphans []domain.Proxy
	)

	if len(newIPs) > 0 || len(newRanges) > 0 || force {
		var err error
		removed, orphans, err = database.RemoveProxiesByIPs(ctx, newIPs)
		if err != nil {
			return nil, err
		}
		var rangeRemoved int64
		var rangeOrphans []domain.Proxy
		rangeRemoved, rangeOrphans, err = database.RemoveProxiesByRanges(ctx, newRanges)
		if err != nil {
			return nil, err
		}
		removed += rangeRemoved
		orphans = append(orphans, rangeOrphans...)
	}

	return &RefreshOutcome{
		Sources:          len(sources),
		TotalFromSources: totalFromSources,
		NewIPs:           len(newIPs),
		NewRanges:        len(newRanges),
		TotalCachedIPs:   len(current),
		TotalRanges:      len(currentRanges),
		RelationsRemoved: removed,
		OrphanedProxies:  orphans,
	}, nil
}

func diffSets(after, before map[string]struct{}) []string {
	if len(after) == 0 {
		return nil
	}
	added := make([]string, 0, len(after))
	for ip := range after {
		if _, found := before[ip]; found {
			continue
		}
		added = append(added, ip)
	}
	return added
}

func diffRanges(after, before []domain.BlacklistedRange) []domain.BlacklistedRange {
	if len(after) == 0 {
		return nil
	}

	type key struct {
		start uint32
		end   uint32
	}
	beforeSet := make(map[key]struct{}, len(before))
	for _, r := range before {
		beforeSet[key{start: r.StartIP, end: r.EndIP}] = struct{}{}
	}

	added := make([]domain.BlacklistedRange, 0, len(after))
	for _, r := range after {
		k := key{start: r.StartIP, end: r.EndIP}
		if _, found := beforeSet[k]; found {
			continue
		}
		added = append(added, r)
	}

	return added
}

func fetchBlacklist(ctx context.Context, source string) ([]string, []domain.BlacklistedRange, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if config.IsWebsiteBlocked(source) {
		return nil, nil, fmt.Errorf("blacklist source blocked: %s", source)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "magpie-blacklist-fetcher/1.0")
	req.Header.Set("Accept", "text/plain, */*;q=0.9")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	limited := io.LimitReader(resp.Body, maxResponseBytes)
	content, err := io.ReadAll(limited)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}

	ips, ranges := parseIPs(content)
	if len(ips) == 0 && len(ranges) == 0 {
		preview := string(bytes.TrimSpace(bytes.SplitN(content, []byte{'\n'}, 2)[0]))
		if preview == "" {
			preview = string(bytes.TrimSpace(content))
		}
		log.Warn("Blacklist source returned no entries",
			"source", source,
			"content_type", resp.Header.Get("Content-Type"),
			"preview", preview,
		)
	}
	return ips, ranges, nil
}

func parseIPs(payload []byte) ([]string, []domain.BlacklistedRange) {
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	seen := make(map[string]struct{})
	var ranges []domain.BlacklistedRange

	for scanner.Scan() {
		line := scanner.Bytes()
		matches := ipRegex.FindAll(line, -1)
		for _, match := range matches {
			ipStr := string(match)
			cidrs, ips := parseCIDROrIP(ipStr)
			for _, ip := range ips {
				seen[ip] = struct{}{}
			}
			if len(cidrs) > 0 {
				ranges = append(ranges, cidrs...)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Warn("Blacklist scanner warning", "error", err)
	}

	out := make([]string, 0, len(seen))
	for ip := range seen {
		out = append(out, ip)
	}
	return out, ranges
}

func normalizeIPv4(raw string) string {
	parsed := net.ParseIP(raw)
	if parsed == nil {
		return ""
	}
	v4 := parsed.To4()
	if v4 == nil {
		return ""
	}
	return v4.String()
}

func parseCIDROrIP(raw string) ([]domain.BlacklistedRange, []string) {
	if !strings.Contains(raw, "/") {
		ip := normalizeIPv4(raw)
		if ip == "" {
			return nil, nil
		}
		return nil, []string{ip}
	}

	_, ipnet, err := net.ParseCIDR(raw)
	if err != nil || ipnet == nil {
		return nil, nil
	}

	base := ipnet.IP.To4()
	if base == nil {
		return nil, nil
	}

	ones, bits := ipnet.Mask.Size()
	if bits != 32 || ones < 0 || ones > 32 {
		return nil, nil
	}

	start := ipToUint32(base.Mask(ipnet.Mask))
	hostCount := uint32(1) << uint32(bits-ones)
	lastIP := start + hostCount - 1

	return []domain.BlacklistedRange{{
		CIDR:    ipnet.String(),
		StartIP: start,
		EndIP:   lastIP,
	}}, nil
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func inRange(ip string, ranges []domain.BlacklistedRange) bool {
	if len(ranges) == 0 {
		return false
	}

	u := ipToUint32(net.ParseIP(ip))

	lo, hi := 0, len(ranges)
	for lo < hi {
		mid := (lo + hi) / 2
		if u < ranges[mid].StartIP {
			hi = mid
			continue
		}
		if u > ranges[mid].EndIP {
			lo = mid + 1
			continue
		}
		return true
	}
	return false
}
