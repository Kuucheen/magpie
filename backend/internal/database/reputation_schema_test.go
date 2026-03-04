package database

import (
	"testing"

	"magpie/internal/domain"
)

func TestEnsureProxyReputationSchema_SQLite(t *testing.T) {
	db := setupRotatingProxyTestDB(t)
	if err := db.AutoMigrate(&domain.ProxyReputation{}); err != nil {
		t.Fatalf("auto migrate proxy reputations: %v", err)
	}

	if _, err := hasProxyReputationIndex(db); err != nil {
		t.Fatalf("hasProxyReputationIndex failed on sqlite: %v", err)
	}

	if err := ensureProxyReputationSchema(db); err != nil {
		t.Fatalf("ensureProxyReputationSchema failed on sqlite: %v", err)
	}
}
