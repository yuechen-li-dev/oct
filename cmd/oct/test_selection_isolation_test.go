package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/cli"
)

func TestSelectionIsolationExplicitFileDoesNotRunImportedPackageTests(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Lib", "lib.oct", "package Lib\nfn FortyTwo() -> Int { return 42 }\n")
	writeOctPkgFile(t, root, "Lib", "lib_with_tests.octest", "package Lib\n[Fact]\nfn DeliberateFailure() -> Void { Assert.Equal(1, 2, \"must fail when selected\") }\n")
	writeOctPkgFile(t, root, "Main", "main_explicit.octest", strings.Join([]string{
		"package Main",
		"import Lib",
		"[Fact]",
		"fn UsesImportedLibrary() -> Void {",
		"    Assert.Equal(42, Lib.FortyTwo(), \"imports still resolve\")",
		"}",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"test", filepath.Join(root, "Main", "main_explicit.octest")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected explicit test to pass, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "PASS Main.UsesImportedLibrary") {
		t.Fatalf("expected explicit main test to run, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "DeliberateFailure") {
		t.Fatalf("unexpected imported package test execution, got %q", stdout.String())
	}
}

func TestSelectionIsolationDirectlySelectedLibraryTestStillRuns(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Lib", "lib.oct", "package Lib\nfn FortyTwo() -> Int { return 42 }\n")
	writeOctPkgFile(t, root, "Lib", "lib_with_tests.octest", "package Lib\n[Fact]\nfn DeliberateFailure() -> Void { Assert.Equal(1, 2, \"must fail when selected\") }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"test", filepath.Join(root, "Lib", "lib_with_tests.octest")}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected selected library test to fail, stderr=%q stdout=%q", stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL Lib.DeliberateFailure") {
		t.Fatalf("expected deliberate library failure, got %q", stdout.String())
	}
}

func TestSelectionIsolationMultipleExplicitFilesRunOnlySelectedFiles(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Deps", "dep.oct", "package Deps\nfn Marker() -> Int { return 7 }\n")
	writeOctPkgFile(t, root, "Deps", "dep_fail.octest", "package Deps\n[Fact]\nfn DependencyFailure() -> Void { Assert.Equal(1, 2, \"must not run unless selected\") }\n")
	writeOctPkgFile(t, root, "Main", "a.octest", "package Main\nimport Deps\n[Fact]\nfn A() -> Void { Assert.Equal(7, Deps.Marker(), \"a runs\") }\n")
	writeOctPkgFile(t, root, "Main", "b.octest", "package Main\nimport Deps\n[Fact]\nfn B() -> Void { Assert.Equal(7, Deps.Marker(), \"b runs\") }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"test", filepath.Join(root, "Main", "a.octest"), filepath.Join(root, "Main", "b.octest")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected selected explicit files to pass, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "PASS Main.A") || !strings.Contains(out, "PASS Main.B") {
		t.Fatalf("expected both explicit files to run, got %q", out)
	}
	if strings.Contains(out, "DependencyFailure") {
		t.Fatalf("unexpected dependency test execution, got %q", out)
	}
}

func TestSelectionIsolationDirectoryModeDiscoversWithinDirectoryOnly(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "Inside")
	outside := filepath.Join(root, "Outside")
	writeOctPkgFile(t, inside, "Main", "inside.octest", "package Main\n[Fact]\nfn InsidePass() -> Void { Assert.Equal(1, 1, \"inside runs\") }\n")
	writeOctPkgFile(t, outside, "Main", "outside.octest", "package Main\n[Fact]\nfn OutsideFail() -> Void { Assert.Equal(1, 2, \"outside should not run\") }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"test", inside}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected directory target to pass, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "PASS Main.InsidePass") {
		t.Fatalf("expected discovered test in requested directory, got %q", out)
	}
	if strings.Contains(out, "OutsideFail") {
		t.Fatalf("unexpected outside directory test execution, got %q", out)
	}
}
