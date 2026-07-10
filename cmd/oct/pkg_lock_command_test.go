//go:build toolchain

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/newpkg"
)

func TestPkgLockLocalSourceAndPlainSyncSeparation(t *testing.T) {
	workspace := t.TempDir()
	signalDir := filepath.Join(workspace, "SignalTools")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "SignalTools", Dir: signalDir}); err != nil {
		t.Fatal(err)
	}
	registryDir := filepath.Join(workspace, "registry")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []string{`PackageEntry { Name: "SignalTools" Version: "0.1.0" Kind: "library" SourceKind: "local" Source: ` + octStringLiteralPath(signalDir) + ` Ref: "" Path: "." Description: "Signal" }`}
	if err := os.WriteFile(filepath.Join(registryDir, "registry.oct"), []byte(registryForCLIEntries(entries)), 0o644); err != nil {
		t.Fatal(err)
	}
	consumerDir := filepath.Join(workspace, "Consumer")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "Consumer", Dir: consumerDir}); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "registry", "add", "local", registryDir); err != nil {
		t.Fatalf("registry add failed: %v %s", err, stderr)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "add", "SignalTools@0.1.0"); err != nil {
		t.Fatalf("pkg add failed: %v %s", err, stderr)
	}
	stdout, stderr, err := executeCLIInDir(consumerDir, "pkg", "sync")
	if err != nil {
		t.Fatalf("plain sync failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(consumerDir, "lock.octagon")); !os.IsNotExist(err) {
		t.Fatalf("plain sync must not create lock.octagon, err=%v", err)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "pkg", "lock")
	if err != nil || !strings.Contains(stdout, "Resolved package graph: 1 packages") || !strings.Contains(stdout, "warning: local source SignalTools@0.1.0 is mutable") || !strings.Contains(stdout, "Wrote lock.octagon") {
		t.Fatalf("pkg lock failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	lockBytes, err := os.ReadFile(filepath.Join(consumerDir, "lock.octagon"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lockBytes), `Mutable: true`) || strings.Contains(string(lockBytes), "GeneratedAt") {
		t.Fatalf("unexpected lock content:\n%s", lockBytes)
	}
	if _, err := os.Stat(filepath.Join(consumerDir, "oct.lock")); !os.IsNotExist(err) {
		t.Fatalf("oct.lock must not be created")
	}
	if _, err := os.Stat(filepath.Join(consumerDir, "lock.oct")); !os.IsNotExist(err) {
		t.Fatalf("lock.oct must not be created")
	}
	first := string(lockBytes)
	stdout, stderr, err = executeCLIInDir(consumerDir, "pkg", "lock")
	if err != nil {
		t.Fatalf("second lock failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	secondBytes, err := os.ReadFile(filepath.Join(consumerDir, "lock.octagon"))
	if err != nil {
		t.Fatal(err)
	}
	if first != string(secondBytes) {
		t.Fatalf("lock output should be byte-identical")
	}
}

func TestPkgSyncLockedMissingLockAndManifestDrift(t *testing.T) {
	workspace := t.TempDir()
	signalDir := filepath.Join(workspace, "SignalTools")
	extraDir := filepath.Join(workspace, "Extra")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "SignalTools", Dir: signalDir}); err != nil {
		t.Fatal(err)
	}
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "Extra", Dir: extraDir}); err != nil {
		t.Fatal(err)
	}
	registryDir := filepath.Join(workspace, "registry")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []string{
		`PackageEntry { Name: "SignalTools" Version: "0.1.0" Kind: "library" SourceKind: "local" Source: ` + octStringLiteralPath(signalDir) + ` Ref: "" Path: "." Description: "Signal" }`,
		`PackageEntry { Name: "Extra" Version: "0.1.0" Kind: "library" SourceKind: "local" Source: ` + octStringLiteralPath(extraDir) + ` Ref: "" Path: "." Description: "Extra" }`,
	}
	if err := os.WriteFile(filepath.Join(registryDir, "registry.oct"), []byte(registryForCLIEntries(entries)), 0o644); err != nil {
		t.Fatal(err)
	}
	consumerDir := filepath.Join(workspace, "Consumer")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "Consumer", Dir: consumerDir}); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "sync", "--locked"); err == nil || !strings.Contains(stderr, "lock.octagon is required for --locked; run oct pkg lock") {
		t.Fatalf("expected missing lock guidance err=%v stderr=%q", err, stderr)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "registry", "add", "local", registryDir); err != nil {
		t.Fatalf("registry add: %v %s", err, stderr)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "add", "SignalTools@0.1.0"); err != nil {
		t.Fatalf("pkg add: %v %s", err, stderr)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "lock"); err != nil {
		t.Fatalf("pkg lock: %v %s", err, stderr)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "add", "Extra@0.1.0"); err != nil {
		t.Fatalf("pkg add extra: %v %s", err, stderr)
	}
	_, stderr, err := executeCLIInDir(consumerDir, "pkg", "sync", "--locked")
	if err == nil || !strings.Contains(stderr, "manifest dependency Extra@0.1.0 is not locked; run oct pkg lock") {
		t.Fatalf("expected drift failure err=%v stderr=%q", err, stderr)
	}
}

func TestPkgLockGitMutableRefPinsResolvedCommit(t *testing.T) {
	requireGit(t)
	workspace := t.TempDir()
	repo := filepath.Join(workspace, "MathCoreRepo")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "MathCore", Dir: repo}); err != nil {
		t.Fatal(err)
	}
	runGitCLIRegistryTestCommand(t, repo, "init")
	runGitCLIRegistryTestCommand(t, repo, "config", "user.name", "oct-test")
	runGitCLIRegistryTestCommand(t, repo, "config", "user.email", "oct-test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "PIN.txt"), []byte("commit A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCLIRegistryTestCommand(t, repo, "add", ".")
	runGitCLIRegistryTestCommand(t, repo, "commit", "-m", "A")
	commitA := strings.TrimSpace(runGitCLIRegistryTestCommand(t, repo, "rev-parse", "HEAD"))
	runGitCLIRegistryTestCommand(t, repo, "tag", "moving")
	if err := os.WriteFile(filepath.Join(repo, "PIN.txt"), []byte("commit B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCLIRegistryTestCommand(t, repo, "add", ".")
	runGitCLIRegistryTestCommand(t, repo, "commit", "-m", "B")
	runGitCLIRegistryTestCommand(t, repo, "tag", "-f", "moving")
	registryDir := filepath.Join(workspace, "registry")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []string{`PackageEntry { Name: "MathCore" Version: "0.1.0" Kind: "library" SourceKind: "git" Source: ` + octStringLiteralPath(repo) + ` Ref: "moving" Path: "." Description: "Math" }`}
	if err := os.WriteFile(filepath.Join(registryDir, "registry.oct"), []byte(registryForCLIEntries(entries)), 0o644); err != nil {
		t.Fatal(err)
	}
	consumerDir := filepath.Join(workspace, "Consumer")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindLibrary, Name: "Consumer", Dir: consumerDir}); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "registry", "add", "local", registryDir); err != nil {
		t.Fatalf("registry add: %v %s", err, stderr)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "add", "MathCore@0.1.0"); err != nil {
		t.Fatalf("pkg add: %v %s", err, stderr)
	}
	// Move the mutable tag back to commit A for lock creation, then to B before locked sync.
	runGitCLIRegistryTestCommand(t, repo, "tag", "-f", "moving", commitA)
	stdout, stderr, err := executeCLIInDir(consumerDir, "pkg", "lock")
	if err != nil || !strings.Contains(stdout, "Wrote lock.octagon") {
		t.Fatalf("pkg lock: err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	runGitCLIRegistryTestCommand(t, repo, "tag", "-f", "moving", "HEAD")
	if err := os.RemoveAll(filepath.Join(consumerDir, ".oct", "packages")); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "pkg", "sync", "--locked")
	if err != nil || !strings.Contains(stdout, "Synced MathCore 0.1.0 from locked git commit "+commitA) {
		t.Fatalf("locked sync: err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	pinned, err := os.ReadFile(filepath.Join(consumerDir, ".oct", "packages", "MathCore", "0.1.0", "PIN.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(pinned) != "commit A\n" {
		t.Fatalf("locked sync checked out wrong content: %q", pinned)
	}
	if _, err := os.Stat(filepath.Join(consumerDir, ".oct", "packages", "MathCore", "0.1.0", ".oct-package-source.oct")); err != nil {
		t.Fatalf("missing source metadata: %v", err)
	}
}
