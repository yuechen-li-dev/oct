package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
