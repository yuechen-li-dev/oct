//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBasicAnalysisResult(t *testing.T) {
	skipUnlessSlow(t)
	root := setupM22cFixture(t)
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Analysis.CumulativeSumMatchesHandComputed") {
		t.Fatalf("expected fact pass output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS Analysis.DiffForwardOnLinearDataIsConstant") {
		t.Fatalf("expected diff fact pass output, got %q", stdout)
	}
}

func TestPlotGeneration(t *testing.T) {
	root := setupM22cAnalysisFixture(t)
	outputPath := filepath.Join(root, "analysis_plot.png")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Analysis",
		"",
		"fn Main() -> Int {",
		"    let xs = [0.0, 1.0, 2.0, 3.0, 4.0]",
		"    let ys = [0.0, 1.0, 4.0, 9.0, 16.0]",
		"    let cs = Analysis.CumulativeSum(ys)!",
		"    return PlotScatter(xs, cs, " + strconv.Quote(outputPath) + ")",
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

func TestAnalysisPackageIntegrationRunAndBuild(t *testing.T) {
	root := setupM22cFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	// main.oct prints Len(cs) = 5, then cs[0] = 0
	if !strings.Contains(stdout, "5\n") {
		t.Fatalf("expected length print output, got %q", stdout)
	}
	if !strings.Contains(stdout, "0\n") {
		t.Fatalf("expected first cumsum value, got %q", stdout)
	}

	buildStdout, buildStderr, buildErr := executeCLI("build", entry)
	if buildErr == nil {
		t.Fatalf("expected build failure for unsupported compiled feature, got success with stdout %q", buildStdout)
	}
	if buildStdout != "" {
		t.Fatalf("expected empty build stdout, got %q", buildStdout)
	}
	if !strings.Contains(buildStderr, "compiled mode does not yet support builtin PlotLine") {
		t.Fatalf("expected unsupported builtin PlotLine diagnostic, got %q", buildStderr)
	}
	if _, statErr := os.Stat(nativeArtifactPath(entry)); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
	}
}

func TestAnalysisBuildFailureDoesNotEmitArtifact(t *testing.T) {
	root := setupM22cAnalysisFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Analysis",
		"",
		"fn Main() -> Int {",
		"    let xs = [0.0, 1.0, 2.0]",
		"    let bad = Analysis.CumulativeSum(xs)!",
		// Pass a String where Float[] is expected to force a type error.
		"    let result = Analysis.RMSE(\"oops\", xs)",
		"    Print(result)",
		"    return 0",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatalf("expected build failure, got success with stdout %q", stdout)
	}
	if !strings.Contains(stderr, "expects Float[]") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
	if _, statErr := os.Stat(nativeArtifactPath(entry)); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact, stat err = %v", statErr)
	}
}

func TestAnalysisUsesAppendPattern(t *testing.T) {
	// Structural proof: the new Analysis library uses Append-based array
	// construction rather than pre-allocated indexed assignment.
	analysisPath := filepath.Join("..", "..", "Libraries", "Analysis", "Analysis.Core.oct")
	contents, err := os.ReadFile(analysisPath)
	if err != nil {
		t.Fatalf("read analysis source: %v", err)
	}
	src := string(contents)

	// Should use Append for building output arrays.
	if !strings.Contains(src, "Append(out,") {
		t.Fatalf("expected Append-based array construction in Analysis.Core.oct")
	}
	// Should not contain the old scaffolding function.
	if strings.Contains(src, "GenerateAndCompute") {
		t.Fatalf("expected old scaffolding function GenerateAndCompute to be removed")
	}
	// Should use for loops not while loops for structured iteration.
	if strings.Contains(src, "condition-switch shape") {
		t.Fatalf("expected no condition-switch shape comment (wrong file)")
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
