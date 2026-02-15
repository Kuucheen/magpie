package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"magpie/internal/api/dto"
	"magpie/internal/config"
	"magpie/internal/domain"
	"magpie/internal/security"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	batchThreshold             = 8191  // Use batches when exceeding this number of records
	maxParamsPerBatch          = 65534 // Conservative default (PostgreSQL's limit) - 1
	minBatchSize               = 100   // Minimum batch size to maintain efficiency
	deleteChunkSize            = 5000  // Keep large deletes under Postgres parameter limits
	autoRemoveCleanupBatchSize = 2000

	proxiesPerPage    = 40
	maxProxiesPerPage = 100
)

var ErrNoProxiesSelected = errors.New("no proxies selected for deletion")

func InsertAndGetProxiesWithUser(proxies []domain.Proxy, userIDs ...uint) ([]domain.Proxy, error) {
	inserted, err := insertAndAssociateProxies(proxies, userIDs)
	if err != nil || len(inserted) == 0 {
		return inserted, err
	}

	proxiesWithUsers, err := fetchProxiesWithUsers(DB, inserted)
	if err != nil {
		return nil, err
	}

	return proxiesWithUsers, nil
}

func insertAndAssociateProxies(proxies []domain.Proxy, userIDs []uint) ([]domain.Proxy, error) {
	if len(proxies) == 0 || len(userIDs) == 0 {
		return nil, nil
	}

	userIDs = normalizeUserIDs(userIDs)
	if len(userIDs) == 0 {
		return nil, nil
	}

	uniqueProxies := deduplicateProxies(proxies)
	if len(uniqueProxies) == 0 {
		return nil, nil
	}

	batchSize := calculateBatchSize(len(uniqueProxies))
	limitCfg := config.GetConfig().ProxyLimits

	tx := DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer transactionRollbackHandler(tx)

	perUserHashes := make(map[uint][]string, len(userIDs))
	allowedHashes := make(map[string]struct{})

	for _, userID := range userIDs {
		if err := lockUserForProxyLimit(tx, userID, limitCfg); err != nil {
			tx.Rollback()
			return nil, err
		}

		hashes, err := filterHashesForUser(tx, uniqueProxies, userID, batchSize, limitCfg)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if len(hashes) == 0 {
			continue
		}
		perUserHashes[userID] = hashes
		for _, hash := range hashes {
			allowedHashes[hash] = struct{}{}
		}
	}

	if len(allowedHashes) == 0 {
		tx.Rollback()
		return nil, nil
	}

	uniqueProxies = filterProxiesByHash(uniqueProxies, allowedHashes)

	if err := insertProxies(tx, uniqueProxies, batchSize); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := ensureProxyIDs(tx, uniqueProxies); err != nil {
		tx.Rollback()
		return nil, err
	}

	hashToID := make(map[string]uint64, len(uniqueProxies))
	for i := range uniqueProxies {
		hashToID[string(uniqueProxies[i].Hash)] = uniqueProxies[i].ID
	}

	for _, userID := range userIDs {
		hashes := perUserHashes[userID]
		if len(hashes) == 0 {
			continue
		}

		proxyIDs := make([]uint64, 0, len(hashes))
		for _, hash := range hashes {
			if id, ok := hashToID[hash]; ok {
				proxyIDs = append(proxyIDs, id)
			}
		}
		if len(proxyIDs) == 0 {
			continue
		}

		if err := createUserAssociations(tx, proxyIDs, userID, batchSize); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return uniqueProxies, nil
}

// Helper functions
func normalizeUserIDs(userIDs []uint) []uint {
	if len(userIDs) == 0 {
		return nil
	}

	normalized := append([]uint(nil), userIDs...)
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i] < normalized[j]
	})

	out := normalized[:0]
	var previous uint
	hasPrevious := false
	for _, userID := range normalized {
		if hasPrevious && userID == previous {
			continue
		}
		out = append(out, userID)
		previous = userID
		hasPrevious = true
	}

	return out
}

func lockUserForProxyLimit(tx *gorm.DB, userID uint, limitCfg config.ProxyLimitConfig) error {
	if tx == nil || !limitCfg.Enabled {
		return nil
	}
	// PostgreSQL needs explicit row locks to serialize count-and-insert across
	// concurrent transactions for the same user. Other dialects are left as-is.
	if tx.Dialector.Name() != "postgres" {
		return nil
	}

	var userIDRow uint
	if err := tx.Model(&domain.User{}).
		Select("id").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", userID).
		Take(&userIDRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("proxy limit lock: user %d not found", userID)
		}
		return err
	}

	return nil
}

func deduplicateProxies(proxies []domain.Proxy) []domain.Proxy {
	seen := make(map[string]struct{}, len(proxies))
	unique := make([]domain.Proxy, 0, len(proxies))
	for _, p := range proxies {
		p.GenerateHash()
		key := string(p.Hash)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			unique = append(unique, p)
		}
	}
	return unique
}

func calculateBatchSize(proxyCount int) int {
	if proxyCount <= batchThreshold {
		return proxyCount
	}

	numFields, err := getNumDatabaseFields(domain.Proxy{}, DB)
	if err != nil || numFields == 0 {
		return minBatchSize // Fallback to safe minimum
	}

	batchSize := maxParamsPerBatch / numFields
	return clamp(batchSize, minBatchSize, proxyCount)
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func insertProxies(tx *gorm.DB, proxies []domain.Proxy, batchSize int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hash"}},
		DoUpdates: clause.AssignmentColumns([]string{"hash", "ip_hash", "ip_int"}), // To get the ids from duplicates
	}).CreateInBatches(proxies, batchSize).Error
}

func filterHashesForUser(tx *gorm.DB, proxies []domain.Proxy, userID uint, chunkSize int, limitCfg config.ProxyLimitConfig) ([]string, error) {
	if len(proxies) == 0 {
		return nil, nil
	}

	if !limitCfg.Enabled {
		return collectHashes(proxies), nil
	}

	if limitCfg.ExcludeAdmins {
		var role string
		if err := tx.Model(&domain.User{}).
			Select("role").
			Where("id = ?", userID).
			Scan(&role).Error; err != nil {
			return nil, err
		}
		if role == "admin" {
			return collectHashes(proxies), nil
		}
	}

	existingSet, err := getExistingHashesForUser(tx, userID, proxies, chunkSize)
	if err != nil {
		return nil, err
	}

	var currentCount int64
	if err := tx.Table("user_proxies").
		Where("user_id = ?", userID).
		Count(&currentCount).Error; err != nil {
		return nil, err
	}

	available := int64(limitCfg.MaxPerUser) - currentCount
	if available < 0 {
		available = 0
	}

	allowed := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		key := string(proxy.Hash)
		if _, ok := existingSet[key]; ok {
			allowed = append(allowed, key)
			continue
		}
		if available == 0 {
			continue
		}
		allowed = append(allowed, key)
		available--
	}

	return allowed, nil
}

func collectHashes(proxies []domain.Proxy) []string {
	if len(proxies) == 0 {
		return nil
	}

	hashes := make([]string, len(proxies))
	for i, proxy := range proxies {
		hashes[i] = string(proxy.Hash)
	}
	return hashes
}

func getExistingHashesForUser(tx *gorm.DB, userID uint, proxies []domain.Proxy, chunkSize int) (map[string]struct{}, error) {
	existing := make(map[string]struct{}, len(proxies))
	if len(proxies) == 0 {
		return existing, nil
	}

	if chunkSize <= 0 || chunkSize > maxParamsPerBatch {
		chunkSize = maxParamsPerBatch
		if len(proxies) < chunkSize {
			chunkSize = len(proxies)
		}
		if chunkSize == 0 {
			chunkSize = minBatchSize
		}
	}

	hashes := make([][]byte, len(proxies))
	for i, proxy := range proxies {
		hashes[i] = proxy.Hash
	}

	for i := 0; i < len(hashes); i += chunkSize {
		end := i + chunkSize
		if end > len(hashes) {
			end = len(hashes)
		}

		var rows [][]byte
		err := tx.Table("user_proxies up").
			Joins("JOIN proxies p ON up.proxy_id = p.id").
			Where("up.user_id = ? AND p.hash IN ?", userID, hashes[i:end]).
			Pluck("p.hash", &rows).Error
		if err != nil {
			return nil, err
		}

		for _, hash := range rows {
			existing[string(hash)] = struct{}{}
		}
	}

	return existing, nil
}

func filterProxiesByHash(proxies []domain.Proxy, allowed map[string]struct{}) []domain.Proxy {
	if len(allowed) == 0 {
		return nil
	}

	filtered := proxies[:0]
	for _, proxy := range proxies {
		if _, ok := allowed[string(proxy.Hash)]; ok {
			filtered = append(filtered, proxy)
		}
	}
	return filtered
}

func ensureProxyIDs(tx *gorm.DB, proxies []domain.Proxy) error {
	var missing [][]byte
	for _, proxy := range proxies {
		if proxy.ID == 0 {
			missing = append(missing, proxy.Hash)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	var results []struct {
		ID   uint64
		Hash []byte
	}
	if err := tx.Model(&domain.Proxy{}).
		Select("id, hash").
		Where("hash IN ?", missing).
		Find(&results).Error; err != nil {
		return err
	}

	lookup := make(map[string]uint64, len(results))
	for _, r := range results {
		lookup[string(r.Hash)] = r.ID
	}

	for i, proxy := range proxies {
		if proxy.ID != 0 {
			continue
		}
		if id, ok := lookup[string(proxy.Hash)]; ok {
			proxies[i].ID = id
		}
	}

	return nil
}

func createUserAssociations(tx *gorm.DB, proxyIDs []uint64, userID uint, batchSize int) error {
	if len(proxyIDs) == 0 {
		return nil
	}

	associations := make([]domain.UserProxy, len(proxyIDs))
	for i, id := range proxyIDs {
		associations[i] = domain.UserProxy{
			UserID:  userID,
			ProxyID: id,
		}
	}

	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "proxy_id"}},
		DoNothing: true,
	}).CreateInBatches(associations, batchSize).Error
}

func fetchProxiesWithUsers(tx *gorm.DB, proxies []domain.Proxy) ([]domain.Proxy, error) {
	ids := make([]uint64, len(proxies))
	for i, p := range proxies {
		ids[i] = p.ID
	}

	var results []domain.Proxy
	for i := 0; i < len(ids); i += maxParamsPerBatch {
		end := i + maxParamsPerBatch
		if end > len(ids) {
			end = len(ids)
		}

		var batch []domain.Proxy
		err := tx.Preload("Users").
			Where("id IN ?", ids[i:end]).
			Find(&batch).Error
		if err != nil {
			return nil, err
		}
		results = append(results, batch...)
	}
	return results, nil
}

func transactionRollbackHandler(tx *gorm.DB) {
	if r := recover(); r != nil {
		tx.Rollback()
		log.Errorf("Transaction rolled back due to panic: %v", r)
	}
}

func getNumDatabaseFields(model interface{}, db *gorm.DB) (int, error) {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return 0, err
	}
	return len(stmt.Schema.DBNames), nil
}

func GetAllProxyCountOfUser(userId uint) int64 {
	var count int64
	DB.Model(&domain.Proxy{}).
		Joins("JOIN user_proxies up ON up.proxy_id = proxies.id AND up.user_id = ?", userId).
		Count(&count)
	return count
}

func GetAllProxies() ([]domain.Proxy, error) {
	var allProxies []domain.Proxy
	const batchSize = maxParamsPerBatch

	collectedProxies := make([]domain.Proxy, 0)

	err := DB.
		Model(&domain.Proxy{}).
		Distinct("proxies.*").
		Joins("JOIN user_proxies up ON up.proxy_id = proxies.id").
		Preload("Users").
		Order("proxies.id").
		FindInBatches(&allProxies, batchSize, func(tx *gorm.DB, batch int) error {
			collectedProxies = append(collectedProxies, allProxies...)
			return nil
		})

	if err.Error != nil {
		return nil, err.Error
	}

	return collectedProxies, nil
}

func GetProxyInfoPage(userId uint, page int) []dto.ProxyInfo {
	proxies, _ := GetProxyInfoPageWithFilters(userId, page, proxiesPerPage, "", dto.ProxyListFilters{})
	return proxies
}

func GetProxyInfoPageWithFilters(userId uint, page int, pageSize int, search string, filters dto.ProxyListFilters) ([]dto.ProxyInfo, int64) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > maxProxiesPerPage {
		pageSize = proxiesPerPage
	}

	subQuery := DB.Model(&domain.ProxyStatistic{}).
		Select("DISTINCT ON (proxy_id) *").
		Order("proxy_id, created_at DESC, id DESC")

	query := DB.Model(&domain.Proxy{}).
		Select(
			"proxies.id AS id, "+
				"proxies.ip AS ip_encrypted, "+
				"proxies.port AS port, "+
				"COALESCE(NULLIF(proxies.estimated_type, ''), 'N/A') AS estimated_type, "+
				"COALESCE(ps.response_time, 0) AS response_time, "+
				"COALESCE(NULLIF(proxies.country, ''), 'N/A') AS country, "+
				"COALESCE(al.name, 'N/A') AS anonymity_level, "+
				"COALESCE(pos.overall_alive, false) AS alive, "+
				"COALESCE(pos.last_checked_at, ps.created_at, '0001-01-01 00:00:00'::timestamp) AS latest_check",
		).
		Joins("JOIN user_proxies up ON up.proxy_id = proxies.id AND up.user_id = ?", userId).
		Joins("LEFT JOIN (?) AS ps ON ps.proxy_id = proxies.id", subQuery).
		Joins("LEFT JOIN proxy_overall_statuses pos ON pos.proxy_id = proxies.id").
		Joins("LEFT JOIN anonymity_levels al ON al.id = ps.level_id").
		Order("alive DESC, latest_check DESC")

	filterQuery := buildProxyListFilterQuery(userId, filters)
	if filterQuery != nil {
		query = query.Where("proxies.id IN (?)", filterQuery)
	}

	rows := make([]dto.ProxyInfoRow, 0)
	normalizedSearch := strings.TrimSpace(search)
	lowerSearch := strings.ToLower(normalizedSearch)

	if normalizedSearch == "" {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
		if err := query.Scan(&rows).Error; err != nil {
			return []dto.ProxyInfo{}, 0
		}

		proxies := proxyInfoRowsToDTO(rows)
		attachReputationsToProxyInfos(proxies)
		if filterQuery == nil {
			total := GetAllProxyCountOfUser(userId)
			return proxies, total
		}

		var total int64
		if err := filterQuery.Distinct("proxies.id").Count(&total).Error; err != nil {
			return proxies, 0
		}
		return proxies, total
	}

	// Proxy IPs are stored encrypted, so the search needs to run after decrypting
	// the values that came back from the database. We therefore filter in-memory
	// once the full result set for the user has been loaded.
	if err := query.Scan(&rows).Error; err != nil {
		return []dto.ProxyInfo{}, 0
	}

	proxies := proxyInfoRowsToDTO(rows)
	attachReputationsToProxyInfos(proxies)
	filtered := filterProxiesBySearch(proxies, lowerSearch)
	total := int64(len(filtered))
	start := (page - 1) * pageSize
	if start >= len(filtered) {
		return []dto.ProxyInfo{}, total
	}

	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	pageSlice := filtered[start:end]
	return pageSlice, total
}

func buildProxyListFilterQuery(userId uint, filters dto.ProxyListFilters) *gorm.DB {
	if !hasProxyListFilters(filters) {
		return nil
	}

	query := DB.Model(&domain.Proxy{}).
		Select("proxies.id").
		Joins("JOIN user_proxies up ON up.proxy_id = proxies.id AND up.user_id = ?", userId)

	if filters.Status == "alive" || filters.Status == "dead" {
		query = query.Joins("LEFT JOIN proxy_overall_statuses pos ON pos.proxy_id = proxies.id")
		if filters.Status == "alive" {
			query = query.Where("COALESCE(pos.overall_alive, false) = ?", true)
		} else {
			query = query.Where("COALESCE(pos.overall_alive, false) = ?", false)
		}
	}

	if len(filters.Countries) > 0 {
		query = query.Where("COALESCE(NULLIF(LOWER(proxies.country), ''), 'n/a') IN ?", filters.Countries)
	}

	if len(filters.Types) > 0 {
		query = query.Where("COALESCE(NULLIF(LOWER(proxies.estimated_type), ''), 'n/a') IN ?", filters.Types)
	}

	needsLatestStats := len(filters.AnonymityLevels) > 0 || filters.MaxTimeout > 0 || filters.MaxRetries > 0
	if needsLatestStats {
		latestStats := DB.Model(&domain.ProxyStatistic{}).
			Select("DISTINCT ON (proxy_id) proxy_id, level_id, response_time, attempt").
			Order("proxy_id, created_at DESC, id DESC")
		query = query.Joins("JOIN (?) AS ps ON ps.proxy_id = proxies.id", latestStats)

		if len(filters.AnonymityLevels) > 0 {
			query = query.Joins("LEFT JOIN anonymity_levels al ON al.id = ps.level_id").
				Where("COALESCE(LOWER(al.name), 'n/a') IN ?", filters.AnonymityLevels)
		}

		if filters.MaxTimeout > 0 {
			query = query.Where("ps.response_time <= ?", filters.MaxTimeout)
		}

		if filters.MaxRetries > 0 {
			query = query.Where("ps.attempt <= ?", filters.MaxRetries)
		}
	}

	if len(filters.Protocols) > 0 {
		protocolStats := DB.Model(&domain.ProxyStatistic{}).
			Select("DISTINCT ON (proxy_id, protocol_id) proxy_id, protocol_id, alive").
			Order("proxy_id, protocol_id, created_at DESC, id DESC")
		query = query.Joins("JOIN (?) AS ps_proto ON ps_proto.proxy_id = proxies.id", protocolStats).
			Joins("JOIN protocols proto ON proto.id = ps_proto.protocol_id").
			Where("ps_proto.alive = ? AND LOWER(proto.name) IN ?", true, filters.Protocols)
	}

	if len(filters.ReputationLabels) > 0 {
		query = applyListReputationFilters(query, filters.ReputationLabels, filters.Protocols)
	}

	return query
}

func hasProxyListFilters(filters dto.ProxyListFilters) bool {
	if filters.Status == "alive" || filters.Status == "dead" {
		return true
	}
	if filters.MaxTimeout > 0 || filters.MaxRetries > 0 {
		return true
	}
	if len(filters.Protocols) > 0 || len(filters.Countries) > 0 || len(filters.Types) > 0 || len(filters.AnonymityLevels) > 0 || len(filters.ReputationLabels) > 0 {
		return true
	}
	return false
}

func GetProxyFilterOptions(userId uint) (dto.ProxyFilterOptions, error) {
	if DB == nil {
		return dto.ProxyFilterOptions{}, fmt.Errorf("database connection was not initialised")
	}

	countries, err := loadDistinctProxyInfoValue(userId, "proxies.country")
	if err != nil {
		return dto.ProxyFilterOptions{}, err
	}

	types, err := loadDistinctProxyInfoValue(userId, "proxies.estimated_type")
	if err != nil {
		return dto.ProxyFilterOptions{}, err
	}

	anonymityLevels, err := loadDistinctAnonymityLevels(userId)
	if err != nil {
		return dto.ProxyFilterOptions{}, err
	}

	return dto.ProxyFilterOptions{
		Countries:       countries,
		Types:           types,
		AnonymityLevels: anonymityLevels,
	}, nil
}

type proxyFilterValueRow struct {
	Value string `gorm:"column:value"`
}

func loadDistinctProxyInfoValue(userId uint, column string) ([]string, error) {
	var rows []proxyFilterValueRow
	if err := DB.Model(&domain.Proxy{}).
		Select("DISTINCT COALESCE(NULLIF("+column+", ''), 'N/A') AS value").
		Joins("JOIN user_proxies up ON up.proxy_id = proxies.id AND up.user_id = ?", userId).
		Order("value").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	return extractProxyFilterValues(rows), nil
}

func loadDistinctAnonymityLevels(userId uint) ([]string, error) {
	var rows []proxyFilterValueRow
	latestStats := DB.Model(&domain.ProxyStatistic{}).
		Select("DISTINCT ON (proxy_id) proxy_id, level_id").
		Order("proxy_id, created_at DESC, id DESC")

	if err := DB.Model(&domain.Proxy{}).
		Select("DISTINCT COALESCE(al.name, 'N/A') AS value").
		Joins("JOIN user_proxies up ON up.proxy_id = proxies.id AND up.user_id = ?", userId).
		Joins("LEFT JOIN (?) AS ps ON ps.proxy_id = proxies.id", latestStats).
		Joins("LEFT JOIN anonymity_levels al ON al.id = ps.level_id").
		Order("value").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	return extractProxyFilterValues(rows), nil
}

func extractProxyFilterValues(rows []proxyFilterValueRow) []string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Value) == "" {
			continue
		}
		values = append(values, row.Value)
	}
	return values
}

func proxyInfoRowsToDTO(rows []dto.ProxyInfoRow) []dto.ProxyInfo {
	results := make([]dto.ProxyInfo, 0, len(rows))
	for _, row := range rows {
		ip, _, err := security.DecryptProxySecret(row.IPEncrypted)
		if err != nil {
			log.Errorf("decrypt proxy ip: %v", err)
			ip = ""
		}

		results = append(results, dto.ProxyInfo{
			Id:             row.Id,
			IP:             ip,
			Port:           row.Port,
			EstimatedType:  row.EstimatedType,
			ResponseTime:   row.ResponseTime,
			Country:        row.Country,
			AnonymityLevel: row.AnonymityLevel,
			Alive:          row.Alive,
			LatestCheck:    row.LatestCheck,
		})
	}

	return results
}

func filterProxiesBySearch(proxies []dto.ProxyInfo, search string) []dto.ProxyInfo {
	if search == "" {
		return proxies
	}

	filtered := make([]dto.ProxyInfo, 0, len(proxies))
	for _, proxy := range proxies {
		if proxyMatchesSearch(proxy, search) {
			filtered = append(filtered, proxy)
		}
	}

	return filtered
}

func proxyMatchesSearch(proxy dto.ProxyInfo, search string) bool {
	lowerSearch := strings.ToLower(strings.TrimSpace(search))
	if lowerSearch == "" {
		return true
	}

	ipLower := strings.ToLower(proxy.IP)
	if strings.Contains(ipLower, lowerSearch) || strings.Contains(lowerSearch, ipLower) {
		return true
	}

	portStr := strconv.FormatUint(uint64(proxy.Port), 10)
	if strings.Contains(portStr, lowerSearch) || strings.Contains(lowerSearch, portStr) {
		return true
	}

	fields := []string{
		strings.ToLower(proxy.EstimatedType),
		strings.ToLower(proxy.Country),
		strings.ToLower(proxy.AnonymityLevel),
	}

	for _, field := range fields {
		if field == "" {
			continue
		}
		if strings.Contains(field, lowerSearch) || strings.Contains(lowerSearch, field) {
			return true
		}
	}

	responseStr := strconv.Itoa(int(proxy.ResponseTime))
	if strings.Contains(responseStr, lowerSearch) || strings.Contains(lowerSearch, responseStr) {
		return true
	}

	aliveLabel := "dead"
	if proxy.Alive {
		aliveLabel = "alive"
	}
	if strings.Contains(aliveLabel, lowerSearch) || strings.Contains(lowerSearch, aliveLabel) {
		return true
	}

	if !proxy.LatestCheck.IsZero() {
		timestamp := strings.ToLower(proxy.LatestCheck.Format(time.RFC3339))
		if strings.Contains(timestamp, lowerSearch) || strings.Contains(lowerSearch, timestamp) {
			return true
		}
	}

	if proxy.Reputation != nil {
		if proxy.Reputation.Overall != nil {
			label := strings.ToLower(strings.TrimSpace(proxy.Reputation.Overall.Label))
			if label != "" && (strings.Contains(label, lowerSearch) || strings.Contains(lowerSearch, label)) {
				return true
			}

			score := strings.TrimSpace(strconv.FormatFloat(float64(proxy.Reputation.Overall.Score), 'f', -1, 32))
			if score != "" && (strings.Contains(score, lowerSearch) || strings.Contains(lowerSearch, score)) {
				return true
			}
		}

		for protocol, rep := range proxy.Reputation.Protocols {
			protocolLower := strings.ToLower(strings.TrimSpace(protocol))
			if protocolLower != "" && (strings.Contains(protocolLower, lowerSearch) || strings.Contains(lowerSearch, protocolLower)) {
				return true
			}

			label := strings.ToLower(strings.TrimSpace(rep.Label))
			if label != "" && (strings.Contains(label, lowerSearch) || strings.Contains(lowerSearch, label)) {
				return true
			}

			score := strings.TrimSpace(strconv.FormatFloat(float64(rep.Score), 'f', -1, 32))
			if score != "" && (strings.Contains(score, lowerSearch) || strings.Contains(lowerSearch, score)) {
				return true
			}
		}
	}

	return false
}

func attachReputationsToProxyInfos(proxies []dto.ProxyInfo) {
	if len(proxies) == 0 {
		return
	}

	proxyIDs := make([]uint64, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy.Id <= 0 {
			continue
		}
		proxyIDs = append(proxyIDs, uint64(proxy.Id))
	}

	if len(proxyIDs) == 0 {
		return
	}

	repMap, err := GetProxyReputations(context.Background(), proxyIDs)
	if err != nil {
		log.Error("failed to load proxy reputations", "error", err)
		return
	}

	missing := make([]uint64, 0)
	dedupMissing := make(map[uint64]struct{}, len(proxies))

	for index := range proxies {
		id := uint64(proxies[index].Id)
		if rows, ok := repMap[id]; ok && len(rows) > 0 {
			proxies[index].Reputation = mapReputationsToSummary(rows)
			continue
		}

		if id == 0 {
			continue
		}

		if _, seen := dedupMissing[id]; !seen {
			dedupMissing[id] = struct{}{}
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		scheduleReputationRecalculation(missing)
	}
}

func mapReputationsToSummary(rows []domain.ProxyReputation) *dto.ProxyReputationSummary {
	if len(rows) == 0 {
		return nil
	}

	summary := &dto.ProxyReputationSummary{
		Protocols: make(map[string]dto.ProxyReputation),
	}

	for _, row := range rows {
		rep := dto.ProxyReputation{
			Kind:  row.Kind,
			Score: row.Score,
			Label: row.Label,
		}

		if row.Kind == domain.ProxyReputationKindOverall {
			overall := rep
			summary.Overall = &overall
		} else {
			summary.Protocols[row.Kind] = rep
		}
	}

	if len(summary.Protocols) == 0 {
		summary.Protocols = nil
	}

	return summary
}

func mapReputationsToBreakdown(rows []domain.ProxyReputation) *dto.ProxyReputationBreakdown {
	if len(rows) == 0 {
		return nil
	}

	breakdown := &dto.ProxyReputationBreakdown{
		Protocols: make(map[string]dto.ProxyReputationDetail),
	}

	for _, row := range rows {
		signals := decodeReputationSignals(row.Signals)
		rep := dto.ProxyReputationDetail{
			Kind:    row.Kind,
			Score:   row.Score,
			Label:   row.Label,
			Signals: signals,
		}

		if row.Kind == domain.ProxyReputationKindOverall {
			overall := rep
			breakdown.Overall = &overall
		} else {
			breakdown.Protocols[row.Kind] = rep
		}
	}

	if len(breakdown.Protocols) == 0 {
		breakdown.Protocols = nil
	}

	return breakdown
}

func decodeReputationSignals(payload []byte) map[string]any {
	if len(payload) == 0 {
		return nil
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		log.Error("failed to decode reputation signals", "error", err)
		return nil
	}

	return decoded
}

func scheduleReputationRecalculation(proxyIDs []uint64) {
	if len(proxyIDs) == 0 {
		return
	}

	ids := append([]uint64(nil), proxyIDs...)

	go func(values []uint64) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := RecalculateProxyReputations(ctx, values); err != nil {
			log.Error("failed to backfill proxy reputations", "error", err, "proxy_ids", values)
		}
	}(ids)
}

func GetProxyDetail(userId uint, proxyId uint64) (*dto.ProxyDetail, error) {
	if proxyId == 0 {
		return nil, nil
	}

	var proxy domain.Proxy
	err := DB.
		Preload("Statistics", func(db *gorm.DB) *gorm.DB {
			return db.
				Order("created_at DESC").
				Limit(1).
				Preload("Protocol").
				Preload("Level").
				Preload("Judge")
		}).
		Preload("Reputations").
		Joins("JOIN user_proxies up ON up.proxy_id = proxies.id").
		Where("up.user_id = ? AND proxies.id = ?", userId, proxyId).
		First(&proxy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var latestStat *dto.ProxyStatistic
	var latestCheck *time.Time
	if len(proxy.Statistics) > 0 {
		mapped := mapProxyStatistic(&proxy.Statistics[0])
		latestStat = &mapped
		latestCheck = &proxy.Statistics[0].CreatedAt
	}

	detail := &dto.ProxyDetail{
		Id:              int(proxy.ID),
		IP:              proxy.GetIp(),
		Port:            proxy.Port,
		Username:        proxy.Username,
		Password:        proxy.Password,
		HasAuth:         proxy.HasAuth(),
		EstimatedType:   normaliseDisplayValue(proxy.EstimatedType, "N/A"),
		Country:         normaliseDisplayValue(proxy.Country, "Unknown"),
		CreatedAt:       proxy.CreatedAt,
		LatestCheck:     latestCheck,
		LatestStatistic: latestStat,
	}

	detail.Reputation = mapReputationsToBreakdown(proxy.Reputations)

	return detail, nil
}

func GetProxyStatistics(userId uint, proxyId uint64, limit int) ([]dto.ProxyStatistic, error) {
	if proxyId == 0 {
		return []dto.ProxyStatistic{}, nil
	}

	if limit <= 0 || limit > 500 {
		limit = 500
	}

	query := DB.Model(&domain.ProxyStatistic{}).
		Preload("Protocol").
		Preload("Level").
		Preload("Judge").
		Joins("JOIN user_proxies up ON up.proxy_id = proxy_statistics.proxy_id").
		Where("proxy_statistics.proxy_id = ? AND up.user_id = ?", proxyId, userId).
		Order("proxy_statistics.created_at DESC").
		Limit(limit)

	rows := make([]domain.ProxyStatistic, 0, limit)
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}

	stats := make([]dto.ProxyStatistic, len(rows))
	for index := range rows {
		stats[index] = mapProxyStatistic(&rows[index])
	}

	return stats, nil
}

type proxyStatisticBodyRow struct {
	ResponseBody string
	Regex        sql.NullString
}

func GetProxyStatisticResponseBody(userId uint, proxyId uint64, statisticId uint64) (dto.ProxyStatisticDetail, error) {
	if proxyId == 0 || statisticId == 0 {
		return dto.ProxyStatisticDetail{}, gorm.ErrRecordNotFound
	}

	var row proxyStatisticBodyRow
	err := DB.Table("proxy_statistics").
		Select("proxy_statistics.response_body", "user_judges.regex").
		Joins("JOIN user_proxies up ON up.proxy_id = proxy_statistics.proxy_id").
		Joins("LEFT JOIN user_judges ON user_judges.judge_id = proxy_statistics.judge_id AND user_judges.user_id = up.user_id").
		Where("proxy_statistics.id = ? AND proxy_statistics.proxy_id = ? AND up.user_id = ?", statisticId, proxyId, userId).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.ProxyStatisticDetail{}, gorm.ErrRecordNotFound
		}
		return dto.ProxyStatisticDetail{}, err
	}

	regex := ""
	if row.Regex.Valid {
		regex = strings.TrimSpace(row.Regex.String)
	}

	return dto.ProxyStatisticDetail{
		ResponseBody: row.ResponseBody,
		Regex:        regex,
	}, nil
}

func mapProxyStatistic(stat *domain.ProxyStatistic) dto.ProxyStatistic {
	if stat == nil {
		return dto.ProxyStatistic{}
	}

	protocol := normaliseDisplayValue(stat.Protocol.Name, "Unknown")
	anonymity := normaliseDisplayValue(stat.Level.Name, "Unknown")
	judge := normaliseDisplayValue(stat.Judge.FullString, "Unknown")

	return dto.ProxyStatistic{
		Id:             stat.ID,
		Alive:          stat.Alive,
		Attempt:        stat.Attempt,
		ResponseTime:   stat.ResponseTime,
		ResponseBody:   stat.ResponseBody,
		Protocol:       protocol,
		AnonymityLevel: anonymity,
		Judge:          judge,
		CreatedAt:      stat.CreatedAt,
	}
}

func normaliseDisplayValue(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func DeleteProxyRelation(userId uint, proxies []int) (int64, []domain.Proxy, error) {
	if len(proxies) == 0 {
		return 0, nil, nil
	}

	var totalDeleted int64
	chunkSize := deleteChunkSize
	if chunkSize > len(proxies) {
		chunkSize = len(proxies)
	}
	if chunkSize <= 0 {
		chunkSize = len(proxies)
	}

	orphanSet := make(map[uint64]struct{})

	for start := 0; start < len(proxies); start += chunkSize {
		end := start + chunkSize
		if end > len(proxies) {
			end = len(proxies)
		}

		chunk := proxies[start:end]
		result := DB.
			Where("user_id = ?", userId).
			Where("proxy_id IN ?", chunk).
			Delete(&domain.UserProxy{})

		if result.Error != nil {
			return totalDeleted, nil, result.Error
		}

		totalDeleted += result.RowsAffected

		orphanIDs, err := collectOrphanProxyIDs(chunk)
		if err != nil {
			return totalDeleted, nil, err
		}
		if len(orphanIDs) == 0 {
			continue
		}

		for _, id := range orphanIDs {
			orphanSet[id] = struct{}{}
		}
	}

	if len(orphanSet) == 0 {
		return totalDeleted, nil, nil
	}

	uniqueIDs := make([]uint64, 0, len(orphanSet))
	for id := range orphanSet {
		uniqueIDs = append(uniqueIDs, id)
	}

	var orphans []domain.Proxy
	if err := DB.Where("id IN ?", uniqueIDs).Find(&orphans).Error; err != nil {
		return totalDeleted, nil, err
	}

	return totalDeleted, orphans, nil
}

func CleanupAutoRemovalViolations(ctx context.Context) (int64, []domain.Proxy, error) {
	if DB == nil {
		return 0, nil, fmt.Errorf("database not initialised")
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	type target struct {
		UserID  uint
		ProxyID uint64
	}

	var (
		batch     []target
		total     int64
		orphaned  = make(map[uint64]domain.Proxy)
		queryBase = db.Table("user_proxies up").
				Select("up.user_id, up.proxy_id").
				Joins("JOIN users u ON u.id = up.user_id").
				Where("u.auto_remove_failing_proxies = ?", true).
				Where("u.auto_remove_failure_threshold > 0").
				Where("up.consecutive_failures >= u.auto_remove_failure_threshold")
	)

	result := queryBase.FindInBatches(&batch, autoRemoveCleanupBatchSize, func(tx *gorm.DB, _ int) error {
		if len(batch) == 0 {
			return nil
		}

		perUser := make(map[uint][]int)
		for _, item := range batch {
			perUser[item.UserID] = append(perUser[item.UserID], int(item.ProxyID))
		}

		for userID, proxyIDs := range perUser {
			removed, orphanList, err := DeleteProxyRelation(userID, proxyIDs)
			if err != nil {
				return err
			}
			total += removed
			for _, proxy := range orphanList {
				orphaned[proxy.ID] = proxy
			}
		}

		return nil
	})
	if result.Error != nil {
		return 0, nil, result.Error
	}

	orphanList := make([]domain.Proxy, 0, len(orphaned))
	for _, proxy := range orphaned {
		orphanList = append(orphanList, proxy)
	}

	return total, orphanList, nil
}

func CleanupProxyLimitViolations(ctx context.Context) (int64, []domain.Proxy, error) {
	limitCfg := config.GetConfig().ProxyLimits
	return cleanupProxyLimitViolationsWithConfig(ctx, limitCfg)
}

func cleanupProxyLimitViolationsWithConfig(ctx context.Context, limitCfg config.ProxyLimitConfig) (int64, []domain.Proxy, error) {
	if !limitCfg.Enabled {
		return 0, nil, nil
	}

	if DB == nil {
		return 0, nil, fmt.Errorf("database not initialised")
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	maxPerUser := int64(limitCfg.MaxPerUser)

	query := db.Table("user_proxies up").
		Select("up.user_id").
		Group("up.user_id").
		Having("COUNT(*) > ?", maxPerUser)
	if limitCfg.ExcludeAdmins {
		query = query.Joins("JOIN users u ON u.id = up.user_id").
			Where("u.role <> ?", "admin")
	}

	var userIDs []uint
	if err := query.Pluck("up.user_id", &userIDs).Error; err != nil {
		return 0, nil, err
	}
	userIDs = normalizeUserIDs(userIDs)
	if len(userIDs) == 0 {
		return 0, nil, nil
	}

	totalRemoved := int64(0)
	orphaned := make(map[uint64]domain.Proxy)

	for _, userID := range userIDs {
		var currentCount int64
		if err := db.Table("user_proxies").
			Where("user_id = ?", userID).
			Count(&currentCount).Error; err != nil {
			return 0, nil, err
		}

		overflow := currentCount - maxPerUser
		if overflow <= 0 {
			continue
		}

		var toRemove []uint64
		if err := db.Table("user_proxies").
			Select("proxy_id").
			Where("user_id = ?", userID).
			Order("created_at DESC, proxy_id DESC").
			Limit(int(overflow)).
			Pluck("proxy_id", &toRemove).Error; err != nil {
			return 0, nil, err
		}
		if len(toRemove) == 0 {
			continue
		}

		proxyIDs := make([]int, 0, len(toRemove))
		for _, proxyID := range toRemove {
			proxyIDs = append(proxyIDs, int(proxyID))
		}

		removed, orphanList, err := DeleteProxyRelation(userID, proxyIDs)
		if err != nil {
			return 0, nil, err
		}
		totalRemoved += removed
		for _, proxy := range orphanList {
			orphaned[proxy.ID] = proxy
		}
	}

	orphanList := make([]domain.Proxy, 0, len(orphaned))
	for _, proxy := range orphaned {
		orphanList = append(orphanList, proxy)
	}

	return totalRemoved, orphanList, nil
}

func collectOrphanProxyIDs(candidateIDs []int) ([]uint64, error) {
	if len(candidateIDs) == 0 {
		return nil, nil
	}

	var stillInUse []int
	if err := DB.Model(&domain.UserProxy{}).
		Where("proxy_id IN ?", candidateIDs).
		Distinct("proxy_id").
		Pluck("proxy_id", &stillInUse).Error; err != nil {
		return nil, err
	}

	inUseSet := make(map[int]struct{}, len(stillInUse))
	for _, id := range stillInUse {
		inUseSet[id] = struct{}{}
	}

	seen := make(map[int]struct{}, len(candidateIDs))
	orphanIDs := make([]uint64, 0, len(candidateIDs))
	for _, id := range candidateIDs {
		if _, alreadySeen := seen[id]; alreadySeen {
			continue
		}
		seen[id] = struct{}{}

		if _, inUse := inUseSet[id]; inUse {
			continue
		}

		orphanIDs = append(orphanIDs, uint64(id))
	}

	if len(orphanIDs) == 0 {
		return nil, nil
	}

	return orphanIDs, nil
}

func GetExistingProxyIDSet(ctx context.Context, proxyIDs []uint64) (map[uint64]struct{}, error) {
	if len(proxyIDs) == 0 {
		return nil, nil
	}
	if DB == nil {
		return nil, fmt.Errorf("database not initialised")
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	result := make(map[uint64]struct{})
	for start := 0; start < len(proxyIDs); start += maxParamsPerBatch {
		end := start + maxParamsPerBatch
		if end > len(proxyIDs) {
			end = len(proxyIDs)
		}

		var ids []uint64
		if err := db.Model(&domain.Proxy{}).
			Where("id IN ?", proxyIDs[start:end]).
			Pluck("id", &ids).Error; err != nil {
			return nil, err
		}

		for _, id := range ids {
			result[id] = struct{}{}
		}
	}

	return result, nil
}

func DeleteOrphanProxies(ctx context.Context) (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("database not initialised")
	}
	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	result := db.
		Where("NOT EXISTS (SELECT 1 FROM user_proxies up WHERE up.proxy_id = proxies.id)").
		Delete(&domain.Proxy{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func DeleteProxiesWithSettings(userID uint, settings dto.DeleteSettings) (int64, []domain.Proxy, error) {
	if settings.Scope == "selected" && len(settings.Proxies) == 0 {
		return 0, nil, ErrNoProxiesSelected
	}

	proxyIDs, err := collectProxyIDsForDeletion(userID, settings)
	if err != nil {
		return 0, nil, err
	}

	if len(proxyIDs) == 0 {
		return 0, nil, nil
	}

	intIDs := make([]int, 0, len(proxyIDs))
	for _, id := range proxyIDs {
		intIDs = append(intIDs, int(id))
	}

	return DeleteProxyRelation(userID, intIDs)
}

func GetProxiesForExport(userID uint, settings dto.ExportSettings) ([]domain.Proxy, error) {
	if DB == nil {
		return nil, fmt.Errorf("database connection was not initialised")
	}

	tx := DB.Begin(&sql.TxOptions{
		ReadOnly:  true,
		Isolation: sql.LevelRepeatableRead,
	})
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if err := tx.Rollback().Error; err != nil && !errors.Is(err, gorm.ErrInvalidTransaction) {
			log.Error("failed to rollback export transaction", "error", err)
		}
	}()

	var proxies []domain.Proxy

	baseQuery := tx.Preload("Statistics", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC, id DESC")
	}).Preload("Statistics.Protocol").
		Preload("Reputations").
		Joins("JOIN user_proxies ON user_proxies.proxy_id = proxies.id").
		Where("user_proxies.user_id = ?", userID)

	baseQuery = applyExportReputationFilters(baseQuery, settings)

	if settings.ProxyStatus == "alive" || settings.ProxyStatus == "dead" {
		isAlive := settings.ProxyStatus == "alive"
		baseQuery = baseQuery.Joins("JOIN proxy_overall_statuses pos ON pos.proxy_id = proxies.id").
			Where("pos.overall_alive = ?", isAlive)
	}

	if len(settings.Proxies) > 0 {
		baseQuery = baseQuery.Where("proxies.id IN ?", settings.Proxies)
	}

	var err error
	if settings.Filter {
		proxies, err = applyAdditionalFilters(baseQuery, settings)
	} else {
		err = baseQuery.Find(&proxies).Error
	}
	if err != nil {
		return nil, err
	}

	filtered := filterProxiesForExport(proxies, settings)

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return filtered, nil
}

// applyAdditionalFilters applies additional filters based on settings
func applyAdditionalFilters(query *gorm.DB, settings dto.ExportSettings) ([]domain.Proxy, error) {
	var proxies []domain.Proxy

	// If any of the filters require proxy_statistics, join it once.
	needsProxyStatistics := settings.Http || settings.Https || settings.Socks4 || settings.Socks5 || settings.MaxTimeout > 0 || settings.MaxRetries > 0
	if needsProxyStatistics {
		query = query.Joins("JOIN proxy_statistics ON proxies.id = proxy_statistics.proxy_id")
	}

	// Apply protocol filters using the protocols join if any protocols are selected.
	if settings.Http || settings.Https || settings.Socks4 || settings.Socks5 {
		var protocols []string
		if settings.Http {
			protocols = append(protocols, "http")
		}
		if settings.Https {
			protocols = append(protocols, "https")
		}
		if settings.Socks4 {
			protocols = append(protocols, "socks4")
		}
		if settings.Socks5 {
			protocols = append(protocols, "socks5")
		}
		// Add the join for protocols once.
		query = query.Joins("JOIN protocols ON proxy_statistics.protocol_id = protocols.id").
			Where("protocols.name IN ?", protocols)
	}

	// Apply response time filter.
	if settings.MaxTimeout > 0 {
		query = query.Where("proxy_statistics.response_time <= ?", settings.MaxTimeout)
	}

	// Apply retry count filter.
	if settings.MaxRetries > 0 {
		query = query.Where("proxy_statistics.attempt <= ?", settings.MaxRetries)
	}

	// Group the results to avoid duplicates.
	// In many cases, grouping by the primary key (proxies.id) is sufficient.
	// If you joined protocols and proxy_statistics then you may need to group by those IDs as well.
	groupBy := "proxies.id"
	if settings.Http || settings.Https || settings.Socks4 || settings.Socks5 {
		groupBy += ", protocols.id"
	}
	// Optionally include the proxy_statistics.id if needed to ensure uniqueness.
	groupBy += ", proxy_statistics.id"
	query = query.Group(groupBy)

	err := query.Find(&proxies).Error
	return proxies, err
}

func filterProxiesForExport(proxies []domain.Proxy, settings dto.ExportSettings) []domain.Proxy {
	if len(settings.ReputationLabels) == 0 {
		return proxies
	}

	allowedLabels, includeUnknown := normalizeReputationLabels(settings.ReputationLabels)
	selectedProtocols := protocolsForExport(settings)

	filtered := make([]domain.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if proxyMatchesReputationFilters(proxy, allowedLabels, includeUnknown, selectedProtocols) {
			filtered = append(filtered, proxy)
		}
	}

	return filtered
}

func normalizeReputationLabels(labels []string) (map[string]struct{}, bool) {
	allowed := make(map[string]struct{}, len(labels))
	includeUnknown := false

	for _, label := range labels {
		trimmed := strings.ToLower(strings.TrimSpace(label))
		if trimmed == "" {
			continue
		}
		if trimmed == "unknown" {
			includeUnknown = true
			continue
		}
		allowed[trimmed] = struct{}{}
	}

	return allowed, includeUnknown
}

func protocolsForExport(settings dto.ExportSettings) []string {
	if !settings.Filter {
		return nil
	}

	protocols := make([]string, 0, 4)
	if settings.Http {
		protocols = append(protocols, "http")
	}
	if settings.Https {
		protocols = append(protocols, "https")
	}
	if settings.Socks4 {
		protocols = append(protocols, "socks4")
	}
	if settings.Socks5 {
		protocols = append(protocols, "socks5")
	}

	return protocols
}

func targetReputationKinds(settings dto.ExportSettings) []string {
	protocols := protocolsForExport(settings)
	if len(protocols) > 0 {
		out := make([]string, 0, len(protocols))
		for _, proto := range protocols {
			if trimmed := strings.ToLower(strings.TrimSpace(proto)); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		if len(out) > 0 {
			return out
		}
	}

	return []string{domain.ProxyReputationKindOverall}
}

func applyExportReputationFilters(query *gorm.DB, settings dto.ExportSettings) *gorm.DB {
	allowedSet, includeUnknown := normalizeReputationLabels(settings.ReputationLabels)
	if len(allowedSet) == 0 && !includeUnknown {
		return query
	}

	targetKinds := targetReputationKinds(settings)
	if len(targetKinds) == 0 {
		return query
	}

	keys := setToSlice(allowedSet)
	labelExpr := "LOWER(COALESCE(NULLIF(pr.label, ''), 'unknown'))"

	if includeUnknown {
		query = query.Joins("LEFT JOIN proxy_reputations pr ON pr.proxy_id = proxies.id AND LOWER(pr.kind) IN ?", targetKinds)
		if len(keys) > 0 {
			query = query.Where(labelExpr+" IN ? OR pr.id IS NULL OR "+labelExpr+" = 'unknown'", keys)
		} else {
			query = query.Where("pr.id IS NULL OR " + labelExpr + " = 'unknown'")
		}
	} else {
		query = query.Joins("JOIN proxy_reputations pr ON pr.proxy_id = proxies.id AND LOWER(pr.kind) IN ?", targetKinds)
		if len(keys) > 0 {
			query = query.Where(labelExpr+" IN ?", keys)
		}
	}

	return query
}

func applyListReputationFilters(query *gorm.DB, labels []string, protocols []string) *gorm.DB {
	allowedSet, includeUnknown := normalizeReputationLabels(labels)
	if len(allowedSet) == 0 && !includeUnknown {
		return query
	}

	targetKinds := targetReputationKindsForProtocols(protocols)
	if len(targetKinds) == 0 {
		return query
	}

	keys := setToSlice(allowedSet)
	labelExpr := "LOWER(COALESCE(NULLIF(pr.label, ''), 'unknown'))"

	if includeUnknown {
		query = query.Joins("LEFT JOIN proxy_reputations pr ON pr.proxy_id = proxies.id AND LOWER(pr.kind) IN ?", targetKinds)
		if len(keys) > 0 {
			query = query.Where(labelExpr+" IN ? OR pr.id IS NULL OR "+labelExpr+" = 'unknown'", keys)
		} else {
			query = query.Where("pr.id IS NULL OR " + labelExpr + " = 'unknown'")
		}
	} else {
		query = query.Joins("JOIN proxy_reputations pr ON pr.proxy_id = proxies.id AND LOWER(pr.kind) IN ?", targetKinds)
		if len(keys) > 0 {
			query = query.Where(labelExpr+" IN ?", keys)
		}
	}

	return query
}

func targetReputationKindsForProtocols(protocols []string) []string {
	if len(protocols) == 0 {
		return []string{domain.ProxyReputationKindOverall}
	}

	out := make([]string, 0, len(protocols))
	for _, proto := range protocols {
		if trimmed := strings.ToLower(strings.TrimSpace(proto)); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	if len(out) == 0 {
		return []string{domain.ProxyReputationKindOverall}
	}

	return out
}

func setToSlice(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	return out
}

func proxyMatchesReputationFilters(proxy domain.Proxy, allowed map[string]struct{}, includeUnknown bool, selectedProtocols []string) bool {
	if len(allowed) == 0 && !includeUnknown {
		return true
	}

	reputations := make(map[string]domain.ProxyReputation, len(proxy.Reputations))
	for _, rep := range proxy.Reputations {
		reputations[strings.ToLower(rep.Kind)] = rep
	}

	targetProtocols := make([]string, 0, len(selectedProtocols))
	for _, proto := range selectedProtocols {
		if trimmed := strings.ToLower(strings.TrimSpace(proto)); trimmed != "" {
			targetProtocols = append(targetProtocols, trimmed)
		}
	}

	if len(targetProtocols) == 0 {
		if len(proxy.Statistics) > 0 {
			proto := strings.ToLower(strings.TrimSpace(proxy.Statistics[0].Protocol.Name))
			if proto != "" {
				targetProtocols = append(targetProtocols, proto)
			}
		}
		if len(targetProtocols) == 0 {
			targetProtocols = append(targetProtocols, domain.ProxyReputationKindOverall)
		}
	}

	for _, proto := range targetProtocols {
		rep, ok := reputations[proto]
		if ok {
			label := strings.ToLower(strings.TrimSpace(rep.Label))
			if label == "" {
				label = "unknown"
			}
			if _, match := allowed[label]; match {
				return true
			}
			if label == "unknown" && includeUnknown {
				return true
			}
			continue
		}

		if includeUnknown {
			return true
		}
	}

	return false
}
