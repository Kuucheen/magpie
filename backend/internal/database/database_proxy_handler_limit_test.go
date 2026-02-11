package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"magpie/internal/config"
	"magpie/internal/domain"
)

func TestNormalizeUserIDs(t *testing.T) {
	got := normalizeUserIDs([]uint{9, 4, 9, 2, 4, 7, 2})
	want := []uint{2, 4, 7, 9}
	if len(got) != len(want) {
		t.Fatalf("normalizeUserIDs length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeUserIDs[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestCleanupProxyLimitViolationsWithConfig_RemovesNewestOverflow(t *testing.T) {
	db := setupRotatingProxyTestDB(t)

	user := domain.User{
		Email:    "limit-cleanup-user@example.com",
		Password: "password123",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	base := time.Now().Add(-10 * time.Minute)
	proxyIDs := make([]uint64, 0, 5)

	for i := 0; i < 5; i++ {
		proxy := domain.Proxy{
			IP:            fmt.Sprintf("10.11.0.%d", i+1),
			Port:          uint16(8000 + i),
			Country:       "US",
			EstimatedType: "datacenter",
		}
		if err := db.Create(&proxy).Error; err != nil {
			t.Fatalf("create proxy %d: %v", i, err)
		}
		link := domain.UserProxy{
			UserID:    user.ID,
			ProxyID:   proxy.ID,
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}
		if err := db.Create(&link).Error; err != nil {
			t.Fatalf("create user proxy %d: %v", i, err)
		}
		proxyIDs = append(proxyIDs, proxy.ID)
	}

	removed, orphaned, err := cleanupProxyLimitViolationsWithConfig(context.Background(), config.ProxyLimitConfig{
		Enabled:       true,
		MaxPerUser:    3,
		ExcludeAdmins: false,
	})
	if err != nil {
		t.Fatalf("cleanup limit violations: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed rows = %d, want 2", removed)
	}
	if len(orphaned) != 2 {
		t.Fatalf("orphaned proxies = %d, want 2", len(orphaned))
	}

	var links []domain.UserProxy
	if err := db.Where("user_id = ?", user.ID).Find(&links).Error; err != nil {
		t.Fatalf("load user proxies: %v", err)
	}
	if len(links) != 3 {
		t.Fatalf("remaining user proxies = %d, want 3", len(links))
	}

	remaining := make(map[uint64]struct{}, len(links))
	for _, link := range links {
		remaining[link.ProxyID] = struct{}{}
	}
	for _, expectedID := range proxyIDs[:3] {
		if _, ok := remaining[expectedID]; !ok {
			t.Fatalf("expected proxy %d to remain after cleanup", expectedID)
		}
	}
}

func TestCleanupProxyLimitViolationsWithConfig_ExcludesAdmins(t *testing.T) {
	db := setupRotatingProxyTestDB(t)

	admin := domain.User{
		Email:    "limit-cleanup-admin@example.com",
		Password: "password123",
		Role:     "admin",
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	for i := 0; i < 4; i++ {
		proxy := domain.Proxy{
			IP:            fmt.Sprintf("10.12.0.%d", i+1),
			Port:          uint16(8100 + i),
			Country:       "US",
			EstimatedType: "datacenter",
		}
		if err := db.Create(&proxy).Error; err != nil {
			t.Fatalf("create proxy %d: %v", i, err)
		}
		if err := db.Create(&domain.UserProxy{
			UserID:  admin.ID,
			ProxyID: proxy.ID,
		}).Error; err != nil {
			t.Fatalf("create admin user proxy %d: %v", i, err)
		}
	}

	removed, orphaned, err := cleanupProxyLimitViolationsWithConfig(context.Background(), config.ProxyLimitConfig{
		Enabled:       true,
		MaxPerUser:    2,
		ExcludeAdmins: true,
	})
	if err != nil {
		t.Fatalf("cleanup limit violations: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed rows = %d, want 0 for excluded admin", removed)
	}
	if len(orphaned) != 0 {
		t.Fatalf("orphaned proxies = %d, want 0", len(orphaned))
	}

	var remaining int64
	if err := db.Model(&domain.UserProxy{}).
		Where("user_id = ?", admin.ID).
		Count(&remaining).Error; err != nil {
		t.Fatalf("count admin proxies: %v", err)
	}
	if remaining != 4 {
		t.Fatalf("admin proxies remaining = %d, want 4", remaining)
	}
}
