package support

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestGetRedisClient_DefersReconnectAttemptsDuringBackoff(t *testing.T) {
	resetRedisClientTestState()
	t.Cleanup(resetRedisClientTestState)

	t.Setenv(envRedisMode, redisModeSingle)
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

	t.Setenv(envRedisMode, redisModeSingle)
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

func TestBuildRedisClientFromEnv_SentinelRequiresMasterAndAddrs(t *testing.T) {
	resetRedisClientTestState()
	t.Cleanup(resetRedisClientTestState)

	t.Setenv(envRedisMode, redisModeSentinel)
	t.Setenv(envRedisMasterName, "")
	t.Setenv(envRedisSentinelAddrs, "")

	_, err := buildRedisClientFromEnv()
	if err == nil || !strings.Contains(err.Error(), envRedisMasterName) {
		t.Fatalf("expected missing master name error, got: %v", err)
	}

	t.Setenv(envRedisMasterName, "mymaster")
	_, err = buildRedisClientFromEnv()
	if err == nil || !strings.Contains(err.Error(), envRedisSentinelAddrs) {
		t.Fatalf("expected missing sentinel addrs error, got: %v", err)
	}
}

func TestParseRedisSentinelAddrs_TrimsAndDropsEmptyValues(t *testing.T) {
	addrs := parseRedisSentinelAddrs(" s1:26379, ,s2:26379,, s3:26379 ")
	if len(addrs) != 3 {
		t.Fatalf("expected 3 addrs, got %d: %v", len(addrs), addrs)
	}
	if addrs[0] != "s1:26379" || addrs[1] != "s2:26379" || addrs[2] != "s3:26379" {
		t.Fatalf("unexpected addrs: %v", addrs)
	}
}

func TestBuildRedisClientFromEnv_InvalidMode(t *testing.T) {
	resetRedisClientTestState()
	t.Cleanup(resetRedisClientTestState)

	t.Setenv(envRedisMode, "unknown")
	_, err := buildRedisClientFromEnv()
	if err == nil || !strings.Contains(err.Error(), "invalid "+envRedisMode) {
		t.Fatalf("expected invalid mode error, got: %v", err)
	}
}

func TestBuildRedisClientFromEnv_ParseErrorDoesNotLeakRedisURL(t *testing.T) {
	resetRedisClientTestState()
	t.Cleanup(resetRedisClientTestState)

	t.Setenv(envRedisMode, redisModeSingle)
	t.Setenv("redisUrl", "redis://user:super-secret-pass@bad host:6379")

	_, err := buildRedisClientFromEnv()
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "failed to parse redisurl") {
		t.Fatalf("expected parse redisUrl error, got: %v", err)
	}
	if strings.Contains(err.Error(), "super-secret-pass") {
		t.Fatalf("error leaked redis credentials: %v", err)
	}
}

func TestGetRedisClientStatus_ReportsModeErrorAndBackoff(t *testing.T) {
	resetRedisClientTestState()
	t.Cleanup(resetRedisClientTestState)

	base := time.Date(2026, time.March, 4, 12, 10, 0, 0, time.UTC)
	redisNow = func() time.Time { return base }

	t.Setenv(envRedisMode, redisModeSentinel)
	t.Setenv(envRedisMasterName, "mymaster")
	t.Setenv(envRedisSentinelAddrs, "127.0.0.1:1")

	redisLastErr = errors.New("sentinel dial failed")
	redisRetryAfter = base.Add(4 * time.Second)

	status := GetRedisClientStatus()
	if status.Mode != redisModeSentinel {
		t.Fatalf("status mode = %q, want %q", status.Mode, redisModeSentinel)
	}
	if status.Connected {
		t.Fatal("expected status connected=false")
	}
	if !strings.Contains(status.LastError, "sentinel dial failed") {
		t.Fatalf("status last_error = %q, want sentinel dial failed", status.LastError)
	}
	if status.RetryAfter <= 0 {
		t.Fatalf("status retry_after = %s, want > 0", status.RetryAfter)
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
