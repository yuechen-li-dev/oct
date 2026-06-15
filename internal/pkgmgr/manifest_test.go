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
	if metadata.Kind != "pure" {
		t.Fatalf("expected missing Kind to normalize to pure, got %q", metadata.Kind)
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

func TestLoadManifestMetadataPackageKindSemantics(t *testing.T) {
	cases := []struct {
		name           string
		recordPatch    string
		bodyPatch      string
		wantKind       string
		wantEntry      string
		wantErr        bool
		wantErrContent string
	}{
		{
			name:        "missing Kind normalizes to pure",
			recordPatch: "    Dependencies: Dependency[]",
			bodyPatch:   "        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantKind:    "pure",
		},
		{
			name:        "empty Kind normalizes to pure",
			recordPatch: "    Kind: String\n    Dependencies: Dependency[]",
			bodyPatch:   "        Kind: \"\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantKind:    "pure",
		},
		{
			name:        "pure Kind accepted",
			recordPatch: "    Kind: String\n    Dependencies: Dependency[]",
			bodyPatch:   "        Kind: \"pure\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantKind:    "pure",
		},
		{
			name:        "experiment Kind accepted",
			recordPatch: "    Kind: String\n    Dependencies: Dependency[]",
			bodyPatch:   "        Kind: \"experiment\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantKind:    "experiment",
		},
		{
			name:           "wrapper Kind requires Wrappers",
			recordPatch:    "    Kind: String\n    Dependencies: Dependency[]",
			bodyPatch:      "        Kind: \"wrapper\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantErr:        true,
			wantErrContent: "Wrappers",
		},
		{
			name:           "invalid Kind rejected",
			recordPatch:    "    Kind: String\n    Dependencies: Dependency[]",
			bodyPatch:      "        Kind: \"banana\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantErr:        true,
			wantErrContent: "Kind",
		},
		{
			name:        "EntryMilestone accepted for experiment",
			recordPatch: "    Kind: String\n    EntryMilestone: String\n    Dependencies: Dependency[]",
			bodyPatch:   "        Kind: \"experiment\"\n        EntryMilestone: \"M0\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantKind:    "experiment",
			wantEntry:   "M0",
		},
		{
			name:           "EntryMilestone rejected for default kind",
			recordPatch:    "    EntryMilestone: String\n    Dependencies: Dependency[]",
			bodyPatch:      "        EntryMilestone: \"M0\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantErr:        true,
			wantErrContent: "EntryMilestone",
		},
		{
			name:           "EntryMilestone rejected for pure",
			recordPatch:    "    Kind: String\n    EntryMilestone: String\n    Dependencies: Dependency[]",
			bodyPatch:      "        Kind: \"pure\"\n        EntryMilestone: \"M0\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantErr:        true,
			wantErrContent: "EntryMilestone",
		},
		{
			name:           "EntryMilestone rejected for wrapper",
			recordPatch:    "    Kind: String\n    EntryMilestone: String\n    Dependencies: Dependency[]",
			bodyPatch:      "        Kind: \"wrapper\"\n        EntryMilestone: \"M0\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantErr:        true,
			wantErrContent: "EntryMilestone",
		},
		{
			name:        "empty EntryMilestone accepted for default kind",
			recordPatch: "    EntryMilestone: String\n    Dependencies: Dependency[]",
			bodyPatch:   "        EntryMilestone: \"\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantKind:    "pure",
		},
		{
			name:        "empty EntryMilestone accepted for pure",
			recordPatch: "    Kind: String\n    EntryMilestone: String\n    Dependencies: Dependency[]",
			bodyPatch:   "        Kind: \"pure\"\n        EntryMilestone: \"\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantKind:    "pure",
		},
		{
			name:           "empty EntryMilestone for wrapper still requires Wrappers",
			recordPatch:    "    Kind: String\n    EntryMilestone: String\n    Dependencies: Dependency[]",
			bodyPatch:      "        Kind: \"wrapper\"\n        EntryMilestone: \"\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			wantErr:        true,
			wantErrContent: "Wrappers",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifestPath := writeManifest(t, validManifestWithEdits(tc.recordPatch, "}", tc.bodyPatch))
			metadata, err := loadManifestMetadata(manifestPath)
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrContent) {
					t.Fatalf("expected %s error, got %v", tc.wantErrContent, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("load manifest metadata: %v", err)
			}
			if metadata.Kind != tc.wantKind {
				t.Fatalf("expected kind %q, got %q", tc.wantKind, metadata.Kind)
			}
			if metadata.EntryMilestone != tc.wantEntry {
				t.Fatalf("expected entry milestone %q, got %q", tc.wantEntry, metadata.EntryMilestone)
			}
		})
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

func TestLoadManifestMetadataRejectsOptionalLiteralFieldsOmittedFromDeclarations(t *testing.T) {
	cases := []struct {
		name        string
		recordPatch string
		depPatch    string
		bodyPatch   string
		want        string
	}{
		{
			name:        "kind",
			recordPatch: "    Dependencies: Dependency[]",
			depPatch:    "}",
			bodyPatch:   "        Kind: \"experiment\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			want:        "Kind",
		},
		{
			name:        "entry milestone",
			recordPatch: "    Dependencies: Dependency[]",
			depPatch:    "}",
			bodyPatch:   "        EntryMilestone: \"M0\"\n        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" }]",
			want:        "EntryMilestone",
		},
		{
			name:        "source",
			recordPatch: "    Dependencies: Dependency[]",
			depPatch:    "}",
			bodyPatch:   "        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"1.0.0\" Source: \"builtin\" }]",
			want:        "Source",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifestPath := writeManifest(t, validManifestWithEdits(tc.recordPatch, tc.depPatch, tc.bodyPatch))
			_, err := loadManifestMetadata(manifestPath)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected undeclared optional field %s error, got %v", tc.want, err)
			}
		})
	}
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

func TestLoadManifestMetadataExtractsWrapperMetadata(t *testing.T) {
	metadata, err := loadManifestMetadata(writeManifest(t, validWrapperManifestSource("Xlsx")))
	if err != nil {
		t.Fatalf("load wrapper manifest: %v", err)
	}
	if metadata.Kind != "wrapper" {
		t.Fatalf("expected wrapper kind, got %q", metadata.Kind)
	}
	expected := []WrapperMetadata{{
		Name:           "xlsx",
		Family:         "Xlsx",
		Protocol:       "octxiliary.v0",
		SidecarCommand: "octxiliary-xlsx",
		GoModuleDir:    "octxiliary",
		Functions: []WrapperFunctionMetadata{
			{OctName: "ReadSheetNames", WireName: "XlsxReadSheetNames", Args: []string{"String"}, Return: "String[]", Fallible: true},
			{OctName: "WriteBytes", WireName: "XlsxWriteBytes", Args: []string{"String", "Bytes"}, Return: "Int", Fallible: false},
		},
	}}
	if !reflect.DeepEqual(metadata.Wrappers, expected) {
		t.Fatalf("unexpected wrappers:\ngot  %#v\nwant %#v", metadata.Wrappers, expected)
	}
}

func TestLoadManifestMetadataAllowsEmptyWrappersForNonWrapperKinds(t *testing.T) {
	cases := []struct {
		name  string
		kind  string
		entry string
	}{
		{name: "pure", kind: "pure"},
		{name: "experiment", kind: "experiment", entry: "M0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := nonWrapperManifestWithEmptyWrappers("DemoPkg", tc.kind, tc.entry)
			metadata, err := loadManifestMetadata(writeManifest(t, src))
			if err != nil {
				t.Fatalf("load manifest: %v", err)
			}
			if metadata.Kind != tc.kind {
				t.Fatalf("expected kind %q, got %q", tc.kind, metadata.Kind)
			}
			if len(metadata.Wrappers) != 0 {
				t.Fatalf("expected no wrapper metadata, got %#v", metadata.Wrappers)
			}
		})
	}
}

func TestLoadManifestMetadataRejectsMalformedWrapperMetadata(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{name: "wrapper omitted Wrappers", src: strings.Replace(validWrapperManifestSource("Xlsx"), "        Dependencies: []\n        Wrappers: [\n"+wrapperArrayLiteral()+"\n        ]", "        Dependencies: []", 1), want: "Wrappers"},
		{name: "wrapper empty Wrappers", src: strings.Replace(validWrapperManifestSource("Xlsx"), "[\n"+wrapperArrayLiteral()+"\n        ]", "[]", 1), want: "non-empty Wrappers"},
		{name: "pure non-empty Wrappers", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Kind: \"wrapper\"", "Kind: \"pure\"", 1), want: "Wrappers"},
		{name: "experiment non-empty Wrappers", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Kind: \"wrapper\"", "Kind: \"experiment\"", 1), want: "Wrappers"},
		{name: "wrapper non-empty EntryMilestone", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Kind: \"wrapper\"", "Kind: \"wrapper\"\n        EntryMilestone: \"M0\"", 1), want: "EntryMilestone"},
		{name: "literal Wrappers undeclared", src: strings.Replace(validWrapperManifestSource("Xlsx"), "    Wrappers: Wrapper[]\n", "", 1), want: "Wrappers"},
		{name: "missing Wrapper record", src: removeRecord(validWrapperManifestSource("Xlsx"), "Wrapper"), want: "Wrapper"},
		{name: "missing WrapperFunction record", src: removeRecord(validWrapperManifestSource("Xlsx"), "WrapperFunction"), want: "WrapperFunction"},
		{name: "empty wrapper Name", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Name: \"xlsx\"", "Name: \"\"", 1), want: "Name"},
		{name: "empty Family", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Family: \"Xlsx\"", "Family: \"\"", 1), want: "Family"},
		{name: "invalid Protocol", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Protocol: \"octxiliary.v0\"", "Protocol: \"other\"", 1), want: "Protocol"},
		{name: "empty SidecarCommand", src: strings.Replace(validWrapperManifestSource("Xlsx"), "SidecarCommand: \"octxiliary-xlsx\"", "SidecarCommand: \"\"", 1), want: "SidecarCommand"},
		{name: "empty GoModuleDir", src: strings.Replace(validWrapperManifestSource("Xlsx"), "GoModuleDir: \"octxiliary\"", "GoModuleDir: \"\"", 1), want: "GoModuleDir"},
		{name: "absolute GoModuleDir", src: strings.Replace(validWrapperManifestSource("Xlsx"), "GoModuleDir: \"octxiliary\"", "GoModuleDir: \"/tmp/octxiliary\"", 1), want: "relative"},
		{name: "traversal GoModuleDir", src: strings.Replace(validWrapperManifestSource("Xlsx"), "GoModuleDir: \"octxiliary\"", "GoModuleDir: \"../octxiliary\"", 1), want: "path traversal"},
		{name: "empty Functions", src: strings.Replace(validWrapperManifestSource("Xlsx"), "[\n"+functionArrayLiteral()+"\n]", "[]", 1), want: "Functions"},
		{name: "duplicate Wrapper Name", src: strings.Replace(validWrapperManifestSource("Xlsx"), wrapperArrayLiteral(), wrapperArrayLiteral()+",\n"+wrapperArrayLiteral(), 1), want: "duplicate Wrapper.Name"},
		{name: "duplicate Wrapper Family", src: duplicateWrapperFamilySource(), want: "duplicate Wrapper.Family"},
		{name: "duplicate Wrapper SidecarCommand", src: duplicateWrapperSidecarCommandSource(), want: "duplicate Wrapper.SidecarCommand"},
		{name: "empty OctName", src: strings.Replace(validWrapperManifestSource("Xlsx"), "OctName: \"ReadSheetNames\"", "OctName: \"\"", 1), want: "OctName"},
		{name: "empty WireName", src: strings.Replace(validWrapperManifestSource("Xlsx"), "WireName: \"XlsxReadSheetNames\"", "WireName: \"\"", 1), want: "WireName"},
		{name: "unsupported Arg type", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Args: [\"String\"]", "Args: [\"Record\"]", 1), want: "unsupported transport type"},
		{name: "unsupported Return type", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Return: \"String[]\"", "Return: \"Handle\"", 1), want: "unsupported transport type"},
		{name: "duplicate OctName", src: strings.Replace(validWrapperManifestSource("Xlsx"), "OctName: \"WriteBytes\"", "OctName: \"ReadSheetNames\"", 1), want: "duplicate OctName"},
		{name: "duplicate WireName", src: strings.Replace(validWrapperManifestSource("Xlsx"), "WireName: \"XlsxWriteBytes\"", "WireName: \"XlsxReadSheetNames\"", 1), want: "duplicate WireName"},
		{name: "Fallible not Bool", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Fallible: true", "Fallible: \"true\"", 1), want: "Fallible"},
		{name: "Args not array", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Args: [\"String\"]", "Args: \"String\"", 1), want: "Args"},
		{name: "Args non-string", src: strings.Replace(validWrapperManifestSource("Xlsx"), "Args: [\"String\"]", "Args: [3]", 1), want: "Args"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadManifestMetadata(writeManifest(t, tc.src))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}

func TestLoadManifestMetadataAllowsDimensionedIntTransport(t *testing.T) {
	src := strings.Replace(validWrapperManifestSource("Image"), "Return: \"String[]\"", "Return: \"Int<px>\"", 1)
	metadata, err := loadManifestMetadata(writeManifest(t, src))
	if err != nil {
		t.Fatalf("dimensioned Int transport manifest rejected: %v", err)
	}
	if got := metadata.Wrappers[0].Functions[0].Return; got != "Int<px>" {
		t.Fatalf("expected dimensioned Int return to be preserved, got %q", got)
	}
}

func validWrapperManifestSource(name string) string {
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
		"    Wrappers: Wrapper[]",
		"}",
		"",
		"record Dependency {",
		"    Name: String",
		"    VersionRequirement: String",
		"}",
		"",
		"record Wrapper {",
		"    Name: String",
		"    Family: String",
		"    Protocol: String",
		"    SidecarCommand: String",
		"    GoModuleDir: String",
		"    Functions: WrapperFunction[]",
		"}",
		"",
		"record WrapperFunction {",
		"    OctName: String",
		"    WireName: String",
		"    Args: String[]",
		"    Return: String",
		"    Fallible: Bool",
		"}",
		"",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"" + name + "\"",
		"        Version: \"0.1.0\"",
		"        Description: \"Excel workbook wrapper library\"",
		"        Kind: \"wrapper\"",
		"        Dependencies: []",
		"        Wrappers: [",
		wrapperArrayLiteral(),
		"        ]",
		"    }",
		"}",
	}, "\n") + "\n"
}

func wrapperArrayLiteral() string {
	return strings.Join([]string{
		"Wrapper {",
		"Name: \"xlsx\"",
		"Family: \"Xlsx\"",
		"Protocol: \"octxiliary.v0\"",
		"SidecarCommand: \"octxiliary-xlsx\"",
		"GoModuleDir: \"octxiliary\"",
		"Functions: [",
		functionArrayLiteral(),
		"]",
		"}",
	}, "\n")
}

func functionArrayLiteral() string {
	return strings.Join([]string{
		"WrapperFunction {",
		"OctName: \"ReadSheetNames\"",
		"WireName: \"XlsxReadSheetNames\"",
		"Args: [\"String\"]",
		"Return: \"String[]\"",
		"Fallible: true",
		"},",
		"WrapperFunction {",
		"OctName: \"WriteBytes\"",
		"WireName: \"XlsxWriteBytes\"",
		"Args: [\"String\", \"Bytes\"]",
		"Return: \"Int\"",
		"Fallible: false",
		"}",
	}, "\n")
}

func nonWrapperManifestWithEmptyWrappers(name string, kind string, entry string) string {
	entryLine := ""
	if entry != "" {
		entryLine = "        EntryMilestone: \"" + entry + "\"\n"
	}
	return strings.Replace(validWrapperManifestSource(name), "        Kind: \"wrapper\"\n        Dependencies: []\n        Wrappers: [\n"+wrapperArrayLiteral()+"\n        ]", "        Kind: \""+kind+"\"\n"+entryLine+"        Dependencies: []\n        Wrappers: []", 1)
}

func removeRecord(src string, name string) string {
	start := strings.Index(src, "record "+name+" {")
	if start < 0 {
		return src
	}
	end := strings.Index(src[start:], "}\n")
	if end < 0 {
		return src
	}
	return src[:start] + src[start+end+2:]
}

func duplicateWrapperFamilySource() string {
	second := strings.Replace(wrapperArrayLiteral(), "Name: \"xlsx\"", "Name: \"xlsx2\"", 1)
	return strings.Replace(validWrapperManifestSource("Xlsx"), wrapperArrayLiteral(), wrapperArrayLiteral()+",\n"+second, 1)
}

func duplicateWrapperSidecarCommandSource() string {
	second := wrapperArrayLiteral()
	second = strings.Replace(second, "Name: \"xlsx\"", "Name: \"xlsx2\"", 1)
	second = strings.Replace(second, "Family: \"Xlsx\"", "Family: \"Xlsx2\"", 1)
	return strings.Replace(validWrapperManifestSource("Xlsx"), wrapperArrayLiteral(), wrapperArrayLiteral()+",\n"+second, 1)
}

func TestLoadManifestMetadataHandleTransportTypes(t *testing.T) {
	valid := strings.Join([]string{
		"package Manifest",
		"record PackageManifest { Name: String Version: String Description: String Kind: String Dependencies: Dependency[] Wrappers: Wrapper[] }",
		"record Dependency { Name: String VersionRequirement: String }",
		"record Wrapper { Name: String Family: String Protocol: String SidecarCommand: String GoModuleDir: String TransportTypes: WrapperTransportType[] Functions: WrapperFunction[] }",
		"record WrapperTransportType { Name: String Kind: String Fields: WrapperTransportField[] }",
		"record WrapperTransportField { Name: String Type: String }",
		"record WrapperFunction { OctName: String WireName: String Args: String[] Return: String Fallible: Bool }",
		"fn Manifest() -> PackageManifest { return PackageManifest { Name: \"IO\" Version: \"0.1.0\" Description: \"io\" Kind: \"wrapper\" Dependencies: [] Wrappers: [Wrapper { Name: \"xlsx\" Family: \"Xlsx\" Protocol: \"octxiliary.v0\" SidecarCommand: \"octxiliary-xlsx\" GoModuleDir: \"octxiliary\" TransportTypes: [WrapperTransportType { Name: \"IO.Workbook\" Kind: \"handle\" Fields: [WrapperTransportField { Name: \"Handle\" Type: \"Int\" }] }] Functions: [WrapperFunction { OctName: \"CreateWorkbook\" WireName: \"XlsxCreateWorkbook\" Args: [] Return: \"IO.Workbook\" Fallible: false }, WrapperFunction { OctName: \"AddSheet\" WireName: \"XlsxAddSheet\" Args: [\"IO.Workbook\", \"String\"] Return: \"Int\" Fallible: true }] }] } }",
	}, "\n") + "\n"
	metadata, err := loadManifestMetadata(writeManifest(t, valid))
	if err != nil {
		t.Fatalf("valid handle transport manifest rejected: %v", err)
	}
	if got := metadata.Wrappers[0].TransportTypes[0]; got.Kind != "handle" || got.Name != "IO.Workbook" || len(got.Fields) != 1 || got.Fields[0].Name != "Handle" || got.Fields[0].Type != "Int" {
		t.Fatalf("unexpected handle metadata: %#v", got)
	}

	cases := []struct{ name, src, want string }{
		{"zero fields", strings.Replace(valid, "Fields: [WrapperTransportField { Name: \"Handle\" Type: \"Int\" }]", "Fields: []", 1), "Fields"},
		{"two fields", strings.Replace(valid, "WrapperTransportField { Name: \"Handle\" Type: \"Int\" }", "WrapperTransportField { Name: \"Handle\" Type: \"Int\" }, WrapperTransportField { Name: \"Other\" Type: \"Int\" }", 1), "exactly one"},
		{"wrong field name", strings.Replace(valid, "Name: \"Handle\" Type: \"Int\"", "Name: \"ID\" Type: \"Int\"", 1), "Handle"},
		{"wrong field type", strings.Replace(valid, "Name: \"Handle\" Type: \"Int\"", "Name: \"Handle\" Type: \"String\"", 1), "Int"},
		{"record return rejected", strings.Replace(strings.Replace(valid, "Kind: \"handle\"", "Kind: \"record\"", 1), "Return: \"IO.Workbook\"", "Return: \"IO.Workbook\"", 1), "record returns"},
		{"undeclared custom arg", strings.Replace(valid, "Args: [\"IO.Workbook\", \"String\"]", "Args: [\"Other.Workbook\", \"String\"]", 1), "unsupported transport type"},
		{"undeclared custom return", strings.Replace(valid, "Return: \"IO.Workbook\"", "Return: \"Other.Workbook\"", 1), "unsupported transport type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadManifestMetadata(writeManifest(t, tc.src))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}

func TestLoadManifestMetadataAcceptsAuthorsAndDate(t *testing.T) {
	manifestPath := writeManifest(t, strings.Join([]string{
		"package Manifest",
		"",
		"record PackageManifest {",
		"    Name: String",
		"    Version: String",
		"    Description: String",
		"    Authors: String[]",
		"    Date: String",
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
		"        Authors: [\"Codex\", \"Claude\"]",
		"        Date: \"2026-06-15\"",
		"        Dependencies: []",
		"    }",
		"}",
	}, "\n")+"\n")

	metadata, err := loadManifestMetadata(manifestPath)
	if err != nil {
		t.Fatalf("load manifest metadata: %v", err)
	}
	if !reflect.DeepEqual(metadata.Authors, []string{"Codex", "Claude"}) {
		t.Fatalf("unexpected authors: %#v", metadata.Authors)
	}
	if metadata.Date != "2026-06-15" {
		t.Fatalf("unexpected date: %q", metadata.Date)
	}
}

func TestLoadManifestMetadataRejectsInvalidAuthorsAndDate(t *testing.T) {
	cases := []struct {
		name    string
		record  string
		literal string
		want    string
	}{
		{name: "authors declaration", record: "    Authors: String", literal: "        Authors: [\"Codex\"]", want: "Authors"},
		{name: "authors literal", record: "    Authors: String[]", literal: "        Authors: \"Codex\"", want: "Authors"},
		{name: "date declaration", record: "    Date: Int", literal: "        Date: \"2026-06-15\"", want: "Date"},
		{name: "date literal", record: "    Date: String", literal: "        Date: 20260615", want: "Date"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifestPath := writeManifest(t, strings.Join([]string{
				"package Manifest",
				"",
				"record PackageManifest {",
				"    Name: String",
				"    Version: String",
				"    Description: String",
				tc.record,
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
				tc.literal,
				"        Dependencies: []",
				"    }",
				"}",
			}, "\n")+"\n")
			_, err := loadManifestMetadata(manifestPath)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}
