package support

import (
	"crypto/tls"
	"testing"
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
