package support

import (
	"fmt"
	"net/mail"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
)

const (
	envMailFromAddress = "MAIL_FROM_ADDRESS"
	envMailFromName    = "MAIL_FROM_NAME"
	envSMTPHost        = "SMTP_HOST"
	envSMTPPort        = "SMTP_PORT"
	envSMTPUsername    = "SMTP_USERNAME"
	envSMTPPassword    = "SMTP_PASSWORD"

	defaultSMTPPort = 587
)

type EmailConfig struct {
	FromAddress  string
	FromName     string
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
}

func ReadEmailConfig() (EmailConfig, error) {
	cfg := EmailConfig{
		FromAddress:  strings.TrimSpace(GetEnv(envMailFromAddress, "")),
		FromName:     strings.TrimSpace(GetEnv(envMailFromName, "")),
		SMTPHost:     strings.TrimSpace(GetEnv(envSMTPHost, "")),
		SMTPUsername: strings.TrimSpace(GetEnv(envSMTPUsername, "")),
		SMTPPassword: strings.TrimSpace(GetEnv(envSMTPPassword, "")),
		SMTPPort:     defaultSMTPPort,
	}

	rawPort := strings.TrimSpace(GetEnv(envSMTPPort, ""))
	if rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil {
			return EmailConfig{}, fmt.Errorf("invalid %s value %q: must be an integer port", envSMTPPort, rawPort)
		}
		cfg.SMTPPort = port
	}

	if err := cfg.Validate(); err != nil {
		return EmailConfig{}, err
	}

	return cfg, nil
}

func RequireEmailConfigValid() error {
	if _, err := ReadEmailConfig(); err != nil {
		log.Error("email delivery configuration invalid", "error", err)
		return err
	}
	return nil
}

func (c EmailConfig) IsConfigured() bool {
	return c.FromAddress != "" && c.SMTPHost != ""
}

func (c EmailConfig) HasSMTPAuth() bool {
	return c.SMTPUsername != "" && c.SMTPPassword != ""
}

func (c EmailConfig) SMTPImplicitTLS() bool {
	return c.SMTPPort == 465
}

func (c EmailConfig) SMTPAddress() string {
	if c.SMTPHost == "" || c.SMTPPort <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", c.SMTPHost, c.SMTPPort)
}

func (c EmailConfig) FormattedFrom() string {
	if c.FromAddress == "" {
		return ""
	}
	if c.FromName == "" {
		return c.FromAddress
	}
	return (&mail.Address{Name: c.FromName, Address: c.FromAddress}).String()
}

func (c EmailConfig) Validate() error {
	if !c.hasAnyValue() {
		return nil
	}

	var missing []string
	if c.FromAddress == "" {
		missing = append(missing, envMailFromAddress)
	}
	if c.SMTPHost == "" {
		missing = append(missing, envSMTPHost)
	}
	if len(missing) > 0 {
		return fmt.Errorf("email delivery configuration incomplete: set %s", strings.Join(missing, ", "))
	}

	if _, err := mail.ParseAddress(c.FromAddress); err != nil {
		return fmt.Errorf("invalid %s value %q: %w", envMailFromAddress, c.FromAddress, err)
	}
	if c.FromName != "" {
		if _, err := mail.ParseAddress(c.FormattedFrom()); err != nil {
			return fmt.Errorf("invalid %s/%s combination: %w", envMailFromName, envMailFromAddress, err)
		}
	}

	if c.SMTPPort < 1 || c.SMTPPort > 65535 {
		return fmt.Errorf("invalid %s value %d: must be between 1 and 65535", envSMTPPort, c.SMTPPort)
	}

	if (c.SMTPUsername == "") != (c.SMTPPassword == "") {
		return fmt.Errorf("SMTP authentication is incomplete: set both %s and %s or leave both unset", envSMTPUsername, envSMTPPassword)
	}

	return nil
}

func (c EmailConfig) hasAnyValue() bool {
	return c.FromAddress != "" ||
		c.FromName != "" ||
		c.SMTPHost != "" ||
		c.SMTPPort != defaultSMTPPort ||
		c.SMTPUsername != "" ||
		c.SMTPPassword != ""
}
