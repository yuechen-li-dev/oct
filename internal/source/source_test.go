package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsFile(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "example.oct")
	if err := os.WriteFile(sourcePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	file, err := Load(sourcePath)
	if err != nil {
		t.Fatalf("load source: %v", err)
	}

	if file.Path != sourcePath {
		t.Fatalf("expected path %q, got %q", sourcePath, file.Path)
	}

	if file.Text != "hello\n" {
		t.Fatalf("expected text %q, got %q", "hello\n", file.Text)
	}
}

func TestLoadMissingFileReturnsDeterministicError(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "missing.oct")

	_, err := Load(sourcePath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expected := "source file not found: " + sourcePath
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}
