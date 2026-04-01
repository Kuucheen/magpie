package support

import (
	"bytes"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

func SendEmail(cfg EmailConfig, toAddress, subject, body string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if !cfg.IsConfigured() {
		return fmt.Errorf("email delivery is not configured")
	}

	recipient := strings.TrimSpace(toAddress)
	if _, err := mail.ParseAddress(recipient); err != nil {
		return fmt.Errorf("invalid recipient address %q: %w", recipient, err)
	}

	var message bytes.Buffer
	message.WriteString("From: " + cfg.FormattedFrom() + "\r\n")
	message.WriteString("To: " + recipient + "\r\n")
	message.WriteString("Subject: " + strings.TrimSpace(subject) + "\r\n")
	message.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	message.WriteString("\r\n")
	message.WriteString(body)
	if !strings.HasSuffix(body, "\r\n") {
		message.WriteString("\r\n")
	}

	var auth smtp.Auth
	if cfg.HasSMTPAuth() {
		auth = smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)
	}

	if err := smtp.SendMail(cfg.SMTPAddress(), auth, cfg.FromAddress, []string{recipient}, message.Bytes()); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}
