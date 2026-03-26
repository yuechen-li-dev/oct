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
			wantErr: "package 'Geometry' has no function 'Nope'",
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

func TestM15aQualifiedRecordTypeInSignatureAndFieldAccess(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nrecord Point { X: Float<m> Y: Float<m> }\nfn MakePoint() -> Point { return Point { X: 4m Y: 2m } }\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn UsePoint(p: Geometry.Point) -> Float<m> { return p.X }\nfn Main() -> Float<m> { return UsePoint(Geometry.MakePoint()) }\n")

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "4m\n" {
		t.Fatalf("expected 4m, got %q", stdout)
	}
}

func TestM15aQualifiedRecordConstruction(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nrecord Point { X: Float<m> Y: Float<m> }\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main() -> Geometry.Point { return Geometry.Point { X: 1m Y: 2m } }\n")

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "Geometry.Point{X: 1m, Y: 2m}\n" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestM15aQualifiedEnumValueReference(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Physics", "physics.oct", "package Physics\nenum Method { Euler Rk4 }\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Physics\nfn Main() -> Physics.Method { return Physics.Method.Euler }\n")

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "Physics.Method.Euler\n" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestM15aRejectsUnknownQualifiedType(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nrecord Point { X: Int }\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main(p: Geometry.Nope) -> Int { return 0 }\n")

	_, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err == nil {
		t.Fatal("expected run to fail")
	}
	if !strings.Contains(stderr, "package 'Geometry' has no type 'Nope'") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestM15aRejectsUnknownQualifiedEnumVariant(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Physics", "physics.oct", "package Physics\nenum Method { Euler Rk4 }\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Physics\nfn Main() -> Physics.Method { return Physics.Method.Bogus }\n")

	_, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err == nil {
		t.Fatal("expected run to fail")
	}
	if !strings.Contains(stderr, "enum 'Physics.Method' has no variant 'Bogus'") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestM15aRejectsQualifiedRecordLiteralWrongField(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nrecord Point { X: Float<m> Y: Float<m> }\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main() -> Geometry.Point { return Geometry.Point { X: 1m Z: 2m } }\n")

	_, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err == nil {
		t.Fatal("expected run to fail")
	}
	if !strings.Contains(stderr, "record 'Geometry.Point' has no field 'Z'") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestM18PackageCoexistenceWithMutableLocalReassignment(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nrecord Point { X: Int Y: Int }\nfn P1() -> Point { return Point { X: 1 Y: 2 } }\nfn P2() -> Point { return Point { X: 3 Y: 4 } }\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main() -> Geometry.Point { var p = Geometry.P1() p = Geometry.P2() return p }\n")

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "Point{X: 3, Y: 4}\n" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestM15aRejectsQualifiedSymbolKindMismatch(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nrecord Point { X: Int }\nfn Distance() -> Int { return 42 }\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main() -> Int { let x = Geometry.Point() return 0 }\n")

	_, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err == nil {
		t.Fatal("expected run to fail")
	}
	if !strings.Contains(stderr, "package-qualified type 'Geometry.Point' used where a function is required") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestM15aBuiltinsCoexistWithQualifiedTypesAndFunctions(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Geometry", "geometry.oct", "package Geometry\nrecord Point { X: Int Y: Int }\nfn Origin() -> Point { return Point { X: 0 Y: 0 } }\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Geometry\nfn Main() -> Int { let p = Geometry.Origin() Print(p.X) return 0 }\n")

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "0\n0\n" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestM15aBuildRejectsInvalidQualifiedPrograms(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Physics", "physics.oct", "package Physics\nenum Method { Euler Rk4 }\n")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Physics\nfn Main() -> Physics.Method { return Physics.Method.Nope }\n")

	_, stderr, err := executeCLI("build", filepath.Join(root, "Main", "main.oct"))
	if err == nil {
		t.Fatal("expected build to fail")
	}
	if !strings.Contains(stderr, "enum 'Physics.Method' has no variant 'Nope'") {
		t.Fatalf("unexpected stderr: %q", stderr)
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
