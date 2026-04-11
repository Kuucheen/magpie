package support

import "testing"

func TestReadPasswordResetConfig_RequiresPublicAppURLWhenEmailConfigured(t *testing.T) {
	t.Setenv(envMailFromAddress, "sender@example.com")
	t.Setenv(envSMTPHost, "smtp.example.com")

	if err := RequirePasswordResetConfigValid(); err == nil {
		t.Fatal("RequirePasswordResetConfigValid unexpectedly succeeded without PUBLIC_APP_URL")
	}
}

func TestBuildPasswordResetURL_UsesConfiguredPublicURL(t *testing.T) {
	url, err := BuildPasswordResetURL(PasswordResetConfig{
		PublicAppURL: "https://app.example/base",
		TokenTTL:     defaultPasswordResetTokenTTL,
	}, "token123")
	if err != nil {
		t.Fatalf("BuildPasswordResetURL returned error: %v", err)
	}

	if url != "https://app.example/base/reset-password?token=token123" {
		t.Fatalf("BuildPasswordResetURL() = %q", url)
	}
}
