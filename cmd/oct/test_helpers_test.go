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
