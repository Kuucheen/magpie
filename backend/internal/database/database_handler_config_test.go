package database

import (
	"os"
	"testing"

	"magpie/internal/config"
)

const envDBAutoMigrate = "DB_AUTO_MIGRATE"

func TestDefaultConfig_AutoMigrateDefaultsByMode(t *testing.T) {
	restoreProductionMode := setProductionModeForTest(t, true)
	defer restoreProductionMode()

	unsetEnvForTest(t, envDBAutoMigrate)

	cfg := defaultConfig()
	if cfg.AutoMigrate {
		t.Fatal("expected AutoMigrate=false in production when env is unset")
	}

	config.SetProductionMode(false)
	cfg = defaultConfig()
	if !cfg.AutoMigrate {
		t.Fatal("expected AutoMigrate=true outside production when env is unset")
	}
}

func TestDefaultConfig_AutoMigrateExplicitOverrideWins(t *testing.T) {
	restoreProductionMode := setProductionModeForTest(t, true)
	defer restoreProductionMode()

	t.Setenv(envDBAutoMigrate, "true")
	cfg := defaultConfig()
	if !cfg.AutoMigrate {
		t.Fatal("expected AutoMigrate=true when DB_AUTO_MIGRATE=true")
	}

	config.SetProductionMode(false)
	t.Setenv(envDBAutoMigrate, "false")
	cfg = defaultConfig()
	if cfg.AutoMigrate {
		t.Fatal("expected AutoMigrate=false when DB_AUTO_MIGRATE=false")
	}
}

func setProductionModeForTest(t *testing.T, production bool) func() {
	t.Helper()

	prev := config.InProductionMode
	config.SetProductionMode(production)

	return func() {
		config.SetProductionMode(prev)
	}
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()

	previousValue, hadValue := os.LookupEnv(key)
	if hadValue {
		t.Cleanup(func() {
			_ = os.Setenv(key, previousValue)
		})
	} else {
		t.Cleanup(func() {
			_ = os.Unsetenv(key)
		})
	}

	_ = os.Unsetenv(key)
}
