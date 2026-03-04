package support

import (
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestGetRedisClient_DefersReconnectAttemptsDuringBackoff(t *testing.T) {
	resetRedisClientTestState()
	t.Cleanup(resetRedisClientTestState)

	t.Setenv("redisUrl", "redis://127.0.0.1:1")

	base := time.Date(2026, time.March, 4, 12, 0, 0, 0, time.UTC)
	redisNow = func() time.Time { return base }

	_, err := GetRedisClient()
	if err == nil {
		t.Fatal("expected first redis connect attempt to fail")
	}
	if strings.Contains(err.Error(), "deferred") {
		t.Fatalf("first attempt should not be deferred, got: %v", err)
	}

	_, err = GetRedisClient()
	if err == nil {
		t.Fatal("expected second redis connect attempt to be deferred")
	}
	if !strings.Contains(err.Error(), "redis reconnect deferred") {
		t.Fatalf("expected deferred reconnect error, got: %v", err)
	}

	redisNow = func() time.Time {
		return base.Add(redisConnectRetryBackoff() + time.Millisecond)
	}
	_, err = GetRedisClient()
	if err == nil {
		t.Fatal("expected reconnect attempt after backoff to fail while redis is still down")
	}
	if strings.Contains(err.Error(), "redis reconnect deferred") {
		t.Fatalf("expected actual reconnect attempt after backoff, got deferred error: %v", err)
	}
}

func TestGetRedisClient_RecoversAfterBackoffWindow(t *testing.T) {
	resetRedisClientTestState()
	t.Cleanup(resetRedisClientTestState)

	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run failed: %v", err)
	}
	t.Cleanup(redisServer.Close)

	base := time.Date(2026, time.March, 4, 12, 5, 0, 0, time.UTC)
	redisNow = func() time.Time { return base }

	t.Setenv("redisUrl", "redis://127.0.0.1:1")
	_, err = GetRedisClient()
	if err == nil {
		t.Fatal("expected initial connect to fail while redis is unavailable")
	}

	t.Setenv("redisUrl", "redis://"+redisServer.Addr())

	_, err = GetRedisClient()
	if err == nil || !strings.Contains(err.Error(), "redis reconnect deferred") {
		t.Fatalf("expected deferred reconnect before backoff window expires, got: %v", err)
	}

	redisNow = func() time.Time {
		return base.Add(redisConnectRetryBackoff() + time.Millisecond)
	}

	clientA, err := GetRedisClient()
	if err != nil {
		t.Fatalf("expected reconnect to succeed after backoff, got: %v", err)
	}
	clientB, err := GetRedisClient()
	if err != nil {
		t.Fatalf("expected cached redis client, got: %v", err)
	}
	if clientA != clientB {
		t.Fatal("expected GetRedisClient to return cached client instance")
	}
}

func resetRedisClientTestState() {
	redisMu.Lock()
	defer redisMu.Unlock()

	if redisClient != nil {
		_ = redisClient.Close()
	}
	redisClient = nil
	redisRetryAfter = time.Time{}
	redisLastErr = nil
	redisNow = time.Now
}
