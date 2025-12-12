package rotatingproxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"magpie/internal/api/dto"
	"magpie/internal/domain"
)

func TestAuthenticateClient_SucceedsWithValidCredentials(t *testing.T) {
	handler := &proxyHandler{
		rotator: domain.RotatingProxy{
			AuthRequired: true,
			AuthUsername: "proxy-user",
			AuthPassword: "proxy-pass",
		},
	}

	request := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	cred := base64.StdEncoding.EncodeToString([]byte("proxy-user:proxy-pass"))
	request.Header.Set("Proxy-Authorization", "Basic "+cred)

	recorder := httptest.NewRecorder()

	if ok := handler.authenticateClient(recorder, request); !ok {
		t.Fatal("authenticateClient returned false for valid credentials")
	}
	if recorder.Result().StatusCode != http.StatusOK && recorder.Code != 0 {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}
}

func TestAuthenticateClient_RejectsMissingOrInvalidCredentials(t *testing.T) {
	handler := &proxyHandler{
		rotator: domain.RotatingProxy{
			AuthRequired: true,
			AuthUsername: "proxy-user",
			AuthPassword: "proxy-pass",
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	if ok := handler.authenticateClient(recorder, request); ok {
		t.Fatal("authenticateClient should reject missing credentials")
	}
	if recorder.Code != http.StatusProxyAuthRequired {
		t.Fatalf("expected status %d, got %d", http.StatusProxyAuthRequired, recorder.Code)
	}
	if header := recorder.Header().Get("Proxy-Authenticate"); header == "" {
		t.Fatal("expected Proxy-Authenticate header to be set")
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	cred := base64.StdEncoding.EncodeToString([]byte("proxy-user:bad-pass"))
	request.Header.Set("Proxy-Authorization", "Basic "+cred)

	if ok := handler.authenticateClient(recorder, request); ok {
		t.Fatal("authenticateClient should reject invalid credentials")
	}
	if recorder.Code != http.StatusProxyAuthRequired {
		t.Fatalf("expected status %d for invalid credentials, got %d", http.StatusProxyAuthRequired, recorder.Code)
	}
}

func TestAuthenticateClient_AllowsUnauthenticatedAccessWhenDisabled(t *testing.T) {
	handler := &proxyHandler{
		rotator: domain.RotatingProxy{
			AuthRequired: false,
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	if ok := handler.authenticateClient(recorder, request); !ok {
		t.Fatal("authenticateClient rejected request when authentication is disabled")
	}
	if recorder.Code != 0 && recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}
}

func TestSupportedUpstream(t *testing.T) {
	cases := map[string]bool{
		"http":   true,
		"HTTP":   true,
		"https":  true,
		"socks4": true,
		"socks5": true,
		"socks":  false,
		"":       false,
	}

	for protocol, want := range cases {
		got := supportedUpstream(protocol)
		if got != want {
			t.Fatalf("supportedUpstream(%q) = %v, want %v", protocol, got, want)
		}
	}
}

func TestBuildHTTPTransport_ConfiguresProxyURL(t *testing.T) {
	withAuth := &dto.RotatingProxyNext{
		Protocol: "https",
		IP:       "127.0.0.1",
		Port:     9000,
		HasAuth:  true,
		Username: "user",
		Password: "pass",
	}

	transport := buildHTTPTransport(withAuth)
	if transport.Proxy == nil {
		t.Fatal("expected proxy function to be configured")
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("proxy func returned error: %v", err)
	}

	if proxyURL.Scheme != "http" {
		t.Fatalf("proxy scheme = %q, want http", proxyURL.Scheme)
	}
	if proxyURL.Host != "127.0.0.1:9000" {
		t.Fatalf("proxy host = %q, want 127.0.0.1:9000", proxyURL.Host)
	}

	user := proxyURL.User.Username()
	pass, _ := proxyURL.User.Password()
	if user != "user" || pass != "pass" {
		t.Fatalf("proxy credentials = %s:%s, want user:pass", user, pass)
	}

	if transport.TLSClientConfig != nil {
		t.Fatal("expected no TLS config when dialing upstream proxy")
	}

	withoutAuth := &dto.RotatingProxyNext{
		Protocol: "http",
		IP:       "127.0.0.1",
		Port:     8000,
		HasAuth:  false,
	}
	transport = buildHTTPTransport(withoutAuth)
	proxyURL, err = transport.Proxy(req)
	if err != nil {
		t.Fatalf("proxy func returned error for http proxy: %v", err)
	}
	if proxyURL.Scheme != "http" {
		t.Fatalf("proxy scheme = %q, want http", proxyURL.Scheme)
	}
	if proxyURL.User != nil {
		t.Fatal("expected no credentials for proxy without auth")
	}
	if transport.TLSClientConfig != nil {
		t.Fatal("expected no TLS config for http proxy")
	}
}

func TestHandleConnect_ProxiesDataThroughUpstream(t *testing.T) {
	handler := &proxyHandler{
		rotator: domain.RotatingProxy{
			ID:     42,
			UserID: 7,
		},
	}

	originalGetNext := getNextRotatingProxyFunc
	getNextRotatingProxyFunc = func(userID uint, rotatorID uint64) (*dto.RotatingProxyNext, error) {
		if userID != 7 || rotatorID != 42 {
			t.Fatalf("unexpected identifiers: userID=%d rotatorID=%d", userID, rotatorID)
		}
		return &dto.RotatingProxyNext{
			ProxyID:  1,
			IP:       "192.0.2.10",
			Port:     8080,
			Protocol: "http",
		}, nil
	}
	t.Cleanup(func() { getNextRotatingProxyFunc = originalGetNext })

	upstreamClient, upstreamServer := net.Pipe()
	originalConnect := connectThroughUpstreamFunc
	connectThroughUpstreamFunc = func(target string, next *dto.RotatingProxyNext) (net.Conn, error) {
		if target != "example.com:443" {
			t.Fatalf("expected target host example.com:443, got %s", target)
		}
		return upstreamServer, nil
	}
	t.Cleanup(func() {
		connectThroughUpstreamFunc = originalConnect
	})

	request := httptest.NewRequest(http.MethodConnect, "http://example.com:443", nil)
	request.Host = "example.com:443"

	clientEnd, serverEnd := net.Pipe()
	rw := newMockHijackResponseWriter(serverEnd)

	done := make(chan struct{})
	go func() {
		handler.handleConnect(rw, request)
		close(done)
	}()

	response := make([]byte, len(connectEstablishedResponse))
	if _, err := io.ReadFull(clientEnd, response); err != nil {
		t.Fatalf("read connect response: %v", err)
	}
	if string(response) != connectEstablishedResponse {
		t.Fatalf("unexpected connect response: %q", string(response))
	}

	if _, err := clientEnd.Write([]byte("ping")); err != nil {
		t.Fatalf("write client payload: %v", err)
	}

	upstreamPayload := make([]byte, 4)
	if _, err := io.ReadFull(upstreamClient, upstreamPayload); err != nil {
		t.Fatalf("read upstream payload: %v", err)
	}
	if string(upstreamPayload) != "ping" {
		t.Fatalf("upstream payload = %q, want ping", string(upstreamPayload))
	}

	if _, err := upstreamClient.Write([]byte("pong")); err != nil {
		t.Fatalf("write upstream response: %v", err)
	}

	clientPayload := make([]byte, 4)
	if _, err := io.ReadFull(clientEnd, clientPayload); err != nil {
		t.Fatalf("read client payload: %v", err)
	}
	if string(clientPayload) != "pong" {
		t.Fatalf("client payload = %q, want pong", string(clientPayload))
	}

	_ = clientEnd.Close()
	_ = upstreamClient.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleConnect did not return after closing connections")
	}
}

func TestSocks5Handler_WithAuthenticationAndPiping(t *testing.T) {
	handler := newSocksProxyHandler(domain.RotatingProxy{
		ID:           10,
		UserID:       5,
		AuthRequired: true,
		AuthUsername: "rot-user",
		AuthPassword: "rot-pass",
		Protocol:     domain.Protocol{Name: "socks5"},
	})

	origNext := getNextRotatingProxyFunc
	getNextRotatingProxyFunc = func(userID uint, rotatorID uint64) (*dto.RotatingProxyNext, error) {
		if userID != 5 || rotatorID != 10 {
			t.Fatalf("unexpected identifiers: userID=%d rotatorID=%d", userID, rotatorID)
		}
		return &dto.RotatingProxyNext{
			ProxyID:  99,
			IP:       "192.0.2.44",
			Port:     1080,
			Protocol: "socks5",
			HasAuth:  true,
			Username: "up-user",
			Password: "up-pass",
		}, nil
	}
	t.Cleanup(func() { getNextRotatingProxyFunc = origNext })

	upClient, upServer := net.Pipe()
	origConnect := connectThroughUpstreamFunc
	connectThroughUpstreamFunc = func(target string, next *dto.RotatingProxyNext) (net.Conn, error) {
		if target != "example.com:80" {
			t.Fatalf("expected target example.com:80, got %s", target)
		}
		return upServer, nil
	}
	t.Cleanup(func() { connectThroughUpstreamFunc = origConnect })

	clientConn, serverConn := net.Pipe()

	done := make(chan struct{})
	go func() {
		handler.handle(serverConn)
		close(done)
	}()

	if _, err := clientConn.Write([]byte{0x05, 0x01, 0x02}); err != nil {
		t.Fatalf("write greeting: %v", err)
	}
	greetResp := make([]byte, 2)
	if _, err := io.ReadFull(clientConn, greetResp); err != nil {
		t.Fatalf("read greeting response: %v", err)
	}
	if greetResp[1] != 0x02 {
		t.Fatalf("expected auth method 0x02, got %02x", greetResp[1])
	}

	authPayload := []byte{0x01, byte(len("rot-user"))}
	authPayload = append(authPayload, []byte("rot-user")...)
	authPayload = append(authPayload, byte(len("rot-pass")))
	authPayload = append(authPayload, []byte("rot-pass")...)
	if _, err := clientConn.Write(authPayload); err != nil {
		t.Fatalf("write auth payload: %v", err)
	}

	authResp := make([]byte, 2)
	if _, err := io.ReadFull(clientConn, authResp); err != nil {
		t.Fatalf("read auth response: %v", err)
	}
	if authResp[1] != 0x00 {
		t.Fatalf("authentication failed with code %02x", authResp[1])
	}

	host := []byte("example.com")
	request := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	request = append(request, host...)
	request = append(request, 0x00, 0x50) // port 80

	if _, err := clientConn.Write(request); err != nil {
		t.Fatalf("write connect request: %v", err)
	}

	reply := make([]byte, 10)
	if _, err := io.ReadFull(clientConn, reply); err != nil {
		t.Fatalf("read connect reply: %v", err)
	}
	if reply[1] != 0x00 {
		t.Fatalf("expected success reply, got %02x", reply[1])
	}

	if _, err := clientConn.Write([]byte("ping")); err != nil {
		t.Fatalf("write data to handler: %v", err)
	}

	payload := make([]byte, 4)
	if _, err := io.ReadFull(upClient, payload); err != nil {
		t.Fatalf("read payload upstream: %v", err)
	}
	if string(payload) != "ping" {
		t.Fatalf("upstream payload = %q, want ping", string(payload))
	}

	if _, err := upClient.Write([]byte("pong")); err != nil {
		t.Fatalf("write response upstream: %v", err)
	}

	if _, err := io.ReadFull(clientConn, payload); err != nil {
		t.Fatalf("read response from handler: %v", err)
	}
	if string(payload) != "pong" {
		t.Fatalf("client received %q, want pong", string(payload))
	}

	_ = clientConn.Close()
	_ = upClient.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("socks5 handler did not finish")
	}
}

func TestSocks4Handler_WithUserIDAuth(t *testing.T) {
	handler := newSocksProxyHandler(domain.RotatingProxy{
		ID:           22,
		UserID:       9,
		AuthRequired: true,
		AuthUsername: "rot-user",
		AuthPassword: "rot-pass",
		Protocol:     domain.Protocol{Name: "socks4"},
	})

	origNext := getNextRotatingProxyFunc
	getNextRotatingProxyFunc = func(userID uint, rotatorID uint64) (*dto.RotatingProxyNext, error) {
		if userID != 9 || rotatorID != 22 {
			t.Fatalf("unexpected identifiers: userID=%d rotatorID=%d", userID, rotatorID)
		}
		return &dto.RotatingProxyNext{
			ProxyID:  101,
			IP:       "198.51.100.10",
			Port:     9050,
			Protocol: "socks4",
		}, nil
	}
	t.Cleanup(func() { getNextRotatingProxyFunc = origNext })

	upClient, upServer := net.Pipe()
	origConnect := connectThroughUpstreamFunc
	connectThroughUpstreamFunc = func(target string, next *dto.RotatingProxyNext) (net.Conn, error) {
		if target != "1.1.1.1:1080" {
			t.Fatalf("expected target 1.1.1.1:1080, got %s", target)
		}
		return upServer, nil
	}
	t.Cleanup(func() { connectThroughUpstreamFunc = origConnect })

	clientConn, serverConn := net.Pipe()

	done := make(chan struct{})
	go func() {
		handler.handle(serverConn)
		close(done)
	}()

	req := []byte{0x04, 0x01, 0x04, 0x38, 0x01, 0x01, 0x01, 0x01} // port 1080, ip 1.1.1.1
	req = append(req, []byte("rot-user:rot-pass")...)
	req = append(req, 0x00)

	if _, err := clientConn.Write(req); err != nil {
		t.Fatalf("write socks4 request: %v", err)
	}

	resp := make([]byte, 8)
	if _, err := io.ReadFull(clientConn, resp); err != nil {
		t.Fatalf("read socks4 response: %v", err)
	}
	if resp[1] != 0x5A {
		t.Fatalf("expected request granted, got %02x", resp[1])
	}

	if _, err := clientConn.Write([]byte("test")); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	payload := make([]byte, 4)
	if _, err := io.ReadFull(upClient, payload); err != nil {
		t.Fatalf("read upstream payload: %v", err)
	}
	if string(payload) != "test" {
		t.Fatalf("unexpected upstream payload %q", string(payload))
	}

	if _, err := upClient.Write([]byte("back")); err != nil {
		t.Fatalf("write upstream response: %v", err)
	}

	if _, err := io.ReadFull(clientConn, payload); err != nil {
		t.Fatalf("read payload from handler: %v", err)
	}
	if string(payload) != "back" {
		t.Fatalf("unexpected response %q", string(payload))
	}

	_ = clientConn.Close()
	_ = upClient.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("socks4 handler did not finish")
	}
}

func TestDialProxyWithFallback_AllowsPlainHTTPProxy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 2; i++ {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
			_ = conn.Close()
		}
	}()

	next := &dto.RotatingProxyNext{
		Protocol: "https",
		IP:       "127.0.0.1",
		Port:     0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := dialProxyWithFallback(ctx, "tcp", ln.Addr().String(), next)
	if err != nil {
		t.Fatalf("dialProxyWithFallback error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	_ = conn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("listener goroutine did not exit")
	}
}

type mockHijackResponseWriter struct {
	header http.Header
	conn   net.Conn
	buf    *bufio.ReadWriter
}

func newMockHijackResponseWriter(conn net.Conn) *mockHijackResponseWriter {
	return &mockHijackResponseWriter{
		header: http.Header{},
		conn:   conn,
		buf:    bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
	}
}

func (m *mockHijackResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockHijackResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (m *mockHijackResponseWriter) WriteHeader(_ int) {}

func (m *mockHijackResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return m.conn, m.buf, nil
}
