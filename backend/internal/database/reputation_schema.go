package database

import (
	"fmt"

	"gorm.io/gorm"
)

const proxyReputationIndexName = "idx_proxy_reputation_proxy_kind"

func ensureProxyReputationSchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("nil database connection")
	}
	if !db.Migrator().HasTable("proxy_reputations") {
		return nil
	}

	indexExists, err := hasProxyReputationIndex(db)
	if err != nil {
		return fmt.Errorf("check proxy reputation index: %w", err)
	}
	if indexExists {
		return nil
	}

	if err := removeDuplicateProxyReputations(db); err != nil {
		return fmt.Errorf("deduplicate proxy reputations: %w", err)
	}

	if err := ensureProxyReputationIndex(db); err != nil {
		return err
	}

	return nil
}

func ensureProxyReputationIndex(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("nil database connection")
	}

	query := fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON proxy_reputations (proxy_id, kind)", proxyReputationIndexName)
	if err := db.Exec(query).Error; err != nil {
		return fmt.Errorf("create proxy reputation index: %w", err)
	}

	return nil
}

func hasProxyReputationIndex(db *gorm.DB) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("nil database connection")
	}
	return db.Migrator().HasIndex("proxy_reputations", proxyReputationIndexName), nil
}

func removeDuplicateProxyReputations(db *gorm.DB) error {
	const cleanupQuery = `
WITH ranked AS (
	SELECT
		id,
		ROW_NUMBER() OVER (PARTITION BY proxy_id, kind ORDER BY calculated_at DESC, id DESC) AS rn
	FROM proxy_reputations
)
DELETE FROM proxy_reputations
WHERE id IN (SELECT id FROM ranked WHERE rn > 1);
`
	return db.Exec(cleanupQuery).Error
}
