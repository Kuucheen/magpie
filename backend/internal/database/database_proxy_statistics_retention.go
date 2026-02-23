package database

import (
	"context"
	"fmt"
	"time"
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

	result := db.Exec(`
DELETE FROM proxy_statistics
WHERE id IN (
	SELECT id FROM (
		SELECT ps.id
		FROM proxy_statistics ps
		LEFT JOIN proxy_latest_statistics pls ON pls.statistic_id = ps.id
		WHERE ps.created_at < ?
		  AND pls.statistic_id IS NULL
		ORDER BY ps.created_at ASC, ps.id ASC
		LIMIT ?
	) AS deletable
)
`, olderThan.UTC(), limit)

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

	result := db.Exec(`
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
	) AS prunable
)
`, olderThan.UTC(), limit)

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}
