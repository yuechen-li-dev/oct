package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/newpkg"
)

func TestPkgRegistryGoldenPathAddSyncAndTest(t *testing.T) {
	workspace := t.TempDir()
	signalDir := filepath.Join(workspace, "SignalTools")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "SignalTools", Dir: signalDir}); err != nil {
		t.Fatalf("scaffold SignalTools: %v", err)
	}
	registryDir := filepath.Join(workspace, "registry")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(registryDir, "registry.oct"), []byte(validRegistryForCLI(signalDir)), 0o644); err != nil {
		t.Fatal(err)
	}
	consumerDir := filepath.Join(workspace, "Consumer")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "Consumer", Dir: consumerDir}); err != nil {
		t.Fatalf("scaffold Consumer: %v", err)
	}
	consumerTest := `package Consumer

import SignalTools

[Fact]
fn UsesRegistrySyncedDependency() -> Void {
    Assert.Equal(5, SignalTools.Identity(5), "registry dependency should load")
}
`
	if err := os.WriteFile(filepath.Join(consumerDir, "Consumer.Registry.octest"), []byte(consumerTest), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := executeCLIInDir(consumerDir, "pkg", "registry", "add", "local", registryDir)
	if err != nil || !strings.Contains(stdout, "Added package registry local") {
		t.Fatalf("registry add failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "pkg", "add", "SignalTools@0.1.0")
	if err != nil || !strings.Contains(stdout, "Added dependency SignalTools 0.1.0 from registry local") {
		t.Fatalf("pkg add failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	manifest, err := os.ReadFile(filepath.Join(consumerDir, "manifest.oct"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `Dependency { Name: "SignalTools" VersionRequirement: "0.1.0" }`) && !strings.Contains(string(manifest), `Dependency { Name: "SignalTools" VersionRequirement: "0.1.0" }`) {
		t.Fatalf("expected dependency without Source, got %s", manifest)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "pkg", "sync")
	if err != nil || !strings.Contains(stdout, "Synced SignalTools 0.1.0 to .oct/packages/SignalTools/0.1.0") {
		t.Fatalf("pkg sync failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(consumerDir, ".oct", "packages", "SignalTools", "0.1.0", "manifest.oct")); err != nil {
		t.Fatalf("missing synced manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(consumerDir, ".oct", "packages", "SignalTools", "0.1.0", ".oct-package-source.oct")); err != nil {
		t.Fatalf("missing metadata: %v", err)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "test", ".")
	if err != nil || !strings.Contains(stdout, "PASS Consumer.UsesRegistrySyncedDependency") {
		t.Fatalf("oct test failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
}

func TestPkgRegistryListRemove(t *testing.T) {
	project := t.TempDir()
	stdout, stderr, err := executeCLIInDir(project, "pkg", "registry", "list")
	if err != nil || !strings.Contains(stdout, "No package registries configured") {
		t.Fatalf("list empty err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if _, stderr, err = executeCLIInDir(project, "pkg", "registry", "add", "local", "../registry"); err != nil {
		t.Fatalf("add registry: %v %s", err, stderr)
	}
	stdout, stderr, err = executeCLIInDir(project, "pkg", "registry", "list")
	if err != nil || !strings.Contains(stdout, "* local ../registry") {
		t.Fatalf("list err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	stdout, stderr, err = executeCLIInDir(project, "pkg", "registry", "remove", "local")
	if err != nil || !strings.Contains(stdout, "Removed package registry local") {
		t.Fatalf("remove err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
}

func TestPkgAddRejectsMissingAndRangeVersions(t *testing.T) {
	project := t.TempDir()
	for _, spec := range []string{"SignalTools", "SignalTools@latest", "SignalTools@^0.1.0", "SignalTools@0.1.*"} {
		stdout, stderr, err := executeCLIInDir(project, "pkg", "add", spec)
		if err == nil {
			t.Fatalf("expected add failure for %s stdout=%q stderr=%q", spec, stdout, stderr)
		}
	}
}

func validRegistryForCLI(signalDir string) string {
	return `package Registry

record RegistryIndex { Packages: PackageEntry[] }
record PackageEntry { Name: String Version: String Kind: String SourceKind: String Source: String Path: String Description: String }
fn Registry() -> RegistryIndex {
    return RegistryIndex { Packages: [PackageEntry { Name: "SignalTools" Version: "0.1.0" Kind: "library" SourceKind: "local" Source: "` + signalDir + `" Path: "." Description: "Signal helper functions" }] }
}
`
}
