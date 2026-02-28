package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithRequestID_UsesIncomingHeader(t *testing.T) {
	handler := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := requestIDFromRequest(r); got != "req-123" {
			t.Fatalf("requestIDFromRequest = %q, want req-123", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/checkLogin", nil)
	req.Header.Set(requestIDHeader, "req-123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get(requestIDHeader); got != "req-123" {
		t.Fatalf("response request id = %q, want req-123", got)
	}
}

func TestWithRequestID_GeneratesHeaderWhenMissing(t *testing.T) {
	handler := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestIDFromRequest(r) == "" {
			t.Fatal("expected generated request id")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rr.Header().Get(requestIDHeader); got == "" {
		t.Fatal("expected response request id header")
	}
}

func TestWithPanicRecovery_ReturnsInternalServerError(t *testing.T) {
	handler := withRequestID(withAccessLog(withPanicRecovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rr.Body.String(), "Internal server error") {
		t.Fatalf("expected sanitized error response, got %q", rr.Body.String())
	}
}
