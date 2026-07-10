//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExperimentMilestoneManifestFallbackTestAndArtifact(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Exp", "manifest.oct", manifestSource("Exp", "exp root"))
	if err := os.WriteFile(filepath.Join(root, "Exp", "REPORT.md"), []byte("# report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	writeOctPkgFile(t, root, "Exp", "base.oct", "package Exp\nfn One() -> Int { return 1 }\n")

	m1 := filepath.Join(root, "Exp", "M1")
	m2 := filepath.Join(root, "Exp", "M2")
	if err := os.MkdirAll(m1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(m2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m1, "specific.octest"), []byte("package Exp\n[Fact]\nfn M1Pass() -> Void { Assert.Equal(1, 1, \"ok\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m2, "other.octest"), []byte("package Exp\n[Fact]\nfn M2MustNotRun() -> Void { Assert.True(false, \"must not run\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeCLI("test", m1)
	if err != nil {
		t.Fatalf("test m1 failed: %v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Exp.M1Pass") || strings.Contains(stdout, "M2MustNotRun") {
		t.Fatalf("unexpected milestone discovery scope stdout=%q", stdout)
	}

	artifactOut := filepath.Join(t.TempDir(), "m1.octagon")
	if err := os.WriteFile(filepath.Join(m1, "artifact.octest"), []byte("package Exp\n[Artifact]\nfn Emit() -> Void { WriteOctagon("+octStringLiteralPath(artifactOut)+", 1) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifactStdout, artifactStderr, artifactErr := executeCLI("artifact", m1)
	if artifactErr != nil {
		t.Fatalf("artifact m1 failed: %v stderr=%q stdout=%q", artifactErr, artifactStderr, artifactStdout)
	}
	if !strings.Contains(artifactStdout, "PASS Exp.Emit") {
		t.Fatalf("expected artifact pass output, got %q", artifactStdout)
	}
	if _, err := os.Stat(artifactOut); err != nil {
		t.Fatalf("expected emitted artifact %s: %v", artifactOut, err)
	}

	selectedStdout, selectedStderr, selectedErr := executeCLI("test", filepath.Join(m1, "specific.octest"))
	if selectedErr != nil {
		t.Fatalf("selected-file test failed: %v stderr=%q stdout=%q", selectedErr, selectedStderr, selectedStdout)
	}
	if !strings.Contains(selectedStdout, "PASS Exp.M1Pass") || strings.Contains(selectedStdout, "M2MustNotRun") {
		t.Fatalf("unexpected selected-file scope stdout=%q", selectedStdout)
	}
}

func TestManifestDetectionIgnoresChildMilestoneManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "M1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "M1", "manifest.oct"), []byte("package Manifest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "M1", "suite.octest"), []byte("package Main\n[Fact]\nfn A() -> Void { Assert.True(true, \"ok\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := executeCLI("test", filepath.Join(root, "M1"))
	if err == nil {
		t.Fatalf("expected test failure without package context")
	}
	if strings.Contains(stderr, "package manifest missing") {
		t.Fatalf("expected child manifest not to activate root manifest mode, got %q", stderr)
	}
}
