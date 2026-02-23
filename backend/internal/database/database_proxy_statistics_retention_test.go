package database

import (
	"context"
	"testing"
	"time"

	"magpie/internal/domain"

	"gorm.io/gorm"
)

func TestDeleteOldProxyStatistics_PreservesLatestReferencedRows(t *testing.T) {
	db := setupRotatingProxyTestDB(t)
	proxy, protocol, judge := createProxyStatisticRetentionFixture(t, db)
	now := time.Now().UTC()

	oldReferenced := domain.ProxyStatistic{
		Alive:        true,
		Attempt:      1,
		ResponseTime: 120,
		ResponseBody: "referenced-body",
		ProtocolID:   protocol.ID,
		ProxyID:      proxy.ID,
		JudgeID:      judge.ID,
		CreatedAt:    now.Add(-45 * 24 * time.Hour),
	}
	if err := db.Create(&oldReferenced).Error; err != nil {
		t.Fatalf("create old referenced statistic: %v", err)
	}

	deletableOld := domain.ProxyStatistic{
		Alive:        false,
		Attempt:      2,
		ResponseTime: 800,
		ResponseBody: "delete-me",
		ProtocolID:   protocol.ID,
		ProxyID:      proxy.ID,
		JudgeID:      judge.ID,
		CreatedAt:    now.Add(-40 * 24 * time.Hour),
	}
	if err := db.Create(&deletableOld).Error; err != nil {
		t.Fatalf("create old deletable statistic: %v", err)
	}

	recent := domain.ProxyStatistic{
		Alive:        true,
		Attempt:      1,
		ResponseTime: 95,
		ResponseBody: "recent",
		ProtocolID:   protocol.ID,
		ProxyID:      proxy.ID,
		JudgeID:      judge.ID,
		CreatedAt:    now.Add(-2 * 24 * time.Hour),
	}
	if err := db.Create(&recent).Error; err != nil {
		t.Fatalf("create recent statistic: %v", err)
	}

	latest := domain.ProxyLatestStatistic{
		ProxyID:     proxy.ID,
		ProtocolID:  protocol.ID,
		Alive:       true,
		StatisticID: oldReferenced.ID,
		CheckedAt:   oldReferenced.CreatedAt,
	}
	if err := db.Create(&latest).Error; err != nil {
		t.Fatalf("create latest statistic pointer: %v", err)
	}

	deleted, err := DeleteOldProxyStatistics(context.Background(), now.Add(-30*24*time.Hour), 10)
	if err != nil {
		t.Fatalf("delete old statistics: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted rows = %d, want 1", deleted)
	}

	assertProxyStatisticExists(t, db, oldReferenced.ID, true)
	assertProxyStatisticExists(t, db, deletableOld.ID, false)
	assertProxyStatisticExists(t, db, recent.ID, true)
}

func TestPruneProxyStatisticResponseBodies_PrunesOldBodiesOnly(t *testing.T) {
	db := setupRotatingProxyTestDB(t)
	proxy, protocol, judge := createProxyStatisticRetentionFixture(t, db)
	now := time.Now().UTC()

	oldWithBody := domain.ProxyStatistic{
		Alive:        true,
		Attempt:      1,
		ResponseTime: 140,
		ResponseBody: "old-body",
		ProtocolID:   protocol.ID,
		ProxyID:      proxy.ID,
		JudgeID:      judge.ID,
		CreatedAt:    now.Add(-20 * 24 * time.Hour),
	}
	if err := db.Create(&oldWithBody).Error; err != nil {
		t.Fatalf("create old statistic with body: %v", err)
	}

	oldWithoutBody := domain.ProxyStatistic{
		Alive:        false,
		Attempt:      2,
		ResponseTime: 900,
		ResponseBody: "",
		ProtocolID:   protocol.ID,
		ProxyID:      proxy.ID,
		JudgeID:      judge.ID,
		CreatedAt:    now.Add(-18 * 24 * time.Hour),
	}
	if err := db.Create(&oldWithoutBody).Error; err != nil {
		t.Fatalf("create old statistic without body: %v", err)
	}

	recentWithBody := domain.ProxyStatistic{
		Alive:        true,
		Attempt:      1,
		ResponseTime: 90,
		ResponseBody: "recent-body",
		ProtocolID:   protocol.ID,
		ProxyID:      proxy.ID,
		JudgeID:      judge.ID,
		CreatedAt:    now.Add(-2 * 24 * time.Hour),
	}
	if err := db.Create(&recentWithBody).Error; err != nil {
		t.Fatalf("create recent statistic with body: %v", err)
	}

	pruned, err := PruneProxyStatisticResponseBodies(context.Background(), now.Add(-7*24*time.Hour), 10)
	if err != nil {
		t.Fatalf("prune response bodies: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("pruned rows = %d, want 1", pruned)
	}

	var oldWithBodyReloaded domain.ProxyStatistic
	if err := db.First(&oldWithBodyReloaded, oldWithBody.ID).Error; err != nil {
		t.Fatalf("reload oldWithBody: %v", err)
	}
	if oldWithBodyReloaded.ResponseBody != "" {
		t.Fatalf("old response body = %q, want empty", oldWithBodyReloaded.ResponseBody)
	}

	var oldWithoutBodyReloaded domain.ProxyStatistic
	if err := db.First(&oldWithoutBodyReloaded, oldWithoutBody.ID).Error; err != nil {
		t.Fatalf("reload oldWithoutBody: %v", err)
	}
	if oldWithoutBodyReloaded.ResponseBody != "" {
		t.Fatalf("old empty response body changed to %q", oldWithoutBodyReloaded.ResponseBody)
	}

	var recentWithBodyReloaded domain.ProxyStatistic
	if err := db.First(&recentWithBodyReloaded, recentWithBody.ID).Error; err != nil {
		t.Fatalf("reload recentWithBody: %v", err)
	}
	if recentWithBodyReloaded.ResponseBody != "recent-body" {
		t.Fatalf("recent response body = %q, want %q", recentWithBodyReloaded.ResponseBody, "recent-body")
	}
}

func createProxyStatisticRetentionFixture(t *testing.T, db *gorm.DB) (domain.Proxy, domain.Protocol, domain.Judge) {
	t.Helper()

	protocol := domain.Protocol{Name: "http"}
	if err := db.Create(&protocol).Error; err != nil {
		t.Fatalf("create protocol: %v", err)
	}

	judge := domain.Judge{FullString: "https://judge-retention.example.com"}
	if err := db.Create(&judge).Error; err != nil {
		t.Fatalf("create judge: %v", err)
	}

	proxy := domain.Proxy{
		IP:            "10.90.0.1",
		Port:          3128,
		Country:       "US",
		EstimatedType: "datacenter",
	}
	if err := db.Create(&proxy).Error; err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	return proxy, protocol, judge
}

func assertProxyStatisticExists(t *testing.T, db *gorm.DB, id uint64, want bool) {
	t.Helper()

	var count int64
	if err := db.Model(&domain.ProxyStatistic{}).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatalf("count proxy statistic %d: %v", id, err)
	}

	if want && count == 0 {
		t.Fatalf("proxy statistic %d was unexpectedly deleted", id)
	}
	if !want && count != 0 {
		t.Fatalf("proxy statistic %d still exists", id)
	}
}
