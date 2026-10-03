package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const strictnessManifestRecords = `package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}
`

func strictnessManifest(name string) string {
	return strictnessManifestRecords + `
fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "` + name + `"
        Version: "0.1.0"
        Description: "fixture"
        Dependencies: []
    }
}
`
}

func writeStrictnessFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The manifest of an imported package is read whenever it exists. A program
// in a directory that requires no manifest still gets the error when it
// imports a package whose manifest is wrong: dropping the manifest would drop
// the package's wrapper declarations with it.
func TestImportedManifestIsValidatedWhereManifestsAreNotRequired(t *testing.T) {
	repo := t.TempDir()
	library := filepath.Join(repo, "Libraries", "Widget")
	writeStrictnessFile(t, filepath.Join(library, "Widget.oct"), "package Widget\n\nfn Size() -> Int {\n    return 3\n}\n")
	program := filepath.Join(repo, "work", "main.oct")
	writeStrictnessFile(t, program, "package Main\n\nimport Widget\n\nfn Main() -> Int {\n    return Widget.Size()\n}\n")

	writeStrictnessFile(t, filepath.Join(library, "manifest.oct"), strictnessManifest("Widget"))
	if _, err := Load(program); err != nil {
		t.Fatalf("a valid manifest was rejected: %v", err)
	}

	writeStrictnessFile(t, filepath.Join(library, "manifest.oct"), strictnessManifest("SomethingElse"))
	if _, err := Load(program); err == nil || !strings.Contains(err.Error(), "invalid package metadata") {
		t.Fatalf("a manifest naming another package was accepted: %v", err)
	}

	writeStrictnessFile(t, filepath.Join(library, "manifest.oct"), "package Manifest\n\nfn Manifest( {\n")
	if _, err := Load(program); err == nil {
		t.Fatalf("a manifest that does not parse was accepted")
	}
}

// A milestone directory with no manifest borrows the one of its experiment
// family. That manifest names the family, not the milestone's package, so it
// is not held to the package name where manifests are not required.
func TestBorrowedFamilyManifestIsNotHeldToTheMilestonePackageName(t *testing.T) {
	family := filepath.Join(t.TempDir(), "Experiments", "Signal")
	writeStrictnessFile(t, filepath.Join(family, "manifest.oct"), strictnessManifest("Signal"))
	writeStrictnessFile(t, filepath.Join(family, "REPORT.md"), "# Signal\n")
	milestone := filepath.Join(family, "M4", "signal_m4.oct")
	writeStrictnessFile(t, milestone, "package SignalM4\n\nfn Main() -> Int {\n    return 4\n}\n")

	program, err := Load(milestone)
	if err != nil {
		t.Fatalf("a milestone under a family manifest did not load: %v", err)
	}
	if program.Entry != "SignalM4" {
		t.Fatalf("entry package = %q", program.Entry)
	}
}

// A file selected on its own runs beside a manifest that is wrong. Only an
// imported package's manifest is held to account where none is required.
func TestEntryPackageManifestIsNotValidatedForASelectedFile(t *testing.T) {
	directory := t.TempDir()
	writeStrictnessFile(t, filepath.Join(directory, "manifest.oct"), strictnessManifest("SomethingElse"))
	program := filepath.Join(directory, "main.octest")
	writeStrictnessFile(t, program, "package Main\n\n[Fact]\nfn Holds() -> Void {\n    Assert.True(true, \"runs\")\n}\n")
	if _, err := LoadForTest(program); err != nil {
		t.Fatalf("a selected file was refused for its directory's manifest: %v", err)
	}
}
