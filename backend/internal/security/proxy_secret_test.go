package security

import "testing"

const testEncryptionKey = "unit-test-encryption-key"

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
