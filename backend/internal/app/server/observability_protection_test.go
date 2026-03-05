package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"magpie/internal/config"
)

func TestObservabilityProtection_LocalDefaultAllowsPublic(t *testing.T) {
	t.Setenv("APP_ENV", "development")

	handler := withObservabilityProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestObservabilityProtection_ProductionRequiresTokenOrLoopback(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv(envAllowPublicObservabilityEndpoints, "false")
	t.Setenv(envObservabilityToken, "secret-token")

	handler := withObservabilityProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	deniedReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	deniedReq.RemoteAddr = "203.0.113.10:3000"
	deniedRR := httptest.NewRecorder()
	handler.ServeHTTP(deniedRR, deniedReq)
	if deniedRR.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", deniedRR.Code, http.StatusForbidden)
	}

	allowedReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	allowedReq.RemoteAddr = "203.0.113.10:3000"
	allowedReq.Header.Set(headObservabilityToken, "secret-token")
	allowedRR := httptest.NewRecorder()
	handler.ServeHTTP(allowedRR, allowedReq)
	if allowedRR.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", allowedRR.Code, http.StatusNoContent)
	}
}

func TestObservabilityProtection_ProductionAllowsLoopbackWithoutToken(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv(envAllowPublicObservabilityEndpoints, "false")
	t.Setenv(envObservabilityToken, "")

	handler := withObservabilityProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.RemoteAddr = "127.0.0.1:9090"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestObservabilityProtection_UsesInProductionModeFlagDefault(t *testing.T) {
	prev := config.InProductionMode
	config.SetProductionMode(true)
	t.Cleanup(func() {
		config.SetProductionMode(prev)
	})

	t.Setenv("APP_ENV", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("GO_ENV", "")
	t.Setenv("MAGPIE_ENV", "")
	t.Setenv(envAllowPublicObservabilityEndpoints, "")
	t.Setenv(envObservabilityToken, "")

	handler := withObservabilityProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}
