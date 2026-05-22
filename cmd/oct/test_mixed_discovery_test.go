package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMixedDiscoveryMixedOctestAndOctfailExecuteTogether(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "mixed.octest", "package Main\n[Fact]\nfn Runs() -> Void { Assert.Equal(1, 1, \"fact runs\") }\n")
	fixture := strings.Join([]string{
		"expect error: \"undefined variable: missing_name\"",
		"",
		"package Main",
		"",
		"fn Main() -> Int {",
		"    let _ = missing_name",
		"    return 0",
		"}",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "Main", "mixed.invalid.octfail"), []byte(fixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	stdout, stderr, err := executeCLI("test", filepath.Join(root, "Main"))
	if err != nil {
		t.Fatalf("expected mixed suite to pass, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Main.Runs") {
		t.Fatalf("expected .octest fact output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS mixed.invalid.octfail") {
		t.Fatalf("expected .octfail output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Result: 2 passed, 0 failed") {
		t.Fatalf("expected combined summary, got %q", stdout)
	}
}

func TestMixedDiscoveryOctfailDoesNotMaskOctestParseErrors(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "broken.octest", "package Main\n[Fact]\nfn Broken() -> Void { let x = [] }\n")
	fixture := strings.Join([]string{
		"expect error: \"undefined variable: missing_name\"",
		"",
		"package Main",
		"",
		"fn Main() -> Int {",
		"    let _ = missing_name",
		"    return 0",
		"}",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "Main", "mixed.invalid.octfail"), []byte(fixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, stderr, err := executeCLI("test", filepath.Join(root, "Main"))
	if err == nil {
		t.Fatalf("expected parse failure from .octest")
	}
	if !strings.Contains(stderr, "empty array literal `[]` requires an expected array type") {
		t.Fatalf("expected octest parse error to surface, got %q", stderr)
	}
}
