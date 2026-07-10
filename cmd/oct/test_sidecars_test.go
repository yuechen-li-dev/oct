package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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
	testOctBinOnce     sync.Once
	testOctBinDir      string
	testOctBinPath     string
	testOctBinErr      error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if testSidecarDirPath != "" {
		_ = os.RemoveAll(testSidecarDirPath)
	}
	if testOctBinDir != "" {
		_ = os.RemoveAll(testOctBinDir)
	}
	os.Exit(code)
}

func sharedTestSidecarDir(t *testing.T, names ...string) string {
	t.Helper()
	requireSlowOctxiliary(t)
	if envDir, ok := existingSidecarDir(names...); ok {
		return envDir
	}
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

func existingSidecarDir(names ...string) (string, bool) {
	wrapperPath := os.Getenv("OCT_WRAPPER_PATH")
	if wrapperPath == "" {
		return "", false
	}
	info, err := os.Stat(wrapperPath)
	if err != nil || !info.IsDir() {
		return "", false
	}
	for _, name := range names {
		command := octxiliaryCommandName(name)
		if _, err := os.Stat(filepath.Join(wrapperPath, sidecarBinaryName(command))); err != nil {
			return "", false
		}
	}
	return wrapperPath, true
}

func executeCLIWithSidecars(t *testing.T, command string, sourcePath string, names ...string) (string, string, error) {
	t.Helper()
	repo := filepath.Join("..", "..")
	target := repoRelativeTestPath(t, repo, sourcePath)
	args := []string{command, target}
	cmd := exec.Command(sharedTestOctBinary(t), args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+sharedTestSidecarDir(t, names...))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func executeCLIWithSidecarsInDir(t *testing.T, workDir string, command string, sourcePath string, names ...string) (string, string, error) {
	t.Helper()
	return executeOctWithSidecarsInDir(t, workDir, append([]string{command}, sourcePath), names...)
}

func executeOctWithSidecarsInDir(t *testing.T, workDir string, args []string, names ...string) (string, string, error) {
	t.Helper()
	return executeOctWithCustomWrapperPathInDir(t, workDir, sharedTestSidecarDir(t, names...), args)
}

func executeOctWithCustomWrapperPathInDir(t *testing.T, workDir string, wrapperPath string, args []string) (string, string, error) {
	t.Helper()
	requireSlowOctxiliary(t)
	cmd := exec.Command(sharedTestOctBinary(t), args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"OCT_WRAPPER_PATH="+wrapperPath,
		"OCT_PKG_CACHE_DIR="+filepath.Join(workDir, ".oct", "pkg-cache"),
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func sharedTestOctBinary(t *testing.T) string {
	t.Helper()
	testOctBinOnce.Do(func() {
		testOctBinDir, testOctBinErr = os.MkdirTemp("", "oct-test-bin-*")
		if testOctBinErr != nil {
			return
		}
		repo := filepath.Join("..", "..")
		name := "oct-test"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		testOctBinPath = filepath.Join(testOctBinDir, name)
		build := exec.Command("go", "build", "-o", testOctBinPath, "./cmd/oct")
		build.Dir = repo
		if out, err := build.CombinedOutput(); err != nil {
			testOctBinErr = fmt.Errorf("build oct test binary: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	})
	if testOctBinErr != nil {
		t.Fatalf("%v", testOctBinErr)
	}
	return testOctBinPath
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

func assertNoCompiledFallback(t *testing.T, stdout string, stderr string) {
	t.Helper()
	combined := stdout + stderr
	assertNoMissingSidecarFallback(t, stdout, stderr)
	if strings.Contains(combined, "compiled mode does not yet support builtin") {
		t.Fatalf("expected compiled wrapper support, got unsupported-builtin output:\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if fallback, ok := executionSummaryCount(t, stdout, "interpreted fallback"); ok && fallback != 0 {
		t.Fatalf("expected no interpreted fallback, got %d:\nstdout:\n%s\nstderr:\n%s", fallback, stdout, stderr)
	}
}

func assertCompiledCountAtLeast(t *testing.T, stdout string, min int) {
	t.Helper()
	compiled, ok := executionSummaryCount(t, stdout, "compiled")
	if !ok {
		t.Fatalf("expected execution summary with compiled count in stdout, got:\n%s", stdout)
	}
	if compiled < min {
		t.Fatalf("expected compiled count >= %d, got %d:\n%s", min, compiled, stdout)
	}
}

func executionSummaryCount(t *testing.T, stdout string, label string) (int, bool) {
	t.Helper()
	pattern := regexp.MustCompile(regexp.QuoteMeta(label) + `: ([0-9]+)`)
	match := pattern.FindStringSubmatch(stdout)
	if match == nil {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("parse %s count %q: %v", label, match[1], err)
	}
	return value, true
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
	requireSlowOctxiliary(t)
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
