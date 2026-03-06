package checker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"magpie/internal/domain"
	"magpie/internal/support"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestProxyCheckRequest_ReusesTransportForSameRequestShape(t *testing.T) {
	resetCheckerHTTPClientCacheForTests()
	originalFactory := checkerTransportFactory
	t.Cleanup(func() {
		checkerTransportFactory = originalFactory
		resetCheckerHTTPClientCacheForTests()
	})

	var createCalls atomic.Int32
	checkerTransportFactory = func(domain.Proxy, *domain.Judge, string, string) (http.RoundTripper, func(), error) {
		createCalls.Add(1)
		return roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		}), func() {}, nil
	}

	proxy := domain.Proxy{IP: "127.0.0.1", Port: 8080}
	judge := &domain.Judge{FullString: "https://judge.example.com"}

	for i := 0; i < 2; i++ {
		html, err := ProxyCheckRequest(proxy, judge, "http", support.TransportTCP, 500)
		if err != nil {
			t.Fatalf("ProxyCheckRequest returned error: %v", err)
		}
		if html != "ok" {
			t.Fatalf("expected response body %q, got %q", "ok", html)
		}
	}

	if got := createCalls.Load(); got != 1 {
		t.Fatalf("expected transport to be created once for identical requests, got %d", got)
	}
}

func TestProxyCheckRequest_DoesNotForceConnectionClose(t *testing.T) {
	resetCheckerHTTPClientCacheForTests()
	originalFactory := checkerTransportFactory
	t.Cleanup(func() {
		checkerTransportFactory = originalFactory
		resetCheckerHTTPClientCacheForTests()
	})

	var connectionHeader atomic.Value
	connectionHeader.Store("")

	checkerTransportFactory = func(domain.Proxy, *domain.Judge, string, string) (http.RoundTripper, func(), error) {
		return roundTripFunc(func(req *http.Request) (*http.Response, error) {
			connectionHeader.Store(req.Header.Get("Connection"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		}), func() {}, nil
	}

	proxy := domain.Proxy{IP: "127.0.0.1", Port: 8080}
	judge := &domain.Judge{FullString: "https://judge.example.com"}

	if _, err := ProxyCheckRequest(proxy, judge, "http", support.TransportTCP, 500); err != nil {
		t.Fatalf("ProxyCheckRequest returned error: %v", err)
	}

	if got := connectionHeader.Load().(string); strings.EqualFold(got, "close") {
		t.Fatalf("expected Connection header to avoid forced close, got %q", got)
	}
}

func TestDefaultRequest_UsesTimeout(t *testing.T) {
	t.Setenv("ALLOW_PRIVATE_NETWORK_EGRESS", "true")
	t.Setenv(envCheckerDefaultRequestTimeoutMS, "20")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte("late"))
	}))
	t.Cleanup(server.Close)

	startedAt := time.Now()
	_, err := DefaultRequest(server.URL)
	elapsed := time.Since(startedAt)
	if err == nil {
		t.Fatal("DefaultRequest unexpectedly succeeded despite timeout")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("DefaultRequest timeout took too long: %s", elapsed)
	}
}

func TestDefaultRequestWithContext_RespectsCancellation(t *testing.T) {
	t.Setenv("ALLOW_PRIVATE_NETWORK_EGRESS", "true")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DefaultRequestWithContext(ctx, server.URL)
	if err == nil {
		t.Fatal("DefaultRequestWithContext unexpectedly succeeded with canceled context")
	}
}

func TestDefaultRequest_BlocksUnsafeOutboundTarget(t *testing.T) {
	_, err := DefaultRequest("http://127.0.0.1")
	if !errors.Is(err, support.ErrUnsafeOutboundTarget) {
		t.Fatalf("DefaultRequest err = %v, want ErrUnsafeOutboundTarget", err)
	}
}
