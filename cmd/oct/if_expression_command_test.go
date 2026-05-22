package main

import (
	"path/filepath"
	"testing"
)

func TestCrossPackageIfExpression(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nrecord Point {\n    X: Int\n    Y: Int\n}\nfn Make(flag: Bool) -> Point {\n    return if flag { Point { X: 1 Y: 2 } } else { Point { X: 3 Y: 4 } }\n}\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\n\nfn Main() -> Int {\n    let p = Geometry.Make(true)\n    return p.X\n}\n")

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if stdout != "1\n" {
		t.Fatalf("expected stdout %q, got %q", "1\\n", stdout)
	}
}
