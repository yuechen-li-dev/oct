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

func TestLoadRejectsOptionalLiteralFieldsOmittedFromDeclarations(t *testing.T) {
	cases := []struct {
		name        string
		declaration string
		want        string
	}{
		{name: "Kind", declaration: "    Kind: String\n", want: "Kind"},
		{name: "EntryMilestone", declaration: "    EntryMilestone: String\n", want: "EntryMilestone"},
		{name: "Source", declaration: "    Source: String\n", want: "Source"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := strings.Replace(optionalManifestSource("Main"), tc.declaration, "", 1)
			writeProjectPackage(t, root, "Main", manifest, "package Main\nfn Main() -> Int { return 0 }\n")
			_, err := Load(root)
			if err == nil {
				t.Fatalf("expected undeclared optional field %s to be rejected", tc.want)
			}
		})
	}
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

func TestLoadValidatesPackageKindSemantics(t *testing.T) {
	cases := []struct {
		name                  string
		kindLiteral           string
		includeKind           bool
		entryLiteral          string
		includeEntryMilestone bool
		wantErr               bool
		wantErrContent        string
	}{
		{name: "missing Kind accepted as default pure"},
		{name: "empty Kind accepted as default pure", includeKind: true},
		{name: "pure Kind accepted", kindLiteral: "pure", includeKind: true},
		{name: "experiment Kind accepted", kindLiteral: "experiment", includeKind: true},
		{name: "wrapper Kind requires Wrappers", kindLiteral: "wrapper", includeKind: true, wantErr: true, wantErrContent: "Wrappers"},
		{name: "invalid Kind rejected", kindLiteral: "banana", includeKind: true, wantErr: true, wantErrContent: "Kind"},
		{name: "EntryMilestone rejected for default kind", entryLiteral: "M0", includeEntryMilestone: true, wantErr: true, wantErrContent: "EntryMilestone"},
		{name: "EntryMilestone rejected for pure", kindLiteral: "pure", includeKind: true, entryLiteral: "M0", includeEntryMilestone: true, wantErr: true, wantErrContent: "EntryMilestone"},
		{name: "EntryMilestone rejected for wrapper", kindLiteral: "wrapper", includeKind: true, entryLiteral: "M0", includeEntryMilestone: true, wantErr: true, wantErrContent: "EntryMilestone"},
		{name: "EntryMilestone accepted for experiment", kindLiteral: "experiment", includeKind: true, entryLiteral: "M0", includeEntryMilestone: true},
		{name: "empty EntryMilestone accepted for pure", kindLiteral: "pure", includeKind: true, includeEntryMilestone: true},
		{name: "empty EntryMilestone for wrapper still requires Wrappers", kindLiteral: "wrapper", includeKind: true, includeEntryMilestone: true, wantErr: true, wantErrContent: "Wrappers"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := packageKindManifestSource("Main", tc.kindLiteral, tc.entryLiteral, tc.includeKind, tc.includeEntryMilestone)
			writeProjectPackage(t, root, "Main", manifest, "package Main\nfn Main() -> Int { return 0 }\n")
			_, err := Load(root)
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrContent) {
					t.Fatalf("expected %s error, got %v", tc.wantErrContent, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load should accept manifest: %v", err)
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

func packageKindManifestSource(name string, kind string, entryMilestone string, includeKind bool, includeEntryMilestone bool) string {
	kindLine := ""
	if includeKind {
		kindLine = "        Kind: \"" + kind + "\"\n"
	}
	entryLine := ""
	if includeEntryMilestone {
		entryLine = "        EntryMilestone: \"" + entryMilestone + "\"\n"
	}
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
		"}",
		"",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"" + name + "\"",
		"        Version: \"0.1.0\"",
		"        Description: \"package kind semantics\"",
		kindLine + entryLine + "        Dependencies: []",
		"    }",
		"}",
	}, "\n") + "\n"
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

func TestLoadValidatesWrapperManifestMetadata(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{name: "valid wrapper manifest", src: projectWrapperManifestSource("Main")},
		{name: "wrapper omitted Wrappers", src: strings.Replace(projectWrapperManifestSource("Main"), "        Dependencies: []\n        Wrappers: [\n"+projectWrapperLiteral()+"\n        ]", "        Dependencies: []", 1), want: "Wrappers"},
		{name: "pure non-empty Wrappers", src: strings.Replace(projectWrapperManifestSource("Main"), "Kind: \"wrapper\"", "Kind: \"pure\"", 1), want: "Wrappers"},
		{name: "experiment non-empty Wrappers", src: strings.Replace(projectWrapperManifestSource("Main"), "Kind: \"wrapper\"", "Kind: \"experiment\"", 1), want: "Wrappers"},
		{name: "invalid Protocol", src: strings.Replace(projectWrapperManifestSource("Main"), "Protocol: \"octxiliary.v0\"", "Protocol: \"other\"", 1), want: "Protocol"},
		{name: "invalid transport", src: strings.Replace(projectWrapperManifestSource("Main"), "Return: \"String[]\"", "Return: \"Handle\"", 1), want: "transport"},
		{name: "invalid GoModuleDir", src: strings.Replace(projectWrapperManifestSource("Main"), "GoModuleDir: \"octxiliary\"", "GoModuleDir: \"../octxiliary\"", 1), want: "path traversal"},
		{name: "missing Wrapper record", src: projectRemoveRecord(projectWrapperManifestSource("Main"), "Wrapper"), want: "Wrapper"},
		{name: "missing WrapperFunction record", src: projectRemoveRecord(projectWrapperManifestSource("Main"), "WrapperFunction"), want: "WrapperFunction"},
		{name: "declaration-aware Wrappers", src: strings.Replace(projectWrapperManifestSource("Main"), "    Wrappers: Wrapper[]\n", "", 1), want: "invalid package metadata"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeProjectPackage(t, root, "Main", tc.src, "package Main\nfn Main() -> Int { return 0 }\n")
			_, err := Load(root)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Load should accept wrapper manifest: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}

func projectWrapperManifestSource(name string) string {
	return strings.Join([]string{
		"package Manifest",
		"record PackageManifest {",
		"    Name: String",
		"    Version: String",
		"    Description: String",
		"    Kind: String",
		"    EntryMilestone: String",
		"    Dependencies: Dependency[]",
		"    Wrappers: Wrapper[]",
		"}",
		"record Dependency {",
		"    Name: String",
		"    VersionRequirement: String",
		"}",
		"record Wrapper {",
		"    Name: String",
		"    Family: String",
		"    Protocol: String",
		"    SidecarCommand: String",
		"    GoModuleDir: String",
		"    Functions: WrapperFunction[]",
		"}",
		"record WrapperFunction {",
		"    OctName: String",
		"    WireName: String",
		"    Args: String[]",
		"    Return: String",
		"    Fallible: Bool",
		"}",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"" + name + "\"",
		"        Version: \"0.1.0\"",
		"        Description: \"wrapper package\"",
		"        Kind: \"wrapper\"",
		"        Dependencies: []",
		"        Wrappers: [",
		projectWrapperLiteral(),
		"        ]",
		"    }",
		"}",
	}, "\n") + "\n"
}

func projectWrapperLiteral() string {
	return strings.Join([]string{
		"Wrapper {",
		"Name: \"xlsx\"",
		"Family: \"Xlsx\"",
		"Protocol: \"octxiliary.v0\"",
		"SidecarCommand: \"octxiliary-xlsx\"",
		"GoModuleDir: \"octxiliary\"",
		"Functions: [WrapperFunction { OctName: \"ReadSheetNames\" WireName: \"XlsxReadSheetNames\" Args: [\"String\"] Return: \"String[]\" Fallible: true }]",
		"}",
	}, "\n")
}

func projectRemoveRecord(src string, name string) string {
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
