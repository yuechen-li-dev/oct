//go:build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func TestOctTestRunsFactFilesAndReportsPassFail(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Math", "math.oct", "package Math\nfn FortyTwo() -> Int { return 42 }\n")
	writeOctPkgFile(t, root, "Math", "math.octest", strings.Join([]string{
		"package Math",
		"[Fact]",
		"fn Alpha() -> Void { Assert.Equal(42, FortyTwo(), \"alpha\") }",
		"[Fact]",
		"fn Beta() -> Void { Assert.True(false, \"beta fails\") }",
		"[Fact]",
		"fn Gamma() -> Void { let x = 1 / 0 Assert.True(x > 0, \"unreachable\") }",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"test", root, "--all-packages"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected test failure")
	}
	if !strings.Contains(stdout.String(), "PASS Math.Alpha") {
		t.Fatalf("expected pass output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL Math.Beta") {
		t.Fatalf("expected fail output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL Math.Gamma") {
		t.Fatalf("expected runtime fail output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "test failed: 2 test(s) failed") {
		t.Fatalf("expected command failure summary, got %q", stderr.String())
	}
}

func TestOctTestDeterministicOrderingAndVoidReturns(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "order.octest", strings.Join([]string{
		"package Main",
		"[Fact]",
		"fn Zeta() -> Void { Assert.Equal(1, 1, \"zeta executed\") }",
		"[Fact]",
		"fn Alpha() -> Void { Assert.Near(2.0, 1.9999, 0.01, \"near passes\") }",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"test", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected success, got err=%v stderr=%q", err, stderr.String())
	}
	alpha := strings.Index(stdout.String(), "PASS Main.Alpha")
	zeta := strings.Index(stdout.String(), "PASS Main.Zeta")
	if alpha < 0 || zeta < 0 || alpha > zeta {
		t.Fatalf("expected alphabetical order, got %q", stdout.String())
	}
}

func TestOctTestRejectsInvalidFactShapesAndVoidRules(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bad.octest", "package Main\n[Fact]\nfn Bad(x: Int) -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"test", root}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "[Fact] function must not declare parameters") {
		t.Fatalf("expected invalid [Fact] error, got err=%v stderr=%q", err, stderr.String())
	}

	root2 := t.TempDir()
	writeOctPkgFile(t, root2, "Main", "main.oct", "package Main\n[Fact]\nfn NotAllowed() -> Int { return 1 }\nfn Main() -> Int { return 0 }\n")
	stdout.Reset()
	stderr.Reset()
	err = cli.Execute([]string{"build", filepath.Join(root2, "Main", "main.oct")}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "[Fact] is only valid in .octest files") {
		t.Fatalf("expected .oct [Fact] rejection, got err=%v stderr=%q", err, stderr.String())
	}
}

func TestOctTestRunsTheoryInlineDataCasesAndReportsPerCase(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "theory.octest", strings.Join([]string{
		"package Main",
		"[Theory]",
		"[InlineData(1, 2, 3)]",
		"[InlineData(2, 2, 5)]",
		"fn AddsCorrectly(a: Int, b: Int, expected: Int) -> Void { Assert.Equal(expected, a + b, \"sum mismatch\") }",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"test", root}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected one failing theory case")
	}
	if !strings.Contains(stdout.String(), "PASS Main.AddsCorrectly[0]") {
		t.Fatalf("expected first theory case pass output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL Main.AddsCorrectly[1]") {
		t.Fatalf("expected second theory case fail output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "test failed: 1 test(s) failed") {
		t.Fatalf("expected overall failure summary, got %q", stderr.String())
	}
}

func TestOctTestRejectsInvalidTheoryAndInlineDataShapes(t *testing.T) {
	type testCase struct {
		name    string
		content string
		wantErr string
	}

	cases := []testCase{
		{
			name:    "zero parameter theory",
			content: "package Main\n[Theory]\n[InlineData(1)]\nfn Bad() -> Void { return }\n",
			wantErr: "[Theory] function must declare at least one parameter",
		},
		{
			name:    "theory with no data",
			content: "package Main\n[Theory]\nfn Bad(x: Int) -> Void { return }\n",
			wantErr: "[Theory] function must declare at least one [InlineData] row",
		},
		{
			name:    "arity mismatch",
			content: "package Main\n[Theory]\n[InlineData(1)]\nfn Bad(x: Int, y: Int) -> Void { return }\n",
			wantErr: "argument count mismatch",
		},
		{
			name:    "type mismatch",
			content: "package Main\n[Theory]\n[InlineData(\"oops\")]\nfn Bad(x: Int) -> Void { return }\n",
			wantErr: "expects Int, got String",
		},
		{
			name:    "inline data on non theory",
			content: "package Main\n[InlineData(1)]\nfn Bad(x: Int) -> Void { return }\n",
			wantErr: "[InlineData] must apply to a [Theory] function",
		},
		{
			name:    "fact and theory together",
			content: "package Main\n[Fact]\n[Theory]\n[InlineData(1)]\nfn Bad(x: Int) -> Void { return }\n",
			wantErr: "[Fact] and [Theory] cannot both apply to the same function",
		},
		{
			name:    "inline data on fact",
			content: "package Main\n[Fact]\n[InlineData(1)]\nfn Bad() -> Void { return }\n",
			wantErr: "[InlineData] cannot be used with [Fact]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
			writeOctPkgFile(t, root, "Main", "bad.octest", tc.content)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := cli.Execute([]string{"test", root}, &stdout, &stderr)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got err=%v stderr=%q stdout=%q", tc.wantErr, err, stderr.String(), stdout.String())
			}
		})
	}
}

func TestOctTestSuiteSelection(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nimport Lib\nfn Main() -> Int { return Lib.One() }\n")
	writeOctPkgFile(t, root, "Main", "suite.octest", "package Main\nimport Lib\n[Suite(\"A\")]\n[Fact]\nfn InA() -> Void { Assert.Equal(1, Lib.One(), \"a\") }\n[Suite(\"B\")]\n[Fact]\nfn InB() -> Void { Assert.True(false, \"b fail\") }\n[Fact]\nfn Unsuited() -> Void { Assert.True(false, \"unsuited fail\") }\n[Suite(\"Slow\")]\n[Theory]\n[CycleTime(45.0s)]\n[InlineData(1)]\nfn SlowTheory(x: Int) -> Void { Assert.Equal(1, x, \"ok\") }\n")
	writeOctPkgFile(t, root, "Lib", "lib.oct", "package Lib\nfn One() -> Int { return 1 }\n")
	writeOctPkgFile(t, root, "Lib", "lib.octest", "package Lib\n[Suite(\"Libraries.Bad\")]\n[Fact]\nfn Bad() -> Void { Assert.True(false, \"library bad\") }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"test", root, "--suite", "A"}, &stdout, &stderr); err != nil {
		t.Fatalf("suite A should pass, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "PASS Main.InA") || strings.Contains(stdout.String(), "InB") || strings.Contains(stdout.String(), "Unsuited") || strings.Contains(stdout.String(), "Lib.Bad") {
		t.Fatalf("unexpected suite output: %q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	err := cli.Execute([]string{"test", root, "--suite", "Libraries.Bad"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "no tests found for suite `Libraries.Bad`") {
		t.Fatalf("expected default entry-package filtering for library suite, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	err = cli.Execute([]string{"test", root, "--suite", "Libraries.Bad", "--all-packages"}, &stdout, &stderr)
	if err == nil || !strings.Contains(stdout.String(), "FAIL Lib.Bad") {
		t.Fatalf("expected --all-packages library suite failure, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if err := cli.Execute([]string{"test", root, "--suite", "Slow"}, &stdout, &stderr); err != nil || !strings.Contains(stdout.String(), "PASS Main.SlowTheory[0]") {
		t.Fatalf("slow suite should pass, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
}

func writeOctPkgFile(t *testing.T, root, pkg, name, content string) {
	t.Helper()
	dir := filepath.Join(root, pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifestPath := filepath.Join(dir, "manifest.oct")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		manifest := strings.Join([]string{
			"package Manifest",
			"",
			"record PackageManifest {",
			"    Name: String",
			"    Version: String",
			"    Description: String",
			"    Dependencies: Dependency[]",
			"}",
			"",
			"record Dependency {",
			"    Name: String",
			"    VersionRequirement: String",
			"}",
			"",
			"fn Manifest() -> PackageManifest {",
			"    return PackageManifest {",
			"        Name: \"" + pkg + "\"",
			"        Version: \"0.1.0\"",
			"        Description: \"" + pkg + " test package\"",
			"        Dependencies: [Dependency { Name: \"OctStd\" VersionRequirement: \"0.1.0\" }]",
			"    }",
			"}",
			"",
		}, "\n")
		if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
