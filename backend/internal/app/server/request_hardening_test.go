package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsSameHostOrigin_StrictOriginMatching(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://api.example.com/test", nil)
	request.Host = "api.example.com"

	if !isSameHostOrigin("http://api.example.com", request) {
		t.Fatal("expected same host+scheme+port origin to be allowed")
	}
}

func TestIsSameHostOrigin_RejectsSchemeMismatch(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://api.example.com/test", nil)
	request.Host = "api.example.com"

	if isSameHostOrigin("https://api.example.com", request) {
		t.Fatal("expected different scheme to be rejected")
	}
}

func TestIsSameHostOrigin_RejectsPortMismatch(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://api.example.com/test", nil)
	request.Host = "api.example.com:8080"

	if isSameHostOrigin("http://api.example.com:9090", request) {
		t.Fatal("expected different port to be rejected")
	}
}

func TestIsSameHostOrigin_UsesForwardedProto(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://api.example.com/test", nil)
	request.Host = "api.example.com"
	request.Header.Set("X-Forwarded-Proto", "https")

	if !isSameHostOrigin("https://api.example.com", request) {
		t.Fatal("expected forwarded https scheme with matching host/port to be allowed")
	}
}
