package scraper

import (
	"errors"
	"magpie/internal/support"
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
)

func TestContentLengthExceedsLimit(t *testing.T) {
	headers := proto.NetworkHeaders{
		"Content-Length": gson.New("1024"),
	}

	if !contentLengthExceedsLimit(headers, 512) {
		t.Fatal("expected content length to exceed limit")
	}
	if contentLengthExceedsLimit(headers, 1024) {
		t.Fatal("did not expect content length to exceed equal limit")
	}
	if contentLengthExceedsLimit(headers, 2048) {
		t.Fatal("did not expect content length to exceed larger limit")
	}
}

func TestContentLengthExceedsLimit_GracefulOnMissingOrInvalidHeader(t *testing.T) {
	if contentLengthExceedsLimit(proto.NetworkHeaders{}, 1024) {
		t.Fatal("missing content-length should not exceed limit")
	}
	if contentLengthExceedsLimit(proto.NetworkHeaders{"Content-Length": gson.New("invalid")}, 1024) {
		t.Fatal("invalid content-length should not exceed limit")
	}
	if !contentLengthExceedsLimit(proto.NetworkHeaders{}, 0) {
		t.Fatal("non-positive limit should always be treated as exceeded")
	}
}

func TestPagePoolCapacityMatchesMaxScraperPages(t *testing.T) {
	if got := cap(pagePool); got != maxScraperPages {
		t.Fatalf("page pool capacity = %d, want %d", got, maxScraperPages)
	}
}

func TestValidateScrapeTarget_BlocksPrivateNetworkTargets(t *testing.T) {
	err := validateScrapeTarget("http://127.0.0.1:8080", 0)
	if !errors.Is(err, support.ErrUnsafeOutboundTarget) {
		t.Fatalf("validateScrapeTarget err = %v, want ErrUnsafeOutboundTarget", err)
	}
}

func TestValidateScrapeRuntimeURL_RejectsPrivateTargets(t *testing.T) {
	err := validateScrapeRuntimeURL("http://localhost/internal", 0)
	if !errors.Is(err, support.ErrUnsafeOutboundTarget) {
		t.Fatalf("validateScrapeRuntimeURL err = %v, want ErrUnsafeOutboundTarget", err)
	}
}

func TestValidateScrapeRuntimeURL_IgnoresNonHTTPSchemes(t *testing.T) {
	if err := validateScrapeRuntimeURL("data:text/plain,hello", 0); err != nil {
		t.Fatalf("validateScrapeRuntimeURL non-http should pass, got %v", err)
	}
}

func TestValidateScrapeRemoteIP_RejectsPrivateAddresses(t *testing.T) {
	err := validateScrapeRemoteIP("127.0.0.1")
	if !errors.Is(err, support.ErrUnsafeOutboundTarget) {
		t.Fatalf("validateScrapeRemoteIP err = %v, want ErrUnsafeOutboundTarget", err)
	}
}
