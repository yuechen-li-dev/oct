package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM22aBisectionConverges(t *testing.T) {
	root := setupM22aNumericsFixture(t)
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Numerics",
		"",
		"fn Main() -> Int {",
		"    let options = Numerics.SolveOptions { MaxIterations: 64 Tolerance: 0.000001 }",
		"    match Numerics.Bisection(Numerics.ProblemSqrt2(), 1.0, 2.0, options) {",
		"        ok(result) => {",
		"            if result.Converged == false { return 1 }",
		"            if result.Iterations <= 0 { return 2 }",
		"            if result.ReasonCode != Numerics.ReasonConverged() { return 3 }",
		"            if Abs(result.Value - 1.414213562) > 0.001 { return 4 }",
		"            return 0",
		"        }",
		"        err(e) => { return 5 }",
		"    }",
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

func TestM22aNewtonRaphsonConverges(t *testing.T) {
	root := setupM22aNumericsFixture(t)
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Numerics",
		"",
		"fn Main() -> Int {",
		"    let options = Numerics.SolveOptions { MaxIterations: 64 Tolerance: 0.000001 }",
		"    match Numerics.NewtonRaphson(Numerics.ProblemCubic(), 1.5, options) {",
		"        ok(result) => {",
		"            if result.Converged == false { return 1 }",
		"            if result.ReasonCode != Numerics.ReasonConverged() { return 2 }",
		"            if Abs(result.Value - 1.521379706) > 0.001 { return 3 }",
		"            return 0",
		"        }",
		"        err(e) => { return 4 }",
		"    }",
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

func TestM22aInvalidBracketReturnsError(t *testing.T) {
	root := setupM22aNumericsFixture(t)
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Numerics",
		"",
		"fn Main() -> Int {",
		"    let options = Numerics.SolveOptions { MaxIterations: 10 Tolerance: 0.000001 }",
		"    match Numerics.Bisection(Numerics.ProblemSqrt2(), 3.0, 4.0, options) {",
		"        ok(result) => { return 1 }",
		"        err(e) => { return 0 }",
		"    }",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "0\n" {
		t.Fatalf("expected error branch code 0, got %q", stdout)
	}
}

func TestM22aZeroDerivativeReturnsError(t *testing.T) {
	root := setupM22aNumericsFixture(t)
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Numerics",
		"",
		"fn Main() -> Int {",
		"    let options = Numerics.SolveOptions { MaxIterations: 10 Tolerance: 0.000001 }",
		"    match Numerics.NewtonRaphson(Numerics.ProblemSqrt2(), 0.0, options) {",
		"        ok(result) => { return 1 }",
		"        err(e) => { return 0 }",
		"    }",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "0\n" {
		t.Fatalf("expected error branch code 0, got %q", stdout)
	}
}

func TestM22aMaxIterationsReturnsStructuredResult(t *testing.T) {
	root := setupM22aNumericsFixture(t)
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Numerics",
		"",
		"fn Main() -> Int {",
		"    let options = Numerics.SolveOptions { MaxIterations: 1 Tolerance: 0.000000000001 }",
		"    match Numerics.Bisection(Numerics.ProblemSqrt2(), 1.0, 2.0, options) {",
		"        ok(result) => {",
		"            if result.Converged { return 1 }",
		"            if result.ReasonCode != Numerics.ReasonMaxIterationsReached() { return 2 }",
		"            if result.Iterations != 1 { return 3 }",
		"            return 0",
		"        }",
		"        err(e) => { return 4 }",
		"    }",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "0\n" {
		t.Fatalf("expected max-iteration code 0, got %q", stdout)
	}
}

func TestM22aPackageIntegrationRunAndBuild(t *testing.T) {
	root := setupM22aFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "Converged: true") {
		t.Fatalf("expected converged result output, got %q", stdout)
	}
	if !strings.Contains(stdout, "invalid bracket") {
		t.Fatalf("expected invalid bracket output, got %q", stdout)
	}

	buildStdout, buildStderr, buildErr := executeCLI("build", entry)
	if buildErr == nil {
		t.Fatalf("expected build failure for unsupported compiled feature, got success with stdout %q", buildStdout)
	}
	if buildStdout != "" {
		t.Fatalf("expected empty build stdout, got %q", buildStdout)
	}
	if !strings.Contains(buildStderr, "compiled mode does not yet support match") {
		t.Fatalf("expected unsupported match diagnostic, got %q", buildStderr)
	}
	if _, statErr := os.Stat(entry + ".octbin"); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
	}
}

func TestM22aBuildFailureDoesNotEmitArtifact(t *testing.T) {
	root := setupM22aNumericsFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", "package Main\nimport Numerics\nfn Main() -> Int { return Numerics.Missing() }\n")

	stdout, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatalf("expected build failure, got success with stdout %q", stdout)
	}
	if !strings.Contains(stderr, "package 'Numerics' has no function 'Missing'") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
	if _, statErr := os.Stat(entry + ".octbin"); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact, stat err = %v", statErr)
	}
}

func setupM22aNumericsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Packages", "Numerics"), filepath.Join(root, "Numerics"))
	return root
}

func setupM22aFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Packages", "Numerics"), filepath.Join(root, "Numerics"))
	copyDir(t, filepath.Join("..", "..", "testdata", "m22a", "valid", "Main"), filepath.Join(root, "Main"))
	return root
}

func copyDir(t *testing.T, src string, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read dir %s: %v", src, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			copyDir(t, srcPath, dstPath)
			continue
		}
		data, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			t.Fatalf("read file %s: %v", srcPath, readErr)
		}
		if writeErr := os.WriteFile(dstPath, data, 0o644); writeErr != nil {
			t.Fatalf("write file %s: %v", dstPath, writeErr)
		}
	}
}
