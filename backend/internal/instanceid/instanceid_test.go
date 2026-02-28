package instanceid

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[45][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func resetForTest(t *testing.T) {
	t.Helper()
	ResetForTests()
	t.Cleanup(ResetForTests)
}

func TestGet_UsesConfiguredValue(t *testing.T) {
	resetForTest(t)
	t.Setenv(envInstanceID, "  primary-node ")
	t.Setenv(envInstanceIDFile, filepath.Join(t.TempDir(), "instance_id"))

	got := Get()
	if got != "primary-node" {
		t.Fatalf("Get() = %q, want %q", got, "primary-node")
	}
}

func TestGet_PersistsGeneratedIDAcrossResets(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "instance_id")
	t.Setenv(envInstanceID, "")
	t.Setenv(envInstanceIDFile, path)

	first := Get()
	if !uuidPattern.MatchString(first) {
		t.Fatalf("Get() = %q, want UUID format", first)
	}
	ResetForTests()
	second := Get()
	if second != first {
		t.Fatalf("expected same persisted instance id, got %q then %q", first, second)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted instance id: %v", err)
	}
	if stored := string(raw); stored != first+"\n" {
		t.Fatalf("persisted id file = %q, want %q", stored, first+"\\n")
	}
}

func TestGet_UsesPersistedIDFileValue(t *testing.T) {
	resetForTest(t)
	path := filepath.Join(t.TempDir(), "instance_id")
	t.Setenv(envInstanceID, "")
	t.Setenv(envInstanceIDFile, path)
	if err := os.WriteFile(path, []byte("node-from-disk\n"), instanceIDFileMode); err != nil {
		t.Fatalf("write instance id file: %v", err)
	}

	if got := Get(); got != "node-from-disk" {
		t.Fatalf("Get() = %q, want %q", got, "node-from-disk")
	}
}

func TestGet_StableScopedIDWithoutFilePath(t *testing.T) {
	resetForTest(t)
	t.Setenv(envInstanceID, "")
	t.Setenv(envInstanceIDFile, "")
	t.Setenv(envInstanceScope, "worker-a")

	first := Get()
	if !uuidPattern.MatchString(first) {
		t.Fatalf("Get() = %q, want UUID format", first)
	}
	ResetForTests()
	second := Get()
	if first != second {
		t.Fatalf("expected stable scoped id across resets, got %q then %q", first, second)
	}
}

func TestGet_DifferentBackendPortsProduceDifferentScopedIDs(t *testing.T) {
	resetForTest(t)
	t.Setenv(envInstanceID, "")
	t.Setenv(envInstanceIDFile, "")
	t.Setenv(envInstanceScope, "")
	t.Setenv(envBackendPort, "5656")

	first := Get()
	ResetForTests()
	t.Setenv(envBackendPort, "5757")
	second := Get()
	if first == second {
		t.Fatalf("expected different IDs for different backend ports, got %q", first)
	}
}

func TestGenerateStableFallbackID_IgnoresWorkingDirectory(t *testing.T) {
	resetForTest(t)
	t.Setenv(envInstanceScope, "")
	t.Setenv(envInstanceID, "")
	t.Setenv(envInstanceIDFile, "")

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	first := generateStableFallbackID()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	second := generateStableFallbackID()
	if first != second {
		t.Fatalf("expected cwd-independent fallback id, got %q then %q", first, second)
	}
}
