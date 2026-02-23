package app

import (
	"testing"

	"github.com/charmbracelet/log"
)

func TestReadPort(t *testing.T) {
	t.Setenv("MAGPIE_PORT_VALID", "12345")
	if got := readPort("MAGPIE_PORT_VALID"); got != 12345 {
		t.Fatalf("readPort returned %d, want 12345", got)
	}

	t.Setenv("MAGPIE_PORT_INVALID", "not-a-number")
	if got := readPort("MAGPIE_PORT_INVALID"); got != 0 {
		t.Fatalf("readPort with invalid value returned %d, want 0", got)
	}

	t.Setenv("MAGPIE_PORT_ZERO", "0")
	if got := readPort("MAGPIE_PORT_ZERO"); got != 0 {
		t.Fatalf("readPort with zero value returned %d, want 0", got)
	}
}

func TestResolvePort(t *testing.T) {
	t.Run("primary env overrides fallback", func(t *testing.T) {
		t.Setenv("PRIMARY_PORT", "5050")
		if got := resolvePort("PRIMARY_PORT", "LEGACY_PORT", 8080); got != 5050 {
			t.Fatalf("resolvePort returned %d, want 5050", got)
		}
	})

	t.Run("legacy env used when primary missing", func(t *testing.T) {
		t.Setenv("LEGACY_PORT", "6060")
		if got := resolvePort("PRIMARY_MISSING", "LEGACY_PORT", 8080); got != 6060 {
			t.Fatalf("resolvePort returned %d, want 6060", got)
		}
	})

	t.Run("fallback used when env unset", func(t *testing.T) {
		if got := resolvePort("UNSET_PRIMARY", "UNSET_LEGACY", 9090); got != 9090 {
			t.Fatalf("resolvePort returned %d, want 9090", got)
		}
	})
}

func TestConfigureLogLevel(t *testing.T) {
	t.Run("defaults to info when unset", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "")
		configureLogLevel()
		if got := log.GetLevel(); got != log.InfoLevel {
			t.Fatalf("configureLogLevel() level = %v, want %v", got, log.InfoLevel)
		}
	})

	t.Run("supports debug", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "debug")
		configureLogLevel()
		if got := log.GetLevel(); got != log.DebugLevel {
			t.Fatalf("configureLogLevel() level = %v, want %v", got, log.DebugLevel)
		}
	})

	t.Run("invalid value falls back to info", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "not-a-level")
		configureLogLevel()
		if got := log.GetLevel(); got != log.InfoLevel {
			t.Fatalf("configureLogLevel() level = %v, want %v", got, log.InfoLevel)
		}
	})
}
