package main

import (
	"os"
	"os/exec"
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
	if !strings.Contains(string(manifest), `Dependency { Name: "SignalTools" VersionRequirement: "0.1.0" }`) {
		t.Fatalf("expected dependency without Source, got %s", manifest)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "pkg", "sync")
	expectedSyncPath := filepath.Join(".oct", "packages", "SignalTools", "0.1.0")
	if err != nil || !strings.Contains(stdout, "Synced SignalTools 0.1.0 to "+expectedSyncPath) {
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

func TestPkgRegistryGitSourceGoldenPath(t *testing.T) {
	requireGit(t)
	workspace := t.TempDir()
	signalDir := filepath.Join(workspace, "SignalTools")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "SignalTools", Dir: signalDir}); err != nil {
		t.Fatalf("scaffold SignalTools: %v", err)
	}
	runGitCLIRegistryTestCommand(t, signalDir, "init")
	runGitCLIRegistryTestCommand(t, signalDir, "config", "user.email", "oct@example.invalid")
	runGitCLIRegistryTestCommand(t, signalDir, "config", "user.name", "Oct Test")
	runGitCLIRegistryTestCommand(t, signalDir, "add", ".")
	runGitCLIRegistryTestCommand(t, signalDir, "commit", "-m", "initial")
	runGitCLIRegistryTestCommand(t, signalDir, "tag", "v0.1.0")
	registryDir := filepath.Join(workspace, "registry")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(registryDir, "registry.oct"), []byte(validGitRegistryForCLI(signalDir, "v0.1.0", ".")), 0o644); err != nil {
		t.Fatal(err)
	}
	consumerDir := filepath.Join(workspace, "Consumer")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "Consumer", Dir: consumerDir}); err != nil {
		t.Fatalf("scaffold Consumer: %v", err)
	}
	consumerTest := `package Consumer

import SignalTools

[Fact]
fn UsesGitRegistrySyncedDependency() -> Void {
    Assert.Equal(5, SignalTools.Identity(5), "git registry dependency should load")
}
`
	if err := os.WriteFile(filepath.Join(consumerDir, "Consumer.GitRegistry.octest"), []byte(consumerTest), 0o644); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, err := executeCLIInDir(consumerDir, "pkg", "registry", "add", "local", registryDir); err != nil || !strings.Contains(stdout, "Added package registry local") {
		t.Fatalf("registry add failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if stdout, stderr, err := executeCLIInDir(consumerDir, "pkg", "add", "SignalTools@0.1.0"); err != nil || !strings.Contains(stdout, "Added dependency SignalTools 0.1.0") {
		t.Fatalf("pkg add failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	stdout, stderr, err := executeCLIInDir(consumerDir, "pkg", "sync")
	if err != nil || !strings.Contains(stdout, "Cloned SignalTools 0.1.0 ref v0.1.0 resolved") || !strings.Contains(stdout, "warning: Git ref") {
		t.Fatalf("pkg sync failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(consumerDir, ".oct", "packages", "SignalTools", "0.1.0", ".git")); !os.IsNotExist(err) {
		t.Fatalf("expected no .git in synced package, err=%v", err)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "test", ".")
	if err != nil || !strings.Contains(stdout, "PASS Consumer.UsesGitRegistrySyncedDependency") {
		t.Fatalf("oct test failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
}

func TestPkgRegistryTransitiveSyncGoldenPath(t *testing.T) {
	workspace := t.TempDir()
	bDir := filepath.Join(workspace, "B")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "B", Dir: bDir}); err != nil {
		t.Fatal(err)
	}
	aDir := filepath.Join(workspace, "A")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "A", Dir: aDir}); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(aDir, "manifest.oct")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(manifest), `Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]`, `Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }, Dependency { Name: "B" VersionRequirement: "0.1.0" }]`, 1)
	if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aDir, "A.Transitive.oct"), []byte("package A\n\nimport B\n\nfn UseB(x: Int) -> Int {\n    return B.Identity(x)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	registryDir := filepath.Join(workspace, "registry")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []string{
		`PackageEntry { Name: "A" Version: "0.1.0" Kind: "library" SourceKind: "local" Source: ` + octStringLiteralPath(aDir) + ` Path: "." Description: "A" }`,
		`PackageEntry { Name: "B" Version: "0.1.0" Kind: "library" SourceKind: "local" Source: ` + octStringLiteralPath(bDir) + ` Path: "." Description: "B" }`,
	}
	if err := os.WriteFile(filepath.Join(registryDir, "registry.oct"), []byte(registryForCLIEntries(entries)), 0o644); err != nil {
		t.Fatal(err)
	}
	consumerDir := filepath.Join(workspace, "Consumer")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "Consumer", Dir: consumerDir}); err != nil {
		t.Fatal(err)
	}
	consumerTest := `package Consumer

import A

[Fact]
fn UsesTransitiveRegistryDependency() -> Void {
    Assert.Equal(7, A.UseB(7), "transitive registry dependency should load")
}
`
	if err := os.WriteFile(filepath.Join(consumerDir, "Consumer.TransitiveRegistry.octest"), []byte(consumerTest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "registry", "add", "local", registryDir); err != nil {
		t.Fatalf("registry add failed: %v %s", err, stderr)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "add", "A@0.1.0"); err != nil {
		t.Fatalf("pkg add failed: %v %s", err, stderr)
	}
	stdout, stderr, err := executeCLIInDir(consumerDir, "pkg", "sync")
	if err != nil || !strings.Contains(stdout, "Synced B 0.1.0") || !strings.Contains(stdout, "Synced A 0.1.0") {
		t.Fatalf("pkg sync failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	for _, dep := range []string{"A", "B"} {
		if _, err := os.Stat(filepath.Join(consumerDir, ".oct", "packages", dep, "0.1.0", "manifest.oct")); err != nil {
			t.Fatalf("missing synced %s: %v", dep, err)
		}
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "test", ".")
	if err != nil || !strings.Contains(stdout, "PASS Consumer.UsesTransitiveRegistryDependency") {
		t.Fatalf("oct test failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
}

func runGitCLIRegistryTestCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, strings.TrimSpace(string(out)))
	}
	return string(out)
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

func validGitRegistryForCLI(source string, ref string, path string) string {
	return registryForCLIEntries([]string{`PackageEntry { Name: "SignalTools" Version: "0.1.0" Kind: "library" SourceKind: "git" Source: ` + octStringLiteralPath(source) + ` Ref: "` + ref + `" Path: ` + octStringLiteralPath(path) + ` Description: "Signal helper functions" }`})
}

func registryForCLIEntries(entries []string) string {
	return "package Registry\n\nrecord RegistryIndex { Packages: PackageEntry[] }\nrecord PackageEntry { Name: String Version: String Kind: String SourceKind: String Source: String Ref: String Path: String Description: String }\nfn Registry() -> RegistryIndex {\n    return RegistryIndex { Packages: [" + strings.Join(entries, ", ") + "] }\n}\n"
}

func validRegistryForCLI(signalDir string) string {
	return `package Registry

record RegistryIndex { Packages: PackageEntry[] }
record PackageEntry { Name: String Version: String Kind: String SourceKind: String Source: String Path: String Description: String }
fn Registry() -> RegistryIndex {
    return RegistryIndex { Packages: [PackageEntry { Name: "SignalTools" Version: "0.1.0" Kind: "library" SourceKind: "local" Source: ` + octStringLiteralPath(signalDir) + ` Path: "." Description: "Signal helper functions" }] }
}
`
}
