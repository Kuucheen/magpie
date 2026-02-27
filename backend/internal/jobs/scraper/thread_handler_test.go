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

func TestResolveScraperPagePoolCaps_DefaultAndClamp(t *testing.T) {
	t.Setenv(envScraperPagePoolMin, "")
	t.Setenv(envScraperPagePoolMax, "")
	minPages, maxPages := resolveScraperPagePoolCaps()
	if minPages != defaultScraperPagePoolMin || maxPages != defaultScraperPagePoolMax {
		t.Fatalf("caps = (%d,%d), want (%d,%d)", minPages, maxPages, defaultScraperPagePoolMin, defaultScraperPagePoolMax)
	}

	t.Setenv(envScraperPagePoolMin, "0")
	t.Setenv(envScraperPagePoolMax, "5000")
	minPages, maxPages = resolveScraperPagePoolCaps()
	if minPages != 1 || maxPages != maxScraperPages {
		t.Fatalf("caps = (%d,%d), want (1,%d)", minPages, maxPages, maxScraperPages)
	}

	t.Setenv(envScraperPagePoolMin, "200")
	t.Setenv(envScraperPagePoolMax, "50")
	minPages, maxPages = resolveScraperPagePoolCaps()
	if minPages != 50 || maxPages != 50 {
		t.Fatalf("caps = (%d,%d), want (50,50)", minPages, maxPages)
	}
}

func TestCalculateRequiredPages_PerInstanceAndCap(t *testing.T) {
	required := calculateRequiredPages(
		2000, // total sites
		4,    // replicas
		1000, // timeout ms
		0,    // retries
		1000, // interval ms
		1,
		maxScraperPages,
	)
	if required != 500 {
		t.Fatalf("required pages = %d, want 500", required)
	}

	capped := calculateRequiredPages(
		2000,
		4,
		1000,
		0,
		1000,
		1,
		100,
	)
	if capped != 100 {
		t.Fatalf("capped pages = %d, want 100", capped)
	}
}
