package database

import "testing"

func TestNormalizeProxyHistoryLimit(t *testing.T) {
	if got := normalizeProxyHistoryLimit(0); got != defaultProxyHistoryLimit {
		t.Fatalf("normalizeProxyHistoryLimit(0) = %d, want %d", got, defaultProxyHistoryLimit)
	}

	if got := normalizeProxyHistoryLimit(-5); got != defaultProxyHistoryLimit {
		t.Fatalf("normalizeProxyHistoryLimit(-5) = %d, want %d", got, defaultProxyHistoryLimit)
	}

	if got := normalizeProxyHistoryLimit(48); got != 48 {
		t.Fatalf("normalizeProxyHistoryLimit(48) = %d, want 48", got)
	}

	if got := normalizeProxyHistoryLimit(maxProxyHistoryLimit + 100); got != maxProxyHistoryLimit {
		t.Fatalf("normalizeProxyHistoryLimit(too large) = %d, want %d", got, maxProxyHistoryLimit)
	}
}
