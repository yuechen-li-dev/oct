//go:build toolchain

package main

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPkgGetPkgGetFetchesIntoSharedCache(t *testing.T) {
	requireGit(t)
	cacheDir := t.TempDir()
	t.Setenv("OCT_PKG_CACHE_DIR", cacheDir)

	source := createGitRepo(t, true)
	stdout, stderr, err := executeCLIArgs("pkg", "get", source)
	if err != nil {
		t.Fatalf("expected pkg get success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "fetched") {
		t.Fatalf("expected fetch status, got %q", stdout)
	}
	if !strings.Contains(stdout, "package: DemoPkg@0.1.0") {
		t.Fatalf("expected package identity in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "dependencies: 1") {
		t.Fatalf("expected dependency count in output, got %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "index.json")); err != nil {
		t.Fatalf("expected cache index: %v", err)
	}
}

func TestPkgGetPkgGetUsesCacheHitOnRepeatedFetch(t *testing.T) {
	requireGit(t)
	cacheDir := t.TempDir()
	t.Setenv("OCT_PKG_CACHE_DIR", cacheDir)

	source := createGitRepo(t, true)
	stdout1, stderr1, err1 := executeCLIArgs("pkg", "get", source)
	if err1 != nil {
		t.Fatalf("first get failed: err=%v stderr=%q stdout=%q", err1, stderr1, stdout1)
	}
	cachePath := parseOutputField(stdout1, "cache")
	if cachePath == "" {
		t.Fatalf("expected cache path in first output: %q", stdout1)
	}
	fixedModTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(cachePath, fixedModTime, fixedModTime); err != nil {
		t.Fatalf("set deterministic cache modtime: %v", err)
	}
	infoBefore, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("expected cached repo path %s: %v", cachePath, err)
	}
	stdout2, stderr2, err2 := executeCLIArgs("pkg", "get", source)
	if err2 != nil {
		t.Fatalf("second get failed: err=%v stderr=%q stdout=%q", err2, stderr2, stdout2)
	}
	if !strings.Contains(stdout2, "cache hit") {
		t.Fatalf("expected cache hit output, got %q", stdout2)
	}
	infoAfter, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("expected cached repo path after second fetch: %v", err)
	}
	if !infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		t.Fatalf("expected cache entry modtime to remain unchanged for hit; before=%v after=%v", infoBefore.ModTime(), infoAfter.ModTime())
	}
}

func TestPkgGetPkgGetFailsWhenManifestMissing(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	source := createGitRepo(t, false)
	stdout, stderr, err := executeCLIArgs("pkg", "get", source)
	if err == nil {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "fetched repository has no manifest.oct") {
		t.Fatalf("expected missing manifest error, got %q", stderr)
	}
}

func TestPkgGetPkgListShowsCachedEntries(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	source := createGitRepo(t, true)
	if _, stderr, err := executeCLIArgs("pkg", "get", source); err != nil {
		t.Fatalf("pkg get failed: err=%v stderr=%q", err, stderr)
	}
	stdout, stderr, err := executeCLIArgs("pkg", "list")
	if err != nil {
		t.Fatalf("expected pkg list success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "DemoPkg@0.1.0") {
		t.Fatalf("expected package identity in list output, got %q", stdout)
	}
	if !strings.Contains(stdout, source) {
		t.Fatalf("expected source URL in list output, got %q", stdout)
	}
	if !strings.Contains(stdout, "deps: 1") {
		t.Fatalf("expected dependency count in list output, got %q", stdout)
	}
}

func TestPkgGetPkgGetRejectsBadSource(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	stdout, stderr, err := executeCLIArgs("pkg", "get", "not-a-url")
	if err == nil {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "invalid package source URL") {
		t.Fatalf("expected invalid source error, got %q", stderr)
	}
}

func TestPkgGetPkgGetRejectsMalformedManifestShape(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	source := createGitRepoWithManifest(t, strings.Join([]string{
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
		"        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: 3 }]",
		"    }",
		"}",
	}, "\n")+"\n")
	stdout, stderr, err := executeCLIArgs("pkg", "get", source)
	if err == nil {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "manifest dependency at index 0") {
		t.Fatalf("expected malformed dependency error, got %q", stderr)
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
}

func createGitRepo(t *testing.T, withManifest bool) string {
	t.Helper()
	repoDir := filepath.Join(t.TempDir(), "remote")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	runCmd(t, repoDir, "git", "init")
	runCmd(t, repoDir, "git", "config", "user.name", "oct-test")
	runCmd(t, repoDir, "git", "config", "user.email", "oct-test@example.com")
	if withManifest {
		manifest := strings.Join([]string{
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
			"        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"^1.0.0\" }]",
			"    }",
			"}",
		}, "\n") + "\n"
		writeRepoManifest(t, repoDir, manifest)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.oct"), []byte("package Demo\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "init")
	return localRepoSourceURL(t, repoDir)
}

func createGitRepoWithManifest(t *testing.T, manifest string) string {
	t.Helper()
	repoDir := filepath.Join(t.TempDir(), "remote")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	runCmd(t, repoDir, "git", "init")
	runCmd(t, repoDir, "git", "config", "user.name", "oct-test")
	runCmd(t, repoDir, "git", "config", "user.email", "oct-test@example.com")
	writeRepoManifest(t, repoDir, manifest)
	if err := os.WriteFile(filepath.Join(repoDir, "main.oct"), []byte("package Demo\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "init")
	return localRepoSourceURL(t, repoDir)
}

func localRepoSourceURL(t *testing.T, repoDir string) string {
	t.Helper()
	path := filepath.ToSlash(repoDir)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func writeRepoManifest(t *testing.T, repoDir string, manifest string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, "manifest.oct"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %v failed: %v\noutput:%s", name, args, err, string(out))
	}
}

func parseOutputField(output string, key string) string {
	prefix := key + ": "
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}
