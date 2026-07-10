//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBasicVec2ForceOperation(t *testing.T) {
	skipUnlessSlow(t)
	root := setupM22bFixture(t)
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Mechanics.AddForceAndMagnitudePreserveUnits") {
		t.Fatalf("expected fact pass output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS Mechanics.DominantAxisMatchesKnownCases[2]") {
		t.Fatalf("expected theory case pass output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS Mechanics.InternalForceMatchesKnownMatrixCase") {
		t.Fatalf("expected internal force fact pass output, got %q", stdout)
	}
}

func TestMechanicsPackageIntegrationRunAndBuild(t *testing.T) {
	root := setupM22bFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "EquilibriumReport") {
		t.Fatalf("expected equilibrium report output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Vec2Force") {
		t.Fatalf("expected internal force output, got %q", stdout)
	}

	buildStdout, buildStderr, buildErr := executeCLI("build", entry)
	if buildErr != nil {
		t.Fatalf("expected Mechanics build success after matrix/scalar lowering cleanup, got err=%v stderr=%s stdout=%s", buildErr, buildStderr, buildStdout)
	}
	if !strings.Contains(buildStdout, "build succeeded:") {
		t.Fatalf("expected build success stdout, got %q", buildStdout)
	}
	if buildStderr != "" {
		t.Fatalf("expected empty build stderr, got %q", buildStderr)
	}
	if _, statErr := os.Stat(nativeArtifactPath(entry)); statErr != nil {
		t.Fatalf("expected compiled artifact after successful Mechanics build, stat err = %v", statErr)
	}
}

func TestMechanicsBuildFailureDoesNotEmitArtifact(t *testing.T) {
	root := setupM22bMechanicsFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Mechanics",
		"",
		"fn Main() -> Int {",
		"    let badDisplacement = Mechanics.Displacement2 { X: 1s Y: 2m }",
		"    Print(badDisplacement)",
		"    return 0",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatalf("expected build failure, got success with stdout %q", stdout)
	}
	if !strings.Contains(stderr, "expects Float<m>, got Int<s>") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
	if _, statErr := os.Stat(nativeArtifactPath(entry)); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact, stat err = %v", statErr)
	}
}

func setupM22bMechanicsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Libraries", "Mechanics"), filepath.Join(root, "Mechanics"))
	return root
}

func setupM22bFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Libraries", "Mechanics"), filepath.Join(root, "Mechanics"))
	copyDir(t, filepath.Join("..", "..", "testdata", "m22b", "valid", "Main"), filepath.Join(root, "Main"))
	return root
}
