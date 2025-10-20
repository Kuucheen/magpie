package database

import (
	"context"
	"fmt"

	"magpie/internal/api/dto"
	"magpie/internal/domain"

	"gorm.io/gorm"
)

const defaultProxySnapshotLimit = 96

// SaveProxySnapshots stores snapshots for alive and scraped proxies per user.
func SaveProxySnapshots(ctx context.Context) error {
	if DB == nil {
		return fmt.Errorf("database: connection was not configured")
	}

	tx := DB
	if ctx != nil {
		tx = tx.WithContext(ctx)
	}

	var userIDs []uint
	if err := tx.Model(&domain.User{}).Pluck("id", &userIDs).Error; err != nil {
		return fmt.Errorf("proxy snapshot: fetch user ids: %w", err)
	}

	if len(userIDs) == 0 {
		return nil
	}

	aliveCountByUser, err := aliveProxyCountByUser(tx, userIDs)
	if err != nil {
		return err
	}

	scrapedCountByUser, err := scrapedProxyCountByUser(tx, userIDs)
	if err != nil {
		return err
	}

	snapshots := make([]domain.ProxySnapshot, 0, len(userIDs)*2)
	for _, userID := range userIDs {
		snapshots = append(snapshots,
			domain.ProxySnapshot{
				UserID: userID,
				Metric: domain.ProxySnapshotMetricAlive,
				Count:  aliveCountByUser[userID],
			},
			domain.ProxySnapshot{
				UserID: userID,
				Metric: domain.ProxySnapshotMetricScraped,
				Count:  scrapedCountByUser[userID],
			},
		)
	}

	if len(snapshots) == 0 {
		return nil
	}

	if err := tx.Create(&snapshots).Error; err != nil {
		return fmt.Errorf("proxy snapshot: insert rows: %w", err)
	}

	return nil
}

// GetProxySnapshotEntries returns the most recent proxy snapshot entries for a user/metric combination.
func GetProxySnapshotEntries(userID uint, metric string, limit int) []dto.ProxySnapshotEntry {
	if DB == nil {
		return nil
	}

	if metric != domain.ProxySnapshotMetricAlive && metric != domain.ProxySnapshotMetricScraped {
		return nil
	}

	if limit <= 0 {
		limit = defaultProxySnapshotLimit
	}

	rows := make([]domain.ProxySnapshot, 0, limit)

	DB.Where("user_id = ? AND metric = ?", userID, metric).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows)

	if len(rows) == 0 {
		return nil
	}

	entries := make([]dto.ProxySnapshotEntry, len(rows))
	for index := range rows {
		row := rows[len(rows)-1-index]
		entries[index] = dto.ProxySnapshotEntry{
			Count:      row.Count,
			RecordedAt: row.CreatedAt,
		}
	}

	return entries
}

func aliveProxyCountByUser(tx *gorm.DB, userIDs []uint) (map[uint]int64, error) {
	latestStats := tx.Model(&domain.ProxyStatistic{}).
		Select("proxy_id, MAX(created_at) AS created_at").
		Group("proxy_id")

	var rows []struct {
		UserID     uint
		AliveCount int64
	}

	if err := tx.Table("user_proxies AS up").
		Select("up.user_id AS user_id, COUNT(DISTINCT up.proxy_id) AS alive_count").
		Joins("JOIN (?) AS latest_stats ON latest_stats.proxy_id = up.proxy_id", latestStats).
		Joins("JOIN proxy_statistics ps ON ps.proxy_id = up.proxy_id AND ps.created_at = latest_stats.created_at").
		Where("up.user_id IN ?", userIDs).
		Where("ps.alive = ?", true).
		Group("up.user_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("proxy snapshot: aggregate alive counts: %w", err)
	}

	counts := make(map[uint]int64, len(userIDs))
	for _, userID := range userIDs {
		counts[userID] = 0
	}

	for _, row := range rows {
		counts[row.UserID] = row.AliveCount
	}

	return counts, nil
}

func scrapedProxyCountByUser(tx *gorm.DB, userIDs []uint) (map[uint]int64, error) {
	var rows []struct {
		UserID       uint
		ScrapedCount int64
	}

	if err := tx.Table("user_proxies AS up").
		Select("up.user_id AS user_id, COUNT(DISTINCT ps.proxy_id) AS scraped_count").
		Joins("JOIN proxy_scrape_site ps ON ps.proxy_id = up.proxy_id").
		Where("up.user_id IN ?", userIDs).
		Group("up.user_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("proxy snapshot: aggregate scraped counts: %w", err)
	}

	counts := make(map[uint]int64, len(userIDs))
	for _, userID := range userIDs {
		counts[userID] = 0
	}

	for _, row := range rows {
		counts[row.UserID] = row.ScrapedCount
	}

	return counts, nil
}

// GetCurrentAliveProxyCount returns the latest alive proxy count for a user based on proxy statistics.
func GetCurrentAliveProxyCount(userID uint) int64 {
	if DB == nil {
		return 0
	}

	counts, err := aliveProxyCountByUser(DB, []uint{userID})
	if err != nil {
		return 0
	}

	return counts[userID]
}
