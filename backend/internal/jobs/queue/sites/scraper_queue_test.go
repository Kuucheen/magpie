package sitequeue

import (
	"context"
	"errors"
	"testing"
	"time"

	"magpie/internal/support"
)

func TestQueueKeyForMember_DeterministicShard(t *testing.T) {
	q := &RedisScrapeSiteQueue{
		queueShardKeys: buildScrapeQueueShardKeys(8),
	}

	member := "https://example.com/list.txt"
	key1 := q.queueKeyForMember(member)
	key2 := q.queueKeyForMember(member)
	if key1 != key2 {
		t.Fatalf("expected deterministic shard key, got %q and %q", key1, key2)
	}
	if key1 == legacyScrapesiteQueueKey {
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

func TestParseScrapePopResult(t *testing.T) {
	found, err := parseScrapePopResult([]interface{}{int64(1), "member", "payload", int64(1234), int64(2)})
	if err != nil {
		t.Fatalf("unexpected parse error for found result: %v", err)
	}
	if !found.Found || found.SiteJSON != "payload" || found.ScoreMs != 1234 {
		t.Fatalf("unexpected found parse result: %#v", found)
	}

	empty, err := parseScrapePopResult([]interface{}{int64(0), "", "", int64(5555), int64(-1)})
	if err != nil {
		t.Fatalf("unexpected parse error for empty result: %v", err)
	}
	if empty.Found || empty.NextReadyMs != 5555 {
		t.Fatalf("unexpected empty parse result: %#v", empty)
	}
}

func TestParseIntervalStateMillis(t *testing.T) {
	fallback := 2 * time.Second
	if got := parseIntervalStateMillis("2500", fallback); got != 2500*time.Millisecond {
		t.Fatalf("parsed interval = %s, want 2500ms", got)
	}
	if got := parseIntervalStateMillis("", fallback); got != fallback {
		t.Fatalf("empty interval should fallback to %s, got %s", fallback, got)
	}
	if got := parseIntervalStateMillis("oops", fallback); got != fallback {
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
