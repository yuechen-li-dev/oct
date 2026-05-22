package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPkgSyncPkgSyncDirectDependencies(t *testing.T) {
	requireGit(t)
	cacheDir := t.TempDir()
	t.Setenv("OCT_PKG_CACHE_DIR", cacheDir)

	depA := createGitRepoWithManifest(t, manifestWithDeps("Signal", "1.0.0", nil))
	depB := createGitRepoWithManifest(t, manifestWithDeps("Numerics", "2.0.0", nil))
	projectDir := createProjectWithManifest(t, projectManifestWithDeps([]string{
		`Dependency { Name: "Signal" VersionRequirement: "^1.0.0" Source: "` + depA + `" }`,
		`Dependency { Name: "Numerics" VersionRequirement: "~2.0" Source: "` + depB + `" }`,
	}))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "sync")
	if err != nil {
		t.Fatalf("expected pkg sync success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "dependencies: 2") {
		t.Fatalf("expected dependency count, got %q", stdout)
	}
	if !strings.Contains(stdout, "Signal (^1.0.0) [fetched]") || !strings.Contains(stdout, "Numerics (~2.0) [fetched]") {
		t.Fatalf("expected fetched statuses, got %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "index.json")); err != nil {
		t.Fatalf("expected cache index: %v", err)
	}
}

func TestPkgSyncPkgSyncCacheHitReuse(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	dep := createGitRepoWithManifest(t, manifestWithDeps("Signal", "1.0.0", nil))
	projectDir := createProjectWithManifest(t, projectManifestWithDeps([]string{
		`Dependency { Name: "Signal" VersionRequirement: "^1.0.0" Source: "` + dep + `" }`,
	}))

	stdout1, stderr1, err := executeCLIInDir(projectDir, "pkg", "sync")
	if err != nil {
		t.Fatalf("first sync failed: err=%v stderr=%q stdout=%q", err, stderr1, stdout1)
	}
	cachePath := parseDependencyCachePath(stdout1, "Signal")
	if cachePath == "" {
		t.Fatalf("expected cache path in output: %q", stdout1)
	}
	before, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("stat cache path: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	stdout2, stderr2, err := executeCLIInDir(projectDir, "pkg", "sync")
	if err != nil {
		t.Fatalf("second sync failed: err=%v stderr=%q stdout=%q", err, stderr2, stdout2)
	}
	if !strings.Contains(stdout2, "Signal (^1.0.0) [cache hit]") {
		t.Fatalf("expected cache hit output, got %q", stdout2)
	}
	after, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("stat cache path after second run: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("expected repo cache unchanged on hit; before=%v after=%v", before.ModTime(), after.ModTime())
	}
}

func TestPkgSyncPkgSyncMissingManifest(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	dir := t.TempDir()
	stdout, stderr, err := executeCLIInDir(dir, "pkg", "sync")
	if err == nil {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "project manifest not found") {
		t.Fatalf("expected missing manifest error, got %q", stderr)
	}
}

func TestPkgSyncPkgSyncFailsWhenDependencySourceMissing(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, projectManifestWithDeps([]string{
		`Dependency { Name: "Signal" VersionRequirement: "^1.0.0" }`,
	}))
	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "sync")
	if err == nil {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "missing fetchable source metadata") {
		t.Fatalf("expected missing source mapping error, got %q", stderr)
	}
}

func TestPkgSyncPkgSyncFailsOnMalformedDependencyMetadata(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, strings.Join([]string{
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
		"    Source: String",
		"}",
		"",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"Main\"",
		"        Version: \"0.1.0\"",
		"        Description: \"main project\"",
		"        Dependencies: [Dependency { Name: \"Signal\" VersionRequirement: \"^1.0.0\" Source: 3 }]",
		"    }",
		"}",
	}, "\n")+"\n")
	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "sync")
	if err == nil {
		t.Fatalf("expected failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "manifest dependency at index 0") || !strings.Contains(stderr, "Source") {
		t.Fatalf("expected malformed metadata error, got %q", stderr)
	}
}

func TestPkgSyncPkgSyncDeterministicOutput(t *testing.T) {
	requireGit(t)
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	dep := createGitRepoWithManifest(t, manifestWithDeps("Signal", "1.0.0", nil))
	projectDir := createProjectWithManifest(t, projectManifestWithDeps([]string{
		`Dependency { Name: "Signal" VersionRequirement: "^1.0.0" Source: "` + dep + `" }`,
	}))

	stdout1, stderr1, err := executeCLIInDir(projectDir, "pkg", "sync")
	if err != nil {
		t.Fatalf("first sync failed: err=%v stderr=%q stdout=%q", err, stderr1, stdout1)
	}
	stdout2, stderr2, err := executeCLIInDir(projectDir, "pkg", "sync")
	if err != nil {
		t.Fatalf("second sync failed: err=%v stderr=%q stdout=%q", err, stderr2, stdout2)
	}
	if !strings.Contains(stdout1, "sync complete") || !strings.Contains(stdout2, "sync complete") {
		t.Fatalf("expected completion output: first=%q second=%q", stdout1, stdout2)
	}
	if parseOutputField(stdout1, "dependencies") != parseOutputField(stdout2, "dependencies") {
		t.Fatalf("expected stable dependency count output: first=%q second=%q", stdout1, stdout2)
	}
}

func executeCLIInDir(dir string, args ...string) (string, string, error) {
	oldWD, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	if err := os.Chdir(dir); err != nil {
		return "", "", err
	}
	defer func() { _ = os.Chdir(oldWD) }()
	return executeCLIArgs(args...)
}

func createProjectWithManifest(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.oct"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write project manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.oct"), []byte("package Main\n"), 0o644); err != nil {
		t.Fatalf("write project source: %v", err)
	}
	return dir
}

func projectManifestWithDeps(depLiterals []string) string {
	deps := ""
	if len(depLiterals) > 0 {
		deps = "\n            " + strings.Join(depLiterals, ",\n            ") + "\n        "
	}
	return strings.Join([]string{
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
		"    Source: String",
		"}",
		"",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"Main\"",
		"        Version: \"0.1.0\"",
		"        Description: \"main project\"",
		"        Dependencies: [" + deps + "]",
		"    }",
		"}",
	}, "\n") + "\n"
}

func manifestWithDeps(name, version string, depLiterals []string) string {
	deps := "\n            Dependency { Name: \"Core\" VersionRequirement: \"^0.1.0\" }\n        "
	if len(depLiterals) > 0 {
		deps = "\n            " + strings.Join(depLiterals, ",\n            ") + "\n        "
	}
	return strings.Join([]string{
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
		"        Name: \"" + name + "\"",
		"        Version: \"" + version + "\"",
		"        Description: \"dep\"",
		"        Dependencies: [" + deps + "]",
		"    }",
		"}",
	}, "\n") + "\n"
}

func parseDependencyCachePath(output string, depName string) string {
	lines := strings.Split(output, "\n")
	for idx := 0; idx < len(lines)-1; idx++ {
		if strings.HasPrefix(lines[idx], "- "+depName+" ") {
			next := strings.TrimSpace(lines[idx+1])
			if strings.HasPrefix(next, "source: ") {
				if idx+2 >= len(lines) {
					return ""
				}
				cacheLine := strings.TrimSpace(lines[idx+2])
				if strings.HasPrefix(cacheLine, "cache: ") {
					return strings.TrimSpace(strings.TrimPrefix(cacheLine, "cache: "))
				}
			}
		}
	}
	return ""
}
