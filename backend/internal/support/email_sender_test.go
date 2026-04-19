package support

import (
	"crypto/tls"
	"strings"
	"testing"
	"time"
)

func TestValidateSMTPTransportSecurity_RejectsMissingTLSOnSubmissionPort(t *testing.T) {
	err := validateSMTPTransportSecurity(false, false, 0)
	if err == nil {
		t.Fatal("expected error for missing STARTTLS")
	}
}

func TestValidateSMTPTransportSecurity_RejectsLegacyTLS(t *testing.T) {
	err := validateSMTPTransportSecurity(false, true, tls.VersionTLS11)
	if err == nil {
		t.Fatal("expected error for legacy TLS")
	}
}

func TestValidateSMTPTransportSecurity_AcceptsModernTLS(t *testing.T) {
	err := validateSMTPTransportSecurity(false, true, tls.VersionTLS12)
	if err != nil {
		t.Fatalf("expected TLS 1.2 to be accepted, got %v", err)
	}
}

func TestBuildEmailMessage_UsesPlainTextForPlainBody(t *testing.T) {
	cfg := EmailConfig{FromAddress: "sender@example.com", FromName: "Magpie"}
	message := string(buildEmailMessage(cfg, "user@example.com", "Hello", "Plain body", time.Unix(1, 0).UTC()))

	if !strings.Contains(message, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("message did not use text/plain content type:\n%s", message)
	}
	if strings.Contains(message, "multipart/alternative") {
		t.Fatalf("plain message unexpectedly used multipart content:\n%s", message)
	}
}

func TestBuildEmailMessage_UsesMultipartAlternativeForHTMLBody(t *testing.T) {
	cfg := EmailConfig{FromAddress: "sender@example.com", FromName: "Magpie"}
	htmlBody := `<!doctype html><html><body><h1>Reset password</h1><p><a href="https://app.example/reset-password?token=abc">Reset password</a></p></body></html>`
	message := string(buildEmailMessage(cfg, "user@example.com", "Reset", htmlBody, time.Unix(1, 0).UTC()))

	for _, want := range []string{
		"Content-Type: multipart/alternative;",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Type: text/html; charset=UTF-8",
		"Reset password",
		"https://app.example/reset-password?token=abc",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q:\n%s", want, message)
		}
	}
}
