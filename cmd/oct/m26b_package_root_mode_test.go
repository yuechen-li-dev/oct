package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM26bDirectPackageRootTestCommand(t *testing.T) {
	tests := []struct {
		name       string
		packageDir string
		want       string
	}{
		{
			name:       "numerics package root",
			packageDir: filepath.Join("..", "..", "Libraries", "Numerics"),
			want:       "PASS Numerics.BisectionConvergesForSqrt2",
		},
		{
			name:       "mechanics package root",
			packageDir: filepath.Join("..", "..", "Libraries", "Mechanics"),
			want:       "PASS Mechanics.AddForceAndMagnitudePreserveUnits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := executeCLI("test", tt.packageDir)
			if err != nil {
				t.Fatalf("oct test failed: %v stderr=%q stdout=%q", err, stderr, stdout)
			}
			if !strings.Contains(stdout, tt.want) {
				t.Fatalf("expected output to contain %q, got %q", tt.want, stdout)
			}
		})
	}
}

func TestM26bDirectPackageRootBenchCommand(t *testing.T) {
	stdout, stderr, err := executeCLI("bench", filepath.Join("..", "..", "Libraries", "Signal"))
	if err != nil {
		t.Fatalf("oct bench failed: %v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Signal.MovingAverageSmall") {
		t.Fatalf("expected migrated benchmark output, got %q", stdout)
	}
}

func TestM26bDirectPackageRootArtifactCommand(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Tooling", "manifest.oct", manifestSource("Tooling", "artifact package"))
	writeOctPkgFile(t, root, "Tooling", "tooling.oct", "package Tooling\nfn Meaning() -> Int { return 42 }\n")
	writeOctPkgFile(t, root, "Tooling", "tooling.octest", "package Tooling\n[Artifact]\nfn Emit() -> Void { Print(\"artifact,ok\") }\n")

	stdout, stderr, err := executeCLI("artifact", filepath.Join(root, "Tooling"))
	if err != nil {
		t.Fatalf("oct artifact failed: %v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Tooling.Emit") {
		t.Fatalf("expected artifact pass output, got %q", stdout)
	}
}

func TestM26bDirectPackageRootManifestValidationStillStrict(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Mismatch", "manifest.oct", manifestSource("WrongName", "bad manifest"))
	writeOctPkgFile(t, root, "Mismatch", "mismatch.oct", "package Mismatch\nfn One() -> Int { return 1 }\n")
	writeOctPkgFile(t, root, "Mismatch", "mismatch.octest", "package Mismatch\n[Fact]\nfn Roundtrip() -> Void { Assert.EqualInt(One(), 1) }\n")

	stdout, stderr, err := executeCLI("test", filepath.Join(root, "Mismatch"))
	if err == nil {
		t.Fatalf("expected manifest validation failure, got success stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "manifest function returned invalid package metadata") {
		t.Fatalf("expected manifest metadata error, got %q", stderr)
	}
}

func TestM66PackageRootLoadsMilestoneFiles(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "MilestonePkg", "manifest.oct", manifestSource("MilestonePkg", "milestone layout package"))
	milestoneDir := filepath.Join(root, "MilestonePkg", "M0")
	if err := os.MkdirAll(milestoneDir, 0o755); err != nil {
		t.Fatalf("mkdir milestone: %v", err)
	}
	if err := os.WriteFile(filepath.Join(milestoneDir, "probe.oct"), []byte("package MilestonePkg\nfn One() -> Int { return 1 }\n"), 0o644); err != nil {
		t.Fatalf("write milestone source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(milestoneDir, "probe.octest"), []byte("package MilestonePkg\n[Fact]\nfn Roundtrip() -> Void { return }\n"), 0o644); err != nil {
		t.Fatalf("write milestone test: %v", err)
	}

	stdout, stderr, err := executeCLI("test", filepath.Join(root, "MilestonePkg"))
	if err != nil {
		t.Fatalf("oct test failed for milestone package layout: %v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS MilestonePkg.Roundtrip") {
		t.Fatalf("expected milestone package test output, got %q", stdout)
	}
}
