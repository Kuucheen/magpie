package config

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
)

type Config struct {
	Protocols struct {
		HTTP   bool `json:"http"`
		HTTPS  bool `json:"https"`
		Socks4 bool `json:"socks4"`
		Socks5 bool `json:"socks5"`
	} `json:"protocols"`

	Checker struct {
		DynamicThreads bool   `json:"dynamic_threads"`
		Threads        uint32 `json:"threads"`
		MaxThreads     uint32 `json:"max_threads"`
		Retries        uint32 `json:"retries"`
		Timeout        uint32 `json:"timeout"`
		CheckerTimer   Timer  `json:"checker_timer"`

		JudgesThreads uint32  `json:"judges_threads"`
		JudgesTimeout uint32  `json:"judges_timeout"`
		Judges        []judge `json:"judges"`
		JudgeTimer    Timer   `json:"judge_timer"` // Only for production

		UseHttpsForSocks bool     `json:"use_https_for_socks"`
		IpLookup         string   `json:"ip_lookup"`
		StandardHeader   []string `json:"standard_header"`
		ProxyHeader      []string `json:"proxy_header"`
	} `json:"checker"`

	Scraper struct {
		DynamicThreads bool   `json:"dynamic_threads"`
		Threads        uint32 `json:"threads"`
		MaxThreads     uint32 `json:"max_threads"`
		Retries        uint32 `json:"retries"`
		Timeout        uint32 `json:"timeout"`
		RespectRobots  bool   `json:"respect_robots_txt"`

		ScraperTimer Timer `json:"scraper_timer"`

		ScrapeSites []string `json:"scrape_sites"`
	} `json:"scraper"`

	ProxyLimits ProxyLimitConfig `json:"proxy_limits"`

	Runtime struct {
		ProxyGeoRefreshTimer Timer `json:"proxy_geo_refresh_timer"`
	} `json:"runtime"`

	GeoLite struct {
		APIKey        string `json:"api_key"`
		AutoUpdate    bool   `json:"auto_update"`
		UpdateTimer   Timer  `json:"update_timer"`
		LastUpdatedAt string `json:"last_updated_at,omitempty"`
	} `json:"geolite"`

	BlacklistSources []string `json:"blacklist_sources"`
	BlacklistTimer   Timer    `json:"blacklist_timer"`

	WebsiteBlacklist []string `json:"website_blacklist"`
}

type judge struct {
	URL   string `json:"url"`
	Regex string `json:"regex"`
}

type Timer struct {
	Days    uint32 `json:"days"`
	Hours   uint32 `json:"hours"`
	Minutes uint32 `json:"minutes"`
	Seconds uint32 `json:"seconds"`
}

type ProxyLimitConfig struct {
	Enabled       bool   `json:"enabled"`
	MaxPerUser    uint32 `json:"max_per_user"`
	ExcludeAdmins bool   `json:"exclude_admins"`
}

const settingsFilePath = "data/settings.json"

var (
	//go:embed default_settings.json
	defaultConfig []byte

	configValue atomic.Value
	currentIp   atomic.Value
	configMu    sync.Mutex

	InProductionMode bool
)

func init() {
	// Initialize configValue with a default Config instance
	configValue.Store(Config{})
	currentIp.Store("")
}

func ReadSettings() {

	data, err := os.ReadFile(settingsFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warn("Settings file not found, creating with default configuration")

			err = os.MkdirAll("data", os.ModePerm)
			if err != nil {
				log.Error("Error creating directory for settings file:", err)
				return
			}

			err = os.WriteFile(settingsFilePath, defaultConfig, os.ModePerm)
			if err != nil {
				log.Error("Error writing default settings file:", err)
				return
			}

			data = defaultConfig
		} else {
			log.Error("Error reading settings file:", err)
			return
		}
	}

	var newConfig Config
	err = json.Unmarshal(data, &newConfig)
	if err != nil {
		log.Error("Error unmarshalling settings file:", err)
		return
	}

	if err := applyConfigUpdate(newConfig, configUpdateOptions{source: "file"}); err != nil {
		log.Error("Error applying configuration from settings file:", err)
		return
	}

	log.Debug("Settings file loaded successfully")
}

func SetConfig(newConfig Config) {
	if err := applyConfigUpdate(newConfig, configUpdateOptions{persistToFile: true, broadcast: true, source: "local"}); err != nil {
		log.Error("Error applying configuration update:", err)
		return
	}

	log.Debug("Default Configuration updated and written to file successfully")
}

func UpdateGeoLiteConfig(updater func(cfg *Config)) error {
	if updater == nil {
		return errors.New("config: geolite updater cannot be nil")
	}

	cfg := GetConfig()
	updater(&cfg)

	return applyConfigUpdate(cfg, configUpdateOptions{persistToFile: true, broadcast: true, source: "geolite"})
}

func MarkGeoLiteUpdated(ts time.Time) error {
	return UpdateGeoLiteConfig(func(cfg *Config) {
		cfg.GeoLite.LastUpdatedAt = ts.UTC().Format(time.RFC3339)
	})
}

type configUpdateOptions struct {
	persistToFile bool
	broadcast     bool
	source        string
}

func applyConfigUpdate(newConfig Config, opts configUpdateOptions) error {
	configMu.Lock()
	defer configMu.Unlock()

	normalizeThreadSettings(&newConfig)
	newConfig.WebsiteBlacklist = NormalizeWebsiteBlacklist(newConfig.WebsiteBlacklist)

	configValue.Store(newConfig)
	SetBetweenTime()
	updateWebsiteBlocklist(newConfig.WebsiteBlacklist)

	var errs []error

	if opts.persistToFile {
		data, err := json.MarshalIndent(newConfig, "", "  ")
		if err != nil {
			log.Error("Error marshalling new configuration:", err)
			errs = append(errs, err)
		} else if err := os.WriteFile(settingsFilePath, data, os.ModePerm); err != nil {
			log.Error("Error writing new configuration to file:", err)
			errs = append(errs, err)
		}
	}

	if opts.broadcast {
		payload, err := json.Marshal(newConfig)
		if err != nil {
			log.Error("Error serializing configuration for broadcast:", err)
			errs = append(errs, err)
		} else if err := broadcastConfigUpdate(payload); err != nil {
			log.Error("Error broadcasting configuration update:", err)
			errs = append(errs, err)
		}
	}

	if opts.source != "" {
		log.Debug("Configuration applied", "source", opts.source)
	} else {
		log.Debug("Configuration applied")
	}

	return errors.Join(errs...)
}

func normalizeThreadSettings(cfg *Config) {
	cfg.Checker.MaxThreads = normalizeMaxThreads(cfg.Checker.MaxThreads, cfg.Checker.Threads, 250)
	cfg.Scraper.MaxThreads = normalizeMaxThreads(cfg.Scraper.MaxThreads, cfg.Scraper.Threads, 250)
}

func normalizeMaxThreads(maxThreads uint32, threads uint32, fallback uint32) uint32 {
	if maxThreads > 0 {
		return maxThreads
	}
	if threads > 0 {
		return threads
	}
	return fallback
}

func GetConfig() Config {
	// Get the current Config atomically
	return configValue.Load().(Config)
}

func SetProductionMode(productionMode bool) {
	InProductionMode = productionMode
}

func GetCurrentIp() string {
	return currentIp.Load().(string)
}

func SetCurrentIp(ip string) {
	currentIp.Store(ip)
}
