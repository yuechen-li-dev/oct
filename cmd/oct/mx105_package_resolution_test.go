package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMx105CanonicalRepoWideImportResolutionParity(t *testing.T) {
	repoRoot := t.TempDir()
	writeOctFile(t, repoRoot, "Libraries", "Octomata", "octomata.oct", strings.Join([]string{
		"package Octomata",
		"",
		"fn Seed() -> Int {",
		"    return 40",
		"}",
		"",
		"fn PlusTwo() -> Int {",
		"    return 2",
		"}",
	}, "\n")+"\n")
	writeOctFile(t, repoRoot, "Libraries", "VectorOps", "vector_ops.oct", strings.Join([]string{
		"package VectorOps",
		"",
		"import Octomata",
		"",
		"fn FromLibrary() -> Int {",
		"    return Octomata.Seed() + Octomata.PlusTwo()",
		"}",
	}, "\n")+"\n")

	milestoneRoot := filepath.Join(repoRoot, "Experiments", "PrometheusSgemmAlgorithmLab", "M1")
	writeOctFile(t, milestoneRoot, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"",
		"import Octomata",
		"import VectorOps",
		"",
		"fn Main() -> Int {",
		"    return VectorOps.FromLibrary() + Octomata.PlusTwo()",
		"}",
	}, "\n")+"\n")
	writeOctFile(t, milestoneRoot, "Main", "main.octest", strings.Join([]string{
		"package Main",
		"",
		"import Octomata",
		"",
		"[Fact]",
		"fn OctomataImportsInOctest() -> Void {",
		"    Assert.Equal(40, Octomata.Seed(), \"octest import parity\")",
		"}",
		"",
		"[Artifact]",
		"fn EmitArtifact() -> Void {",
		"    Print(Octomata.PlusTwo())",
		"}",
	}, "\n")+"\n")

	runStdout, runStderr, runErr := executeCLIInDir(repoRoot, "run", milestoneRoot)
	if runErr != nil {
		t.Fatalf("run failed: err=%v stderr=%q stdout=%q", runErr, runStderr, runStdout)
	}
	if strings.TrimSpace(runStdout) != "44" {
		t.Fatalf("expected run output 44, got %q", runStdout)
	}

	buildStdout, buildStderr, buildErr := executeCLIInDir(filepath.Join(repoRoot, "Experiments"), "build", milestoneRoot)
	if buildErr != nil {
		t.Fatalf("build failed: err=%v stderr=%q stdout=%q", buildErr, buildStderr, buildStdout)
	}
	artifactPath := filepath.Join(milestoneRoot, "Main.octbin")
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("expected build artifact at %s: %v", artifactPath, err)
	}

	testStdout, testStderr, testErr := executeCLIInDir(repoRoot, "test", milestoneRoot)
	if testErr != nil {
		t.Fatalf("test failed: err=%v stderr=%q stdout=%q", testErr, testStderr, testStdout)
	}
	if !strings.Contains(testStdout, "PASS Main.OctomataImportsInOctest") {
		t.Fatalf("expected octest pass output, got %q", testStdout)
	}

	artifactStdout, artifactStderr, artifactErr := executeCLIInDir(repoRoot, "artifact", milestoneRoot)
	if artifactErr != nil {
		t.Fatalf("artifact command failed: err=%v stderr=%q stdout=%q", artifactErr, artifactStderr, artifactStdout)
	}
	if !strings.Contains(artifactStdout, "PASS Main.EmitArtifact") {
		t.Fatalf("expected artifact pass output, got %q", artifactStdout)
	}

	runFromNestedStdout, runFromNestedStderr, runFromNestedErr := executeCLIInDir(filepath.Join(repoRoot, "Experiments", "PrometheusSgemmAlgorithmLab"), "run", milestoneRoot)
	if runFromNestedErr != nil {
		t.Fatalf("run from nested cwd failed: err=%v stderr=%q stdout=%q", runFromNestedErr, runFromNestedStderr, runFromNestedStdout)
	}
	if strings.TrimSpace(runFromNestedStdout) != "44" {
		t.Fatalf("expected run output 44 from nested cwd, got %q", runFromNestedStdout)
	}
}

func TestMx105MissingPackageDiagnosticIncludesResolverContext(t *testing.T) {
	repoRoot := t.TempDir()
	mainPath := writeOctFile(t, repoRoot, "Experiments", "ImportDiagnostics", "M0", "Main", "main.oct", strings.Join([]string{
		"package Main",
		"",
		"import MissingPkg",
		"",
		"fn Main() -> Int {",
		"    return MissingPkg.Value()",
		"}",
	}, "\n")+"\n")

	stdout, stderr, err := executeCLIInDir(repoRoot, "run", mainPath)
	if err == nil {
		t.Fatalf("expected import failure, got success stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "unknown package 'MissingPkg'") {
		t.Fatalf("expected missing package name in diagnostic, got %q", stderr)
	}
	if !strings.Contains(stderr, "imported by package 'Main'") {
		t.Fatalf("expected importer package in diagnostic, got %q", stderr)
	}
	if !strings.Contains(stderr, "active root:") {
		t.Fatalf("expected active root in diagnostic, got %q", stderr)
	}
	if !strings.Contains(stderr, "searched:") {
		t.Fatalf("expected searched roots in diagnostic, got %q", stderr)
	}
}

func writeOctFile(t *testing.T, root string, parts ...string) string {
	t.Helper()
	if len(parts) < 2 {
		t.Fatalf("writeOctFile requires at least one path segment and content")
	}
	content := parts[len(parts)-1]
	relPath := filepath.Join(parts[:len(parts)-1]...)
	fullPath := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", fullPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", fullPath, err)
	}
	return fullPath
}
