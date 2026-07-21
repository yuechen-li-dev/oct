package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRepoRootSkipsLowercaseImplementationDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "libraries", "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Libraries"), 0755); err != nil {
		t.Fatal(err)
	}
	got := detectRepoRoot(filepath.Join(root, "internal", "libraries", "nested"))
	if got != root {
		t.Fatalf("detectRepoRoot() = %q, want %q", got, root)
	}
}
