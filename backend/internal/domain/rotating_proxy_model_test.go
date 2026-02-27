package domain

import (
	"magpie/internal/instanceid"
	"testing"
)

func TestDefaultInstanceID_UsesSharedInstanceIdentity(t *testing.T) {
	t.Setenv("MAGPIE_INSTANCE_ID", "")
	instanceid.ResetForTests()
	t.Cleanup(instanceid.ResetForTests)

	got := defaultInstanceID()
	want := instanceid.Get()
	if got != want {
		t.Fatalf("defaultInstanceID() = %q, want shared instance id %q", got, want)
	}
}
