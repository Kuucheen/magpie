package database

import "testing"

func TestNormalizeProxySnapshotLimit(t *testing.T) {
	if got := normalizeProxySnapshotLimit(0); got != defaultProxySnapshotLimit {
		t.Fatalf("normalizeProxySnapshotLimit(0) = %d, want %d", got, defaultProxySnapshotLimit)
	}

	if got := normalizeProxySnapshotLimit(-3); got != defaultProxySnapshotLimit {
		t.Fatalf("normalizeProxySnapshotLimit(-3) = %d, want %d", got, defaultProxySnapshotLimit)
	}

	if got := normalizeProxySnapshotLimit(144); got != 144 {
		t.Fatalf("normalizeProxySnapshotLimit(144) = %d, want 144", got)
	}

	if got := normalizeProxySnapshotLimit(maxProxySnapshotLimit + 500); got != maxProxySnapshotLimit {
		t.Fatalf("normalizeProxySnapshotLimit(too large) = %d, want %d", got, maxProxySnapshotLimit)
	}
}
