//go:build toolchain

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
	if !strings.Contains(stderr, "duplicate Wrapper.SidecarCommand") {
		t.Fatalf("expected duplicate Wrapper.SidecarCommand error, got %q", stderr)
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

func TestPkgWrappersCompressionPackage(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Compression", pkgWrappersCompressionLiteral()))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers")
	if err != nil {
		t.Fatalf("expected pkg wrappers success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout,
		"native wrappers: yes",
		"sidecars: 1",
		"* package Compression 0.1.0",
		"wrapper: compression",
		"family: Compression",
		"command: octxiliary-compression",
		"protocol: octxiliary.v0",
		"functions: 4",
		"No wrapper sidecars were built or executed.",
	)
}

func TestPkgWrappersCompressionRegistryOutput(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Compression", pkgWrappersCompressionLiteral()))
	registryPath := filepath.Join(t.TempDir(), "compression-registry.octagon")

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
		"Family: \"Compression\"",
		"SidecarCommand: \"octxiliary-compression\"",
	)
}

func pkgWrappersCompressionLiteral() string {
	return strings.Join([]string{
		"Wrapper {",
		"Name: \"compression\"",
		"Family: \"Compression\"",
		"Protocol: \"octxiliary.v0\"",
		"SidecarCommand: \"octxiliary-compression\"",
		"GoModuleDir: \"octxiliary\"",
		"Functions: [",
		"WrapperFunction { OctName: \"CompressBytes\" WireName: \"GzipCompressBytes\" Args: [\"Bytes\"] Return: \"Bytes\" Fallible: true },",
		"WrapperFunction { OctName: \"DecompressBytes\" WireName: \"GzipDecompressBytes\" Args: [\"Bytes\"] Return: \"Bytes\" Fallible: true },",
		"WrapperFunction { OctName: \"CompressFile\" WireName: \"GzipCompressFile\" Args: [\"String\", \"String\"] Return: \"Int\" Fallible: true },",
		"WrapperFunction { OctName: \"DecompressFile\" WireName: \"GzipDecompressFile\" Args: [\"String\", \"String\"] Return: \"Int\" Fallible: true }",
		"]",
		"}",
	}, "\n")
}

func TestPkgWrappersTimePackage(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Time", pkgWrappersTimeLiteral()))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers")
	if err != nil {
		t.Fatalf("expected pkg wrappers success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout,
		"native wrappers: yes",
		"sidecars: 1",
		"* package Time 0.1.0",
		"wrapper: time",
		"family: Time",
		"command: octxiliary-time",
		"protocol: octxiliary.v0",
		"functions: 5",
		"No wrapper sidecars were built or executed.",
	)
}

func TestPkgWrappersTimeRegistryOutput(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Time", pkgWrappersTimeLiteral()))
	registryPath := filepath.Join(t.TempDir(), "time-registry.octagon")

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
		"Family: \"Time\"",
		"SidecarCommand: \"octxiliary-time\"",
	)
}

func pkgWrappersTimeLiteral() string {
	return strings.Join([]string{
		"Wrapper {",
		"Name: \"time\"",
		"Family: \"Time\"",
		"Protocol: \"octxiliary.v0\"",
		"SidecarCommand: \"octxiliary-time\"",
		"GoModuleDir: \"octxiliary\"",
		"Functions: [",
		"WrapperFunction { OctName: \"NowIso8601\" WireName: \"TimeNowIso8601\" Args: [] Return: \"String\" Fallible: false },",
		"WrapperFunction { OctName: \"ParseIso8601\" WireName: \"TimeParseIso8601\" Args: [\"String\"] Return: \"String\" Fallible: true },",
		"WrapperFunction { OctName: \"FormatIso8601\" WireName: \"TimeFormatIso8601\" Args: [\"String\"] Return: \"String\" Fallible: true },",
		"WrapperFunction { OctName: \"UnixSecondsNow\" WireName: \"TimeUnixSecondsNow\" Args: [] Return: \"Int\" Fallible: false },",
		"WrapperFunction { OctName: \"FormatUnixSeconds\" WireName: \"TimeFormatUnixSecond\" Args: [\"Int\"] Return: \"String\" Fallible: true }",
		"]",
		"}",
	}, "\n")
}

func TestPkgWrappersTextPackage(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Text", pkgWrappersTextLiteral()))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers")
	if err != nil {
		t.Fatalf("expected pkg wrappers success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout,
		"native wrappers: yes",
		"sidecars: 1",
		"* package Text 0.1.0",
		"wrapper: text",
		"family: Text",
		"command: octxiliary-text",
		"protocol: octxiliary.v0",
		"functions: 4",
		"No wrapper sidecars were built or executed.",
	)
}

func TestPkgWrappersTextRegistryOutput(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Text", pkgWrappersTextLiteral()))
	registryPath := filepath.Join(t.TempDir(), "text-registry.octagon")

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
		"Family: \"Text\"",
		"SidecarCommand: \"octxiliary-text\"",
	)
}

func pkgWrappersTextLiteral() string {
	return strings.Join([]string{
		"Wrapper {",
		"Name: \"text\"",
		"Family: \"Text\"",
		"Protocol: \"octxiliary.v0\"",
		"SidecarCommand: \"octxiliary-text\"",
		"GoModuleDir: \"octxiliary\"",
		"Functions: [",
		"WrapperFunction { OctName: \"IsMatch\" WireName: \"RegexIsMatch\" Args: [\"String\", \"String\"] Return: \"Bool\" Fallible: true },",
		"WrapperFunction { OctName: \"FindAll\" WireName: \"RegexFindAll\" Args: [\"String\", \"String\"] Return: \"String[]\" Fallible: true },",
		"WrapperFunction { OctName: \"ReplaceAll\" WireName: \"RegexReplaceAll\" Args: [\"String\", \"String\", \"String\"] Return: \"String\" Fallible: true },",
		"WrapperFunction { OctName: \"Split\" WireName: \"RegexSplit\" Args: [\"String\", \"String\"] Return: \"String[]\" Fallible: true }",
		"]",
		"}",
	}, "\n")
}

func TestPkgWrappersCsvPackage(t *testing.T) {
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Csv", pkgWrappersCsvLiteral()))
	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers")
	if err != nil {
		t.Fatalf("expected pkg wrappers success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout,
		"native wrappers: yes",
		"wrapper: csv",
		"family: Csv",
		"command: octxiliary-csv",
		"functions: 2",
	)
}

func TestPkgWrappersCsvRegistryOutput(t *testing.T) {
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Csv", pkgWrappersCsvLiteral()))
	registryPath := filepath.Join(t.TempDir(), "csv-registry.octagon")
	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers", "--registry-out", registryPath)
	if err != nil {
		t.Fatalf("expected pkg wrappers registry success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, "Wrote Octxiliary registry: "+registryPath)
	body, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("expected registry file: %v", err)
	}
	registry := string(body)
	assertOutputContains(t, registry,
		"Family: \"Csv\"",
		"SidecarCommand: \"octxiliary-csv\"",
		"Return: \"String[][]\"",
	)
}

func pkgWrappersCsvLiteral() string {
	return strings.Join([]string{
		"Wrapper {",
		"Name: \"csv\"",
		"Family: \"Csv\"",
		"Protocol: \"octxiliary.v0\"",
		"SidecarCommand: \"octxiliary-csv\"",
		"GoModuleDir: \"octxiliary\"",
		"Functions: [",
		"WrapperFunction { OctName: \"Read\" WireName: \"CsvRead\" Args: [\"String\"] Return: \"String[][]\" Fallible: true },",
		"WrapperFunction { OctName: \"Write\" WireName: \"CsvWrite\" Args: [\"String\", \"String[][]\"] Return: \"Int\" Fallible: true }",
		"]",
		"}",
	}, "\n")
}
