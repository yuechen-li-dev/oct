package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommandSucceeds(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "example.oct")
	if err := os.WriteFile(sourcePath, []byte("ignored for m0\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/oct", "run", sourcePath)
	cmd.Dir = filepath.Clean(filepath.Join("..", ".."))

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run command failed: %v\n%s", err, output)
	}

	if !strings.Contains(string(output), "hello from oct") {
		t.Fatalf("expected hello output, got %q", output)
	}
}

func TestBuildCommandSucceeds(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "example.oct")
	if err := os.WriteFile(sourcePath, []byte("ignored for m0\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/oct", "build", sourcePath)
	cmd.Dir = filepath.Clean(filepath.Join("..", ".."))

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build command failed: %v\n%s", err, output)
	}

	artifactPath := sourcePath + ".octbin"
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}

	if string(artifact) != "oct m0 placeholder artifact\n" {
		t.Fatalf("unexpected artifact body %q", artifact)
	}

	if !strings.Contains(string(output), "build succeeded: "+artifactPath) {
		t.Fatalf("expected build success output, got %q", output)
	}
}

func TestMissingFileFailsDeterministically(t *testing.T) {
	tempDir := t.TempDir()
	missingPath := filepath.Join(tempDir, "missing.oct")

	for _, command := range []string{"run", "build"} {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command("go", "run", "./cmd/oct", command, missingPath)
			cmd.Dir = filepath.Clean(filepath.Join("..", ".."))

			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected failure, got success with %q", output)
			}

			expected := command + " failed: source file not found: " + missingPath
			if !strings.Contains(string(output), expected) {
				t.Fatalf("expected deterministic error %q, got %q", expected, output)
			}
		})
	}
}
