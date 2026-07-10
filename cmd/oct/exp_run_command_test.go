//go:build toolchain

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExpRunExpRunFetchesAndExecutesExperiment(t *testing.T) {
	requireGit(t)
	cacheDir := t.TempDir()
	t.Setenv("OCT_PKG_CACHE_DIR", cacheDir)
	baseDep := sharedExperimentBaseDependency(t)

	source := createExperimentGitRepo(t, experimentRepoSpec{
		Manifest: experimentManifestWithDeps("DemoExperiment", "0.1.0", "experiment", "", depLiterals(baseDep)),
		Milestones: map[string]string{
			"M0": milestoneFactSource("M0Runs"),
		},
	})

	stdout, stderr, err := executeCLIArgs("exp", "run", source)
	if err != nil {
		t.Fatalf("expected exp run success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "experiment fetch: fetched") {
		t.Fatalf("expected fetch status output, got %q", stdout)
	}
	if !strings.Contains(stdout, "entry milestone: canonical experiment root") {
		t.Fatalf("expected canonical root entry output, got %q", stdout)
	}
	if !strings.Contains(stdout, "MILESTONE M0 (test)") {
		t.Fatalf("expected milestone execution output, got %q", stdout)
	}
	if !strings.Contains(stdout, "MILESTONE PASS M0 (test)") {
		t.Fatalf("expected milestone pass output, got %q", stdout)
	}
}

func TestExpRunExpRunUsesCacheHitOnRepeatedRuns(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	baseDep := sharedExperimentBaseDependency(t)
	source := createExperimentGitRepo(t, experimentRepoSpec{
		Manifest: experimentManifestWithDeps("CacheExperiment", "0.1.0", "experiment", "", depLiterals(baseDep)),
		Milestones: map[string]string{
			"M0": milestoneFactSource("CacheRuns"),
		},
	})

	stdout1, stderr1, err := executeCLIArgs("exp", "run", source)
	if err != nil {
		t.Fatalf("first exp run failed: err=%v stderr=%q stdout=%q", err, stderr1, stdout1)
	}
	cachePath := parseOutputField(stdout1, "experiment cache")
	if cachePath == "" {
		t.Fatalf("expected experiment cache path output, got %q", stdout1)
	}
	fixedModTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(cachePath, fixedModTime, fixedModTime); err != nil {
		t.Fatalf("set deterministic cache modtime: %v", err)
	}
	before, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("stat cache path: %v", err)
	}
	stdout2, stderr2, err := executeCLIArgs("exp", "run", source)
	if err != nil {
		t.Fatalf("second exp run failed: err=%v stderr=%q stdout=%q", err, stderr2, stdout2)
	}
	if !strings.Contains(stdout2, "experiment fetch: cache hit") {
		t.Fatalf("expected cache hit output, got %q", stdout2)
	}
	after, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("stat cache path second run: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("expected unchanged cache path modtime; before=%v after=%v", before.ModTime(), after.ModTime())
	}
}

func TestExpRunExpRunSyncsDirectDependencies(t *testing.T) {
	requireGit(t)
	cacheDir := t.TempDir()
	t.Setenv("OCT_PKG_CACHE_DIR", cacheDir)

	depSource := createGitRepoWithManifest(t, manifestWithDeps("Signal", "1.0.0", nil))
	source := createExperimentGitRepo(t, experimentRepoSpec{
		Manifest: experimentManifestWithDeps("DepsExperiment", "0.1.0", "experiment", "", []string{
			`Dependency { Name: "Signal" VersionRequirement: "^1.0.0" Source: "` + depSource + `" }`,
		}),
		Milestones: map[string]string{"M0": milestoneFactSource("DepsRun")},
	})

	stdout, stderr, err := executeCLIArgs("exp", "run", source)
	if err != nil {
		t.Fatalf("expected exp run success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "dependency sync: 1 direct dependency(ies)") {
		t.Fatalf("expected dependency sync summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "- Signal (^1.0.0) [fetched]") {
		t.Fatalf("expected dependency status output, got %q", stdout)
	}
	index := readCacheIndexFile(t, filepath.Join(cacheDir, "index.json"))
	if !strings.Contains(index, depSource) {
		t.Fatalf("expected dependency source in cache index, got %q", index)
	}
}

func TestExpRunExpRunRejectsInvalidExperimentShape(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	baseDep := sharedExperimentBaseDependency(t)
	source := createExperimentGitRepo(t, experimentRepoSpec{
		Manifest: experimentManifestWithDeps("BadExperiment", "0.1.0", "experiment", "", depLiterals(baseDep)),
		NoReport: true,
		Milestones: map[string]string{
			"M0": milestoneFactSource("ShouldNotRun"),
		},
	})

	stdout, stderr, err := executeCLIArgs("exp", "run", source)
	if err == nil {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "missing REPORT.md") {
		t.Fatalf("expected missing REPORT failure, got %q", stderr)
	}
}

func TestExpRunExpRunRespectsExplicitEntryMilestone(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	baseDep := sharedExperimentBaseDependency(t)
	source := createExperimentGitRepo(t, experimentRepoSpec{
		Manifest: experimentManifestWithDeps("EntryExperiment", "0.1.0", "experiment", "M1", depLiterals(baseDep)),
		Milestones: map[string]string{
			"M0": milestoneFactSource("ShouldNotRunM0"),
			"M1": milestoneFactSource("RunsM1"),
		},
	})

	stdout, stderr, err := executeCLIArgs("exp", "run", source)
	if err != nil {
		t.Fatalf("expected exp run success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "entry milestone: M1") {
		t.Fatalf("expected explicit entry output, got %q", stdout)
	}
	if strings.Contains(stdout, "MILESTONE M0 (test)") || strings.Contains(stdout, "MILESTONE M1 (test)") {
		t.Fatalf("expected direct milestone execution for explicit entry, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS Demo.RunsM1") {
		t.Fatalf("expected selected milestone fact output, got %q", stdout)
	}
	if strings.Contains(stdout, "ShouldNotRunM0") {
		t.Fatalf("expected M0 exclusion for explicit entry, got %q", stdout)
	}
}

func TestExpRunExpRunFailsWhenExplicitEntryMissing(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	baseDep := sharedExperimentBaseDependency(t)
	source := createExperimentGitRepo(t, experimentRepoSpec{
		Manifest:   experimentManifestWithDeps("EntryMissing", "0.1.0", "experiment", "M7", depLiterals(baseDep)),
		Milestones: map[string]string{"M0": milestoneFactSource("M0Runs")},
	})

	stdout, stderr, err := executeCLIArgs("exp", "run", source)
	if err == nil {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, `explicit entry milestone "M7" not found`) {
		t.Fatalf("expected explicit entry missing failure, got %q", stderr)
	}
}

func TestExpRunExpRunFallsBackToCanonicalMilestonesOnly(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	baseDep := sharedExperimentBaseDependency(t)
	source := createExperimentGitRepo(t, experimentRepoSpec{
		Manifest: experimentManifestWithDeps("FallbackExperiment", "0.1.0", "experiment", "", depLiterals(baseDep)),
		Milestones: map[string]string{
			"M0":  milestoneFactSource("CanonicalRuns"),
			"Mx1": milestoneFactSource("AuxShouldNotRun"),
		},
	})

	stdout, stderr, err := executeCLIArgs("exp", "run", source)
	if err != nil {
		t.Fatalf("expected success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "MILESTONE M0 (test)") {
		t.Fatalf("expected canonical milestone run, got %q", stdout)
	}
	if strings.Contains(stdout, "Mx1") || strings.Contains(stdout, "AuxShouldNotRun") {
		t.Fatalf("expected auxiliary milestones to be excluded, got %q", stdout)
	}
}

func TestExpRunExpRunDeterministicForRepeatedRuns(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	baseDep := sharedExperimentBaseDependency(t)
	source := createExperimentGitRepo(t, experimentRepoSpec{
		Manifest:   experimentManifestWithDeps("DeterministicExperiment", "0.1.0", "experiment", "", depLiterals(baseDep)),
		Milestones: map[string]string{"M0": milestoneFactSource("DeterministicRuns")},
	})

	stdout1, stderr1, err := executeCLIArgs("exp", "run", source)
	if err != nil {
		t.Fatalf("first run failed: err=%v stderr=%q stdout=%q", err, stderr1, stdout1)
	}
	stdout2, stderr2, err := executeCLIArgs("exp", "run", source)
	if err != nil {
		t.Fatalf("second run failed: err=%v stderr=%q stdout=%q", err, stderr2, stdout2)
	}
	for _, fragment := range []string{"entry milestone: canonical experiment root", "MILESTONE M0 (test)", "MILESTONE PASS M0 (test)"} {
		if !strings.Contains(stdout1, fragment) || !strings.Contains(stdout2, fragment) {
			t.Fatalf("expected deterministic fragment %q in both outputs\nfirst=%q\nsecond=%q", fragment, stdout1, stdout2)
		}
	}
}

type experimentRepoSpec struct {
	Manifest   string
	NoReport   bool
	Milestones map[string]string
}

func createExperimentGitRepo(t *testing.T, spec experimentRepoSpec) string {
	t.Helper()
	repoDir := filepath.Join(t.TempDir(), "experiment")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir experiment repo: %v", err)
	}
	runCmd(t, repoDir, "git", "init")
	runCmd(t, repoDir, "git", "config", "user.name", "oct-test")
	runCmd(t, repoDir, "git", "config", "user.email", "oct-test@example.com")
	writeRepoManifest(t, repoDir, spec.Manifest)
	if !spec.NoReport {
		if err := os.WriteFile(filepath.Join(repoDir, "REPORT.md"), []byte("# report\n"), 0o644); err != nil {
			t.Fatalf("write report: %v", err)
		}
	}
	for milestone, testSource := range spec.Milestones {
		milestoneDir := filepath.Join(repoDir, milestone)
		if err := os.MkdirAll(milestoneDir, 0o755); err != nil {
			t.Fatalf("mkdir milestone %s: %v", milestone, err)
		}
		if err := os.WriteFile(filepath.Join(milestoneDir, "demo.oct"), []byte("package Demo\n\nfn Answer() -> Int {\n    return 42\n}\n"), 0o644); err != nil {
			t.Fatalf("write milestone source: %v", err)
		}
		if err := os.WriteFile(filepath.Join(milestoneDir, "demo.octest"), []byte(testSource), 0o644); err != nil {
			t.Fatalf("write milestone test: %v", err)
		}
	}
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "init")
	return localRepoSourceURL(t, repoDir)
}

func sharedExperimentBaseDependency(t *testing.T) string {
	t.Helper()
	sharedExpBaseOnce.Do(func() {
		sharedExpBaseDir, sharedExpBaseErr = os.MkdirTemp("", "oct-exp-base-*")
		if sharedExpBaseErr != nil {
			return
		}
		repoDir := filepath.Join(sharedExpBaseDir, "remote")
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			sharedExpBaseErr = err
			return
		}
		for _, command := range [][]string{
			{"git", "init"},
			{"git", "config", "user.name", "oct-test"},
			{"git", "config", "user.email", "oct-test@example.com"},
		} {
			if err := runSharedExperimentSetupCommand(repoDir, command[0], command[1:]...); err != nil {
				sharedExpBaseErr = err
				return
			}
		}
		if err := os.WriteFile(filepath.Join(repoDir, "manifest.oct"), []byte(manifestWithDeps("Base", "1.0.0", nil)), 0o644); err != nil {
			sharedExpBaseErr = err
			return
		}
		if err := os.WriteFile(filepath.Join(repoDir, "main.oct"), []byte("package Demo\n"), 0o644); err != nil {
			sharedExpBaseErr = err
			return
		}
		if err := runSharedExperimentSetupCommand(repoDir, "git", "add", "."); err != nil {
			sharedExpBaseErr = err
			return
		}
		if err := runSharedExperimentSetupCommand(repoDir, "git", "commit", "-m", "init"); err != nil {
			sharedExpBaseErr = err
			return
		}
		sharedExpBaseURL = localRepoSourceURL(t, repoDir)
	})
	if sharedExpBaseErr != nil {
		t.Fatalf("create shared experiment base dependency: %v", sharedExpBaseErr)
	}
	return sharedExpBaseURL
}

func runSharedExperimentSetupCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w: %s", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func milestoneFactSource(factName string) string {
	return "package Demo\n\n[Fact]\nfn " + factName + "() -> Void {\n    Assert.Equal(42, Answer(), \"answer should be 42\")\n}\n"
}

func depLiterals(source string) []string {
	return []string{
		`Dependency { Name: "Base" VersionRequirement: "^1.0.0" Source: "` + source + `" }`,
	}
}

func experimentManifestWithDeps(name, version, kind, entry string, depLiterals []string) string {
	deps := ""
	if len(depLiterals) > 0 {
		deps = "\n            " + strings.Join(depLiterals, ",\n            ") + "\n        "
	}
	kindLine := ""
	if kind != "" {
		kindLine = "\n        Kind: \"" + kind + "\""
	}
	entryLine := ""
	if entry != "" {
		entryLine = "\n        EntryMilestone: \"" + entry + "\""
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
		"    Source: String",
		"}",
		"",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"" + name + "\"",
		"        Version: \"" + version + "\"",
		"        Description: \"experiment\"" + kindLine + entryLine,
		"        Dependencies: [" + deps + "]",
		"    }",
		"}",
	}, "\n") + "\n"
}

func readCacheIndexFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cache index: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("parse cache index: %v", err)
	}
	return string(body)
}
