package security

import (
	"os"
	"testing"

	"magpie/internal/config"
)

const testEncryptionKey = "unit-test-encryption-key"
const envStrictSecretValidation = "STRICT_SECRET_VALIDATION"

func TestEncryptDecryptProxySecret(t *testing.T) {
	t.Setenv(proxyEncryptionKeyEnv, testEncryptionKey)
	ResetProxyCipherForTests()

	cipherText, err := EncryptProxySecret("super-secret")
	if err != nil {
		t.Fatalf("EncryptProxySecret returned error: %v", err)
	}

	if !IsProxySecretEncrypted(cipherText) {
		t.Fatalf("ciphertext %q is not marked as encrypted", cipherText)
	}

	plain, legacy, err := DecryptProxySecret(cipherText)
	if err != nil {
		t.Fatalf("DecryptProxySecret returned error: %v", err)
	}
	if legacy {
		t.Fatal("DecryptProxySecret flagged encrypted value as legacy")
	}
	if plain != "super-secret" {
		t.Fatalf("DecryptProxySecret returned %q, want super-secret", plain)
	}
}

func TestDecryptLegacyProxySecret(t *testing.T) {
	t.Setenv(proxyEncryptionKeyEnv, testEncryptionKey)
	ResetProxyCipherForTests()

	plain, legacy, err := DecryptProxySecret("legacy-secret")
	if err != nil {
		t.Fatalf("DecryptProxySecret returned error: %v", err)
	}
	if !legacy {
		t.Fatal("expected legacy flag for plain secret")
	}
	if plain != "legacy-secret" {
		t.Fatalf("DecryptProxySecret returned %q, want legacy-secret", plain)
	}
}

func TestRequireProxyEncryptionKeyConfigured_StrictModeRejectsPlaceholderByDefaultInProduction(t *testing.T) {
	ResetProxyCipherForTests()
	setProductionModeForProxyTests(t, true)
	unsetEnvForProxyTests(t, envStrictSecretValidation)
	t.Setenv(proxyEncryptionKeyEnv, "ChangeMeToAStrongKey")

	if err := RequireProxyEncryptionKeyConfigured(); err == nil {
		t.Fatal("expected strict validation to reject placeholder proxy encryption key in production")
	}
}

func TestRequireProxyEncryptionKeyConfigured_StrictModeOverrideCanBeDisabled(t *testing.T) {
	ResetProxyCipherForTests()
	setProductionModeForProxyTests(t, true)
	t.Setenv(envStrictSecretValidation, "false")
	t.Setenv(proxyEncryptionKeyEnv, "ChangeMeToAStrongKey")

	if err := RequireProxyEncryptionKeyConfigured(); err != nil {
		t.Fatalf("expected strict validation override to allow placeholder key, got %v", err)
	}
}

func TestRequireProxyEncryptionKeyConfigured_StrictModeRejectsShortKey(t *testing.T) {
	ResetProxyCipherForTests()
	setProductionModeForProxyTests(t, false)
	t.Setenv(envStrictSecretValidation, "true")
	t.Setenv(proxyEncryptionKeyEnv, "too-short-proxy-key")

	if err := RequireProxyEncryptionKeyConfigured(); err == nil {
		t.Fatal("expected strict validation to reject short proxy key")
	}
}

func setProductionModeForProxyTests(t *testing.T, production bool) {
	t.Helper()

	prev := config.InProductionMode
	config.SetProductionMode(production)
	t.Cleanup(func() {
		config.SetProductionMode(prev)
	})
}

func unsetEnvForProxyTests(t *testing.T, key string) {
	t.Helper()

	prev, had := os.LookupEnv(key)
	if had {
		t.Cleanup(func() {
			_ = os.Setenv(key, prev)
		})
	} else {
		t.Cleanup(func() {
			_ = os.Unsetenv(key)
		})
	}

	_ = os.Unsetenv(key)
}
