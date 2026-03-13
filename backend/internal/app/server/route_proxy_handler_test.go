package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"magpie/internal/domain"
)

func TestHandleExportProxiesStreamError_SanitizesClientMessage(t *testing.T) {
	recorder := httptest.NewRecorder()

	handleExportProxiesStreamError(recorder, errors.New("sql: password auth failed for user postgres"), false)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["error"] != "Could not export proxies" {
		t.Fatalf("error message = %q, want %q", payload["error"], "Could not export proxies")
	}
	if strings.Contains(strings.ToLower(recorder.Body.String()), "password") {
		t.Fatalf("response leaked internal details: %q", recorder.Body.String())
	}
}

func TestHandleExportProxiesStreamError_DoesNotWriteAfterStreamingBegan(t *testing.T) {
	recorder := httptest.NewRecorder()

	handleExportProxiesStreamError(recorder, errors.New("db stream aborted"), true)

	if recorder.Body.Len() != 0 {
		t.Fatalf("expected no body write after streaming started, got %q", recorder.Body.String())
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestRequeueProxy_ReturnsQueuedProxy(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-server-route-secret")

	originalGetProxy := getQueuedProxyForUser
	originalRemoveQueued := removeQueuedProxies
	originalEnqueue := enqueueProxiesNow
	t.Cleanup(func() {
		getQueuedProxyForUser = originalGetProxy
		removeQueuedProxies = originalRemoveQueued
		enqueueProxiesNow = originalEnqueue
	})

	var removed []domain.Proxy
	var queued []domain.Proxy
	getQueuedProxyForUser = func(userID uint, proxyID uint64) (*domain.Proxy, error) {
		proxy := domain.Proxy{
			ID:       42,
			IP:       "127.0.0.1",
			Port:     8080,
			Username: "user",
			Password: "pass",
			Users:    []domain.User{{ID: 7}},
		}
		proxy.GenerateHash()
		return &proxy, nil
	}
	removeQueuedProxies = func(proxies []domain.Proxy) error {
		removed = append([]domain.Proxy{}, proxies...)
		return nil
	}
	enqueueProxiesNow = func(proxies []domain.Proxy) error {
		queued = append([]domain.Proxy{}, proxies...)
		return nil
	}

	req := newAdminRequest(t, http.MethodPost, "/global/proxies/42/requeue", 7)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	requeueProxy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(removed) != 1 || len(queued) != 1 {
		t.Fatalf("removed=%d queued=%d, want 1/1", len(removed), len(queued))
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["message"] != "Proxy queued successfully" {
		t.Fatalf("message = %v, want success response", payload["message"])
	}
	if payload["proxy_id"] != float64(42) {
		t.Fatalf("proxy_id = %v, want 42", payload["proxy_id"])
	}
}

func TestRequeueProxy_ReturnsNotFoundWhenProxyMissing(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-server-route-secret")

	originalGetProxy := getQueuedProxyForUser
	t.Cleanup(func() {
		getQueuedProxyForUser = originalGetProxy
	})

	getQueuedProxyForUser = func(userID uint, proxyID uint64) (*domain.Proxy, error) {
		return nil, nil
	}

	req := newAdminRequest(t, http.MethodPost, "/global/proxies/404/requeue", 7)
	req.SetPathValue("id", "404")
	rec := httptest.NewRecorder()

	requeueProxy(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if payload["error"] != "Proxy not found" {
		t.Fatalf("error = %q, want not found message", payload["error"])
	}
}

func TestRequeueProxy_ReturnsServiceUnavailableOnQueueFailure(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-server-route-secret")

	originalGetProxy := getQueuedProxyForUser
	originalRemoveQueued := removeQueuedProxies
	t.Cleanup(func() {
		getQueuedProxyForUser = originalGetProxy
		removeQueuedProxies = originalRemoveQueued
	})

	getQueuedProxyForUser = func(userID uint, proxyID uint64) (*domain.Proxy, error) {
		proxy := domain.Proxy{
			ID:    proxyID,
			IP:    "127.0.0.1",
			Port:  8080,
			Users: []domain.User{{ID: 7}},
		}
		proxy.GenerateHash()
		return &proxy, nil
	}
	removeQueuedProxies = func(proxies []domain.Proxy) error {
		return errors.New("redis unavailable")
	}

	req := newAdminRequest(t, http.MethodPost, "/global/proxies/42/requeue", 7)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	requeueProxy(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if payload["error"] != "Failed to queue proxy" {
		t.Fatalf("error = %q, want failure message", payload["error"])
	}
}

func TestRequeueProxy_ReturnsBadRequestOnInvalidID(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-server-route-secret")

	req := newAdminRequest(t, http.MethodPost, "/global/proxies/not-a-number/requeue", 7)
	req.SetPathValue("id", "not-a-number")
	rec := httptest.NewRecorder()

	requeueProxy(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if payload["error"] != "Invalid proxy id" {
		t.Fatalf("error = %q, want invalid id message", payload["error"])
	}
}
