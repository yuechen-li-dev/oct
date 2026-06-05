package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	testSidecarDirOnce sync.Once
	testSidecarDirPath string
	testSidecarDirErr  error
	testSidecarMu      sync.Mutex
	testSidecarBuilt   = map[string]struct{}{}
)

func TestMain(m *testing.M) {
	code := m.Run()
	if testSidecarDirPath != "" {
		_ = os.RemoveAll(testSidecarDirPath)
	}
	os.Exit(code)
}

func sharedTestSidecarDir(t *testing.T, names ...string) string {
	t.Helper()
	testSidecarDirOnce.Do(func() {
		testSidecarDirPath, testSidecarDirErr = os.MkdirTemp("", "oct-sidecars-*")
	})
	if testSidecarDirErr != nil {
		t.Fatalf("create shared sidecar dir: %v", testSidecarDirErr)
	}
	repo := filepath.Join("..", "..")
	testSidecarMu.Lock()
	defer testSidecarMu.Unlock()
	for _, name := range names {
		if _, ok := testSidecarBuilt[name]; ok {
			continue
		}
		outPath := filepath.Join(testSidecarDirPath, name)
		build := exec.Command("go", "build", "-o", outPath, "./cmd/"+name)
		build.Dir = repo
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", name, err, strings.TrimSpace(string(out)))
		}
		testSidecarBuilt[name] = struct{}{}
	}
	return testSidecarDirPath
}

func buildTestSidecarsInDir(t *testing.T, binDir string, names ...string) {
	t.Helper()
	repo := filepath.Join("..", "..")
	for _, name := range names {
		outPath := filepath.Join(binDir, name)
		build := exec.Command("go", "build", "-o", outPath, "./cmd/"+name)
		build.Dir = repo
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", name, err, strings.TrimSpace(string(out)))
		}
	}
}
