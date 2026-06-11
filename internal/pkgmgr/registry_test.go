package pkgmgr

import (
	"os"
	"os/exec"
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
	if len(idx.Packages) != 1 || idx.Packages[0].Name != "SignalTools" || idx.Packages[0].Ref != "" {
		t.Fatalf("unexpected registry index: %#v", idx)
	}

	root = writeRegistryIndex(t, registryIndexWithRefEntries([]string{packageEntryWithRef("SignalTools", "0.1.0", "library", "local", "../SignalTools", "", ".")}))
	idx, err = LoadRegistryIndex(root)
	if err != nil {
		t.Fatalf("load registry index with empty local ref: %v", err)
	}
	if idx.Packages[0].Ref != "" {
		t.Fatalf("unexpected local ref: %#v", idx.Packages[0])
	}

	root = writeRegistryIndex(t, registryIndexWithRefEntries([]string{packageEntryWithRef("SignalTools", "0.1.0", "library", "git", "https://example.invalid/repo.git", "v0.1.0", ".")}))
	idx, err = LoadRegistryIndex(root)
	if err != nil {
		t.Fatalf("load registry index with git ref: %v", err)
	}
	if idx.Packages[0].SourceKind != "git" || idx.Packages[0].Ref != "v0.1.0" {
		t.Fatalf("unexpected git entry: %#v", idx.Packages[0])
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
		{"unsupported source kind", validRegistryIndex("SignalTools", "0.1.0", "library", "ftp", "../SignalTools", "."), "SourceKind must be one of local, git"},
		{"local non-empty ref", registryIndexWithRefEntries([]string{packageEntryWithRef("SignalTools", "0.1.0", "library", "local", "../SignalTools", "v0.1.0", ".")}), "Ref must be empty for local"},
		{"git missing ref", validRegistryIndex("SignalTools", "0.1.0", "library", "git", "../SignalTools", "."), "Ref is required for git sources"},
		{"git empty ref", registryIndexWithRefEntries([]string{packageEntryWithRef("SignalTools", "0.1.0", "library", "git", "../SignalTools", "", ".")}), "Ref is required for git sources"},
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

func TestSyncRegistryDependencyGitTagWritesMetadataAndSkipsGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	project := t.TempDir()
	repo := writePackageSource(t, "SignalTools", "0.1.0", "")
	runGitTestCommand(t, repo, "init")
	runGitTestCommand(t, repo, "config", "user.email", "oct@example.invalid")
	runGitTestCommand(t, repo, "config", "user.name", "Oct Test")
	runGitTestCommand(t, repo, "add", ".")
	runGitTestCommand(t, repo, "commit", "-m", "initial")
	runGitTestCommand(t, repo, "tag", "v0.1.0")
	commit := strings.TrimSpace(runGitTestCommand(t, repo, "rev-parse", "HEAD"))

	reg := writeRegistryIndex(t, registryIndexWithRefEntries([]string{packageEntryWithRef("SignalTools", "0.1.0", "library", "git", repo, "v0.1.0", ".")}))
	if _, err := AddRegistry(project, "local", reg); err != nil {
		t.Fatal(err)
	}
	result, err := SyncRegistryDependency(project, DependencyMetadata{Name: "SignalTools", VersionRequirement: "0.1.0"})
	if err != nil {
		t.Fatalf("sync git registry dependency: %v", err)
	}
	if result.SourceKind != "git" || result.Ref != "v0.1.0" || result.ResolvedCommit != commit {
		t.Fatalf("unexpected git sync result: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(result.Destination, ".git")); !os.IsNotExist(err) {
		t.Fatalf("expected .git skipped from final copy, err=%v", err)
	}
	metadata, err := os.ReadFile(filepath.Join(result.Destination, PackageSourceFileName))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	body := string(metadata)
	if !strings.Contains(body, `Ref: "v0.1.0"`) || !strings.Contains(body, `ResolvedCommit: "`+commit+`"`) {
		t.Fatalf("metadata missing ref/commit: %s", body)
	}
}

func TestSyncRegistryDependencyGitSubpath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	project := t.TempDir()
	repo := t.TempDir()
	pkg := writePackageSource(t, "SignalTools", "0.1.0", "")
	if err := os.MkdirAll(filepath.Join(repo, "packages"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(pkg, filepath.Join(repo, "packages", "SignalTools")); err != nil {
		t.Fatal(err)
	}
	runGitTestCommand(t, repo, "init")
	runGitTestCommand(t, repo, "config", "user.email", "oct@example.invalid")
	runGitTestCommand(t, repo, "config", "user.name", "Oct Test")
	runGitTestCommand(t, repo, "add", ".")
	runGitTestCommand(t, repo, "commit", "-m", "initial")
	commit := strings.TrimSpace(runGitTestCommand(t, repo, "rev-parse", "HEAD"))

	reg := writeRegistryIndex(t, registryIndexWithRefEntries([]string{packageEntryWithRef("SignalTools", "0.1.0", "library", "git", repo, commit, "packages/SignalTools")}))
	if _, err := AddRegistry(project, "local", reg); err != nil {
		t.Fatal(err)
	}
	result, err := SyncRegistryDependency(project, DependencyMetadata{Name: "SignalTools", VersionRequirement: "0.1.0"})
	if err != nil {
		t.Fatalf("sync git subpath dependency: %v", err)
	}
	if _, err := os.Stat(filepath.Join(result.Destination, "manifest.oct")); err != nil {
		t.Fatalf("missing synced subpath manifest: %v", err)
	}
}

func TestSyncRegistryDependencyGitCheckoutIgnoresHostAutoCRLF(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	project := t.TempDir()
	repo, commit := writeGitPackageRepoWithLineEndingFixture(t, "SignalTools", "0.1.0")
	enableHostGitAutoCRLF(t)

	reg := writeRegistryIndex(t, registryIndexWithRefEntries([]string{packageEntryWithRef("SignalTools", "0.1.0", "library", "git", repo, commit, ".")}))
	if _, err := AddRegistry(project, "local", reg); err != nil {
		t.Fatal(err)
	}
	result, err := SyncRegistryDependency(project, DependencyMetadata{Name: "SignalTools", VersionRequirement: "0.1.0"})
	if err != nil {
		t.Fatalf("sync git registry dependency: %v", err)
	}
	assertLineEndingFixtureHasLF(t, result.Destination)
}

func TestSyncLockedGitCheckoutIgnoresHostAutoCRLF(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	project := writePackageSourceWithDeps(t, "Consumer", "0.1.0", "", []DependencyMetadata{{Name: "SignalTools", VersionRequirement: "0.1.0"}})
	repo, commit := writeGitPackageRepoWithLineEndingFixture(t, "SignalTools", "0.1.0")
	lock := PackageLock{
		LockVersion: CurrentLockVersion,
		GeneratedBy: LockGeneratedBy,
		Root:        LockRoot{Name: "Consumer", Version: "0.1.0"},
		Packages: []LockPackage{{
			Name:           "SignalTools",
			Version:        "0.1.0",
			Kind:           "library",
			SourceKind:     "git",
			Source:         repo,
			Ref:            "main",
			ResolvedCommit: commit,
			Path:           ".",
			Registry:       "local",
			RegistryPath:   "../registry",
		}},
	}
	if err := WritePackageLock(filepath.Join(project, LockFileName), lock); err != nil {
		t.Fatal(err)
	}
	enableHostGitAutoCRLF(t)

	manager, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.SyncLocked(project)
	if err != nil {
		t.Fatalf("sync locked git dependency: %v", err)
	}
	if len(result.Packages) != 1 {
		t.Fatalf("expected one locked package, got %#v", result.Packages)
	}
	assertLineEndingFixtureHasLF(t, result.Packages[0].Destination)
}

func TestManagerGetGitCloneIgnoresHostAutoCRLF(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	repo, _ := writeGitPackageRepoWithLineEndingFixture(t, "SignalTools", "0.1.0")
	t.Setenv(envCacheDir, t.TempDir())
	enableHostGitAutoCRLF(t)

	manager, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.Get(fileURLFromPath(repo))
	if err != nil {
		t.Fatalf("get git package: %v", err)
	}
	assertLineEndingFixtureHasLF(t, result.Path)
}

func TestSyncRegistryDependencyGitCheckoutFailureIncludesRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	project := t.TempDir()
	repo := writePackageSource(t, "SignalTools", "0.1.0", "")
	runGitTestCommand(t, repo, "init")
	runGitTestCommand(t, repo, "config", "user.email", "oct@example.invalid")
	runGitTestCommand(t, repo, "config", "user.name", "Oct Test")
	runGitTestCommand(t, repo, "add", ".")
	runGitTestCommand(t, repo, "commit", "-m", "initial")
	reg := writeRegistryIndex(t, registryIndexWithRefEntries([]string{packageEntryWithRef("SignalTools", "0.1.0", "library", "git", repo, "missing-ref", ".")}))
	if _, err := AddRegistry(project, "local", reg); err != nil {
		t.Fatal(err)
	}
	_, err := SyncRegistryDependency(project, DependencyMetadata{Name: "SignalTools", VersionRequirement: "0.1.0"})
	if err == nil || !strings.Contains(err.Error(), "checkout") || !strings.Contains(err.Error(), "missing-ref") {
		t.Fatalf("expected checkout ref error, got %v", err)
	}
}

func writeGitPackageRepoWithLineEndingFixture(t *testing.T, name, version string) (string, string) {
	t.Helper()
	repo := writePackageSource(t, name, version, "")
	if err := os.WriteFile(filepath.Join(repo, "LineEndings.txt"), []byte("commit A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTestCommand(t, repo, "init")
	runGitTestCommand(t, repo, "config", "user.email", "oct@example.invalid")
	runGitTestCommand(t, repo, "config", "user.name", "Oct Test")
	runGitTestCommand(t, repo, "add", ".")
	runGitTestCommand(t, repo, "commit", "-m", "initial")
	commit := strings.TrimSpace(runGitTestCommand(t, repo, "rev-parse", "HEAD"))
	return repo, commit
}

func enableHostGitAutoCRLF(t *testing.T) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(configPath, []byte("[core]\n\tautocrlf = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", configPath)
}

func assertLineEndingFixtureHasLF(t *testing.T, packageRoot string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(packageRoot, "LineEndings.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "commit A\n" {
		t.Fatalf("expected git checkout to preserve LF line endings, got %q", string(body))
	}
}

func runGitTestCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, strings.TrimSpace(string(out)))
	}
	return string(out)
}

func TestManagerSyncTransitiveRegistryDependencies(t *testing.T) {
	project := writePackageSourceWithDeps(t, "App", "0.1.0", "", []DependencyMetadata{{Name: "A", VersionRequirement: "1.0.0"}})
	pkgA := writePackageSourceWithDeps(t, "A", "1.0.0", "", []DependencyMetadata{{Name: "B", VersionRequirement: "2.0.0"}})
	pkgB := writePackageSourceWithDeps(t, "B", "2.0.0", "", nil)
	reg := writeRegistryIndex(t, registryIndexWithEntries([]string{
		packageEntry("A", "1.0.0", "library", "local", pkgA, "."),
		packageEntry("B", "2.0.0", "library", "local", pkgB, "."),
	}))
	if _, err := AddRegistry(project, "local", reg); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.Sync(project)
	if err != nil {
		t.Fatalf("sync transitive: %v", err)
	}
	if len(result.RegistryDependencies) != 2 || result.RegistryDependencies[0].Name != "B" || result.RegistryDependencies[1].Name != "A" {
		t.Fatalf("expected dependency-before-importer order B,A; got %#v", result.RegistryDependencies)
	}
	for _, dep := range []struct{ name, version string }{{"A", "1.0.0"}, {"B", "2.0.0"}} {
		if _, err := os.Stat(filepath.Join(project, ProjectPackagesRelDir, dep.name, dep.version, manifestFileName)); err != nil {
			t.Fatalf("missing synced %s@%s: %v", dep.name, dep.version, err)
		}
	}
}

func TestManagerSyncConflictingExactVersionsIncludesChains(t *testing.T) {
	project := writePackageSourceWithDeps(t, "App", "0.1.0", "", []DependencyMetadata{{Name: "A", VersionRequirement: "1.0.0"}, {Name: "B", VersionRequirement: "1.0.0"}})
	pkgA := writePackageSourceWithDeps(t, "A", "1.0.0", "", []DependencyMetadata{{Name: "C", VersionRequirement: "1.0.0"}})
	pkgB := writePackageSourceWithDeps(t, "B", "1.0.0", "", []DependencyMetadata{{Name: "C", VersionRequirement: "2.0.0"}})
	pkgC1 := writePackageSourceWithDeps(t, "C", "1.0.0", "", nil)
	pkgC2 := writePackageSourceWithDeps(t, "C", "2.0.0", "", nil)
	reg := writeRegistryIndex(t, registryIndexWithEntries([]string{
		packageEntry("A", "1.0.0", "library", "local", pkgA, "."),
		packageEntry("B", "1.0.0", "library", "local", pkgB, "."),
		packageEntry("C", "1.0.0", "library", "local", pkgC1, "."),
		packageEntry("C", "2.0.0", "library", "local", pkgC2, "."),
	}))
	if _, err := AddRegistry(project, "local", reg); err != nil {
		t.Fatal(err)
	}
	manager, _ := NewManager()
	_, err := manager.Sync(project)
	if err == nil || !strings.Contains(err.Error(), "conflicting exact versions") || !strings.Contains(err.Error(), "App -> A@1.0.0 -> C@1.0.0") || !strings.Contains(err.Error(), "App -> B@1.0.0 -> C@2.0.0") {
		t.Fatalf("expected conflict with chains, got %v", err)
	}
}

func TestManagerSyncCycleIncludesCyclePath(t *testing.T) {
	project := writePackageSourceWithDeps(t, "App", "0.1.0", "", []DependencyMetadata{{Name: "A", VersionRequirement: "1.0.0"}})
	pkgA := writePackageSourceWithDeps(t, "A", "1.0.0", "", []DependencyMetadata{{Name: "B", VersionRequirement: "1.0.0"}})
	pkgB := writePackageSourceWithDeps(t, "B", "1.0.0", "", []DependencyMetadata{{Name: "A", VersionRequirement: "1.0.0"}})
	reg := writeRegistryIndex(t, registryIndexWithEntries([]string{
		packageEntry("A", "1.0.0", "library", "local", pkgA, "."),
		packageEntry("B", "1.0.0", "library", "local", pkgB, "."),
	}))
	if _, err := AddRegistry(project, "local", reg); err != nil {
		t.Fatal(err)
	}
	manager, _ := NewManager()
	_, err := manager.Sync(project)
	if err == nil || !strings.Contains(err.Error(), "dependency cycle detected") || !strings.Contains(err.Error(), "A@1.0.0 -> B@1.0.0 -> A@1.0.0") {
		t.Fatalf("expected cycle path, got %v", err)
	}
}

func TestManagerSyncTransitiveAmbiguityAndMissingIncludeChain(t *testing.T) {
	for _, tc := range []struct {
		name           string
		secondRegistry bool
		want           string
	}{
		{"ambiguous", true, "multiple registries"},
		{"missing", false, "not found in registries"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := writePackageSourceWithDeps(t, "App", "0.1.0", "", []DependencyMetadata{{Name: "A", VersionRequirement: "1.0.0"}})
			pkgA := writePackageSourceWithDeps(t, "A", "1.0.0", "", []DependencyMetadata{{Name: "B", VersionRequirement: "2.0.0"}})
			pkgB := writePackageSourceWithDeps(t, "B", "2.0.0", "", nil)
			reg1Entries := []string{packageEntry("A", "1.0.0", "library", "local", pkgA, ".")}
			if tc.secondRegistry {
				reg1Entries = append(reg1Entries, packageEntry("B", "2.0.0", "library", "local", pkgB, "."))
			}
			reg1 := writeRegistryIndex(t, registryIndexWithEntries(reg1Entries))
			if _, err := AddRegistry(project, "local", reg1); err != nil {
				t.Fatal(err)
			}
			if tc.secondRegistry {
				reg2 := writeRegistryIndex(t, registryIndexWithEntries([]string{packageEntry("B", "2.0.0", "library", "local", pkgB, ".")}))
				if _, err := AddRegistry(project, "vendor", reg2); err != nil {
					t.Fatal(err)
				}
			}
			manager, _ := NewManager()
			_, err := manager.Sync(project)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "App -> A@1.0.0 -> B@2.0.0") {
				t.Fatalf("expected %s with chain, got %v", tc.want, err)
			}
		})
	}
}

func TestManagerSyncNonExactTransitiveVersionErrors(t *testing.T) {
	project := writePackageSourceWithDeps(t, "App", "0.1.0", "", []DependencyMetadata{{Name: "A", VersionRequirement: "1.0.0"}})
	pkgA := writePackageSourceWithDeps(t, "A", "1.0.0", "", []DependencyMetadata{{Name: "B", VersionRequirement: "^2.0.0"}})
	reg := writeRegistryIndex(t, registryIndexWithEntries([]string{packageEntry("A", "1.0.0", "library", "local", pkgA, ".")}))
	if _, err := AddRegistry(project, "local", reg); err != nil {
		t.Fatal(err)
	}
	manager, _ := NewManager()
	_, err := manager.Sync(project)
	if err == nil || !strings.Contains(err.Error(), "not an exact version") || !strings.Contains(err.Error(), "App -> A@1.0.0 -> B@^2.0.0") {
		t.Fatalf("expected non-exact chain error, got %v", err)
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
	return registryIndexRecord(entries, false)
}

func registryIndexWithRefEntries(entries []string) string {
	return registryIndexRecord(entries, true)
}

func registryIndexRecord(entries []string, includeRef bool) string {
	refField := ""
	if includeRef {
		refField = "    Ref: String\n"
	}
	return "package Registry\n\nrecord RegistryIndex {\n    Packages: PackageEntry[]\n}\n\nrecord PackageEntry {\n    Name: String\n    Version: String\n    Kind: String\n    SourceKind: String\n    Source: String\n" + refField + "    Path: String\n    Description: String\n}\n\nfn Registry() -> RegistryIndex {\n    return RegistryIndex {\n        Packages: [\n            " + strings.Join(entries, ",\n            ") + "\n        ]\n    }\n}\n"
}

func packageEntry(name, version, kind, sourceKind, source, path string) string {
	return `PackageEntry { Name: "` + name + `" Version: "` + version + `" Kind: "` + kind + `" SourceKind: "` + sourceKind + `" Source: ` + octStringLiteralPath(source) + ` Path: ` + octStringLiteralPath(path) + ` Description: "test" }`
}

func packageEntryWithRef(name, version, kind, sourceKind, source, ref, path string) string {
	return `PackageEntry { Name: "` + name + `" Version: "` + version + `" Kind: "` + kind + `" SourceKind: "` + sourceKind + `" Source: ` + octStringLiteralPath(source) + ` Ref: "` + ref + `" Path: ` + octStringLiteralPath(path) + ` Description: "test" }`
}

func octStringLiteralPath(path string) string {
	return strconv.Quote(strings.ReplaceAll(filepath.ToSlash(path), `\`, "/"))
}

func writePackageSourceWithDeps(t *testing.T, name, version, kind string, deps []DependencyMetadata) string {
	t.Helper()
	root := t.TempDir()
	fields := ""
	if kind != "" {
		fields = "        Kind: \"" + kind + "\"\n"
	}
	depLits := make([]string, 0, len(deps))
	for _, dep := range deps {
		lit := "Dependency { Name: \"" + dep.Name + "\" VersionRequirement: \"" + dep.VersionRequirement + "\""
		if dep.Source != "" {
			lit += " Source: \"" + dep.Source + "\""
		}
		lit += " }"
		depLits = append(depLits, lit)
	}
	manifest := "package Manifest\n\nrecord PackageManifest {\n    Name: String\n    Version: String\n    Description: String\n" + func() string {
		if kind != "" {
			return "    Kind: String\n"
		}
		return ""
	}() + "    Dependencies: Dependency[]\n}\n\nrecord Dependency {\n    Name: String\n    VersionRequirement: String\n" + func() string {
		for _, d := range deps {
			if d.Source != "" {
				return "    Source: String\n"
			}
		}
		return ""
	}() + "}\n\nfn Manifest() -> PackageManifest {\n    return PackageManifest {\n        Name: \"" + name + "\"\n        Version: \"" + version + "\"\n        Description: \"test\"\n" + fields + "        Dependencies: [" + strings.Join(depLits, ", ") + "]\n    }\n}\n"
	if err := os.WriteFile(filepath.Join(root, "manifest.oct"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name+".oct"), []byte("package "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
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
