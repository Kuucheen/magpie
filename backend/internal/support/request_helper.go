package support

import (
	"context"
	"crypto/tls"
	"fmt"
	"golang.org/x/net/proxy"
	"io"
	"magpie/internal/config"
	"magpie/internal/domain"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func CreateTransport(proxyToCheck domain.Proxy, judge *domain.Judge, protocol string) (*http.Transport, error) {
	// Base configuration with keep-alives disabled
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(config.GetConfig().Checker.Timeout) * time.Millisecond,
			KeepAlive: 0, // KeepAlive disabled
		}).DialContext,
		DisableKeepAlives:     true,
		MaxIdleConns:          0,
		MaxIdleConnsPerHost:   0,
		IdleConnTimeout:       0,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	switch protocol {
	case "http", "https":
		// Configure HTTP/HTTPS proxy
		proxyURL := &url.URL{
			Scheme: "http",
			Host:   proxyToCheck.GetFullProxy(),
		}
		if proxyToCheck.HasAuth() {
			proxyURL.User = url.UserPassword(proxyToCheck.Username, proxyToCheck.Password)
		}
		transport.Proxy = http.ProxyURL(proxyURL)

		// Override dialer to resolve judge's host to pre-defined IP
		dialer := &net.Dialer{Timeout: time.Duration(config.GetConfig().Checker.Timeout) * time.Millisecond}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if host, port, err := net.SplitHostPort(addr); err == nil && host == judge.GetHostname() {
				addr = net.JoinHostPort(judge.GetIp(), port)
			}
			return dialer.DialContext(ctx, network, addr)
		}

	case "socks5":
		// Handle SOCKS5 proxy
		var auth *proxy.Auth
		if proxyToCheck.HasAuth() {
			auth = &proxy.Auth{User: proxyToCheck.Username, Password: proxyToCheck.Password}
		}
		socksDialer, err := proxy.SOCKS5("tcp", proxyToCheck.GetFullProxy(), auth, &net.Dialer{
			Timeout: time.Duration(config.GetConfig().Checker.Timeout) * time.Millisecond,
		})
		if err != nil {
			return nil, err
		}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return socksDialer.Dial(network, addr)
		}

	case "socks4":
		timeout := time.Duration(config.GetConfig().Checker.Timeout) * time.Millisecond
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialSOCKS4(ctx, proxyToCheck, addr, timeout)
		}

	default:
		return nil, fmt.Errorf("unsupported proxy protocol %q", protocol)
	}

	// Configure TLS to use judge's hostname
	transport.TLSClientConfig = &tls.Config{
		ServerName:         judge.GetHostname(),
		InsecureSkipVerify: false,
	}

	return transport, nil
}

func dialSOCKS4(ctx context.Context, proxyToCheck domain.Proxy, target string, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", proxyToCheck.GetFullProxy())
	if err != nil {
		return nil, err
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 0 || port > 65535 {
		_ = conn.Close()
		return nil, fmt.Errorf("invalid target port %q", portStr)
	}

	ip := net.ParseIP(host)
	ipBytes := ip.To4()
	var domainName string
	if ipBytes == nil {
		ipBytes = []byte{0x00, 0x00, 0x00, 0x01} // SOCKS4a
		domainName = host
	}

	userField := ""
	if proxyToCheck.Username != "" {
		userField = proxyToCheck.Username
		if proxyToCheck.Password != "" {
			userField = fmt.Sprintf("%s:%s", proxyToCheck.Username, proxyToCheck.Password)
		}
	}

	req := []byte{0x04, 0x01, byte(port >> 8), byte(port)}
	req = append(req, ipBytes...)
	req = append(req, []byte(userField)...)
	req = append(req, 0x00)
	if domainName != "" {
		req = append(req, []byte(domainName)...)
		req = append(req, 0x00)
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}

	if _, err := conn.Write(req); err != nil {
		_ = conn.Close()
		return nil, err
	}

	resp := make([]byte, 8)
	if _, err := io.ReadFull(conn, resp); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if len(resp) < 2 || resp[1] != 0x5A {
		_ = conn.Close()
		return nil, fmt.Errorf("socks4 connect failed with code %d", resp[1])
	}

	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}
