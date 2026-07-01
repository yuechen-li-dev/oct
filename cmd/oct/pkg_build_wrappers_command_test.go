package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestPkgBuildWrappersRequiresAllowNativeBeforeOutput(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Xlsx", pkgWrappersLiteral("xlsx", "Xlsx", "octxiliary-xlsx", "sidecar")))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "build-wrappers")
	if err == nil {
		t.Fatalf("expected missing --allow-native failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "native wrapper builds require --allow-native") {
		t.Fatalf("expected allow-native error, got %q", stderr)
	}
	assertNoWrappersOutputDir(t, projectDir)
}

func TestPkgBuildWrappersNoWrapperPackageNoop(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, projectManifestWithDeps(nil))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "build-wrappers", "--allow-native")
	if err != nil {
		t.Fatalf("expected no-op success, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout,
		"Wrapper sidecar build:",
		"sidecars: 0",
		"native permission: yes",
		"No wrapper sidecars to build.",
	)
	assertNoWrappersOutputDir(t, projectDir)
}

func TestPkgBuildWrappersInvalidUsage(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, projectManifestWithDeps(nil))
	cases := [][]string{
		{"pkg", "build-wrappers", "extra"},
		{"pkg", "build-wrappers", "--allow-native", "extra"},
		{"pkg", "build-wrappers", "--unknown"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, err := executeCLIInDir(projectDir, args...)
			if err == nil {
				t.Fatalf("expected invalid usage failure, stdout=%q stderr=%q", stdout, stderr)
			}
			if !strings.Contains(stderr, "usage: oct pkg build-wrappers --allow-native") {
				t.Fatalf("expected usage error, got %q", stderr)
			}
			assertNoWrappersOutputDir(t, projectDir)
		})
	}
}

func TestPkgBuildWrappersInvalidGoModuleDirFailsBeforeOutput(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Xlsx", pkgWrappersLiteral("xlsx", "Xlsx", "octxiliary-xlsx", "")))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "build-wrappers", "--allow-native")
	if err == nil {
		t.Fatalf("expected invalid GoModuleDir failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "GoModuleDir") {
		t.Fatalf("expected GoModuleDir error, got %q", stderr)
	}
	assertNoWrappersOutputDir(t, projectDir)
}

func TestPkgBuildWrappersMissingSourceDirFailsClearly(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Xlsx", pkgWrappersLiteral("xlsx", "Xlsx", "octxiliary-xlsx", "missing-sidecar")))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "build-wrappers", "--allow-native")
	if err == nil {
		t.Fatalf("expected missing source failure, stdout=%q stderr=%q", stdout, stderr)
	}
	assertOutputContains(t, stdout, "Wrapper sidecar build:", "source dir: "+filepath.Join(projectDir, "missing-sidecar"))
	assertOutputContains(t, stderr, "wrapper sidecar build failed", "package Xlsx 0.1.0", "wrapper xlsx", "command octxiliary-xlsx", "source dir does not exist")
}

func TestPkgBuildWrappersGoBuildFailureReportsContext(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Broken", pkgWrappersLiteral("broken", "Broken", "octxiliary-broken", "sidecar")))
	sidecarDir := filepath.Join(projectDir, "sidecar")
	if err := os.MkdirAll(sidecarDir, 0o755); err != nil {
		t.Fatalf("mkdir sidecar: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sidecarDir, "go.mod"), []byte("module example.com/broken\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sidecarDir, "main.go"), []byte("package main\n\nfunc main() {\n    this is not go\n}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "build-wrappers", "--allow-native")
	if err == nil {
		t.Fatalf("expected go build failure, stdout=%q stderr=%q", stdout, stderr)
	}
	assertOutputContains(t, stdout, "command: octxiliary-broken", "output path: "+wrapperBinaryPath(projectDir, "octxiliary-broken"))
	assertOutputContains(t, stderr, "go build failed", "go output:", "package Broken 0.1.0", "wrapper broken")
}

func TestPkgBuildWrappersGeneratedWrapperLibraryGoldenPath(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	stdout, stderr, err := executeCLIInDir(root, "new", "wrapper-library", "OpenCV")
	if err != nil {
		t.Fatalf("oct new wrapper-library failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	packageDir := filepath.Join(root, "OpenCV")
	sidecarDir := filepath.Join(packageDir, "sidecars", "octxiliary-open-cv")
	appendLocalReplace(t, filepath.Join(sidecarDir, "go.mod"))
	sentinel := filepath.Join(sidecarDir, "executed.sentinel")
	patchSidecarSentinel(t, filepath.Join(sidecarDir, "main.go"), sentinel)

	buildStdout, buildStderr, buildErr := executeCLIInDir(packageDir, "pkg", "build-wrappers", "--allow-native")
	if buildErr != nil {
		t.Fatalf("build-wrappers failed: err=%v stderr=%q stdout=%q", buildErr, buildStderr, buildStdout)
	}
	assertOutputContains(t, buildStdout,
		"Wrapper sidecar build:",
		"platform: "+runtime.GOOS+"-"+runtime.GOARCH,
		"output dir: "+filepath.Dir(wrapperBinaryPath(packageDir, "octxiliary-open-cv")),
		"sidecars: 1",
		"native permission: yes",
		"* package OpenCV 0.1.0",
		"wrapper: open-cv",
		"family: OpenCV",
		"command: octxiliary-open-cv",
		"protocol: octxiliary.v0",
		"module: sidecars/octxiliary-open-cv",
		"source dir: "+sidecarDir,
		"output path: "+wrapperBinaryPath(packageDir, "octxiliary-open-cv"),
		"functions: 1",
		"Built wrapper sidecars: 1",
		"Set OCT_WRAPPER_PATH=.oct/wrappers/"+runtime.GOOS+"-"+runtime.GOARCH,
	)
	wrapperPath := wrapperBinaryPath(packageDir, "octxiliary-open-cv")
	info, err := os.Stat(wrapperPath)
	if err != nil {
		t.Fatalf("expected built wrapper binary: %v", err)
	}
	if runtime.GOOS == "windows" {
		if filepath.Ext(wrapperPath) != ".exe" {
			t.Fatalf("expected Windows wrapper binary to use .exe suffix, got %s", wrapperPath)
		}
		if info.Size() == 0 {
			t.Fatalf("expected non-empty Windows wrapper binary at %s", wrapperPath)
		}
	} else if info.Mode()&0o111 == 0 {
		t.Fatalf("expected executable wrapper binary mode, got %v", info.Mode())
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("sidecar appears to have executed; sentinel err=%v", err)
	}
}

func TestPkgWrappersRemainPlanningOnlyNoBuildOutput(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	projectDir := createProjectWithManifest(t, pkgWrappersManifestSource("Xlsx", pkgWrappersLiteral("xlsx", "Xlsx", "octxiliary-xlsx", "octxiliary")))

	stdout, stderr, err := executeCLIInDir(projectDir, "pkg", "wrappers")
	if err != nil {
		t.Fatalf("pkg wrappers failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, "Wrapper build plan:", "No wrapper sidecars were built or executed.")
	assertNoWrappersOutputDir(t, projectDir)

	registryPath := filepath.Join(t.TempDir(), "registry.octagon")
	registryStdout, registryStderr, registryErr := executeCLIInDir(projectDir, "pkg", "wrappers", "--registry-out", registryPath)
	if registryErr != nil {
		t.Fatalf("pkg wrappers registry failed: err=%v stderr=%q stdout=%q", registryErr, registryStderr, registryStdout)
	}
	assertOutputContains(t, registryStdout, "Wrote Octxiliary registry: "+registryPath, "No wrapper sidecars were built or executed.")
	assertNoWrappersOutputDir(t, projectDir)
}

func appendLocalReplace(t *testing.T, goModPath string) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	file, err := os.OpenFile(goModPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open go.mod: %v", err)
	}
	if _, err := file.WriteString("\nreplace github.com/yuechen-li-dev/oct => " + filepath.ToSlash(repoRoot) + "\n"); err != nil {
		file.Close()
		t.Fatalf("append replace: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close go.mod: %v", err)
	}

	// A bare `replace` directive against a local checkout is only ever
	// go.sum-consistent by accident: it depends on the replaced module's
	// full dependency graph (go version, transitive requires) happening to
	// match whatever go.sum/go.mod this scaffold was written with. Any
	// future change to the root module (a new dependency, a go directive
	// bump) invalidates that accident. `go mod tidy` is what a real sidecar
	// author would run immediately after adding a local replace, so run it
	// here too rather than assuming the frozen scaffold stays valid forever.
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = filepath.Dir(goModPath)
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy after local replace: %v\n%s", err, out)
	}
}

func patchSidecarSentinel(t *testing.T, path string, sentinel string) {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sidecar main: %v", err)
	}
	content := string(bytes)
	needle := "func main() {\n"
	replacement := "func main() {\n    _ = os.WriteFile(" + strconv.Quote(sentinel) + ", []byte(\"executed\"), 0o644)\n"
	content = strings.Replace(content, needle, replacement, 1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sidecar main: %v", err)
	}
}

func wrapperBinaryPath(projectDir string, command string) string {
	name := command
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	return filepath.Join(projectDir, ".oct", "wrappers", runtime.GOOS+"-"+runtime.GOARCH, name)
}

func assertNoWrappersOutputDir(t *testing.T, projectDir string) {
	t.Helper()
	path := filepath.Join(projectDir, ".oct", "wrappers")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no wrapper output dir at %s, err=%v", path, err)
	}
}
