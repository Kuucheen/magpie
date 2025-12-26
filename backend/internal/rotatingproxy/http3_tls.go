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
	"net"
	"sync"
	"time"
)

var (
	rotatorTLSOnce  sync.Once
	rotatorTLSValue *tls.Config
	rotatorTLSErr   error
)

func rotatorTLSConfig() (*tls.Config, error) {
	rotatorTLSOnce.Do(func() {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			rotatorTLSErr = err
			return
		}

		serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
		serial, err := rand.Int(rand.Reader, serialLimit)
		if err != nil {
			rotatorTLSErr = err
			return
		}

		template := x509.Certificate{
			SerialNumber: serial,
			Subject:      pkix.Name{CommonName: "magpie-rotator"},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(365 * 24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			DNSNames:     []string{"localhost"},
			IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		}

		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
		if err != nil {
			rotatorTLSErr = err
			return
		}

		keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			rotatorTLSErr = err
			return
		}

		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			rotatorTLSErr = err
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
