package database

import (
	"context"
	"testing"
	"time"

	"magpie/internal/domain"

	"gorm.io/gorm"
)

func TestDeleteOldProxySnapshots_DeletesOnlyRowsOlderThanCutoff(t *testing.T) {
	db := setupRotatingProxyTestDB(t)
	if err := db.AutoMigrate(&domain.ProxySnapshot{}); err != nil {
		t.Fatalf("auto migrate proxy snapshots: %v", err)
	}

	user := domain.User{
		Email:    "snapshot-retention@example.com",
		Password: "password123",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now().UTC()
	oldSnapshot := domain.ProxySnapshot{
		UserID:    user.ID,
		Metric:    domain.ProxySnapshotMetricAlive,
		Count:     12,
		CreatedAt: now.Add(-40 * 24 * time.Hour),
	}
	recentSnapshot := domain.ProxySnapshot{
		UserID:    user.ID,
		Metric:    domain.ProxySnapshotMetricAlive,
		Count:     18,
		CreatedAt: now.Add(-2 * 24 * time.Hour),
	}
	if err := db.Create(&oldSnapshot).Error; err != nil {
		t.Fatalf("create old snapshot: %v", err)
	}
	if err := db.Create(&recentSnapshot).Error; err != nil {
		t.Fatalf("create recent snapshot: %v", err)
	}

	deleted, err := DeleteOldProxySnapshots(context.Background(), now.Add(-30*24*time.Hour), 10)
	if err != nil {
		t.Fatalf("delete old snapshots: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted snapshots = %d, want 1", deleted)
	}

	assertProxySnapshotExists(t, db, oldSnapshot.ID, false)
	assertProxySnapshotExists(t, db, recentSnapshot.ID, true)
}

func TestDeleteOldProxyHistory_DeletesOnlyRowsOlderThanCutoff(t *testing.T) {
	db := setupRotatingProxyTestDB(t)
	if err := db.AutoMigrate(&domain.ProxyHistory{}); err != nil {
		t.Fatalf("auto migrate proxy histories: %v", err)
	}

	user := domain.User{
		Email:    "history-retention@example.com",
		Password: "password123",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now().UTC()
	oldHistory := domain.ProxyHistory{
		UserID:     user.ID,
		ProxyCount: 100,
		CreatedAt:  now.Add(-120 * 24 * time.Hour),
	}
	recentHistory := domain.ProxyHistory{
		UserID:     user.ID,
		ProxyCount: 110,
		CreatedAt:  now.Add(-5 * 24 * time.Hour),
	}
	if err := db.Create(&oldHistory).Error; err != nil {
		t.Fatalf("create old history: %v", err)
	}
	if err := db.Create(&recentHistory).Error; err != nil {
		t.Fatalf("create recent history: %v", err)
	}

	deleted, err := DeleteOldProxyHistory(context.Background(), now.Add(-60*24*time.Hour), 10)
	if err != nil {
		t.Fatalf("delete old histories: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted histories = %d, want 1", deleted)
	}

	assertProxyHistoryExists(t, db, oldHistory.ID, false)
	assertProxyHistoryExists(t, db, recentHistory.ID, true)
}

func assertProxySnapshotExists(t *testing.T, db *gorm.DB, id uint, want bool) {
	t.Helper()

	var count int64
	if err := db.Model(&domain.ProxySnapshot{}).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatalf("count snapshot %d: %v", id, err)
	}

	if want && count == 0 {
		t.Fatalf("snapshot %d was unexpectedly deleted", id)
	}
	if !want && count != 0 {
		t.Fatalf("snapshot %d still exists", id)
	}
}

func assertProxyHistoryExists(t *testing.T, db *gorm.DB, id uint, want bool) {
	t.Helper()

	var count int64
	if err := db.Model(&domain.ProxyHistory{}).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatalf("count history %d: %v", id, err)
	}

	if want && count == 0 {
		t.Fatalf("history %d was unexpectedly deleted", id)
	}
	if !want && count != 0 {
		t.Fatalf("history %d still exists", id)
	}
}
