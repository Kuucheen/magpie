package support

import (
	"magpie/internal/instanceid"
	"regexp"
	"sync"
	"testing"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func resetInstanceIdentityCacheForTests() {
	instanceid.ResetForTests()
	instanceNameOnce = sync.Once{}
	instanceNameVal = ""
	instanceRegOnce = sync.Once{}
	instanceRegVal = ""
}

func TestGetInstanceID_UsesConfiguredValue(t *testing.T) {
	resetInstanceIdentityCacheForTests()
	t.Cleanup(resetInstanceIdentityCacheForTests)

	t.Setenv("MAGPIE_INSTANCE_ID_FILE", t.TempDir()+"/instance_id")
	t.Setenv(envInstanceID, "  replica-a  ")
	got := GetInstanceID()
	if got != "replica-a" {
		t.Fatalf("GetInstanceID() = %q, want %q", got, "replica-a")
	}
	if again := GetInstanceID(); again != got {
		t.Fatalf("GetInstanceID() changed across calls: %q -> %q", got, again)
	}
}

func TestGetInstanceID_GeneratesUUIDWhenNotConfigured(t *testing.T) {
	resetInstanceIdentityCacheForTests()
	t.Cleanup(resetInstanceIdentityCacheForTests)

	t.Setenv("MAGPIE_INSTANCE_ID_FILE", t.TempDir()+"/instance_id")
	t.Setenv(envInstanceID, "")
	got := GetInstanceID()
	if !uuidV4Pattern.MatchString(got) {
		t.Fatalf("GetInstanceID() = %q, want UUIDv4 format", got)
	}
	if got == "default" {
		t.Fatal("GetInstanceID() returned deprecated fallback value \"default\"")
	}
	if again := GetInstanceID(); again != got {
		t.Fatalf("GetInstanceID() changed across calls: %q -> %q", got, again)
	}
}

func TestGetInstanceID_RetainsGeneratedIDAfterCacheReset(t *testing.T) {
	resetInstanceIdentityCacheForTests()
	t.Cleanup(resetInstanceIdentityCacheForTests)

	t.Setenv("MAGPIE_INSTANCE_ID_FILE", t.TempDir()+"/instance_id")
	t.Setenv(envInstanceID, "")
	first := GetInstanceID()
	resetInstanceIdentityCacheForTests()
	second := GetInstanceID()
	if first != second {
		t.Fatalf("expected persisted instance id after reset, got %q then %q", first, second)
	}
}
