package graphql

import "testing"

func TestClampPositiveLimit(t *testing.T) {
	if got := clampPositiveLimit(0, 10); got != 0 {
		t.Fatalf("clampPositiveLimit(0, 10) = %d, want 0", got)
	}

	if got := clampPositiveLimit(-5, 10); got != 0 {
		t.Fatalf("clampPositiveLimit(-5, 10) = %d, want 0", got)
	}

	if got := clampPositiveLimit(7, 10); got != 7 {
		t.Fatalf("clampPositiveLimit(7, 10) = %d, want 7", got)
	}

	if got := clampPositiveLimit(500, 120); got != 120 {
		t.Fatalf("clampPositiveLimit(500, 120) = %d, want 120", got)
	}
}
