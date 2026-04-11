package support

import (
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

const (
	envPublicAppURL                = "PUBLIC_APP_URL"
	envPasswordResetTokenTTLMinute = "PASSWORD_RESET_TOKEN_TTL_MINUTES"

	defaultPasswordResetTokenTTL = 30 * time.Minute
	minPasswordResetTokenTTL     = 5 * time.Minute
	maxPasswordResetTokenTTL     = 24 * time.Hour
)

type PasswordResetConfig struct {
	PublicAppURL string
	TokenTTL     time.Duration
}

func ReadPasswordResetConfig() (PasswordResetConfig, error) {
	cfg := PasswordResetConfig{
		PublicAppURL: strings.TrimSpace(GetEnv(envPublicAppURL, "")),
		TokenTTL:     defaultPasswordResetTokenTTL,
	}

	if raw := strings.TrimSpace(GetEnv(envPasswordResetTokenTTLMinute, "")); raw != "" {
		minutes, err := strconv.Atoi(raw)
		if err != nil {
			return PasswordResetConfig{}, fmt.Errorf("invalid %s value %q: must be an integer number of minutes", envPasswordResetTokenTTLMinute, raw)
		}
		cfg.TokenTTL = time.Duration(minutes) * time.Minute
	}

	if cfg.TokenTTL < minPasswordResetTokenTTL || cfg.TokenTTL > maxPasswordResetTokenTTL {
		return PasswordResetConfig{}, fmt.Errorf(
			"%s out of range: got %d minutes, expected %d-%d",
			envPasswordResetTokenTTLMinute,
			int(cfg.TokenTTL/time.Minute),
			int(minPasswordResetTokenTTL/time.Minute),
			int(maxPasswordResetTokenTTL/time.Minute),
		)
	}

	if cfg.PublicAppURL != "" {
		if _, err := normalizePublicURL(cfg.PublicAppURL); err != nil {
			return PasswordResetConfig{}, fmt.Errorf("invalid %s value %q: %w", envPublicAppURL, cfg.PublicAppURL, err)
		}
	}

	return cfg, nil
}

func RequirePasswordResetConfigValid() error {
	emailCfg, err := ReadEmailConfig()
	if err != nil {
		return err
	}
	if !emailCfg.IsConfigured() {
		return nil
	}

	cfg, err := ReadPasswordResetConfig()
	if err != nil {
		log.Error("password reset configuration invalid", "error", err)
		return err
	}
	if strings.TrimSpace(cfg.PublicAppURL) == "" {
		err := fmt.Errorf("password reset configuration incomplete: set %s", envPublicAppURL)
		log.Error("password reset configuration invalid", "error", err)
		return err
	}
	return nil
}

func BuildPasswordResetURL(cfg PasswordResetConfig, token string) (string, error) {
	baseURL := cfg.PublicAppURL
	if baseURL == "" {
		return "", fmt.Errorf("password reset link cannot be built: set %s", envPublicAppURL)
	}

	base, err := normalizePublicURL(baseURL)
	if err != nil {
		return "", err
	}

	cleanPath := strings.TrimSuffix(base.Path, "/")
	if cleanPath == "" {
		base.Path = "/reset-password"
	} else {
		base.Path = path.Clean(cleanPath + "/reset-password")
		if !strings.HasPrefix(base.Path, "/") {
			base.Path = "/" + base.Path
		}
	}

	query := base.Query()
	query.Set("token", token)
	base.RawQuery = query.Encode()

	return base.String(), nil
}

func normalizePublicURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("host is required")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}
