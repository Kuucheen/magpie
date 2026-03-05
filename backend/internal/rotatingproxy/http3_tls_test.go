package rotatingproxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRotatorTLSConfig_RequiresConfiguredCertAndKey(t *testing.T) {
	resetRotatorTLSConfigState()
	t.Cleanup(resetRotatorTLSConfigState)

	t.Setenv(envRotatingProxyHTTP3TLSCertFile, "")
	t.Setenv(envRotatingProxyHTTP3TLSKeyFile, "")

	cfg, err := rotatorTLSConfig()
	if err == nil {
		t.Fatal("expected error when cert/key env vars are unset")
	}
	if cfg != nil {
		t.Fatal("expected nil tls config when cert/key env vars are unset")
	}
	if !strings.Contains(err.Error(), envRotatingProxyHTTP3TLSCertFile) {
		t.Fatalf("error %q does not mention %s", err.Error(), envRotatingProxyHTTP3TLSCertFile)
	}
	if !strings.Contains(err.Error(), envRotatingProxyHTTP3TLSKeyFile) {
		t.Fatalf("error %q does not mention %s", err.Error(), envRotatingProxyHTTP3TLSKeyFile)
	}
}

func TestRotatorTLSConfig_RejectsIncompleteConfiguration(t *testing.T) {
	resetRotatorTLSConfigState()
	t.Cleanup(resetRotatorTLSConfigState)

	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "cert.pem")
	if err := os.WriteFile(certPath, []byte("not-a-certificate"), 0o600); err != nil {
		t.Fatalf("write cert fixture: %v", err)
	}

	t.Setenv(envRotatingProxyHTTP3TLSCertFile, certPath)
	t.Setenv(envRotatingProxyHTTP3TLSKeyFile, "")

	cfg, err := rotatorTLSConfig()
	if err == nil {
		t.Fatal("expected error when only one TLS file is configured")
	}
	if cfg != nil {
		t.Fatal("expected nil tls config for incomplete TLS configuration")
	}
	if !strings.Contains(err.Error(), "both") {
		t.Fatalf("error %q does not describe incomplete TLS configuration", err.Error())
	}
}

func TestRotatorTLSConfig_LoadsConfiguredCertAndKey(t *testing.T) {
	resetRotatorTLSConfigState()
	t.Cleanup(resetRotatorTLSConfigState)

	tempDir := t.TempDir()
	certPath, keyPath := createSelfSignedRotatorTLSFiles(t, tempDir)

	t.Setenv(envRotatingProxyHTTP3TLSCertFile, certPath)
	t.Setenv(envRotatingProxyHTTP3TLSKeyFile, keyPath)

	cfg, err := rotatorTLSConfig()
	if err != nil {
		t.Fatalf("rotatorTLSConfig returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil tls config")
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("tls min version = %d, want %d", cfg.MinVersion, tls.VersionTLS13)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("certificates count = %d, want 1", len(cfg.Certificates))
	}
}

func createSelfSignedRotatorTLSFiles(t *testing.T, dir string) (string, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		t.Fatalf("serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: "rotator.example"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"rotator.example"},
	}

	derCert, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derCert})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	certPath := filepath.Join(dir, "rotator-cert.pem")
	keyPath := filepath.Join(dir, "rotator-key.pem")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	return certPath, keyPath
}

func resetRotatorTLSConfigState() {
	rotatorTLSOnce = sync.Once{}
	rotatorTLSValue = nil
	rotatorTLSErr = nil
}
