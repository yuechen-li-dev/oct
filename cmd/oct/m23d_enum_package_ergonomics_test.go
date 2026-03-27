package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM23dCrossPackageEnumErgonomics(t *testing.T) {
	root := setupM23dFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "0\n" {
		t.Fatalf("expected success code 0, got %q", stdout)
	}

	buildStdout, buildStderr, buildErr := executeCLI("build", entry)
	if buildErr != nil {
		t.Fatalf("build failed: %v stdout=%s stderr=%s", buildErr, buildStdout, buildStderr)
	}
	if !strings.Contains(buildStdout, "build succeeded") {
		t.Fatalf("expected build success output, got %q", buildStdout)
	}
}

func TestM23dInvalidCrossPackageEnumDiagnostics(t *testing.T) {
	tests := []struct {
		name    string
		mainSrc string
		wantErr string
	}{
		{
			name: "unknown imported enum variant",
			mainSrc: strings.Join([]string{
				"package Main",
				"import Physics",
				"",
				"fn Main() -> Int {",
				"    let method = Physics.Method.Bogus",
				"    return switch method {",
				"        case Physics.Method.Euler => 1",
				"        case Physics.Method.RK4 => 4",
				"    }",
				"}",
			}, "\n"),
			wantErr: "enum 'Physics.Method' has no variant 'Bogus'",
		},
		{
			name: "record field mismatch from wrong package enum type",
			mainSrc: strings.Join([]string{
				"package Main",
				"import Physics",
				"import Other",
				"",
				"record SolveConfig {",
				"    Method: Physics.Method",
				"}",
				"",
				"fn Main() -> Int {",
				"    let cfg = SolveConfig {",
				"        Method: Other.Method.Euler",
				"    }",
				"    Print(cfg)",
				"    return 0",
				"}",
			}, "\n"),
			wantErr: "record 'SolveConfig' field 'Method' expects Physics.Method, got Other.Method",
		},
		{
			name: "switch case from wrong package enum type",
			mainSrc: strings.Join([]string{
				"package Main",
				"import Physics",
				"import Other",
				"",
				"fn Main() -> Int {",
				"    let method = Physics.Method.Euler",
				"    return switch method {",
				"        case Other.Method.Euler => 1",
				"        case Physics.Method.RK4 => 4",
				"    }",
				"}",
			}, "\n"),
			wantErr: "case type Other.Method does not match subject type Physics.Method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := setupM23dPhysicsOnlyFixture(t)
			writePkgFile(t, root, "Main", "main.oct", tt.mainSrc)
			writePkgFile(t, root, "Other", "other.oct", strings.Join([]string{
				"package Other",
				"",
				"enum Method {",
				"    Euler",
				"    RK4",
				"}",
			}, "\n"))

			entry := filepath.Join(root, "Main", "main.oct")
			stdout, stderr, err := executeCLI("build", entry)
			if err == nil {
				t.Fatalf("expected build failure, got success with stdout %q", stdout)
			}
			if !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tt.wantErr, stderr)
			}
			if _, statErr := os.Stat(entry + ".octbin"); !os.IsNotExist(statErr) {
				t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
			}
		})
	}
}

func TestM23dProofRegressionUsesExplicitQualifiedEnumFlow(t *testing.T) {
	root := setupM23dFixture(t)
	mainPath := filepath.Join(root, "Main", "main.oct")
	bytes, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read fixture main: %v", err)
	}
	src := string(bytes)
	if !strings.Contains(src, "Physics.Method.Euler") {
		t.Fatalf("expected explicit qualified enum usage in proof fixture")
	}
	if strings.Contains(src, "problemId") {
		t.Fatalf("expected proof fixture to avoid integer selector fallback")
	}
}

func setupM23dPhysicsOnlyFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Packages", "Physics"), filepath.Join(root, "Physics"))
	return root
}

func setupM23dFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Packages", "Physics"), filepath.Join(root, "Physics"))
	copyDir(t, filepath.Join("..", "..", "testdata", "m23d", "valid", "Main"), filepath.Join(root, "Main"))
	return root
}
