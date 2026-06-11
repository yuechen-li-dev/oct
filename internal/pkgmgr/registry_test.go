package pkgmgr

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRegistryConfigAddListRemove(t *testing.T) {
	root := t.TempDir()
	if _, err := AddRegistry(root, "local", "../oct-registry"); err != nil {
		t.Fatalf("add registry: %v", err)
	}
	if _, err := AddRegistry(root, "local", "../other"); err == nil {
		t.Fatalf("expected duplicate registry error")
	}
	config, err := LoadRegistryConfig(root)
	if err != nil {
		t.Fatalf("load registry config: %v", err)
	}
	if len(config.Registries) != 1 || config.Registries[0].Name != "local" || config.Registries[0].Path != "../oct-registry" {
		t.Fatalf("unexpected config: %#v", config)
	}
	if _, err := RemoveRegistry(root, "local"); err != nil {
		t.Fatalf("remove registry: %v", err)
	}
	config, err = LoadRegistryConfig(root)
	if err != nil {
		t.Fatalf("reload registry config: %v", err)
	}
	if len(config.Registries) != 0 {
		t.Fatalf("expected empty registries, got %#v", config)
	}
}

func TestLoadRegistryIndexParsesValidRegistry(t *testing.T) {
	root := writeRegistryIndex(t, validRegistryIndex("SignalTools", "0.1.0", "library", "local", "../SignalTools", "."))
	idx, err := LoadRegistryIndex(root)
	if err != nil {
		t.Fatalf("load registry index: %v", err)
	}
	if len(idx.Packages) != 1 || idx.Packages[0].Name != "SignalTools" {
		t.Fatalf("unexpected registry index: %#v", idx)
	}
}

func TestRegistryIndexEscapesWindowsPathLiterals(t *testing.T) {
	windowsPath := `C:\Users\RUNNER~1\AppData\Local\Temp\pkg`
	root := writeRegistryIndex(t, validRegistryIndex("SignalTools", "0.1.0", "library", "local", windowsPath, "."))
	idx, err := LoadRegistryIndex(root)
	if err != nil {
		t.Fatalf("load registry index with Windows path: %v", err)
	}
	if got, want := idx.Packages[0].Source, "C:/Users/RUNNER~1/AppData/Local/Temp/pkg"; got != want {
		t.Fatalf("Source = %q, want %q", got, want)
	}
}

func TestLoadRegistryIndexMissing(t *testing.T) {
	_, err := LoadRegistryIndex(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "registry index not found") {
		t.Fatalf("expected missing registry error, got %v", err)
	}
}

func TestLoadRegistryIndexValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"invalid package name", validRegistryIndex("signal-tools", "0.1.0", "library", "local", "../SignalTools", "."), "invalid package name"},
		{"duplicate", registryIndexWithEntries([]string{packageEntry("SignalTools", "0.1.0", "library", "local", "../SignalTools", "."), packageEntry("SignalTools", "0.1.0", "library", "local", "../SignalTools", ".")}), "duplicate package entry"},
		{"source kind", validRegistryIndex("SignalTools", "0.1.0", "library", "git", "../SignalTools", "."), "SourceKind must be local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeRegistryIndex(t, tc.body)
			_, err := LoadRegistryIndex(root)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}

func TestResolveRegistryPackage(t *testing.T) {
	project := t.TempDir()
	reg1 := writeRegistryIndex(t, validRegistryIndex("SignalTools", "0.1.0", "library", "local", "../SignalTools", "."))
	reg2 := writeRegistryIndex(t, validRegistryIndex("SignalTools", "0.1.0", "library", "local", "../SignalTools", "."))
	if _, err := AddRegistry(project, "local", reg1); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveRegistryPackage(project, "SignalTools", "0.1.0", "")
	if err != nil || resolved.Registry.Name != "local" {
		t.Fatalf("resolve: %#v %v", resolved, err)
	}
	if _, err := AddRegistry(project, "staging", reg2); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveRegistryPackage(project, "SignalTools", "0.1.0", ""); err == nil || !strings.Contains(err.Error(), "multiple registries") {
		t.Fatalf("expected ambiguity, got %v", err)
	}
	resolved, err = ResolveRegistryPackage(project, "SignalTools", "0.1.0", "staging")
	if err != nil || resolved.Registry.Name != "staging" {
		t.Fatalf("narrow resolve: %#v %v", resolved, err)
	}
	if _, err := ResolveRegistryPackage(project, "SignalTools", "9.9.9", "staging"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing, got %v", err)
	}
}

func TestSyncRegistryDependencyCopiesAndWritesMetadata(t *testing.T) {
	project := t.TempDir()
	pkg := writePackageSource(t, "SignalTools", "0.1.0", "")
	if err := os.MkdirAll(filepath.Join(pkg, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pkg, ".oct", "wrappers"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pkg, ".oct", "packages"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := writeRegistryIndex(t, validRegistryIndex("SignalTools", "0.1.0", "library", "local", pkg, "."))
	if _, err := AddRegistry(project, "local", reg); err != nil {
		t.Fatal(err)
	}
	result, err := SyncRegistryDependency(project, DependencyMetadata{Name: "SignalTools", VersionRequirement: "0.1.0"})
	if err != nil {
		t.Fatalf("sync registry dependency: %v", err)
	}
	if result.Destination != filepath.Join(project, ".oct", "packages", "SignalTools", "0.1.0") {
		t.Fatalf("unexpected destination %s", result.Destination)
	}
	if _, err := os.Stat(filepath.Join(result.Destination, "manifest.oct")); err != nil {
		t.Fatalf("missing manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(result.Destination, PackageSourceFileName)); err != nil {
		t.Fatalf("missing metadata: %v", err)
	}
	if _, err := os.Stat(filepath.Join(result.Destination, ".git")); !os.IsNotExist(err) {
		t.Fatalf("expected .git skipped, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(result.Destination, ".oct", "wrappers")); !os.IsNotExist(err) {
		t.Fatalf("expected .oct/wrappers skipped, err=%v", err)
	}
}

func TestSafePackageSourcePathRejectsTraversal(t *testing.T) {
	if _, err := safePackageSourcePath(t.TempDir(), "../escape"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
}

func TestValidateCopiedManifestMismatch(t *testing.T) {
	root := writePackageSource(t, "OtherTools", "0.1.0", "")
	err := validateCopiedManifest(root, PackageEntry{Name: "SignalTools", Version: "0.1.0", Kind: "library"})
	if err == nil || !strings.Contains(err.Error(), "copied manifest mismatch") {
		t.Fatalf("expected mismatch, got %v", err)
	}
}

func writeRegistryIndex(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, RegistryIndexFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func validRegistryIndex(name, version, kind, sourceKind, source, path string) string {
	return registryIndexWithEntries([]string{packageEntry(name, version, kind, sourceKind, source, path)})
}

func registryIndexWithEntries(entries []string) string {
	return "package Registry\n\nrecord RegistryIndex {\n    Packages: PackageEntry[]\n}\n\nrecord PackageEntry {\n    Name: String\n    Version: String\n    Kind: String\n    SourceKind: String\n    Source: String\n    Path: String\n    Description: String\n}\n\nfn Registry() -> RegistryIndex {\n    return RegistryIndex {\n        Packages: [\n            " + strings.Join(entries, ",\n            ") + "\n        ]\n    }\n}\n"
}

func packageEntry(name, version, kind, sourceKind, source, path string) string {
	return `PackageEntry { Name: "` + name + `" Version: "` + version + `" Kind: "` + kind + `" SourceKind: "` + sourceKind + `" Source: ` + octStringLiteralPath(source) + ` Path: ` + octStringLiteralPath(path) + ` Description: "test" }`
}

func octStringLiteralPath(path string) string {
	return strconv.Quote(strings.ReplaceAll(filepath.ToSlash(path), `\`, "/"))
}

func writePackageSource(t *testing.T, name, version, kind string) string {
	t.Helper()
	root := t.TempDir()
	fields := ""
	if kind != "" {
		fields = "        Kind: \"" + kind + "\"\n"
	}
	manifest := "package Manifest\n\nrecord PackageManifest {\n    Name: String\n    Version: String\n    Description: String\n" + func() string {
		if kind != "" {
			return "    Kind: String\n"
		}
		return ""
	}() + "    Dependencies: Dependency[]\n}\n\nrecord Dependency {\n    Name: String\n    VersionRequirement: String\n}\n\nfn Manifest() -> PackageManifest {\n    return PackageManifest {\n        Name: \"" + name + "\"\n        Version: \"" + version + "\"\n        Description: \"test\"\n" + fields + "        Dependencies: []\n    }\n}\n"
	if err := os.WriteFile(filepath.Join(root, "manifest.oct"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name+".oct"), []byte("package "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
