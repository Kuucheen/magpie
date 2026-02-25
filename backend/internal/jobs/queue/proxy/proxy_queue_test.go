package proxyqueue

import "testing"

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
