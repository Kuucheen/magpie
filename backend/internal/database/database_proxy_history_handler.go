package database

import (
	"context"
	"fmt"

	"magpie/internal/api/dto"
	"magpie/internal/domain"
)

const (
	defaultProxyHistoryLimit = 24
	maxProxyHistoryLimit     = 720
)

// SaveProxyHistorySnapshot stores a snapshot of the proxy count for every user at the time of invocation.
// It records zero counts as well so we can track when users have no proxies.
func SaveProxyHistorySnapshot(ctx context.Context) error {
	if DB == nil {
		return fmt.Errorf("database: connection was not configured")
	}

	tx := DB
	if ctx != nil {
		tx = tx.WithContext(ctx)
	}

	var userIDs []uint
	if err := tx.Model(&domain.User{}).Pluck("id", &userIDs).Error; err != nil {
		return fmt.Errorf("proxy history: fetch user ids: %w", err)
	}

	if len(userIDs) == 0 {
		return nil
	}

	var counts []struct {
		UserID     uint
		ProxyCount int64
	}

	if err := tx.Table("user_proxies").
		Select("user_id, COUNT(*) AS proxy_count").
		Where("user_id IN ?", userIDs).
		Group("user_id").
		Scan(&counts).Error; err != nil {
		return fmt.Errorf("proxy history: aggregate proxy counts: %w", err)
	}

	countByUser := make(map[uint]int64, len(counts))
	for _, row := range counts {
		countByUser[row.UserID] = row.ProxyCount
	}

	histories := make([]domain.ProxyHistory, 0, len(userIDs))
	for _, userID := range userIDs {
		histories = append(histories, domain.ProxyHistory{
			UserID:     userID,
			ProxyCount: countByUser[userID],
		})
	}

	if len(histories) == 0 {
		return nil
	}

	if err := tx.Create(&histories).Error; err != nil {
		return fmt.Errorf("proxy history: insert rows: %w", err)
	}

	return nil
}

// GetProxyHistoryEntries returns the most recent proxy history snapshots for a user, ordered chronologically.
func GetProxyHistoryEntries(userID uint, limit int) []dto.ProxyHistoryEntry {
	if DB == nil {
		return nil
	}

	limit = normalizeProxyHistoryLimit(limit)

	rows := make([]domain.ProxyHistory, 0, limit)

	DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows)

	if len(rows) == 0 {
		return nil
	}

	entries := make([]dto.ProxyHistoryEntry, len(rows))
	for index := range rows {
		row := rows[len(rows)-1-index]
		entries[index] = dto.ProxyHistoryEntry{
			Count:      row.ProxyCount,
			RecordedAt: row.CreatedAt,
		}
	}

	return entries
}

func normalizeProxyHistoryLimit(limit int) int {
	if limit <= 0 {
		return defaultProxyHistoryLimit
	}
	if limit > maxProxyHistoryLimit {
		return maxProxyHistoryLimit
	}
	return limit
}
