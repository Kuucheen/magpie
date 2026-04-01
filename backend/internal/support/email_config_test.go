package support

import "testing"

func TestReadEmailConfig_DefaultsWhenUnset(t *testing.T) {
	cfg, err := ReadEmailConfig()
	if err != nil {
		t.Fatalf("ReadEmailConfig returned error: %v", err)
	}

	if cfg.SMTPPort != defaultSMTPPort {
		t.Fatalf("ReadEmailConfig SMTPPort = %d, want %d", cfg.SMTPPort, defaultSMTPPort)
	}

	if cfg.IsConfigured() {
		t.Fatal("ReadEmailConfig unexpectedly reported configured email delivery")
	}
}

func TestReadEmailConfig_TrimsAndFormatsValues(t *testing.T) {
	t.Setenv(envMailFromAddress, " sender@example.com ")
	t.Setenv(envMailFromName, " Magpie Bot ")
	t.Setenv(envSMTPHost, " smtp.example.com ")
	t.Setenv(envSMTPPort, " 2525 ")
	t.Setenv(envSMTPUsername, " smtp-user ")
	t.Setenv(envSMTPPassword, " smtp-pass ")

	cfg, err := ReadEmailConfig()
	if err != nil {
		t.Fatalf("ReadEmailConfig returned error: %v", err)
	}

	if cfg.FromAddress != "sender@example.com" {
		t.Fatalf("FromAddress = %q, want sender@example.com", cfg.FromAddress)
	}
	if cfg.FromName != "Magpie Bot" {
		t.Fatalf("FromName = %q, want Magpie Bot", cfg.FromName)
	}
	if cfg.SMTPHost != "smtp.example.com" {
		t.Fatalf("SMTPHost = %q, want smtp.example.com", cfg.SMTPHost)
	}
	if cfg.SMTPPort != 2525 {
		t.Fatalf("SMTPPort = %d, want 2525", cfg.SMTPPort)
	}
	if cfg.SMTPAddress() != "smtp.example.com:2525" {
		t.Fatalf("SMTPAddress() = %q, want smtp.example.com:2525", cfg.SMTPAddress())
	}
	if cfg.FormattedFrom() != "\"Magpie Bot\" <sender@example.com>" {
		t.Fatalf("FormattedFrom() = %q", cfg.FormattedFrom())
	}
	if !cfg.IsConfigured() {
		t.Fatal("IsConfigured() = false, want true")
	}
	if !cfg.HasSMTPAuth() {
		t.Fatal("HasSMTPAuth() = false, want true")
	}
}

func TestReadEmailConfig_RejectsIncompleteConfig(t *testing.T) {
	t.Setenv(envMailFromName, "Magpie Bot")

	if _, err := ReadEmailConfig(); err == nil {
		t.Fatal("ReadEmailConfig unexpectedly succeeded with incomplete config")
	}
}

func TestReadEmailConfig_RejectsInvalidPort(t *testing.T) {
	t.Setenv(envMailFromAddress, "sender@example.com")
	t.Setenv(envSMTPHost, "smtp.example.com")
	t.Setenv(envSMTPPort, "not-a-port")

	if _, err := ReadEmailConfig(); err == nil {
		t.Fatal("ReadEmailConfig unexpectedly succeeded with invalid SMTP_PORT")
	}
}

func TestReadEmailConfig_RejectsPartialAuth(t *testing.T) {
	t.Setenv(envMailFromAddress, "sender@example.com")
	t.Setenv(envSMTPHost, "smtp.example.com")
	t.Setenv(envSMTPUsername, "smtp-user")

	if _, err := ReadEmailConfig(); err == nil {
		t.Fatal("ReadEmailConfig unexpectedly succeeded with partial SMTP auth config")
	}
}
