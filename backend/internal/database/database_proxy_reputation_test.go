package database

import (
	"testing"

	"magpie/internal/domain"
)

func TestDedupeProxyReputationsForUpsert_KeepsLastEntryPerProxyKind(t *testing.T) {
	in := []domain.ProxyReputation{
		{ProxyID: 1, Kind: "http", Score: 10},
		{ProxyID: 2, Kind: "overall", Score: 11},
		{ProxyID: 1, Kind: "HTTP", Score: 12},
		{ProxyID: 1, Kind: " https ", Score: 20},
		{ProxyID: 1, Kind: "https", Score: 21},
	}

	got := dedupeProxyReputationsForUpsert(in)
	if len(got) != 3 {
		t.Fatalf("dedupe length = %d, want 3", len(got))
	}

	if got[0].ProxyID != 1 || got[0].Kind != "HTTP" || got[0].Score != 12 {
		t.Fatalf("unexpected deduped http entry: %#v", got[0])
	}
	if got[1].ProxyID != 2 || got[1].Kind != "overall" || got[1].Score != 11 {
		t.Fatalf("unexpected deduped overall entry: %#v", got[1])
	}
	if got[2].ProxyID != 1 || got[2].Kind != "https" || got[2].Score != 21 {
		t.Fatalf("unexpected deduped https entry: %#v", got[2])
	}
}

func TestProxyReputationAdvisoryLockKey_DeterministicAndNormalized(t *testing.T) {
	a := proxyReputationAdvisoryLockKey(42, " HTTPS ")
	b := proxyReputationAdvisoryLockKey(42, "https")
	c := proxyReputationAdvisoryLockKey(43, "https")

	if a != b {
		t.Fatalf("expected normalized kinds to produce same lock key, got %d and %d", a, b)
	}
	if a == c {
		t.Fatalf("expected different proxy ids to produce different lock keys, got %d", a)
	}
}
