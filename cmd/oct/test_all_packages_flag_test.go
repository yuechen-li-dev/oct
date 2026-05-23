package main

import (
	"strings"
	"testing"
)

func TestDirectoryTargetDefaultsToEntryPackageTestsOnly(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Lib", "lib.oct", "package Lib\nfn FortyTwo() -> Int { return 42 }\n")
	writeOctPkgFile(t, root, "Lib", "lib.octest", "package Lib\n[Fact]\nfn LibraryFact() -> Void { Assert.Equal(1, 2, \"must fail if executed\") }\n")
	writeOctPkgFile(t, root, "Main", "main.octest", strings.Join([]string{
		"package Main",
		"import Lib",
		"[Fact]",
		"fn AppFact() -> Void {",
		"    Assert.Equal(42, Lib.FortyTwo(), \"main test runs\")",
		"}",
	}, "\n")+"\n")

	stdout, stderr, err := executeCLIArgs("test", root)
	if err != nil {
		t.Fatalf("expected default package-only run to pass, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Main.AppFact") {
		t.Fatalf("expected Main.AppFact to run, got %q", stdout)
	}
	if strings.Contains(stdout, "LibraryFact") {
		t.Fatalf("expected LibraryFact to be excluded by default, got %q", stdout)
	}
}

func TestDirectoryTargetAllPackagesIncludesImports(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Lib", "lib.oct", "package Lib\nfn FortyTwo() -> Int { return 42 }\n")
	writeOctPkgFile(t, root, "Lib", "lib.octest", "package Lib\n[Fact]\nfn LibraryFact() -> Void { Assert.Equal(42, FortyTwo(), \"library test runs when requested\") }\n")
	writeOctPkgFile(t, root, "Main", "main.octest", strings.Join([]string{
		"package Main",
		"import Lib",
		"[Fact]",
		"fn AppFact() -> Void {",
		"    Assert.Equal(42, Lib.FortyTwo(), \"main test runs\")",
		"}",
	}, "\n")+"\n")

	stdout, stderr, err := executeCLIArgs("test", root, "--all-packages")
	if err != nil {
		t.Fatalf("expected --all-packages run to pass, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Main.AppFact") || !strings.Contains(stdout, "PASS Lib.LibraryFact") {
		t.Fatalf("expected both package tests, got %q", stdout)
	}
}

