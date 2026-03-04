package config

import (
	"encoding/json"
	"os"
	"testing"
)

func TestReadSettings_ReturnsErrorForInvalidJSONAndKeepsExistingConfig(t *testing.T) {
	withTempSettingsWorkingDir(t)
	resetConfigAfterTest(t)

	if err := os.MkdirAll(settingsDirectoryPath, 0o755); err != nil {
		t.Fatalf("mkdir settings directory: %v", err)
	}
	if err := os.WriteFile(settingsFilePath, []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("write invalid settings: %v", err)
	}

	sentinel := Config{}
	sentinel.Checker.Timeout = 777
	configValue.Store(sentinel)

	if err := ReadSettings(); err == nil {
		t.Fatal("expected ReadSettings to fail for invalid JSON")
	}

	got := GetConfig()
	if got.Checker.Timeout != 777 {
		t.Fatalf("config changed after failed ReadSettings: timeout=%d want %d", got.Checker.Timeout, 777)
	}
}

func TestReadSettings_CreatesDefaultSettingsFileWhenMissing(t *testing.T) {
	withTempSettingsWorkingDir(t)
	resetConfigAfterTest(t)

	if err := ReadSettings(); err != nil {
		t.Fatalf("ReadSettings failed: %v", err)
	}

	if _, err := os.Stat(settingsFilePath); err != nil {
		t.Fatalf("settings file was not created: %v", err)
	}

	got := GetConfig()
	var want Config
	if err := json.Unmarshal(defaultConfig, &want); err != nil {
		t.Fatalf("unmarshal embedded default config: %v", err)
	}
	applyLegacyDefaults(defaultConfig, &want)
	normalizeThreadSettings(&want)
	want.WebsiteBlacklist = NormalizeWebsiteBlacklist(want.WebsiteBlacklist)

	if got.Checker.Timeout != want.Checker.Timeout {
		t.Fatalf("loaded checker timeout = %d, want %d", got.Checker.Timeout, want.Checker.Timeout)
	}
	if got.Scraper.Timeout != want.Scraper.Timeout {
		t.Fatalf("loaded scraper timeout = %d, want %d", got.Scraper.Timeout, want.Scraper.Timeout)
	}
}

func withTempSettingsWorkingDir(t *testing.T) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
}

func resetConfigAfterTest(t *testing.T) {
	t.Helper()

	orig := GetConfig()
	t.Cleanup(func() {
		configValue.Store(orig)
		SetBetweenTime()
		updateWebsiteBlocklist(orig.WebsiteBlacklist)
	})
}
