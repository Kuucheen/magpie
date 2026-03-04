package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"magpie/internal/database"
	"magpie/internal/support"
)

func TestHealthz_ReturnsLivenessPayload(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	healthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var payload probeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload.Status != "ok" {
		t.Fatalf("status field = %q, want ok", payload.Status)
	}
	if payload.Components["process"].Status != componentStatusUp {
		t.Fatalf("process component status = %q, want %q", payload.Components["process"].Status, componentStatusUp)
	}
}

func TestReadyz_FailsWhenDatabaseUnavailable(t *testing.T) {
	prevDB := database.DB
	database.DB = nil
	t.Cleanup(func() {
		database.DB = prevDB
	})

	t.Setenv(envReadyzAllowRedisDegraded, "true")
	_ = support.CloseRedisClient()
	t.Cleanup(func() {
		_ = support.CloseRedisClient()
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	readyz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}

	var payload probeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Components["database"].Status != componentStatusDown {
		t.Fatalf("database component status = %q, want %q", payload.Components["database"].Status, componentStatusDown)
	}
}

func TestCheckRedisComponent_ReportsModeAndErrorDetails(t *testing.T) {
	_ = support.CloseRedisClient()
	t.Cleanup(func() {
		_ = support.CloseRedisClient()
	})

	t.Setenv(envReadyzAllowRedisDegraded, "false")
	t.Setenv("redisUrl", "redis://user:super-secret-pass@127.0.0.1:1")
	t.Setenv("REDIS_MODE", "single")

	component := checkRedisComponent(context.Background())
	if component.Status != componentStatusDown {
		t.Fatalf("redis component status = %q, want %q", component.Status, componentStatusDown)
	}
	if !strings.Contains(component.Details, "mode=single") {
		t.Fatalf("redis details = %q, want mode=single", component.Details)
	}
	if !strings.Contains(component.Details, "error_class=") {
		t.Fatalf("redis details = %q, expected error_class detail", component.Details)
	}
	if strings.Contains(component.Details, "super-secret-pass") {
		t.Fatalf("redis details leaked sensitive value: %q", component.Details)
	}
}
