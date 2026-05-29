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

func TestLoadManifestMetadataExtractsOptionalFields(t *testing.T) {
	manifestPath := writeManifest(t, strings.Join([]string{
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
		"        Name: \"DemoPkg\"",
		"        Version: \"0.1.0\"",
		"        Description: \"demo package\"",
		"        Kind: \"experiment\"",
		"        EntryMilestone: \"M0\"",
		"        Dependencies: [",
		"            Dependency { Name: \"Signal\" VersionRequirement: \"^1.2.0\" Source: \"builtin\" }",
		"        ]",
		"    }",
		"}",
	}, "\n")+"\n")

	metadata, err := loadManifestMetadata(manifestPath)
	if err != nil {
		t.Fatalf("load manifest metadata: %v", err)
	}
	if metadata.Kind != "experiment" {
		t.Fatalf("expected kind experiment, got %q", metadata.Kind)
	}
	if metadata.EntryMilestone != "M0" {
		t.Fatalf("expected entry milestone M0, got %q", metadata.EntryMilestone)
	}
	if len(metadata.Dependencies) != 1 || metadata.Dependencies[0].Source != "builtin" {
		t.Fatalf("expected dependency source builtin, got %#v", metadata.Dependencies)
	}
}

func TestLoadManifestMetadataRejectsUnsupportedFields(t *testing.T) {
	t.Run("package manifest record", func(t *testing.T) {
		manifestPath := writeManifest(t, validManifestWithEdits(
			"    Unsupported: String\n    Dependencies: Dependency[]",
			"}",
			"",
		))
		_, err := loadManifestMetadata(manifestPath)
		if err == nil || !strings.Contains(err.Error(), "Unsupported") {
			t.Fatalf("expected unsupported PackageManifest field error, got %v", err)
		}
	})

	t.Run("dependency record", func(t *testing.T) {
		manifestPath := writeManifest(t, validManifestWithEdits(
			"    Dependencies: Dependency[]",
			"    Extra: String\n}",
			"",
		))
		_, err := loadManifestMetadata(manifestPath)
		if err == nil || !strings.Contains(err.Error(), "Extra") {
			t.Fatalf("expected unsupported Dependency field error, got %v", err)
		}
	})

	t.Run("package manifest literal", func(t *testing.T) {
		manifestPath := writeManifest(t, validManifestWithEdits(
			"    Dependencies: Dependency[]",
			"}",
			"        Unsupported: \"nope\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
		))
		_, err := loadManifestMetadata(manifestPath)
		if err == nil || !strings.Contains(err.Error(), "Unsupported") {
			t.Fatalf("expected unsupported PackageManifest literal field error, got %v", err)
		}
	})

	t.Run("dependency literal", func(t *testing.T) {
		manifestPath := writeManifest(t, validManifestWithEdits(
			"    Dependencies: Dependency[]",
			"}",
			"        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" Extra: \"nope\" }]",
		))
		_, err := loadManifestMetadata(manifestPath)
		if err == nil || !strings.Contains(err.Error(), "Extra") {
			t.Fatalf("expected unsupported Dependency literal field error, got %v", err)
		}
	})
}

func TestLoadManifestMetadataRejectsNonStringOptionalFields(t *testing.T) {
	cases := []struct {
		name        string
		recordPatch string
		depPatch    string
		bodyPatch   string
		want        string
	}{
		{
			name:        "kind",
			recordPatch: "    Kind: String\n    Dependencies: Dependency[]",
			depPatch:    "}",
			bodyPatch:   "        Kind: 3\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			want:        "Kind",
		},
		{
			name:        "entry milestone",
			recordPatch: "    EntryMilestone: String\n    Dependencies: Dependency[]",
			depPatch:    "}",
			bodyPatch:   "        EntryMilestone: 3\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			want:        "EntryMilestone",
		},
		{
			name:        "source",
			recordPatch: "    Dependencies: Dependency[]",
			depPatch:    "    Source: String\n}",
			bodyPatch:   "        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" Source: 3 }]",
			want:        "Source",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifestPath := writeManifest(t, validManifestWithEdits(tc.recordPatch, tc.depPatch, tc.bodyPatch))
			_, err := loadManifestMetadata(manifestPath)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %s error, got %v", tc.want, err)
			}
		})
	}
}

func validManifestWithEdits(packageRecordTail string, dependencyRecordTail string, bodyDependencies string) string {
	if bodyDependencies == "" {
		bodyDependencies = "        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]"
	}
	return strings.Join([]string{
		"package Manifest",
		"record PackageManifest {",
		"    Name: String",
		"    Version: String",
		"    Description: String",
		packageRecordTail,
		"}",
		"record Dependency {",
		"    Name: String",
		"    VersionRequirement: String",
		dependencyRecordTail,
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"DemoPkg\"",
		"        Version: \"0.1.0\"",
		"        Description: \"demo package\"",
		bodyDependencies,
		"    }",
		"}",
	}, "\n") + "\n"
}
