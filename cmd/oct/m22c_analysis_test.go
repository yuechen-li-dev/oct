package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestM22cBasicAnalysisResult(t *testing.T) {
	root := setupM22cFixture(t)
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Analysis.GenerateAndComputeReturnsExpectedSeriesShape") {
		t.Fatalf("expected fact pass output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS Analysis.GenerateAndComputeKnownSamples[2]") {
		t.Fatalf("expected theory case pass output, got %q", stdout)
	}
}

func TestM22cPlotGeneration(t *testing.T) {
	root := setupM22cAnalysisFixture(t)
	outputPath := filepath.Join(root, "analysis_plot.png")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Analysis",
		"",
		"fn Main() -> Int {",
		"    let result = Analysis.GenerateAndCompute(0.0, 1.0)",
		"    return PlotScatter(result.X, result.Y, " + strconv.Quote(outputPath) + ")",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "0\n" {
		t.Fatalf("expected success code 0, got %q", stdout)
	}

	info, statErr := os.Stat(outputPath)
	if statErr != nil {
		t.Fatalf("expected output png %s: %v", outputPath, statErr)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty output png %s", outputPath)
	}
}

func TestM22cPackageIntegrationRunAndBuild(t *testing.T) {
	root := setupM22cFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "5\n") {
		t.Fatalf("expected length print output, got %q", stdout)
	}
	if !strings.Contains(stdout, "-2\n") {
		t.Fatalf("expected first value print output, got %q", stdout)
	}

	buildStdout, buildStderr, buildErr := executeCLI("build", entry)
	if buildErr == nil {
		t.Fatalf("expected build failure for unsupported compiled feature, got success with stdout %q", buildStdout)
	}
	if buildStdout != "" {
		t.Fatalf("expected empty build stdout, got %q", buildStdout)
	}
	if !strings.Contains(buildStderr, "compiled mode does not yet support for") {
		t.Fatalf("expected unsupported for-loop diagnostic, got %q", buildStderr)
	}
	if _, statErr := os.Stat(entry + ".octbin"); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
	}
}

func TestM22cBuildFailureDoesNotEmitArtifact(t *testing.T) {
	root := setupM22cAnalysisFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Analysis",
		"",
		"fn Main() -> Int {",
		"    let result = Analysis.GenerateAndCompute(\"bad\", 1.0)",
		"    Print(result)",
		"    return 0",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatalf("expected build failure, got success with stdout %q", stdout)
	}
	if !strings.Contains(stderr, "function 'Analysis.GenerateAndCompute' argument 1 expects Float, got String") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
	if _, statErr := os.Stat(entry + ".octbin"); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact, stat err = %v", statErr)
	}
}

func TestM22cAnalysisUsesDirectArrayAssignment(t *testing.T) {
	// M22cr proof note: this package now populates arrays with x[i]/y[i]/z[i] assignment.
	analysisPath := filepath.Join("..", "..", "Libraries", "Analysis", "Analysis.Core.oct")
	contents, err := os.ReadFile(analysisPath)
	if err != nil {
		t.Fatalf("read analysis source: %v", err)
	}
	src := string(contents)
	for _, snippet := range []string{"x[i] =", "y[i] =", "z[i] ="} {
		if !strings.Contains(src, snippet) {
			t.Fatalf("expected direct array assignment snippet %q in %s", snippet, analysisPath)
		}
	}
	if strings.Contains(src, "Basis5") || strings.Contains(src, "Splat5") {
		t.Fatalf("expected M22cr refresh to remove whole-array reassignment helpers")
	}
}

func setupM22cAnalysisFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Libraries", "Analysis"), filepath.Join(root, "Analysis"))
	return root
}

func setupM22cFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Libraries", "Analysis"), filepath.Join(root, "Analysis"))
	copyDir(t, filepath.Join("..", "..", "testdata", "m22c", "valid", "Main"), filepath.Join(root, "Main"))
	return root
}
