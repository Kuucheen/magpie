package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
