package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"magpie/internal/api/dto"
	"magpie/internal/auth"
	"magpie/internal/domain"
)

func TestRequeueScrapeSource_ReturnsQueuedSource(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-server-route-secret")

	originalGetDetail := getScrapeSourceDetailForUser
	originalEnqueue := enqueueScrapeSites
	originalRemove := removeScrapeSitesFromQueue
	t.Cleanup(func() {
		getScrapeSourceDetailForUser = originalGetDetail
		enqueueScrapeSites = originalEnqueue
		removeScrapeSitesFromQueue = originalRemove
	})

	var removed []domain.ScrapeSite
	var queued []domain.ScrapeSite
	getScrapeSourceDetailForUser = func(userID uint, sourceID uint64) (*dto.ScrapeSiteDetail, error) {
		if userID != 7 {
			t.Fatalf("userID = %d, want 7", userID)
		}
		if sourceID != 42 {
			t.Fatalf("sourceID = %d, want 42", sourceID)
		}
		return &dto.ScrapeSiteDetail{
			Id:  42,
			Url: "https://example.com/scrape-list.txt",
		}, nil
	}
	removeScrapeSitesFromQueue = func(sites []domain.ScrapeSite) error {
		removed = append([]domain.ScrapeSite{}, sites...)
		return nil
	}
	enqueueScrapeSites = func(sites []domain.ScrapeSite) error {
		queued = append([]domain.ScrapeSite{}, sites...)
		return nil
	}

	req := newAdminRequest(t, http.MethodPost, "/global/scrapeSources/42/requeue", 7)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	requeueScrapeSource(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if len(queued) != 1 {
		t.Fatalf("queued count = %d, want 1", len(queued))
	}
	if len(removed) != 1 {
		t.Fatalf("removed count = %d, want 1", len(removed))
	}
	if queued[0].ID != 42 || queued[0].URL != "https://example.com/scrape-list.txt" {
		t.Fatalf("queued site = %+v, want id=42 url=https://example.com/scrape-list.txt", queued[0])
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["message"] != "Scrape source queued successfully" {
		t.Fatalf("message = %v, want success response", payload["message"])
	}
	if payload["source_id"] != float64(42) {
		t.Fatalf("source_id = %v, want 42", payload["source_id"])
	}
}

func TestRequeueScrapeSource_ReturnsNotFoundWhenSourceMissing(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-server-route-secret")

	originalGetDetail := getScrapeSourceDetailForUser
	t.Cleanup(func() {
		getScrapeSourceDetailForUser = originalGetDetail
	})

	getScrapeSourceDetailForUser = func(userID uint, sourceID uint64) (*dto.ScrapeSiteDetail, error) {
		return nil, nil
	}

	req := newAdminRequest(t, http.MethodPost, "/global/scrapeSources/404/requeue", 7)
	req.SetPathValue("id", "404")
	rec := httptest.NewRecorder()

	requeueScrapeSource(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if payload["error"] != "Scrape source not found" {
		t.Fatalf("error = %q, want not found message", payload["error"])
	}
}

func TestRequeueScrapeSource_ReturnsServiceUnavailableOnQueueFailure(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-server-route-secret")

	originalGetDetail := getScrapeSourceDetailForUser
	originalEnqueue := enqueueScrapeSites
	originalRemove := removeScrapeSitesFromQueue
	t.Cleanup(func() {
		getScrapeSourceDetailForUser = originalGetDetail
		enqueueScrapeSites = originalEnqueue
		removeScrapeSitesFromQueue = originalRemove
	})

	getScrapeSourceDetailForUser = func(userID uint, sourceID uint64) (*dto.ScrapeSiteDetail, error) {
		return &dto.ScrapeSiteDetail{
			Id:  sourceID,
			Url: "https://example.com/failing-source.txt",
		}, nil
	}
	removeScrapeSitesFromQueue = func(sites []domain.ScrapeSite) error {
		return nil
	}
	enqueueScrapeSites = func(sites []domain.ScrapeSite) error {
		return errors.New("redis unavailable")
	}

	req := newAdminRequest(t, http.MethodPost, "/global/scrapeSources/42/requeue", 7)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	requeueScrapeSource(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if payload["error"] != "Failed to queue scrape source" {
		t.Fatalf("error = %q, want failure message", payload["error"])
	}
}

func TestRequeueScrapeSource_ReturnsServiceUnavailableOnRemoveFailure(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-server-route-secret")

	originalGetDetail := getScrapeSourceDetailForUser
	originalRemove := removeScrapeSitesFromQueue
	t.Cleanup(func() {
		getScrapeSourceDetailForUser = originalGetDetail
		removeScrapeSitesFromQueue = originalRemove
	})

	getScrapeSourceDetailForUser = func(userID uint, sourceID uint64) (*dto.ScrapeSiteDetail, error) {
		return &dto.ScrapeSiteDetail{
			Id:  sourceID,
			Url: "https://example.com/failing-source.txt",
		}, nil
	}
	removeScrapeSitesFromQueue = func(sites []domain.ScrapeSite) error {
		return errors.New("redis unavailable")
	}

	req := newAdminRequest(t, http.MethodPost, "/global/scrapeSources/42/requeue", 7)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	requeueScrapeSource(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if payload["error"] != "Failed to queue scrape source" {
		t.Fatalf("error = %q, want failure message", payload["error"])
	}
}

func TestRequeueScrapeSource_ReturnsBadRequestOnInvalidID(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-server-route-secret")

	req := newAdminRequest(t, http.MethodPost, "/global/scrapeSources/not-a-number/requeue", 7)
	req.SetPathValue("id", "not-a-number")
	rec := httptest.NewRecorder()

	requeueScrapeSource(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if payload["error"] != "Invalid scrape source id" {
		t.Fatalf("error = %q, want invalid id message", payload["error"])
	}
}

func newAdminRequest(t *testing.T, method, path string, userID uint) *http.Request {
	t.Helper()

	token, err := auth.GenerateJWT(userID, "admin")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}
