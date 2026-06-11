package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		command := octxiliaryCommandName(name)
		if _, ok := testSidecarBuilt[command]; ok {
			continue
		}
		outPath := filepath.Join(testSidecarDirPath, sidecarBinaryName(command))
		build := exec.Command("go", "build", "-o", outPath, "./cmd/"+command)
		build.Dir = repo
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", command, err, strings.TrimSpace(string(out)))
		}
		testSidecarBuilt[command] = struct{}{}
	}
	return testSidecarDirPath
}

func executeCLIWithSidecars(t *testing.T, command string, sourcePath string, names ...string) (string, string, error) {
	t.Helper()
	repo := filepath.Join("..", "..")
	target := repoRelativeTestPath(t, repo, sourcePath)
	args := []string{"run", "./cmd/oct", command, target}
	cmd := exec.Command("go", args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+sharedTestSidecarDir(t, names...))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func repoRelativeTestPath(t *testing.T, repo string, sourcePath string) string {
	t.Helper()
	if filepath.IsAbs(sourcePath) {
		return sourcePath
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		t.Fatalf("resolve source path %s: %v", sourcePath, err)
	}
	rel, err := filepath.Rel(absRepo, absSource)
	if err != nil {
		t.Fatalf("make %s relative to repo root %s: %v", sourcePath, repo, err)
	}
	return rel
}

func assertNoMissingSidecarFallback(t *testing.T, stdout string, stderr string) {
	t.Helper()
	combined := stdout + stderr
	if strings.Contains(combined, "sidecar not found") || strings.Contains(combined, "not found; set OCT_WRAPPER_PATH") {
		t.Fatalf("expected wrapper sidecars to be discoverable, got missing-sidecar output:\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
}

func octxiliaryCommandName(name string) string {
	if strings.HasPrefix(name, "octxiliary-") {
		return name
	}
	return "octxiliary-" + name
}

func sidecarBinaryName(command string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(command), ".exe") {
		return command + ".exe"
	}
	return command
}

func buildTestSidecarsInDir(t *testing.T, binDir string, names ...string) {
	t.Helper()
	repo := filepath.Join("..", "..")
	for _, name := range names {
		command := octxiliaryCommandName(name)
		outPath := filepath.Join(binDir, sidecarBinaryName(command))
		build := exec.Command("go", "build", "-o", outPath, "./cmd/"+command)
		build.Dir = repo
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", command, err, strings.TrimSpace(string(out)))
		}
	}
}
