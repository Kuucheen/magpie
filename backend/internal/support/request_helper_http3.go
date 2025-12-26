package support

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"magpie/internal/config"
	"magpie/internal/domain"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

func createHTTP3Transport(proxyToCheck domain.Proxy, judge *domain.Judge, protocol string, transportProtocol string) (http.RoundTripper, func(), error) {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "http", "https":
	default:
		return nil, nil, fmt.Errorf("http3 transport does not support proxy protocol %q", protocol)
	}

	if judge == nil {
		return nil, nil, errors.New("http3 transport requires a judge")
	}

	judgeURL, err := url.Parse(judge.FullString)
	if err != nil {
		return nil, nil, err
	}
	if !strings.EqualFold(judgeURL.Scheme, "https") {
		return nil, nil, fmt.Errorf("http3 transport requires https judges, got %q", judgeURL.Scheme)
	}

	proxyAddr := proxyToCheck.GetFullProxy()
	if proxyAddr == "" {
		return nil, nil, errors.New("proxy address is required for http3 transport")
	}

	proxyHost := proxyToCheck.GetIp()
	if proxyHost == "" {
		if host, _, err := net.SplitHostPort(proxyAddr); err == nil {
			proxyHost = host
		} else {
			proxyHost = proxyAddr
		}
	}

	timeout := time.Duration(config.GetConfig().Checker.Timeout) * time.Millisecond
	enableDatagrams := false
	switch NormalizeTransportProtocol(transportProtocol) {
	case TransportQUIC:
		enableDatagrams = true
	default:
		enableDatagrams = false
	}

	quicCfg := &quic.Config{
		HandshakeIdleTimeout: timeout,
		MaxIdleTimeout:       timeout,
		KeepAlivePeriod:      0,
		EnableDatagrams:      enableDatagrams,
	}

	transport := &http3.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         proxyHost,
		},
		QUICConfig:      quicCfg,
		EnableDatagrams: enableDatagrams,
		Dial: func(ctx context.Context, _ string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
			localTLS := tlsCfg
			if localTLS == nil {
				localTLS = &tls.Config{}
			} else {
				localTLS = tlsCfg.Clone()
			}
			localTLS.InsecureSkipVerify = true
			if proxyHost != "" {
				localTLS.ServerName = proxyHost
			}

			dialCfg := cfg
			if dialCfg == nil {
				dialCfg = quicCfg
			}

			return quic.DialAddr(ctx, proxyAddr, localTLS, dialCfg)
		},
	}

	closeFunc := func() {
		_ = transport.Close()
	}

	return transport, closeFunc, nil
}
