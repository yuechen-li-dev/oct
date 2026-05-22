package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/cli"
)

func TestBuildAcceptsValidManifestedPackageTree(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "manifest.oct", manifestSource("Main", "demo entry"))
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nimport Numerics\nfn Main() -> Int { let x = Numerics.One() return x }\n")
	writeOctPkgFile(t, root, "Numerics", "manifest.oct", manifestSource("Numerics", "proof package"))
	writeOctPkgFile(t, root, "Numerics", "numerics.oct", "package Numerics\nfn One() -> Int { return 1 }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"build", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected build success, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
}

func TestBuildRejectsMissingDependencyManifestWhenRootManifestExists(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "manifest.oct", manifestSource("Main", "demo entry"))
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nimport Util\nfn Main() -> Int { return Util.One() }\n")
	utilDir := filepath.Join(root, "Util")
	if err := os.MkdirAll(utilDir, 0o755); err != nil {
		t.Fatalf("mkdir util: %v", err)
	}
	if err := os.WriteFile(filepath.Join(utilDir, "util.oct"), []byte("package Util\nfn One() -> Int { return 1 }\n"), 0o644); err != nil {
		t.Fatalf("write util: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"build", root}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "package manifest missing") {
		t.Fatalf("expected package manifest missing error, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
}

func TestBuildRejectsInvalidManifestShape(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "manifest.oct", strings.Join([]string{
		"package Manifest",
		"record PackageManifest {",
		"    Name: String",
		"    Version: String",
		"    Description: String",
		"    Dependencies: Dependency[]",
		"}",
		"record Dependency {",
		"    Name: String",
		"    VersionRequirement: String",
		"}",
		"fn NotManifest() -> Int { return 1 }",
	}, "\n")+"\n")
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"build", root}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "manifest.oct must define fn Manifest() -> PackageManifest") {
		t.Fatalf("expected missing Manifest() error, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
}

func manifestSource(name string, description string) string {
	return strings.Join([]string{
		"package Manifest",
		"record PackageManifest {",
		"    Name: String",
		"    Version: String",
		"    Description: String",
		"    Dependencies: Dependency[]",
		"}",
		"record Dependency {",
		"    Name: String",
		"    VersionRequirement: String",
		"}",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"" + name + "\"",
		"        Version: \"0.1.0\"",
		"        Description: \"" + description + "\"",
		"        Dependencies: [Dependency { Name: \"OctStd\" VersionRequirement: \"0.1.0\" }]",
		"    }",
		"}",
	}, "\n") + "\n"
}
