package scraper

import "testing"

func TestResolvePostProcessWorkers_DefaultAndClamp(t *testing.T) {
	t.Setenv(envPostProcessWorkers, "")
	if got := resolvePostProcessWorkers(); got != defaultPostProcessWorkers {
		t.Fatalf("workers = %d, want %d", got, defaultPostProcessWorkers)
	}

	t.Setenv(envPostProcessWorkers, "0")
	if got := resolvePostProcessWorkers(); got != 1 {
		t.Fatalf("workers = %d, want 1", got)
	}

	t.Setenv(envPostProcessWorkers, "999")
	if got := resolvePostProcessWorkers(); got != maxPostProcessWorkers {
		t.Fatalf("workers = %d, want %d", got, maxPostProcessWorkers)
	}
}

func TestResolvePostProcessQueueSize_DefaultAndClamp(t *testing.T) {
	t.Setenv(envPostProcessQueueSize, "")
	if got := resolvePostProcessQueueSize(); got != defaultPostProcessQueueSize {
		t.Fatalf("queue size = %d, want %d", got, defaultPostProcessQueueSize)
	}

	t.Setenv(envPostProcessQueueSize, "0")
	if got := resolvePostProcessQueueSize(); got != 1 {
		t.Fatalf("queue size = %d, want 1", got)
	}

	t.Setenv(envPostProcessQueueSize, "999999")
	if got := resolvePostProcessQueueSize(); got != maxPostProcessQueueSize {
		t.Fatalf("queue size = %d, want %d", got, maxPostProcessQueueSize)
	}
}
