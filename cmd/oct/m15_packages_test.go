package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM15RunBasicCrossPackageFunctionCall(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main() -> Int { return Geometry.Distance() }\n")
	writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nfn Distance() -> Int { return 42 }\n")

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "42\n" {
		t.Fatalf("expected 42, got %q", stdout)
	}
}

func TestM15RunSupportsMultiFilePackage(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main() -> Int { return Geometry.DistancePlusOne() }\n")
	writePkgFile(t, root, "Geometry", "base.oct", "package Geometry\nfn Distance() -> Int { return 41 }\n")
	writePkgFile(t, root, "Geometry", "plus.oct", "package Geometry\nfn DistancePlusOne() -> Int { return Distance() + 1 }\n")

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "42\n" {
		t.Fatalf("expected 42, got %q", stdout)
	}
}

func TestM15RejectsInvalidPackagePrograms(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string) string
		wantErr string
	}{
		{
			name: "missing package declaration",
			setup: func(t *testing.T, root string) string {
				writePkgFile(t, root, "Main", "main.oct", "fn Main() -> Int { return 0 }\n")
				return filepath.Join(root, "Main", "main.oct")
			},
			wantErr: "missing package declaration",
		},
		{
			name: "unknown import",
			setup: func(t *testing.T, root string) string {
				writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main() -> Int { return 0 }\n")
				return filepath.Join(root, "Main", "main.oct")
			},
			wantErr: "unknown package 'Geometry'",
		},
		{
			name: "self import",
			setup: func(t *testing.T, root string) string {
				writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Main\nfn Main() -> Int { return 0 }\n")
				return filepath.Join(root, "Main", "main.oct")
			},
			wantErr: "cannot import itself",
		},
		{
			name: "import cycle",
			setup: func(t *testing.T, root string) string {
				writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main() -> Int { return Geometry.Distance() }\n")
				writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nimport Main\nfn Distance() -> Int { return Main.Main() }\n")
				return filepath.Join(root, "Main", "main.oct")
			},
			wantErr: "import cycle detected",
		},
		{
			name: "duplicate declaration in package",
			setup: func(t *testing.T, root string) string {
				writePkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
				writePkgFile(t, root, "Main", "dup.oct", "package Main\nfn Main() -> Int { return 1 }\n")
				return filepath.Join(root, "Main", "main.oct")
			},
			wantErr: "duplicate declaration 'Main' in package 'Main'",
		},
		{
			name: "qualified symbol missing",
			setup: func(t *testing.T, root string) string {
				writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main() -> Int { return Geometry.Nope() }\n")
				writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nfn Distance() -> Int { return 0 }\n")
				return filepath.Join(root, "Main", "main.oct")
			},
			wantErr: "package 'Geometry' has no symbol 'Nope'",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			entry := test.setup(t, root)
			stdout, stderr, err := executeCLI("run", entry)
			if err == nil {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr %q does not contain %q", stderr, test.wantErr)
			}
		})
	}
}

func TestM15BuildAndBuiltinsCoexist(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Tools\nfn Main() -> Int { Print(Tools.Value()) return 0 }\n")
	writePkgFile(t, root, "Tools", "tools.oct", "package Tools\nfn Value() -> Int { return 7 }\n")

	stdout, stderr, err := executeCLI("build", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("build failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "build succeeded") {
		t.Fatalf("expected build success output, got %q", stdout)
	}
}

func writePkgFile(t *testing.T, root string, pkg string, name string, body string) {
	t.Helper()
	dir := filepath.Join(root, pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
