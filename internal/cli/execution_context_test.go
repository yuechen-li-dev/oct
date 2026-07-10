package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteWithContextUsesExplicitWorkingDirectoryWithoutChdir(t *testing.T) {
	t.Parallel()
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(t.TempDir(), "ContextPackage")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err = ExecuteWithContext([]string{"init", "library"}, ExecutionContext{WorkingDir: project, Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("init: %v (%s)", err, stderr.String())
	}
	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("process cwd changed from %q to %q", before, after)
	}
	if _, err := os.Stat(filepath.Join(project, "manifest.oct")); err != nil {
		t.Fatalf("manifest not written in context dir: %v", err)
	}
}

func TestExecuteWithContextUsesExplicitCacheRoot(t *testing.T) {
	t.Parallel()
	cacheRoot := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := ExecuteWithContext([]string{"pkg", "list"}, ExecutionContext{WorkingDir: t.TempDir(), CacheRoot: cacheRoot, Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("pkg list: %v (%s)", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), cacheRoot) {
		t.Fatalf("cache root not reported: %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(cacheRoot, "index.json")); err != nil {
		t.Fatalf("explicit cache root not initialized: %v", err)
	}
}
