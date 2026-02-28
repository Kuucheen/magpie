package proxyqueue

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"magpie/internal/domain"
	"magpie/internal/support"
)

func TestQueueKeyForMember_DeterministicShard(t *testing.T) {
	q := &RedisProxyQueue{
		queueShardKeys: buildQueueShardKeys(8),
	}

	member := "member-a"
	key1 := q.queueKeyForMember(member)
	key2 := q.queueKeyForMember(member)
	if key1 != key2 {
		t.Fatalf("expected deterministic shard key, got %q and %q", key1, key2)
	}
	if key1 == legacyQueueKey {
		t.Fatalf("expected sharded key, got legacy queue key %q", key1)
	}
}

func TestDequeueWaitDuration(t *testing.T) {
	nowMs := int64(10_000)

	if got := dequeueWaitDuration(-1, nowMs); got != idleQueueSleep {
		t.Fatalf("expected idle sleep %s, got %s", idleQueueSleep, got)
	}

	if got := dequeueWaitDuration(nowMs, nowMs); got != minDequeueSleep {
		t.Fatalf("expected min sleep %s, got %s", minDequeueSleep, got)
	}

	if got := dequeueWaitDuration(nowMs+10_000, nowMs); got != maxDequeueSleep {
		t.Fatalf("expected max sleep %s, got %s", maxDequeueSleep, got)
	}
}

func TestParseProxyPopResult(t *testing.T) {
	found, err := parseProxyPopResult([]interface{}{int64(1), "member", "payload", int64(1234), int64(2)})
	if err != nil {
		t.Fatalf("unexpected parse error for found result: %v", err)
	}
	if !found.Found || found.ProxyJSON != "payload" || found.ScoreMs != 1234 {
		t.Fatalf("unexpected found parse result: %#v", found)
	}

	empty, err := parseProxyPopResult([]interface{}{int64(0), "", "", int64(5555), int64(-1)})
	if err != nil {
		t.Fatalf("unexpected parse error for empty result: %v", err)
	}
	if empty.Found || empty.NextReadyMs != 5555 {
		t.Fatalf("unexpected empty parse result: %#v", empty)
	}
}

func TestNewQueuedProxy_StoresUserIDsOnly(t *testing.T) {
	proxy := domain.Proxy{
		ID:       12,
		IP:       "192.0.2.10",
		Port:     8080,
		Username: "u",
		Password: "p",
		Hash:     []byte("hash"),
		Users: []domain.User{
			{ID: 5, Timeout: 1000, Retries: 2},
			{ID: 9, Timeout: 2500, Retries: 5},
			{ID: 5, Timeout: 9999, Retries: 9},
		},
	}

	queued := newQueuedProxy(proxy)
	if len(queued.UserIDs) != 2 || queued.UserIDs[0] != 5 || queued.UserIDs[1] != 9 {
		t.Fatalf("unexpected queued user IDs: %#v", queued.UserIDs)
	}
	if len(queued.Users) != 0 {
		t.Fatalf("expected no legacy user payload, got %#v", queued.Users)
	}

	raw, err := json.Marshal(queued)
	if err != nil {
		t.Fatalf("marshal queued proxy: %v", err)
	}
	payload := string(raw)
	if !strings.Contains(payload, "\"UserIDs\":[5,9]") {
		t.Fatalf("expected compact UserIDs payload, got %s", payload)
	}
	if strings.Contains(payload, "\"Users\"") {
		t.Fatalf("expected Users field to be omitted in new payload, got %s", payload)
	}
}

func TestQueuedProxyToDomainProxy_HandlesLegacyUsersAndUserIDs(t *testing.T) {
	fromUserIDs := queuedProxy{
		ID:      1,
		IP:      "198.51.100.2",
		Port:    9000,
		Hash:    []byte("h"),
		UserIDs: []uint{8, 4, 8},
	}
	got := fromUserIDs.toDomainProxy()
	if len(got.Users) != 2 || got.Users[0].ID != 8 || got.Users[1].ID != 4 {
		t.Fatalf("unexpected users from UserIDs payload: %#v", got.Users)
	}
	if got.Users[0].Timeout != 0 || got.Users[1].Retries != 0 {
		t.Fatalf("expected compact payload to not include checker settings, got %#v", got.Users)
	}

	fromLegacy := queuedProxy{
		ID:   2,
		IP:   "203.0.113.5",
		Port: 1080,
		Hash: []byte("legacy"),
		Users: []queuedProxyUser{
			{ID: 11},
			{ID: 7},
		},
	}
	gotLegacy := fromLegacy.toDomainProxy()
	if len(gotLegacy.Users) != 2 || gotLegacy.Users[0].ID != 11 || gotLegacy.Users[1].ID != 7 {
		t.Fatalf("unexpected users from legacy payload: %#v", gotLegacy.Users)
	}
}

func TestParseIntervalStateMillis(t *testing.T) {
	fallback := 3 * time.Second
	if got := parseIntervalStateMillis("1500", fallback); got != 1500*time.Millisecond {
		t.Fatalf("parsed interval = %s, want 1500ms", got)
	}
	if got := parseIntervalStateMillis("", fallback); got != fallback {
		t.Fatalf("empty interval should fallback to %s, got %s", fallback, got)
	}
	if got := parseIntervalStateMillis("bad", fallback); got != fallback {
		t.Fatalf("invalid interval should fallback to %s, got %s", fallback, got)
	}
}

func TestApplyIntervalUpdateAsLeaderWithRunner_SkipsWhenNotLeader(t *testing.T) {
	called := false
	err := applyIntervalUpdateAsLeaderWithRunner(
		func(context.Context, string, time.Duration, func(context.Context) error) error {
			return support.ErrLeaderLockNotAcquired
		},
		"lock",
		time.Second,
		func(time.Duration) error {
			called = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected nil on lock-not-acquired, got %v", err)
	}
	if called {
		t.Fatal("reschedule should not run when leadership is not acquired")
	}
}

func TestApplyIntervalUpdateAsLeaderWithRunner_PropagatesRescheduleError(t *testing.T) {
	expected := errors.New("boom")
	err := applyIntervalUpdateAsLeaderWithRunner(
		func(_ context.Context, _ string, _ time.Duration, run func(context.Context) error) error {
			return run(context.Background())
		},
		"lock",
		time.Second,
		func(time.Duration) error {
			return expected
		},
	)
	if !errors.Is(err, expected) {
		t.Fatalf("expected reschedule error %v, got %v", expected, err)
	}
}
