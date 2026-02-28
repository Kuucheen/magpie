package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"magpie/internal/config"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
)

const (
	proxyEncryptionKeyEnv = "PROXY_ENCRYPTION_KEY"
	ProxyEncryptionPrefix = "enc:"
	proxyEncryptionKeyMin = 32
)

var (
	proxyCipherOnce sync.Once
	proxyCipherInst *proxyCipher
	proxyCipherErr  error
)

type proxyCipher struct {
	gcm cipher.AEAD
}

func getProxyCipher() (*proxyCipher, error) {
	proxyCipherOnce.Do(func() {
		rawKey := strings.TrimSpace(os.Getenv(proxyEncryptionKeyEnv))
		if rawKey == "" {
			proxyCipherErr = errors.New("proxy encryption key not set: " + proxyEncryptionKeyEnv)
			return
		}
		if err := validateProxyEncryptionKey(rawKey); err != nil {
			proxyCipherErr = err
			return
		}

		key, err := deriveProxyKey(rawKey)
		if err != nil {
			proxyCipherErr = fmt.Errorf("derive proxy key: %w", err)
			return
		}

		block, err := aes.NewCipher(key)
		if err != nil {
			proxyCipherErr = fmt.Errorf("create cipher: %w", err)
			return
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			proxyCipherErr = fmt.Errorf("create gcm: %w", err)
			return
		}

		proxyCipherInst = &proxyCipher{gcm: gcm}
	})

	return proxyCipherInst, proxyCipherErr
}

func RequireProxyEncryptionKeyConfigured() error {
	_, err := getProxyCipher()
	if err != nil {
		log.Error("proxy encryption key configuration invalid", "error", err)
	}
	return err
}

func deriveProxyKey(raw string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil {
		return normalizeKey(decoded), nil
	}

	sum := sha256.Sum256([]byte(raw))
	return sum[:], nil
}

func validateProxyEncryptionKey(raw string) error {
	if !config.StrictSecretValidationEnabled() {
		return nil
	}

	if len(raw) < proxyEncryptionKeyMin {
		return fmt.Errorf("proxy encryption key is too short for strict mode: need at least %d characters", proxyEncryptionKeyMin)
	}

	if isKnownPlaceholderProxySecret(raw) {
		return errors.New("proxy encryption key uses a known placeholder value; set a strong unique secret")
	}

	return nil
}

func isKnownPlaceholderProxySecret(secret string) bool {
	placeholders := map[string]struct{}{
		"changeme":               {},
		"changemetoastrongkey":   {},
		"proxyencryptionkey":     {},
		"yourproxyencryptionkey": {},
	}

	normalized := normalizeSecretValue(secret)
	if normalized == "" {
		return false
	}

	_, exists := placeholders[normalized]
	return exists
}

func normalizeSecretValue(secret string) string {
	secret = strings.ToLower(strings.TrimSpace(secret))
	if secret == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(secret))
	for _, r := range secret {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

func normalizeKey(key []byte) []byte {
	switch len(key) {
	case 16, 24, 32:
		return key
	default:
		sum := sha256.Sum256(key)
		return sum[:]
	}
}

func EncryptProxySecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}

	pc, err := getProxyCipher()
	if err != nil {
		return "", err
	}

	nonce := make([]byte, pc.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	cipherText := pc.gcm.Seal(nil, nonce, []byte(plain), nil)
	payload := append(nonce, cipherText...)

	return ProxyEncryptionPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

func DecryptProxySecret(value string) (string, bool, error) {
	if value == "" {
		return "", false, nil
	}

	if !strings.HasPrefix(value, ProxyEncryptionPrefix) {
		return value, true, nil
	}

	encoded := strings.TrimPrefix(value, ProxyEncryptionPrefix)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", true, fmt.Errorf("decode ciphertext: %w", err)
	}

	pc, err := getProxyCipher()
	if err != nil {
		return "", false, err
	}

	nonceSize := pc.gcm.NonceSize()
	if len(data) <= nonceSize {
		return "", true, errors.New("ciphertext too short")
	}

	nonce := data[:nonceSize]
	cipherText := data[nonceSize:]

	plain, err := pc.gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", true, fmt.Errorf("decrypt ciphertext: %w", err)
	}

	return string(plain), false, nil
}

func IsProxySecretEncrypted(value string) bool {
	return strings.HasPrefix(value, ProxyEncryptionPrefix)
}

func ResetProxyCipherForTests() {
	proxyCipherOnce = sync.Once{}
	proxyCipherInst = nil
	proxyCipherErr = nil
}
