package server

import (
	"container/heap"
	"testing"
	"time"
)

func TestFixedWindowLimiter_LocalCounterExpires(t *testing.T) {
	limiter := newFixedWindowLimiter("test:local", 10, 30*time.Millisecond, 10)

	count, _ := limiter.incrementLocal("client-a")
	if count != 1 {
		t.Fatalf("incrementLocal count = %d, want 1", count)
	}

	time.Sleep(45 * time.Millisecond)

	count, ttl := limiter.currentLocal("client-a")
	if count != 0 {
		t.Fatalf("currentLocal count = %d, want 0", count)
	}
	if ttl > 0 {
		t.Fatalf("currentLocal ttl = %s, want <= 0", ttl)
	}
}

func TestFixedWindowLimiter_PurgeIgnoresStaleHeapEntries(t *testing.T) {
	limiter := newFixedWindowLimiter("test:local", 10, time.Hour, 10)
	now := time.Now()

	limiter.mu.Lock()
	limiter.counters["client-a"] = localCounter{
		count:     2,
		expiresAt: now.Add(30 * time.Minute),
	}
	heap.Push(&limiter.expiries, localCounterExpiry{
		key:       "client-a",
		expiresAt: now.Add(-time.Second),
	})
	limiter.purgeExpiredLocalLocked(now)
	entry, exists := limiter.counters["client-a"]
	limiter.mu.Unlock()

	if !exists {
		t.Fatal("counter was incorrectly purged by stale heap entry")
	}
	if entry.count != 2 {
		t.Fatalf("counter count = %d, want 2", entry.count)
	}
}

func TestFixedWindowLimiter_LocalFallbackEnforcesMaxKeys(t *testing.T) {
	limiter := newFixedWindowLimiter("test:local", 10, time.Hour, 1)

	if count, _ := limiter.incrementLocal("client-a"); count != 1 {
		t.Fatalf("incrementLocal(client-a) count = %d, want 1", count)
	}
	if count, _ := limiter.incrementLocal("client-b"); count != 1 {
		t.Fatalf("incrementLocal(client-b) count = %d, want 1", count)
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if got := len(limiter.counters); got != 1 {
		t.Fatalf("local counters size = %d, want 1", got)
	}
	if _, exists := limiter.counters["client-b"]; !exists {
		t.Fatalf("expected latest key client-b to be present, got keys: %#v", limiter.counters)
	}
}
