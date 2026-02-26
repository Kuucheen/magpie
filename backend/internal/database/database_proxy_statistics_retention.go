package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

func DeleteOldProxyStatistics(ctx context.Context, olderThan time.Time, limit int) (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("proxy statistics retention: database not initialised")
	}

	if olderThan.IsZero() {
		return 0, nil
	}

	if limit <= 0 {
		limit = deleteChunkSize
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	query := `
DELETE FROM proxy_statistics
WHERE id IN (
	SELECT id FROM (
		SELECT ps.id
		FROM proxy_statistics ps
		WHERE ps.created_at < ?
		  AND NOT EXISTS (
			  SELECT 1
			  FROM proxy_latest_statistics pls
			  WHERE pls.statistic_id = ps.id
		  )
		ORDER BY ps.created_at ASC, ps.id ASC
		LIMIT ?
		%s
	) AS deletable
)
`
	skipLocked := ""
	if retentionSupportsSkipLocked(db) {
		skipLocked = "FOR UPDATE SKIP LOCKED"
	}

	result := db.Exec(fmt.Sprintf(query, skipLocked), olderThan.UTC(), limit)

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

func PruneProxyStatisticResponseBodies(ctx context.Context, olderThan time.Time, limit int) (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("proxy statistic response retention: database not initialised")
	}

	if olderThan.IsZero() {
		return 0, nil
	}

	if limit <= 0 {
		limit = deleteChunkSize
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	query := `
UPDATE proxy_statistics
SET response_body = ''
WHERE id IN (
	SELECT id FROM (
		SELECT ps.id
		FROM proxy_statistics ps
		WHERE ps.created_at < ?
		  AND COALESCE(ps.response_body, '') <> ''
		ORDER BY ps.created_at ASC, ps.id ASC
		LIMIT ?
		%s
	) AS prunable
)
`
	skipLocked := ""
	if retentionSupportsSkipLocked(db) {
		skipLocked = "FOR UPDATE SKIP LOCKED"
	}

	result := db.Exec(fmt.Sprintf(query, skipLocked), olderThan.UTC(), limit)

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

func retentionSupportsSkipLocked(db *gorm.DB) bool {
	if db == nil || db.Dialector == nil {
		return false
	}

	name := strings.ToLower(strings.TrimSpace(db.Dialector.Name()))
	return strings.Contains(name, "postgres")
}
