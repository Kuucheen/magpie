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
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
)

const (
	proxyEncryptionKeyEnv = "PROXY_ENCRYPTION_KEY"
	ProxyEncryptionPrefix = "enc:"
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
			//proxyCipherErr = errors.New("proxy encryption key not set: " + proxyEncryptionKeyEnv)
			rawKey = "magpie-default-key"
			log.Warn("proxy encryption key not set, using default")
			//return
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

func deriveProxyKey(raw string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil {
		return normalizeKey(decoded), nil
	}

	sum := sha256.Sum256([]byte(raw))
	return sum[:], nil
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
