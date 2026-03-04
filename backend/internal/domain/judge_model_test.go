package domain

import (
	"testing"
	"time"
)

func TestJudgeGetIp_DropsExpiredCacheEntry(t *testing.T) {
	judge := &Judge{FullString: "https://example.com/path"}
	if err := judge.SetUp(); err != nil {
		t.Fatalf("SetUp failed: %v", err)
	}
	key := judge.cacheKey()
	now := time.Now()

	judgeIPCacheMu.Lock()
	judgeResolvedIPByURL[key] = cachedJudgeIP{
		ip:        "203.0.113.7",
		expiresAt: now.Add(-time.Second),
	}
	nextJudgeIPCacheCleanup = now.Add(-time.Second)
	judgeIPCacheMu.Unlock()

	ip := judge.GetIp()
	if ip != "example.com" {
		t.Fatalf("GetIp() = %q, want hostname fallback %q", ip, "example.com")
	}

	judgeIPCacheMu.Lock()
	_, exists := judgeResolvedIPByURL[key]
	judgeIPCacheMu.Unlock()
	if exists {
		t.Fatal("expired cache entry was not evicted")
	}
}
