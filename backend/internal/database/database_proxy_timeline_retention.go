package database

import (
	"context"
	"fmt"
	"time"
)

func DeleteOldProxySnapshots(ctx context.Context, olderThan time.Time, limit int) (int64, error) {
	return deleteOldRowsByCreatedAt(ctx, "proxy_snapshots", olderThan, limit, "proxy snapshot retention")
}

func DeleteOldProxyHistory(ctx context.Context, olderThan time.Time, limit int) (int64, error) {
	return deleteOldRowsByCreatedAt(ctx, "proxy_histories", olderThan, limit, "proxy history retention")
}

func deleteOldRowsByCreatedAt(ctx context.Context, table string, olderThan time.Time, limit int, scope string) (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("%s: database not initialised", scope)
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
DELETE FROM %s
WHERE id IN (
	SELECT id FROM (
		SELECT t.id
		FROM %s t
		WHERE t.created_at < ?
		ORDER BY t.created_at ASC, t.id ASC
		LIMIT ?
		%s
	) AS deletable
)
`
	skipLocked := ""
	if retentionSupportsSkipLocked(db) {
		skipLocked = "FOR UPDATE SKIP LOCKED"
	}

	result := db.Exec(fmt.Sprintf(query, table, table, skipLocked), olderThan.UTC(), limit)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
