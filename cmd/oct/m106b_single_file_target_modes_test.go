package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestM106bExplicitSingleFileTargetIgnoresInvalidSiblingAndManifestInAllModes(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "manifest.oct", manifestSource("WrongName", "intentionally invalid for directory target strictness"))
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn One() -> Int { return 1 }\n")
	writeOctPkgFile(t, root, "Main", "selected.octest", "package Main\n[Fact]\nfn SelectedPasses() -> Void { Assert.Equal(1, One(), \"selected runs\") }\n")
	writeOctPkgFile(t, root, "Main", "sibling_broken.octest", "package Main\n[Fact]\nfn Broken() -> Void { let x = [ }\n")

	target := filepath.Join(root, "Main", "selected.octest")
	for _, mode := range []string{"interpreted", "compiled", "auto"} {
		stdout, stderr, err := executeCLIArgs("test", target, "--execution", mode)
		if err != nil {
			t.Fatalf("expected explicit single-file target to pass in %s mode, err=%v stderr=%q stdout=%q", mode, err, stderr, stdout)
		}
		if !strings.Contains(stdout, "PASS Main.SelectedPasses") {
			t.Fatalf("expected selected test pass output in %s mode, got %q", mode, stdout)
		}
		if strings.Contains(stdout, "Broken") || strings.Contains(stderr, "sibling_broken.octest") {
			t.Fatalf("unexpected sibling contamination in %s mode, stderr=%q stdout=%q", mode, stderr, stdout)
		}
	}
}
