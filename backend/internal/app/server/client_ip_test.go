package server

import (
	"net/http/httptest"
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
