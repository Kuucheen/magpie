package scraper

import (
	"errors"
	"magpie/internal/support"
	"testing"
	"time"
)

func TestCheckRobotsAllowance_BlocksUnsafeTargets(t *testing.T) {
	result, err := CheckRobotsAllowance("http://127.0.0.1:8080", time.Second)
	if !errors.Is(err, support.ErrUnsafeOutboundTarget) {
		t.Fatalf("CheckRobotsAllowance err = %v, want ErrUnsafeOutboundTarget", err)
	}
	if result.Allowed {
		t.Fatal("unsafe target must not be marked allowed")
	}
}
