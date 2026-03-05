package rotatingproxy

import (
	"crypto/tls"
	"fmt"
	"strings"
	"sync"

	"magpie/internal/support"
)

var (
	rotatorTLSOnce  sync.Once
	rotatorTLSValue *tls.Config
	rotatorTLSErr   error
)

const (
	envRotatingProxyHTTP3TLSCertFile = "ROTATING_PROXY_HTTP3_TLS_CERT_FILE"
	envRotatingProxyHTTP3TLSKeyFile  = "ROTATING_PROXY_HTTP3_TLS_KEY_FILE"
)

func rotatorTLSConfig() (*tls.Config, error) {
	rotatorTLSOnce.Do(func() {
		certFile := strings.TrimSpace(support.GetEnv(envRotatingProxyHTTP3TLSCertFile, ""))
		keyFile := strings.TrimSpace(support.GetEnv(envRotatingProxyHTTP3TLSKeyFile, ""))
		if certFile == "" && keyFile == "" {
			rotatorTLSErr = fmt.Errorf(
				"HTTP/3 rotators require TLS certificate files; set %s and %s",
				envRotatingProxyHTTP3TLSCertFile,
				envRotatingProxyHTTP3TLSKeyFile,
			)
			return
		}
		if certFile == "" || keyFile == "" {
			rotatorTLSErr = fmt.Errorf(
				"HTTP/3 rotator TLS configuration is incomplete; both %s and %s must be set",
				envRotatingProxyHTTP3TLSCertFile,
				envRotatingProxyHTTP3TLSKeyFile,
			)
			return
		}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			rotatorTLSErr = fmt.Errorf("failed to load HTTP/3 rotator TLS cert/key pair: %w", err)
			return
		}

		rotatorTLSValue = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		}
	})

	if rotatorTLSErr != nil {
		return nil, rotatorTLSErr
	}
	return rotatorTLSValue, nil
}
