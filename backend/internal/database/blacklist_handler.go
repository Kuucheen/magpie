package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/bits"
	"net"
	"net/netip"
	"sort"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jackc/pgx/v5"

	"magpie/internal/domain"

	"gorm.io/gorm"
)

const (
	blacklistInsertBatchSize = 500
)

type cidrSpan struct {
	start  uint32
	end    uint32
	source string
}

// ListBlacklistedIPs returns all stored blacklist entries as plain IPv4 strings.
func ListBlacklistedIPs(ctx context.Context) ([]string, error) {
	if DB == nil {
		return nil, errors.New("database not initialised")
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	var ips []string
	if err := db.Model(&domain.BlacklistedIP{}).Pluck("ip", &ips).Error; err != nil {
		return nil, err
	}
	return ips, nil
}

// ListBlacklistedRanges returns all stored ranges ordered by start IP.
func ListBlacklistedRanges(ctx context.Context) ([]domain.BlacklistedRange, error) {
	if DB == nil {
		return nil, errors.New("database not initialised")
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	var rows []struct {
		ID     uint64 `gorm:"column:id"`
		CIDR   string `gorm:"column:cidr"`
		Source string `gorm:"column:source"`
	}
	if err := db.Raw("SELECT id, cidr::text AS cidr, source FROM blacklisted_ranges ORDER BY cidr ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	ranges := make([]domain.BlacklistedRange, 0, len(rows))
	invalidCount := 0
	for _, row := range rows {
		cidr := strings.TrimSpace(row.CIDR)
		start, end, err := cidrBounds(cidr)
		if err != nil {
			invalidCount++
			continue
		}
		ranges = append(ranges, domain.BlacklistedRange{
			ID:      row.ID,
			CIDR:    cidr,
			Source:  row.Source,
			StartIP: start,
			EndIP:   end,
		})
	}
	if invalidCount > 0 {
		log.Warn("Blacklist range parse failures", "count", invalidCount)
	}

	return ranges, nil
}

// ReplaceBlacklistData truncates and bulk-loads blacklist entries using COPY (or a batch fallback).
func ReplaceBlacklistData(ctx context.Context, ips []domain.BlacklistedIP, ranges []domain.BlacklistedRange) (int, int, error) {
	if DB == nil {
		return 0, 0, errors.New("database not initialised")
	}

	cleanIPs := dedupeIPs(ips)
	cleanRanges := dedupeRanges(ranges)

	if len(cleanIPs) == 0 && len(cleanRanges) == 0 {
		// Still clear existing entries to align with "replace" semantics.
		db := DB
		if ctx != nil {
			db = db.WithContext(ctx)
		}
		if err := db.Exec("TRUNCATE TABLE blacklisted_ips, blacklisted_ranges RESTART IDENTITY").Error; err != nil {
			return 0, 0, err
		}
		return 0, 0, nil
	}

	if len(cleanRanges) == 0 {
		if dsn := getDSN(); dsn != "" {
			if ipCount, rangeCount, err := replaceBlacklistWithCopy(ctx, dsn, cleanIPs, cleanRanges); err == nil {
				return ipCount, rangeCount, nil
			}
		}
	}

	ipCount, rangeCount, err := replaceBlacklistWithBatches(ctx, cleanIPs, cleanRanges)
	return ipCount, rangeCount, err
}

func replaceBlacklistWithCopy(ctx context.Context, dsn string, ips []domain.BlacklistedIP, ranges []domain.BlacklistedRange) (int, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "TRUNCATE TABLE blacklisted_ips, blacklisted_ranges RESTART IDENTITY"); err != nil {
		return 0, 0, err
	}

	if len(ips) > 0 {
		rows := make([][]any, len(ips))
		for i := range ips {
			rows[i] = []any{ips[i].IP, ips[i].Source}
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"blacklisted_ips"}, []string{"ip", "source"}, pgx.CopyFromRows(rows)); err != nil {
			return 0, 0, err
		}
	}

	rangeInserted := 0
	if len(ranges) > 0 {
		rows := make([][]any, 0, len(ranges))
		emptyCIDR := 0
		for i := range ranges {
			cidr := strings.TrimSpace(ranges[i].CIDR)
			if cidr == "" {
				emptyCIDR++
				continue
			}
			rows = append(rows, []any{cidr, ranges[i].Source})
		}
		if emptyCIDR > 0 {
			log.Warn("Blacklist ranges missing CIDR", "count", emptyCIDR)
		}
		if len(rows) > 0 {
			if _, err := tx.CopyFrom(ctx, pgx.Identifier{"blacklisted_ranges"}, []string{"cidr", "source"}, pgx.CopyFromRows(rows)); err != nil {
				return 0, 0, err
			}
			rangeInserted = len(rows)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}

	return len(ips), rangeInserted, nil
}

func replaceBlacklistWithBatches(ctx context.Context, ips []domain.BlacklistedIP, ranges []domain.BlacklistedRange) (int, int, error) {
	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	if err := db.Exec("TRUNCATE TABLE blacklisted_ips, blacklisted_ranges RESTART IDENTITY").Error; err != nil {
		return 0, 0, err
	}

	if len(ips) > 0 {
		if err := db.CreateInBatches(ips, blacklistInsertBatchSize).Error; err != nil {
			return 0, 0, err
		}
	}

	if len(ranges) > 0 {
		filtered := make([]domain.BlacklistedRange, 0, len(ranges))
		for _, r := range ranges {
			if r.CIDR == "" {
				continue
			}
			filtered = append(filtered, r)
		}
		if len(filtered) > 0 {
			if err := insertBlacklistRanges(db, filtered, blacklistInsertBatchSize); err != nil {
				return len(ips), 0, err
			}
			return len(ips), len(filtered), nil
		}
	}

	return len(ips), 0, nil
}

func insertBlacklistRanges(db *gorm.DB, ranges []domain.BlacklistedRange, batchSize int) error {
	if len(ranges) == 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = blacklistInsertBatchSize
	}

	for i := 0; i < len(ranges); i += batchSize {
		end := i + batchSize
		if end > len(ranges) {
			end = len(ranges)
		}

		batch := ranges[i:end]
		placeholders := make([]string, 0, len(batch))
		args := make([]any, 0, len(batch)*2)

		for _, r := range batch {
			cidr := strings.TrimSpace(r.CIDR)
			if cidr == "" {
				continue
			}
			placeholders = append(placeholders, "(?::cidr, ?)")
			args = append(args, cidr, r.Source)
		}

		if len(placeholders) == 0 {
			continue
		}

		stmt := fmt.Sprintf(
			"INSERT INTO blacklisted_ranges (cidr, source) VALUES %s",
			strings.Join(placeholders, ","),
		)

		if err := db.Exec(stmt, args...).Error; err != nil {
			return err
		}
	}

	return nil
}

func dedupeIPs(ips []domain.BlacklistedIP) []domain.BlacklistedIP {
	if len(ips) == 0 {
		return nil
	}

	seen := make(map[string]domain.BlacklistedIP, len(ips))
	for _, ip := range ips {
		normalized := normalizeIPv4(ip.IP)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		ip.IP = normalized
		seen[normalized] = ip
	}

	out := make([]domain.BlacklistedIP, 0, len(seen))
	for _, ip := range seen {
		out = append(out, ip)
	}
	return out
}

func dedupeRanges(ranges []domain.BlacklistedRange) []domain.BlacklistedRange {
	if len(ranges) == 0 {
		return nil
	}

	seenCIDR := make(map[string]string, len(ranges))
	for _, r := range ranges {
		cidr, _, _, err := normalizeCIDR(r.CIDR)
		if err != nil {
			continue
		}
		if _, ok := seenCIDR[cidr]; ok {
			continue
		}
		seenCIDR[cidr] = r.Source
	}

	if len(seenCIDR) == 0 {
		return nil
	}

	result := make([]domain.BlacklistedRange, 0, len(seenCIDR))
	for cidr, source := range seenCIDR {
		result = append(result, domain.BlacklistedRange{
			CIDR:   cidr,
			Source: source,
		})
	}

	return result
}

// BackfillProxyIPHashes fills missing proxy IP hashes for legacy records.
func BackfillProxyIPHashes(ctx context.Context) (int64, error) {
	if DB == nil {
		return 0, errors.New("database not initialised")
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	var (
		updated int64
		batch   []domain.Proxy
	)

	result := db.
		Where("COALESCE(octet_length(ip_hash), 0) = 0").
		FindInBatches(&batch, maxParamsPerBatch, func(tx *gorm.DB, _ int) error {
			if len(batch) == 0 {
				return nil
			}

			for i := range batch {
				// AfterFind already decrypted IP; ensure hash exists for persistence.
				batch[i].GenerateHash()
			}

			for i := range batch {
				if len(batch[i].IPHash) == 0 {
					continue
				}
				if err := tx.Model(&domain.Proxy{}).
					Where("id = ?", batch[i].ID).
					Update("ip_hash", batch[i].IPHash).Error; err != nil {
					return err
				}
				updated++
			}

			// Clear slice for next batch
			batch = batch[:0]
			return nil
		})

	if result.Error != nil {
		return updated, result.Error
	}

	return updated, nil
}

// BackfillProxyIPMetadata fills missing IP hashes and IP ints for legacy rows.
func BackfillProxyIPMetadata(ctx context.Context) (int64, int64, error) {
	if DB == nil {
		return 0, 0, errors.New("database not initialised")
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	var (
		hashUpdated int64
		intUpdated  int64
		batch       []domain.Proxy
	)

	result := db.
		Where("COALESCE(octet_length(ip_hash), 0) = 0 OR ip_int = 0").
		FindInBatches(&batch, maxParamsPerBatch, func(tx *gorm.DB, _ int) error {
			if len(batch) == 0 {
				return nil
			}

			for i := range batch {
				batch[i].GenerateHash()
			}

			for i := range batch {
				if len(batch[i].IPHash) > 0 {
					if err := tx.Model(&domain.Proxy{}).
						Where("id = ?", batch[i].ID).
						Updates(map[string]any{
							"ip_hash": batch[i].IPHash,
							"ip_int":  batch[i].IPInt,
						}).Error; err != nil {
						return err
					}
					hashUpdated++
					if batch[i].IPInt != 0 {
						intUpdated++
					}
				}
			}

			batch = batch[:0]
			return nil
		})

	if result.Error != nil {
		return hashUpdated, intUpdated, result.Error
	}

	return hashUpdated, intUpdated, nil
}

// RemoveProxiesByIPs removes proxy/user associations for proxies whose IP is in the given list.
// It returns the number of user-proxy relations removed and any orphaned proxies that can be purged from queues.
func RemoveProxiesByIPs(ctx context.Context, ips []string) (int64, []domain.Proxy, error) {
	if DB == nil {
		return 0, nil, errors.New("database not initialised")
	}

	normalized := normalizeIPList(ips)
	if len(normalized) == 0 {
		return 0, nil, nil
	}

	hashes := make([][]byte, 0, len(normalized))
	for _, ip := range normalized {
		sum := sha256.Sum256([]byte(ip))
		hash := make([]byte, len(sum))
		copy(hash, sum[:])
		hashes = append(hashes, hash)
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	var proxies []domain.Proxy
	if err := db.Preload("Users").
		Where("ip_hash IN ?", hashes).
		Find(&proxies).Error; err != nil {
		return 0, nil, err
	}

	if len(proxies) == 0 {
		return 0, nil, nil
	}

	perUser := make(map[uint][]int)
	orphanSet := make(map[uint64]domain.Proxy)
	for _, proxy := range proxies {
		if len(proxy.Users) == 0 {
			orphanSet[proxy.ID] = proxy
			continue
		}
		for _, user := range proxy.Users {
			perUser[user.ID] = append(perUser[user.ID], int(proxy.ID))
		}
	}

	var (
		totalRemoved int64
	)

	for userID, proxyIDs := range perUser {
		removed, orphans, err := DeleteProxyRelation(userID, proxyIDs)
		if err != nil {
			return totalRemoved, nil, err
		}
		totalRemoved += removed
		for _, orphan := range orphans {
			orphanSet[orphan.ID] = orphan
		}
	}

	orphaned := make([]domain.Proxy, 0, len(orphanSet))
	for _, proxy := range orphanSet {
		orphaned = append(orphaned, proxy)
	}

	return totalRemoved, orphaned, nil
}

func cidrBounds(cidr string) (uint32, uint32, error) {
	_, start, end, err := normalizeCIDRWithBounds(cidr)
	return start, end, err
}

func normalizeCIDR(raw string) (string, uint32, uint32, error) {
	return normalizeCIDRWithBounds(raw)
}

func normalizeCIDRWithBounds(raw string) (string, uint32, uint32, error) {
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		return "", 0, 0, err
	}
	if !prefix.Addr().Is4() {
		return "", 0, 0, fmt.Errorf("non-ipv4 cidr: %s", raw)
	}
	prefix = prefix.Masked()
	start := ipToUint32(prefix.Addr())
	if prefix.Bits() == 32 {
		return prefix.String(), start, start, nil
	}
	size := uint32(1) << (32 - prefix.Bits())
	end := start + size - 1
	return prefix.String(), start, end, nil
}

func ipToUint32(addr netip.Addr) uint32 {
	ip := addr.As4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uint32ToIP(val uint32) net.IP {
	return net.IPv4(
		byte(val>>24),
		byte(val>>16),
		byte(val>>8),
		byte(val),
	).To4()
}

func mergeSpans(spans []cidrSpan) []cidrSpan {
	if len(spans) == 0 {
		return spans
	}

	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start == spans[j].start {
			return spans[i].end < spans[j].end
		}
		return spans[i].start < spans[j].start
	})

	merged := make([]cidrSpan, 0, len(spans))

	for _, s := range spans {
		if len(merged) == 0 {
			merged = append(merged, s)
			continue
		}
		last := &merged[len(merged)-1]
		if s.start <= last.end+1 {
			if s.end > last.end {
				last.end = s.end
			}
			continue
		}
		merged = append(merged, s)
	}

	return merged
}

func rangeToCIDRs(start, end uint32) []string {
	var cidrs []string

	s := uint64(start)
	e := uint64(end)

	for s <= e {
		maxSize := uint64(bits.TrailingZeros32(uint32(s)))
		remaining := e - s + 1
		maxAllowed := uint64(bits.Len64(remaining) - 1)

		hostBits := maxAllowed
		if maxSize < hostBits {
			hostBits = maxSize
		}

		prefixLen := 32 - hostBits

		cidr := fmt.Sprintf("%s/%d", uint32ToIP(uint32(s)).String(), prefixLen)
		cidrs = append(cidrs, cidr)

		s += 1 << hostBits
	}

	return cidrs
}

func normalizeIPList(ips []string) []string {
	if len(ips) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(ips))
	out := make([]string, 0, len(ips))

	for _, raw := range ips {
		ip := normalizeIPv4(raw)
		if ip == "" {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}

	return out
}

func normalizeIPv4(raw string) string {
	parsed := net.ParseIP(raw)
	if parsed == nil {
		return ""
	}
	ipv4 := parsed.To4()
	if ipv4 == nil {
		return ""
	}
	return ipv4.String()
}

// RemoveProxiesByRanges removes proxies whose IP falls inside any of the provided ranges.
func RemoveProxiesByRanges(ctx context.Context, ranges []domain.BlacklistedRange) (int64, []domain.Proxy, error) {
	if DB == nil {
		return 0, nil, errors.New("database not initialised")
	}
	if len(ranges) == 0 {
		return 0, nil, nil
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	type span struct {
		start uint32
		end   uint32
	}
	spans := make([]span, 0, len(ranges))
	for _, r := range ranges {
		start := r.StartIP
		end := r.EndIP
		if start > end {
			start, end = end, start
		}
		spans = append(spans, span{start: start, end: end})
	}

	// Sort and merge to reduce queries
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	merged := make([]span, 0, len(spans))
	for _, s := range spans {
		if len(merged) == 0 {
			merged = append(merged, s)
			continue
		}
		last := &merged[len(merged)-1]
		if s.start <= last.end+1 {
			if s.end > last.end {
				last.end = s.end
			}
			continue
		}
		merged = append(merged, s)
	}

	var proxies []domain.Proxy
	for _, s := range merged {
		var batch []domain.Proxy
		if err := db.Preload("Users").
			Where("ip_int BETWEEN ? AND ?", s.start, s.end).
			Find(&batch).Error; err != nil {
			return 0, nil, err
		}
		proxies = append(proxies, batch...)
	}

	if len(proxies) == 0 {
		return 0, nil, nil
	}

	perUser := make(map[uint][]int)
	orphanSet := make(map[uint64]domain.Proxy)
	for _, proxy := range proxies {
		if len(proxy.Users) == 0 {
			orphanSet[proxy.ID] = proxy
			continue
		}
		for _, user := range proxy.Users {
			perUser[user.ID] = append(perUser[user.ID], int(proxy.ID))
		}
	}

	var totalRemoved int64
	for userID, proxyIDs := range perUser {
		removed, orphans, err := DeleteProxyRelation(userID, proxyIDs)
		if err != nil {
			return totalRemoved, nil, err
		}
		totalRemoved += removed
		for _, orphan := range orphans {
			orphanSet[orphan.ID] = orphan
		}
	}

	orphaned := make([]domain.Proxy, 0, len(orphanSet))
	for _, proxy := range orphanSet {
		orphaned = append(orphaned, proxy)
	}

	return totalRemoved, orphaned, nil
}
