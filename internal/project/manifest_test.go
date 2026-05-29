package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAcceptsManifestOptionalMetadataSchema(t *testing.T) {
	root := t.TempDir()
	writeProjectPackage(t, root, "OptionalManifestFields", optionalManifestSource("OptionalManifestFields"), "package OptionalManifestFields\nfn Main() -> Int { return 0 }\n")

	if _, err := Load(filepath.Join(root, "OptionalManifestFields")); err != nil {
		t.Fatalf("Load should accept optional manifest fields: %v", err)
	}
}

func TestLoadRejectsUnsupportedManifestFields(t *testing.T) {
	t.Run("package manifest literal", func(t *testing.T) {
		root := t.TempDir()
		manifest := strings.Replace(optionalManifestSource("Main"), "        Dependencies: [", "        Unsupported: \"nope\"\n        Dependencies: [", 1)
		writeProjectPackage(t, root, "Main", manifest, "package Main\nfn Main() -> Int { return 0 }\n")
		_, err := Load(root)
		if err == nil {
			t.Fatalf("expected unsupported manifest field to be rejected")
		}
	})

	t.Run("dependency literal", func(t *testing.T) {
		root := t.TempDir()
		manifest := strings.Replace(optionalManifestSource("Main"), "                Source: \"builtin\"", "                Source: \"builtin\"\n                Unsupported: \"nope\"", 1)
		writeProjectPackage(t, root, "Main", manifest, "package Main\nfn Main() -> Int { return 0 }\n")
		_, err := Load(root)
		if err == nil {
			t.Fatalf("expected unsupported dependency field to be rejected")
		}
	})
}

func TestLoadRejectsNonStringOptionalManifestFields(t *testing.T) {
	cases := []struct {
		name string
		old  string
		new  string
	}{
		{name: "Kind", old: "        Kind: \"experiment\"", new: "        Kind: 3"},
		{name: "EntryMilestone", old: "        EntryMilestone: \"M0\"", new: "        EntryMilestone: 3"},
		{name: "Source", old: "                Source: \"builtin\"", new: "                Source: 3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := strings.Replace(optionalManifestSource("Main"), tc.old, tc.new, 1)
			writeProjectPackage(t, root, "Main", manifest, "package Main\nfn Main() -> Int { return 0 }\n")
			_, err := Load(root)
			if err == nil {
				t.Fatalf("expected non-string optional field %s to be rejected", tc.name)
			}
		})
	}
}

func writeProjectPackage(t *testing.T, root string, name string, manifest string, source string) {
	t.Helper()
	pkgDir := filepath.Join(root, name)
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir package: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "manifest.oct"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, strings.ToLower(name)+".oct"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
}

func optionalManifestSource(name string) string {
	return strings.Join([]string{
		"package Manifest",
		"",
		"record PackageManifest {",
		"    Name: String",
		"    Version: String",
		"    Description: String",
		"    Kind: String",
		"    EntryMilestone: String",
		"    Dependencies: Dependency[]",
		"}",
		"",
		"record Dependency {",
		"    Name: String",
		"    VersionRequirement: String",
		"    Source: String",
		"}",
		"",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"" + name + "\"",
		"        Version: \"0.1.0\"",
		"        Description: \"Manifest with optional fields accepted by project loader\"",
		"        Kind: \"experiment\"",
		"        EntryMilestone: \"M0\"",
		"        Dependencies: [",
		"            Dependency {",
		"                Name: \"OctStd\"",
		"                VersionRequirement: \"0.1.0\"",
		"                Source: \"builtin\"",
		"            }",
		"        ]",
		"    }",
		"}",
	}, "\n") + "\n"
}
