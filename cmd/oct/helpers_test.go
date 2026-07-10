package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/build"
	"github.com/yuechen-li-dev/oct/internal/cli"
)

var boundaryTestSlots = make(chan struct{}, 4)

// parallelBoundaryTest caps compiler/external-process concurrency while still
// allowing isolated integration and toolchain cases to overlap.
func parallelBoundaryTest(t *testing.T) {
	t.Helper()
	t.Parallel()
	boundaryTestSlots <- struct{}{}
	t.Cleanup(func() { <-boundaryTestSlots })
}

func nativeArtifactPath(sourcePath string) string {
	return build.OutputPath(sourcePath, build.ArtifactExecutable, build.HostTarget())
}

func octStringLiteralPath(path string) string {
	return strconv.Quote(strings.ReplaceAll(filepath.ToSlash(path), `\`, "/"))
}

func executeCLIArgs(args ...string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func writeSourceFile(t *testing.T, name string, source string) string {
	t.Helper()
	sourcePath := filepath.Join(t.TempDir(), name)
	trimmed := strings.TrimSpace(source)
	if strings.HasSuffix(name, ".oct") && !strings.HasPrefix(trimmed, "package ") {
		source = "package Main\n" + source
	}
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return sourcePath
}

func executeCLI(command string, sourcePath string) (string, string, error) {
	return executeCLIArgs(command, sourcePath)
}

func executeCLIInDir(dir string, args ...string) (string, string, error) {
	return executeCLIInDirWithCache(dir, "", args...)
}

func executeCLIInDirWithCache(dir string, cacheRoot string, args ...string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.ExecuteWithContext(args, cli.ExecutionContext{WorkingDir: dir, CacheRoot: cacheRoot, Stdout: &stdout, Stderr: &stderr})
	return stdout.String(), stderr.String(), err
}

func executeCLIWithCache(cacheRoot string, args ...string) (string, string, error) {
	return executeCLIInDirWithCache(".", cacheRoot, args...)
}

func assertFileContains(t *testing.T, path string, snippets ...string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	content := string(contents)
	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected %s to contain %q, got:\n%s", path, snippet, content)
		}
	}
}

func assertOutputContains(t *testing.T, output string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got:\n%s", snippet, output)
		}
	}
}

func copyDir(t *testing.T, src string, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read dir %s: %v", src, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			copyDir(t, srcPath, dstPath)
			continue
		}
		data, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			t.Fatalf("read file %s: %v", srcPath, readErr)
		}
		if writeErr := os.WriteFile(dstPath, data, 0o644); writeErr != nil {
			t.Fatalf("write file %s: %v", dstPath, writeErr)
		}
	}
}

func copyTestFixture(t *testing.T, src string, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %s: %v", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir fixture dst %s: %v", filepath.Dir(dst), err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", dst, err)
	}
}

func copyFixtureDir(t *testing.T, srcDir string, dstDir string) {
	t.Helper()
	copyDir(t, srcDir, dstDir)
}

func octxiliaryFixturePath(t *testing.T, name string) string {
	t.Helper()
	return repoPath(t, "testdata", "octxiliary_fixtures", name)
}

func copyOctxiliaryFixture(t *testing.T, name string, dst string) {
	t.Helper()
	copyTestFixture(t, octxiliaryFixturePath(t, name), dst)
}

func copyOctxiliaryFixtureDir(t *testing.T, name string, dst string) {
	t.Helper()
	copyFixtureDir(t, octxiliaryFixturePath(t, name), dst)
}

func newWrapperTempProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".oct"), 0o755); err != nil {
		t.Fatalf("mkdir wrapper temp project state: %v", err)
	}
	return root
}

func repoPath(t *testing.T, parts ...string) string {
	t.Helper()
	all := append([]string{"..", ".."}, parts...)
	path, err := filepath.Abs(filepath.Join(all...))
	if err != nil {
		t.Fatalf("resolve repo path %v: %v", parts, err)
	}
	return path
}
