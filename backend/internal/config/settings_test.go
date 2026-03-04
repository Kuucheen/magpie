package config

import (
	"encoding/json"
	"errors"
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

func TestNormalizeThreadSettings_ClampsThreadSettingsToSafetyLimit(t *testing.T) {
	cfg := Config{}
	cfg.Checker.Threads = maxThreadSettingLimit + 500
	cfg.Checker.MaxThreads = maxThreadSettingLimit + 1000
	cfg.Scraper.Threads = maxThreadSettingLimit + 250
	cfg.Scraper.MaxThreads = 0

	normalizeThreadSettings(&cfg)

	if cfg.Checker.Threads != maxThreadSettingLimit {
		t.Fatalf("checker threads = %d, want %d", cfg.Checker.Threads, maxThreadSettingLimit)
	}
	if cfg.Checker.MaxThreads != maxThreadSettingLimit {
		t.Fatalf("checker max threads = %d, want %d", cfg.Checker.MaxThreads, maxThreadSettingLimit)
	}
	if cfg.Scraper.Threads != maxThreadSettingLimit {
		t.Fatalf("scraper threads = %d, want %d", cfg.Scraper.Threads, maxThreadSettingLimit)
	}
	if cfg.Scraper.MaxThreads != maxThreadSettingLimit {
		t.Fatalf("scraper max threads = %d, want %d", cfg.Scraper.MaxThreads, maxThreadSettingLimit)
	}
}

func TestNormalizeThreadSettings_EnforcesMinimumThreadCount(t *testing.T) {
	cfg := Config{}
	cfg.Checker.Threads = 0
	cfg.Checker.MaxThreads = 0
	cfg.Scraper.Threads = 0
	cfg.Scraper.MaxThreads = 0

	normalizeThreadSettings(&cfg)

	if cfg.Checker.Threads != minThreadSettingLimit {
		t.Fatalf("checker threads = %d, want %d", cfg.Checker.Threads, minThreadSettingLimit)
	}
	if cfg.Checker.MaxThreads != defaultThreadFallback {
		t.Fatalf("checker max threads = %d, want %d", cfg.Checker.MaxThreads, defaultThreadFallback)
	}
	if cfg.Scraper.Threads != minThreadSettingLimit {
		t.Fatalf("scraper threads = %d, want %d", cfg.Scraper.Threads, minThreadSettingLimit)
	}
	if cfg.Scraper.MaxThreads != defaultThreadFallback {
		t.Fatalf("scraper max threads = %d, want %d", cfg.Scraper.MaxThreads, defaultThreadFallback)
	}
}

func TestSetConfig_DoesNotPersistConfigWhenBroadcastFails(t *testing.T) {
	withTempSettingsWorkingDir(t)
	resetConfigAfterTest(t)

	previousCfg := Config{}
	previousCfg.Checker.Timeout = 1111
	previousCfg.Checker.Threads = 120
	previousCfg.Checker.MaxThreads = 120
	previousCfg.Scraper.Timeout = 2222
	previousCfg.Scraper.Threads = 80
	previousCfg.Scraper.MaxThreads = 80

	configValue.Store(previousCfg)

	data, err := json.MarshalIndent(previousCfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal previous config: %v", err)
	}
	if err := os.MkdirAll(settingsDirectoryPath, settingsDirectoryMode); err != nil {
		t.Fatalf("mkdir settings directory: %v", err)
	}
	if err := os.WriteFile(settingsFilePath, data, settingsFileMode); err != nil {
		t.Fatalf("write previous settings file: %v", err)
	}

	origBroadcast := broadcastConfigUpdateFn
	broadcastConfigUpdateFn = func(_ []byte) error {
		return errors.New("broadcast failed")
	}
	t.Cleanup(func() {
		broadcastConfigUpdateFn = origBroadcast
	})

	newCfg := previousCfg
	newCfg.Checker.Timeout = 9999
	newCfg.Checker.Threads = 150
	newCfg.Checker.MaxThreads = 150

	if err := SetConfig(newCfg); err == nil {
		t.Fatal("expected SetConfig to fail when broadcast fails")
	}

	got := GetConfig()
	if got.Checker.Timeout != previousCfg.Checker.Timeout {
		t.Fatalf("in-memory config changed on failed update: timeout=%d want %d", got.Checker.Timeout, previousCfg.Checker.Timeout)
	}

	fileData, err := os.ReadFile(settingsFilePath)
	if err != nil {
		t.Fatalf("read settings file after failed update: %v", err)
	}

	var persisted Config
	if err := json.Unmarshal(fileData, &persisted); err != nil {
		t.Fatalf("unmarshal persisted settings: %v", err)
	}

	if persisted.Checker.Timeout != previousCfg.Checker.Timeout {
		t.Fatalf("settings file changed on failed update: timeout=%d want %d", persisted.Checker.Timeout, previousCfg.Checker.Timeout)
	}
}

func TestSetConfig_SucceedsWhenPostRenamePermissionEnforcementFails(t *testing.T) {
	withTempSettingsWorkingDir(t)
	resetConfigAfterTest(t)

	previousCfg := Config{}
	previousCfg.Checker.Timeout = 100
	previousCfg.Checker.Threads = 10
	previousCfg.Checker.MaxThreads = 10
	previousCfg.Scraper.Timeout = 200
	previousCfg.Scraper.Threads = 10
	previousCfg.Scraper.MaxThreads = 10
	configValue.Store(previousCfg)

	data, err := json.MarshalIndent(previousCfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal previous config: %v", err)
	}
	if err := os.MkdirAll(settingsDirectoryPath, settingsDirectoryMode); err != nil {
		t.Fatalf("mkdir settings directory: %v", err)
	}
	if err := os.WriteFile(settingsFilePath, data, settingsFileMode); err != nil {
		t.Fatalf("write previous settings file: %v", err)
	}

	origBroadcast := broadcastConfigUpdateFn
	broadcastConfigUpdateFn = func(_ []byte) error {
		return nil
	}
	t.Cleanup(func() {
		broadcastConfigUpdateFn = origBroadcast
	})

	origEnforce := enforceSettingsFilePermissionsFn
	enforceSettingsFilePermissionsFn = func() error {
		return errors.New("chmod failed")
	}
	t.Cleanup(func() {
		enforceSettingsFilePermissionsFn = origEnforce
	})

	newCfg := previousCfg
	newCfg.Checker.Timeout = 999

	if err := SetConfig(newCfg); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	got := GetConfig()
	if got.Checker.Timeout != newCfg.Checker.Timeout {
		t.Fatalf("in-memory config not updated: timeout=%d want %d", got.Checker.Timeout, newCfg.Checker.Timeout)
	}

	fileData, err := os.ReadFile(settingsFilePath)
	if err != nil {
		t.Fatalf("read settings file after update: %v", err)
	}
	var persisted Config
	if err := json.Unmarshal(fileData, &persisted); err != nil {
		t.Fatalf("unmarshal persisted settings: %v", err)
	}
	if persisted.Checker.Timeout != newCfg.Checker.Timeout {
		t.Fatalf("settings file not updated: timeout=%d want %d", persisted.Checker.Timeout, newCfg.Checker.Timeout)
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
