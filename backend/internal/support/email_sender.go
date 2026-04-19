package support

import (
	"bytes"
	"crypto/tls"
	"fmt"
	stdhtml "html"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
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

	message := buildEmailMessage(cfg, recipient, subject, body, time.Now().UTC())

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
	if _, err := writer.Write(message); err != nil {
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

func buildEmailMessage(cfg EmailConfig, recipient, subject, body string, sentAt time.Time) []byte {
	var message bytes.Buffer
	message.WriteString("From: " + cfg.FormattedFrom() + "\r\n")
	message.WriteString("To: " + strings.TrimSpace(recipient) + "\r\n")
	message.WriteString("Subject: " + sanitizeEmailHeader(subject) + "\r\n")
	message.WriteString("Date: " + sentAt.UTC().Format(time.RFC1123Z) + "\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")
	writeEmailBody(&message, body, sentAt)
	return message.Bytes()
}

func writeEmailBody(message *bytes.Buffer, body string, sentAt time.Time) {
	if isHTMLBody(body) {
		boundary := fmt.Sprintf("magpie-alt-%d", sentAt.UnixNano())
		message.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
		message.WriteString("\r\n")
		message.WriteString("--" + boundary + "\r\n")
		message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		message.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		message.WriteString("\r\n")
		message.WriteString(normalizeEmailLineEndings(htmlToPlainText(body)))
		message.WriteString("\r\n")
		message.WriteString("--" + boundary + "\r\n")
		message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		message.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		message.WriteString("\r\n")
		message.WriteString(normalizeEmailLineEndings(body))
		message.WriteString("\r\n")
		message.WriteString("--" + boundary + "--\r\n")
		return
	}

	message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	message.WriteString("\r\n")
	plainBody := normalizeEmailLineEndings(body)
	message.WriteString(plainBody)
	if !strings.HasSuffix(plainBody, "\r\n") {
		message.WriteString("\r\n")
	}
}

func sanitizeEmailHeader(value string) string {
	return strings.Join(strings.Fields(strings.NewReplacer("\r", " ", "\n", " ").Replace(strings.TrimSpace(value))), " ")
}

func isHTMLBody(body string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(body))
	return strings.HasPrefix(trimmed, "<!doctype html") ||
		strings.HasPrefix(trimmed, "<html") ||
		strings.Contains(trimmed, "<body")
}

func normalizeEmailLineEndings(value string) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.ReplaceAll(normalized, "\n", "\r\n")
}

func htmlToPlainText(body string) string {
	doc, err := xhtml.Parse(strings.NewReader(body))
	if err != nil {
		return "Open this message in an HTML-capable email client."
	}

	var builder strings.Builder
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.TextNode {
			appendPlainText(&builder, node.Data)
			return
		}
		if node.Type == xhtml.ElementNode {
			switch node.Data {
			case "head", "style", "script", "meta", "title":
				return
			case "br":
				appendPlainNewline(&builder)
			case "p", "div", "table", "tr", "h1", "h2", "h3", "li":
				appendPlainNewline(&builder)
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}

		if node.Type == xhtml.ElementNode {
			if node.Data == "a" {
				if href := htmlNodeAttr(node, "href"); href != "" {
					appendPlainText(&builder, "("+href+")")
				}
			}
			switch node.Data {
			case "p", "div", "table", "tr", "h1", "h2", "h3", "li":
				appendPlainNewline(&builder)
			}
		}
	}
	walk(doc)

	plain := strings.TrimSpace(builder.String())
	if plain == "" {
		return "Open this message in an HTML-capable email client."
	}
	return stdhtml.UnescapeString(plain)
}

func appendPlainText(builder *strings.Builder, value string) {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return
	}
	current := builder.String()
	if current != "" && !strings.HasSuffix(current, "\n") && !strings.HasSuffix(current, " ") {
		builder.WriteByte(' ')
	}
	builder.WriteString(strings.Join(parts, " "))
}

func appendPlainNewline(builder *strings.Builder) {
	current := builder.String()
	if current != "" && !strings.HasSuffix(current, "\n") {
		builder.WriteByte('\n')
	}
}

func htmlNodeAttr(node *xhtml.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
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
