package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestM22cBasicAnalysisResult(t *testing.T) {
	root := setupM22cAnalysisFixture(t)
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Analysis",
		"",
		"fn Main() -> Int {",
		"    let result = Analysis.GenerateAndCompute(0.0, 1.0)",
		"    if Len(result.X) != Len(result.Y) { return 1 }",
		"    if Len(result.X) <= 0 { return 2 }",
		"    if Abs(result.X[0] - 0.0) > 0.0001 { return 3 }",
		"    if Abs(result.X[2] - 2.0) > 0.0001 { return 4 }",
		"    if Abs(result.X[4] - 4.0) > 0.0001 { return 5 }",
		"    if Abs(result.Y[0] + 2.0) > 0.0001 { return 6 }",
		"    if Abs(result.Y[2] - 2.0) > 0.0001 { return 7 }",
		"    if Abs(result.Y[4] - 14.0) > 0.0001 { return 8 }",
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
	if buildErr != nil {
		t.Fatalf("build failed: %v stdout=%s stderr=%s", buildErr, buildStdout, buildStderr)
	}
	if !strings.Contains(buildStdout, "build succeeded") {
		t.Fatalf("expected build success output, got %q", buildStdout)
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

func setupM22cAnalysisFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "testdata", "m22c", "valid", "Analysis"), filepath.Join(root, "Analysis"))
	return root
}

func setupM22cFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "testdata", "m22c", "valid", "Analysis"), filepath.Join(root, "Analysis"))
	copyDir(t, filepath.Join("..", "..", "testdata", "m22c", "valid", "Main"), filepath.Join(root, "Main"))
	return root
}
