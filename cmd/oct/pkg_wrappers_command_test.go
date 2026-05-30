package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPkgWrappersPurePackageNoWrappers(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, projectManifestWithDeps(nil))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers")
	if err != nil {
		t.Fatalf("expected pkg wrappers success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout,
		"Wrapper build plan:",
		"native wrappers: no",
		"sidecars: 0",
		"No wrapper sidecars were built or executed.",
	)
}

func TestPkgWrappersWrapperPackageOneSidecar(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Xlsx", pkgWrappersLiteral("xlsx", "Xlsx", "octxiliary-xlsx", "octxiliary")))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers")
	if err != nil {
		t.Fatalf("expected pkg wrappers success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout,
		"native wrappers: yes",
		"requires native build permission: yes",
		"sidecars: 1",
		"* package Xlsx 0.1.0",
		"wrapper: xlsx",
		"family: Xlsx",
		"command: octxiliary-xlsx",
		"protocol: octxiliary.v0",
		"module: octxiliary",
		"module path: "+filepath.Join(projectDir, "octxiliary"),
		"functions: 2",
		"No wrapper sidecars were built or executed.",
	)
}

func TestPkgWrappersRegistryOutput(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Xlsx", pkgWrappersLiteral("xlsx", "Xlsx", "octxiliary-xlsx", "octxiliary")))
	registryPath := filepath.Join(t.TempDir(), "nested", "registry.octagon")

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers", "--registry-out", registryPath)
	if err != nil {
		t.Fatalf("expected pkg wrappers registry success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout,
		"Wrote Octxiliary registry: "+registryPath,
		"No wrapper sidecars were built or executed.",
	)
	body, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("expected registry file: %v", err)
	}
	registry := string(body)
	assertOutputContains(t, registry,
		"OctxiliaryRegistry",
		"Version: \"octxiliary.registry.v0\"",
		"PackageName: \"Xlsx\"",
		"SidecarCommand: \"octxiliary-xlsx\"",
	)
}

func TestPkgWrappersInvalidRegistryPath(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, projectManifestWithDeps(nil))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers", "--registry-out", "registry.txt")
	if err == nil {
		t.Fatalf("expected invalid registry path failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, ".octagon") {
		t.Fatalf("expected .octagon error, got %q", stderr)
	}
}

func TestPkgWrappersInvalidUsage(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, projectManifestWithDeps(nil))
	cases := [][]string{
		{"pkg", "wrappers", "extra"},
		{"pkg", "wrappers", "--registry-out"},
		{"pkg", "wrappers", "--registry-out", "registry.octagon", "extra"},
		{"pkg", "wrappers", "--unknown"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, err := executeCLIInDir(projectDir, args...)
			if err == nil {
				t.Fatalf("expected invalid usage failure, stdout=%q stderr=%q", stdout, stderr)
			}
			if !strings.Contains(stderr, "usage: oct pkg wrappers [--registry-out <path>]") {
				t.Fatalf("expected usage error, got %q", stderr)
			}
		})
	}
}

func TestPkgWrappersConflictFailsClearly(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	first := pkgWrappersLiteral("xlsx", "Xlsx", "octxiliary-shared", "wrappers/xlsx")
	second := pkgWrappersLiteral("csv", "Csv", "octxiliary-shared", "wrappers/csv")
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Adapters", first+",\n"+second))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers")
	if err == nil {
		t.Fatalf("expected conflict failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "duplicate sidecar command") {
		t.Fatalf("expected duplicate sidecar command error, got %q", stderr)
	}
}

func assertOutputContains(t *testing.T, output string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got:\n%s", snippet, output)
		}
	}
}

func pkgWrappersManifestSource(name string, wrappers string) string {
	return strings.Join([]string{
		"package Manifest",
		"",
		"record PackageManifest {",
		"    Name: String",
		"    Version: String",
		"    Description: String",
		"    Kind: String",
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
		"        Description: \"wrapper package\"",
		"        Kind: \"wrapper\"",
		"        Dependencies: []",
		"        Wrappers: [",
		wrappers,
		"        ]",
		"    }",
		"}",
	}, "\n") + "\n"
}

func pkgWrappersLiteral(name string, family string, command string, moduleDir string) string {
	return strings.Join([]string{
		"Wrapper {",
		"Name: \"" + name + "\"",
		"Family: \"" + family + "\"",
		"Protocol: \"octxiliary.v0\"",
		"SidecarCommand: \"" + command + "\"",
		"GoModuleDir: \"" + moduleDir + "\"",
		"Functions: [",
		"WrapperFunction {",
		"OctName: \"ReadSheetNames\"",
		"WireName: \"" + family + "ReadSheetNames\"",
		"Args: [\"String\"]",
		"Return: \"String[]\"",
		"Fallible: true",
		"},",
		"WrapperFunction {",
		"OctName: \"WriteBytes\"",
		"WireName: \"" + family + "WriteBytes\"",
		"Args: [\"String\", \"Bytes\"]",
		"Return: \"Int\"",
		"Fallible: false",
		"}",
		"]",
		"}",
	}, "\n")
}
