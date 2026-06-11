package interpret

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSidecarInDirWindowsExeSuffix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "octxiliary-csv.exe")
	if err := os.WriteFile(path, []byte("sidecar"), 0o755); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	got, ok := resolveSidecarInDir(dir, "octxiliary-csv", "windows")
	if !ok {
		t.Fatal("expected Windows resolver to find .exe sidecar")
	}
	if got != path {
		t.Fatalf("resolved path = %q, want %q", got, path)
	}
}

func TestResolveSidecarFromWrapperPathWindowsExplicitExe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "octxiliary-json.exe")
	if err := os.WriteFile(path, []byte("sidecar"), 0o755); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	got, ok := resolveSidecarFromWrapperPath(path, "octxiliary-json", "windows")
	if !ok {
		t.Fatal("expected Windows resolver to accept explicit .exe wrapper path")
	}
	if got != path {
		t.Fatalf("resolved path = %q, want %q", got, path)
	}
}

func TestResolveSidecarInDirNonWindowsDoesNotTryExeSuffix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "octxiliary-csv.exe")
	if err := os.WriteFile(path, []byte("sidecar"), 0o755); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	if got, ok := resolveSidecarInDir(dir, "octxiliary-csv", "linux"); ok {
		t.Fatalf("expected non-Windows resolver not to find .exe suffix, got %q", got)
	}
}

func TestResolveSidecarFromWrapperPathRejectsMismatchedExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "octxiliary-other.exe")
	if err := os.WriteFile(path, []byte("sidecar"), 0o755); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	if got, ok := resolveSidecarFromWrapperPath(path, "octxiliary-json", "windows"); ok {
		t.Fatalf("expected mismatched explicit wrapper path to be rejected, got %q", got)
	}
}
