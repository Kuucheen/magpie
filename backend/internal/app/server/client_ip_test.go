package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientIPFromRequest_IgnoresForwardedHeadersWhenRemoteNotTrusted(t *testing.T) {
	t.Setenv(envTrustedProxyCIDRs, "10.0.0.0/8")

	req := httptest.NewRequest("GET", "/api/login", nil)
	req.RemoteAddr = "198.51.100.44:4141"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.Header.Set("X-Real-IP", "203.0.113.8")

	expected := "198.51.100.44"
	if got := clientIPFromRequest(req); got != expected {
		t.Fatalf("clientIPFromRequest = %q, want %q", got, expected)
	}
	if got := getAuthClientIP(req); got != expected {
		t.Fatalf("getAuthClientIP = %q, want %q", got, expected)
	}
}

func TestClientIPFromRequest_UsesForwardedHeadersWhenRemoteTrusted(t *testing.T) {
	t.Setenv(envTrustedProxyCIDRs, "10.0.0.0/8,192.168.0.0/16")

	req := httptest.NewRequest("GET", "/api/login", nil)
	req.RemoteAddr = "10.20.30.40:5656"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.20.30.40")
	req.Header.Set("X-Real-IP", "203.0.113.10")

	expected := "203.0.113.9"
	if got := clientIPFromRequest(req); got != expected {
		t.Fatalf("clientIPFromRequest = %q, want %q", got, expected)
	}
	if got := getAuthClientIP(req); got != expected {
		t.Fatalf("getAuthClientIP = %q, want %q", got, expected)
	}
}

func TestClientIPFromRequest_IgnoresSpoofedForwardedChainWhenAllHopsTrusted(t *testing.T) {
	t.Setenv(envTrustedProxyCIDRs, "10.0.0.0/8")

	req := httptest.NewRequest("GET", "/api/login", nil)
	req.RemoteAddr = "10.20.30.40:5656"
	req.Header.Set("X-Forwarded-For", "10.30.40.50,10.60.70.80")

	expected := "10.20.30.40"
	if got := clientIPFromRequest(req); got != expected {
		t.Fatalf("clientIPFromRequest = %q, want %q", got, expected)
	}
}

func TestClientIPFromRequest_WildcardTrustedProxyCIDRIsIgnored(t *testing.T) {
	t.Setenv(envTrustedProxyCIDRs, "0.0.0.0/0")

	req := httptest.NewRequest("GET", "/api/login", nil)
	req.RemoteAddr = "198.51.100.44:4141"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	expected := "198.51.100.44"
	if got := clientIPFromRequest(req); got != expected {
		t.Fatalf("clientIPFromRequest = %q, want %q", got, expected)
	}
}

func TestGetAuthRateLimitKey_AddsFingerprintWhenProxyTrustIsMissing(t *testing.T) {
	t.Setenv(envTrustedProxyCIDRs, "10.0.0.0/8")

	reqA := httptest.NewRequest("POST", "/api/login", nil)
	reqA.RemoteAddr = "172.20.0.10:8080"
	reqA.Header.Set("X-Forwarded-For", "203.0.113.7")
	reqA.Header.Set("User-Agent", "ua-a")

	reqB := httptest.NewRequest("POST", "/api/login", nil)
	reqB.RemoteAddr = "172.20.0.10:8080"
	reqB.Header.Set("X-Forwarded-For", "203.0.113.8")
	reqB.Header.Set("User-Agent", "ua-b")

	keyA := getAuthRateLimitKey(reqA)
	keyB := getAuthRateLimitKey(reqB)
	if !strings.HasPrefix(keyA, "172.20.0.10:") {
		t.Fatalf("auth key A = %q, want prefix %q", keyA, "172.20.0.10:")
	}
	if !strings.HasPrefix(keyB, "172.20.0.10:") {
		t.Fatalf("auth key B = %q, want prefix %q", keyB, "172.20.0.10:")
	}
	if keyA == keyB {
		t.Fatalf("expected distinct auth keys when forwarded identity hints differ, got %q", keyA)
	}
}
