package pkgmgr

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"os"
)

func TestLoadManifestMetadataExtractsIdentityAndDependencies(t *testing.T) {
	manifestPath := writeManifest(t, strings.Join([]string{
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
		"    Source: String",
		"}",
		"",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"DemoPkg\"",
		"        Version: \"0.1.0\"",
		"        Description: \"demo package\"",
		"        Dependencies: [",
		"            Dependency { Name: \"Signal\" VersionRequirement: \"^1.2.0\" Source: \"file:///tmp/signal\" },",
		"            Dependency { Name: \"Numerics\" VersionRequirement: \"~0.4\" Source: \"file:///tmp/numerics\" }",
		"        ]",
		"    }",
		"}",
	}, "\n")+"\n")

	metadata, err := loadManifestMetadata(manifestPath)
	if err != nil {
		t.Fatalf("load manifest metadata: %v", err)
	}
	if metadata.Name != "DemoPkg" {
		t.Fatalf("expected name DemoPkg, got %q", metadata.Name)
	}
	if metadata.Version != "0.1.0" {
		t.Fatalf("expected version 0.1.0, got %q", metadata.Version)
	}
	if metadata.Description != "demo package" {
		t.Fatalf("expected description, got %q", metadata.Description)
	}
	expectedDeps := []DependencyMetadata{
		{Name: "Signal", VersionRequirement: "^1.2.0", Source: "file:///tmp/signal"},
		{Name: "Numerics", VersionRequirement: "~0.4", Source: "file:///tmp/numerics"},
	}
	if !reflect.DeepEqual(metadata.Dependencies, expectedDeps) {
		t.Fatalf("unexpected dependencies: got %#v want %#v", metadata.Dependencies, expectedDeps)
	}
}

func TestLoadManifestMetadataRejectsMalformedDependency(t *testing.T) {
	manifestPath := writeManifest(t, strings.Join([]string{
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
		"        Name: \"DemoPkg\"",
		"        Version: \"0.1.0\"",
		"        Description: \"demo package\"",
		"        Dependencies: [",
		"            Dependency { Name: \"Signal\" VersionRequirement: 3 }",
		"        ]",
		"    }",
		"}",
	}, "\n")+"\n")

	_, err := loadManifestMetadata(manifestPath)
	if err == nil {
		t.Fatalf("expected malformed dependency error")
	}
	if !strings.Contains(err.Error(), "manifest dependency at index 0") {
		t.Fatalf("expected dependency index in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "VersionRequirement") {
		t.Fatalf("expected field detail in error, got %v", err)
	}
}

func TestLoadManifestMetadataDeterministic(t *testing.T) {
	manifestPath := writeManifest(t, strings.Join([]string{
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
		"        Name: \"DemoPkg\"",
		"        Version: \"0.1.0\"",
		"        Description: \"demo package\"",
		"        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"^1.2.0\" }]",
		"    }",
		"}",
	}, "\n")+"\n")

	first, err := loadManifestMetadata(manifestPath)
	if err != nil {
		t.Fatalf("first read failed: %v", err)
	}
	second, err := loadManifestMetadata(manifestPath)
	if err != nil {
		t.Fatalf("second read failed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("manifest reads are not deterministic: first=%#v second=%#v", first, second)
	}
}

func writeManifest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.oct")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}
