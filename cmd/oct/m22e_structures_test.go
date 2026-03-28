package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM22eAxialStiffnessMatrix(t *testing.T) {
	root := setupM22eFixture(t)
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Structures.AnalyzeAxialElementProducesExpectedStiffnessAndForce") {
		t.Fatalf("expected fact pass output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS Structures.AxialForceFromDisplacementMatchesKnownCases[1]") {
		t.Fatalf("expected theory case pass output, got %q", stdout)
	}
}

func TestM22ePackageIntegrationRunAndBuild(t *testing.T) {
	root := setupM22eFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "matrix[[2e+09kg/s^2") {
		t.Fatalf("expected stiffness matrix print output, got %q", stdout)
	}
	if !strings.Contains(stdout, "vector[-2e+06m*kg/s^2, 2e+06m*kg/s^2]") {
		t.Fatalf("expected internal force vector output, got %q", stdout)
	}

	buildStdout, buildStderr, buildErr := executeCLI("build", entry)
	if buildErr == nil {
		t.Fatalf("expected build failure for unsupported compiled feature, got success with stdout %q", buildStdout)
	}
	if buildStdout != "" {
		t.Fatalf("expected empty build stdout, got %q", buildStdout)
	}
	if !strings.Contains(buildStderr, "unsupported expression ast.MatrixLiteralExpr") {
		t.Fatalf("expected unsupported matrix literal diagnostic, got %q", buildStderr)
	}
	if _, statErr := os.Stat(entry + ".octbin"); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
	}
}

func TestM22eBuildFailureDoesNotEmitArtifact(t *testing.T) {
	root := setupM22eStructuresFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Structures",
		"",
		"fn Main() -> Int {",
		"    let bad = Structures.BarElement2D {",
		"        Area: 0.02m*m",
		"        YoungsModulus: 200000000000kg/m/s/s",
		"        Length: 2s",
		"    }",
		"    Print(bad)",
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

func setupM22eStructuresFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Libraries", "Structures"), filepath.Join(root, "Structures"))
	return root
}

func setupM22eFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Libraries", "Structures"), filepath.Join(root, "Structures"))
	copyDir(t, filepath.Join("..", "..", "testdata", "m22e", "valid", "Main"), filepath.Join(root, "Main"))
	return root
}
