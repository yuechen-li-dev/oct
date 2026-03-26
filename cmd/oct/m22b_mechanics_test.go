package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM22bBasicVec2ForceOperation(t *testing.T) {
	root := setupM22bMechanicsFixture(t)
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Mechanics",
		"",
		"fn Main() -> Int {",
		"    let a = Mechanics.Vec2Force { X: 2kg*m/s^2 Y: 3kg*m/s^2 }",
		"    let b = Mechanics.Vec2Force { X: 5kg*m/s^2 Y: 0kg*m/s^2 - 1kg*m/s^2 }",
		"    let c = Mechanics.AddForce(a, b)",
		"    if c.X != 7kg*m/s^2 { return 1 }",
		"    if c.Y != 2kg*m/s^2 { return 2 }",
		"    return 0",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "0\n" {
		t.Fatalf("expected success code 0, got %q", stdout)
	}
}

func TestM22bUnitPreservingComputation(t *testing.T) {
	root := setupM22bMechanicsFixture(t)
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Mechanics",
		"",
		"fn Main() -> Int {",
		"    let force = Mechanics.Vec2Force { X: 4.0kg*m/s^2 Y: 3.0kg*m/s^2 }",
		"    let magSq = Mechanics.MagnitudeSquared(force)",
		"    if Abs(Sqrt(magSq) - 5.0kg*m/s^2) > 0.0001kg*m/s^2 { return 1 }",
		"    if Abs(magSq - 25.0kg^2*m^2/s^4) > 0.0001kg^2*m^2/s^4 { return 2 }",
		"    return 0",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "0\n" {
		t.Fatalf("expected success code 0, got %q", stdout)
	}
}

func TestM22bMatrixBackedComputation(t *testing.T) {
	root := setupM22bMechanicsFixture(t)
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Mechanics",
		"",
		"fn Main() -> Int {",
		"    let stiffness = Mechanics.Stiffness2 {",
		"        K: matrix[[100kg/s^2, 0kg/s^2] [0kg/s^2, 50kg/s^2]]",
		"    }",
		"    let d = Mechanics.Displacement2 { X: 0.3m Y: 0m - 0.2m }",
		"    let force = Mechanics.InternalForce(stiffness, d)",
		"    if Abs(force.X - 30kg*m/s^2) > 0.0001kg*m/s^2 { return 1 }",
		"    if Abs(force.Y + 10kg*m/s^2) > 0.0001kg*m/s^2 { return 2 }",
		"    return 0",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "0\n" {
		t.Fatalf("expected success code 0, got %q", stdout)
	}
}

func TestM22bPackageIntegrationRunAndBuild(t *testing.T) {
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
		t.Fatalf("build failed: %v stdout=%s stderr=%s", buildErr, buildStdout, buildStderr)
	}
	if !strings.Contains(buildStdout, "build succeeded") {
		t.Fatalf("expected build success output, got %q", buildStdout)
	}
}

func TestM22bBuildFailureDoesNotEmitArtifact(t *testing.T) {
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
	if _, statErr := os.Stat(entry + ".octbin"); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact, stat err = %v", statErr)
	}
}

func setupM22bMechanicsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "testdata", "m22b", "valid", "Mechanics"), filepath.Join(root, "Mechanics"))
	return root
}

func setupM22bFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "testdata", "m22b", "valid", "Mechanics"), filepath.Join(root, "Mechanics"))
	copyDir(t, filepath.Join("..", "..", "testdata", "m22b", "valid", "Main"), filepath.Join(root, "Main"))
	return root
}
