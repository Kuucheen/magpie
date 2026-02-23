package database

import "testing"

func TestResolveDBSSLMode(t *testing.T) {
	t.Run("defaults to require", func(t *testing.T) {
		t.Setenv("DB_SSLMODE", "")
		if got := resolveDBSSLMode(); got != "require" {
			t.Fatalf("resolveDBSSLMode() = %q, want %q", got, "require")
		}
	})

	t.Run("allows disable", func(t *testing.T) {
		t.Setenv("DB_SSLMODE", "disable")
		if got := resolveDBSSLMode(); got != "disable" {
			t.Fatalf("resolveDBSSLMode() = %q, want %q", got, "disable")
		}
	})

	t.Run("invalid value falls back to require", func(t *testing.T) {
		t.Setenv("DB_SSLMODE", "invalid")
		if got := resolveDBSSLMode(); got != "require" {
			t.Fatalf("resolveDBSSLMode() = %q, want %q", got, "require")
		}
	})
}
