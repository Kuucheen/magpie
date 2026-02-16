package checker

import (
	"testing"

	"magpie/internal/domain"
	"magpie/internal/support"
)

func TestCheckProxyWithRetries_PerformsInitialAttemptWhenRetriesZero(t *testing.T) {
	proxy := domain.Proxy{
		IP:   "127.0.0.1",
		Port: 8080,
	}
	judge := &domain.Judge{
		FullString: "://invalid-url",
	}

	html, err, _, attempt := CheckProxyWithRetries(proxy, judge, "http", support.TransportTCP, 100, 0)
	if err == nil {
		t.Fatalf("expected at least one attempt and an error for invalid judge URL, got nil error and html=%q", html)
	}
	if attempt != 0 {
		t.Fatalf("expected first attempt index 0 for retries=0, got %d", attempt)
	}
}
