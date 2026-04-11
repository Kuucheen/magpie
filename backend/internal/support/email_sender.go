package support

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
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

	client, err := dialSMTPClient(cfg)
	if err != nil {
		return fmt.Errorf("connect smtp: %w", err)
	}
	defer client.Close()

	if err := ensureSMTPTransportSecurity(client, cfg); err != nil {
		return err
	}

	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth failed: %w", err)
			}
		} else {
			return fmt.Errorf("smtp auth failed: server does not advertise AUTH")
		}
	}

	if err := client.Mail(cfg.FromAddress); err != nil {
		return fmt.Errorf("smtp MAIL FROM failed: %w", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("smtp RCPT TO failed: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA failed: %w", err)
	}
	if _, err := writer.Write(message.Bytes()); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp finalize failed: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit failed: %w", err)
	}

	return nil
}

func dialSMTPClient(cfg EmailConfig) (*smtp.Client, error) {
	timeout := 15 * time.Second
	dialer := net.Dialer{Timeout: timeout}

	if cfg.SMTPImplicitTLS() {
		conn, err := tls.DialWithDialer(&dialer, "tcp", cfg.SMTPAddress(), &tls.Config{
			ServerName: cfg.SMTPHost,
			MinVersion: tls.VersionTLS12,
		})
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, cfg.SMTPHost)
	}

	conn, err := dialer.Dial("tcp", cfg.SMTPAddress())
	if err != nil {
		return nil, err
	}

	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	if ok, _ := client.Extension("STARTTLS"); !ok {
		_ = client.Close()
		return nil, fmt.Errorf("smtp server does not advertise STARTTLS on port %d", cfg.SMTPPort)
	}

	if err := client.StartTLS(&tls.Config{
		ServerName: cfg.SMTPHost,
		MinVersion: tls.VersionTLS12,
	}); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("smtp STARTTLS failed: %w", err)
	}

	return client, nil
}

func ensureSMTPTransportSecurity(client *smtp.Client, cfg EmailConfig) error {
	if client == nil {
		return fmt.Errorf("smtp client is nil")
	}

	state, ok := client.TLSConnectionState()
	return validateSMTPTransportSecurity(cfg.SMTPImplicitTLS(), ok, state.Version)
}

func validateSMTPTransportSecurity(implicitTLS, hasTLS bool, tlsVersion uint16) error {
	if !hasTLS || tlsVersion < tls.VersionTLS12 {
		if implicitTLS {
			return fmt.Errorf("smtp connection is not protected by TLS")
		}
		return fmt.Errorf("smtp connection is not protected by STARTTLS")
	}

	return nil
}
