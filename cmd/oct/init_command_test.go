package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesManifestForExistingDirectories(t *testing.T) {
	cases := []struct {
		kind     string
		name     string
		snippets []string
	}{
		{kind: "experiment", name: "BrownNoiseKalman", snippets: []string{"Name: \"BrownNoiseKalman\"", "Kind: \"experiment\"", "EntryMilestone: \"M0\""}},
		{kind: "library", name: "SignalTools", snippets: []string{"Name: \"SignalTools\"", "Description: \"SignalTools package\""}},
		{kind: "wrapper-library", name: "OpenCV", snippets: []string{"Name: \"OpenCV\"", "Kind: \"wrapper\"", "SidecarCommand: \"octxiliary-open-cv\""}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, tc.name)
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatalf("mkdir init target: %v", err)
			}
			stdout, stderr, err := executeCLIInDir(dir, "init", tc.kind)
			if err != nil {
				t.Fatalf("oct init %s failed: err=%v stderr=%q stdout=%q", tc.kind, err, stderr, stdout)
			}
			assertOutputContains(t, stdout, "Initialized "+tc.kind+" package "+tc.name)
			assertFileContains(t, filepath.Join(dir, "manifest.oct"), tc.snippets...)
		})
	}
}

func TestInitRefusesToOverwriteExistingManifest(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "SignalTools")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir init target: %v", err)
	}
	manifestPath := filepath.Join(dir, "manifest.oct")
	original := "package Manifest\n"
	if err := os.WriteFile(manifestPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	stdout, stderr, err := executeCLIInDir(dir, "init", "library")
	if err == nil {
		t.Fatalf("expected overwrite failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "refuses to overwrite") {
		t.Fatalf("expected overwrite diagnostic, got %q", stderr)
	}
	data, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		t.Fatalf("read manifest: %v", readErr)
	}
	if string(data) != original {
		t.Fatalf("manifest was overwritten: %q", string(data))
	}
}

func TestInitInvalidDirectoryBasenameFailsWithSuggestion(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "signal_tools")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir init target: %v", err)
	}
	stdout, stderr, err := executeCLIInDir(dir, "init", "library")
	if err == nil {
		t.Fatalf("expected invalid basename failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "invalid package name \"signal_tools\"") || !strings.Contains(stderr, "rename the directory") {
		t.Fatalf("expected invalid basename suggestion, got %q", stderr)
	}
}

func TestInitHelpAndUsage(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "SignalTools")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir init target: %v", err)
	}
	stdout, stderr, err := executeCLIInDir(dir, "init", "--help")
	if err != nil {
		t.Fatalf("oct init --help failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, "usage: oct init <experiment|library|wrapper-library>")

	for _, kind := range []string{"experiment", "library", "wrapper-library"} {
		stdout, stderr, err = executeCLIInDir(dir, "init", kind, "--help")
		if err != nil {
			t.Fatalf("oct init %s --help failed: err=%v stderr=%q stdout=%q", kind, err, stderr, stdout)
		}
		assertOutputContains(t, stdout, "usage: oct init "+kind)
	}

	stdout, stderr, err = executeCLIInDir(dir, "init", "unknown")
	if err == nil {
		t.Fatalf("expected unknown usage failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "usage: oct init <experiment|library|wrapper-library>") {
		t.Fatalf("expected init usage, got %q", stderr)
	}
}

func TestMissingManifestImportDiagnosticSuggestsInit(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "FrictionProbe")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir probe: %v", err)
	}
	testSource := strings.Join([]string{
		"package FrictionProbe",
		"",
		"import Assert",
		"",
		"[Fact]",
		"fn UsesAssert() -> Void {",
		"    Assert.True(true, \"works\")",
		"}",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "probe.octest"), []byte(testSource), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	stdout, stderr, err := executeCLIInDir(root, "test", dir, "--execution", "interpreted")
	if err == nil {
		t.Fatalf("expected missing manifest diagnostic, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "manifest.oct") || !strings.Contains(stderr, "oct init experiment") {
		t.Fatalf("expected missing manifest init suggestion, got stderr=%q stdout=%q", stderr, stdout)
	}

	if stdout, stderr, err = executeCLIInDir(dir, "init", "experiment"); err != nil {
		t.Fatalf("oct init experiment failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	stdout, stderr, err = executeCLIInDir(root, "test", dir, "--execution", "interpreted")
	if err == nil {
		t.Fatalf("expected next dependency diagnostic after init because Assert is not a package dependency, stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stderr, "oct init experiment") {
		t.Fatalf("did not expect missing-manifest suggestion after init, got %q", stderr)
	}
}
