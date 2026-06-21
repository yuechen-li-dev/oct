package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLibraryCreatesScaffoldAndTestsPass(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	stdout, stderr, err := executeCLIInDir(root, "new", "library", "SignalTools")
	if err != nil {
		t.Fatalf("oct new library failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, "Created library package SignalTools at SignalTools")
	assertNewFilesExist(t, filepath.Join(root, "SignalTools"), "manifest.oct", "README.md", "SignalTools.Core.oct", "SignalTools.Core.octest")
	assertFileContains(t, filepath.Join(root, "SignalTools", "manifest.oct"), "Name: \"SignalTools\"", "Description: \"SignalTools package\"", "Authors: [\"Unknown\"]", "Date: \"2026-06-15\"")
	assertGeneratedPackageTestsPass(t, root, "SignalTools")
}

func TestNewExperimentCreatesScaffoldAndTestsPass(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	stdout, stderr, err := executeCLIInDir(root, "new", "experiment", "BrownNoiseKalman")
	if err != nil {
		t.Fatalf("oct new experiment failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, "Created experiment package BrownNoiseKalman at BrownNoiseKalman")
	packageDir := filepath.Join(root, "BrownNoiseKalman")
	assertNewFilesExist(t, packageDir, "manifest.oct", "README.md", "REPORT.md", "M0/brown_noise_kalman_m0.oct", "M0/brown_noise_kalman_m0.octest")
	assertFileContains(t, filepath.Join(packageDir, "manifest.oct"), "Kind: \"experiment\"", "EntryMilestone: \"M0\"", "Authors: [\"Unknown\"]", "Date: \"2026-06-15\"")
	assertGeneratedPackageTestsPass(t, root, "BrownNoiseKalman")
}

func TestNewApplicationAliasesCreateScaffoldRunAndTest(t *testing.T) {
	for _, kind := range []string{"app", "application"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
			root := t.TempDir()
			stdout, stderr, err := executeCLIInDir(root, "new", kind, "MyApp")
			if err != nil {
				t.Fatalf("oct new %s failed: err=%v stderr=%q stdout=%q", kind, err, stderr, stdout)
			}
			assertOutputContains(t, stdout, "Created "+kind+" package MyApp at MyApp")
			packageDir := filepath.Join(root, "MyApp")
			assertNewFilesExist(t, packageDir, "manifest.oct", "README.md", "Main.oct", "Main.octest")
			assertFileContains(t, filepath.Join(packageDir, "manifest.oct"), "Kind: \"application\"", "Description: \"Runnable Oct application.\"", "Authors: [\"Unknown\"]", "Date: \"2026-06-15\"")
			assertFileContains(t, filepath.Join(packageDir, "Main.oct"), "package MyApp", "fn Main() -> Int", "Hello from MyApp")

			runOut, runErrOut, runErr := executeCLIInDir(root, "run", packageDir)
			if runErr != nil {
				t.Fatalf("oct run generated app failed: err=%v stderr=%q stdout=%q", runErr, runErrOut, runOut)
			}
			assertOutputContains(t, runOut, "Hello from MyApp")
			assertGeneratedPackageTestsPass(t, root, "MyApp")
		})
	}
}

func TestNewWrapperLibraryCreatesScaffoldAndWrappersMetadata(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	stdout, stderr, err := executeCLIInDir(root, "new", "wrapper-library", "OpenCV")
	if err != nil {
		t.Fatalf("oct new wrapper-library failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, "Created wrapper-library package OpenCV at OpenCV")
	packageDir := filepath.Join(root, "OpenCV")
	assertNewFilesExist(t, packageDir,
		"manifest.oct", "README.md", "OpenCV.Core.oct", "OpenCV.Core.octest",
		"sidecars/octxiliary-open-cv/go.mod", "sidecars/octxiliary-open-cv/main.go", "sidecars/octxiliary-open-cv/README.md",
	)
	assertFileContains(t, filepath.Join(packageDir, "manifest.oct"), "SidecarCommand: \"octxiliary-open-cv\"", "WireName: \"OpenCVEchoString\"", "Authors: [\"Unknown\"]", "Date: \"2026-06-15\"")
	assertGeneratedPackageTestsPass(t, root, "OpenCV")

	wrappersOut, wrappersErr, wrappersRunErr := executeCLIInDir(packageDir, "pkg", "wrappers")
	if wrappersRunErr != nil {
		t.Fatalf("pkg wrappers failed: err=%v stderr=%q stdout=%q", wrappersRunErr, wrappersErr, wrappersOut)
	}
	assertOutputContains(t, wrappersOut,
		"native wrappers: yes",
		"sidecars: 1",
		"* package OpenCV 0.1.0",
		"wrapper: open-cv",
		"family: OpenCV",
		"command: octxiliary-open-cv",
		"module: sidecars/octxiliary-open-cv",
		"No wrapper sidecars were built or executed.",
	)

	registryPath := filepath.Join(root, "registry.octagon")
	registryOut, registryErr, registryRunErr := executeCLIInDir(packageDir, "pkg", "wrappers", "--registry-out", registryPath)
	if registryRunErr != nil {
		t.Fatalf("pkg wrappers registry failed: err=%v stderr=%q stdout=%q", registryRunErr, registryErr, registryOut)
	}
	assertOutputContains(t, registryOut, "Wrote Octxiliary registry: "+registryPath)
	registryBytes, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	assertOutputContains(t, string(registryBytes), "PackageName: \"OpenCV\"", "WrapperName: \"open-cv\"", "SidecarCommand: \"octxiliary-open-cv\"", "WireName: \"OpenCVEchoString\"")
}

func TestNewDefaultsToCollectionDirectoriesWhenPresent(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Experiments"), 0o755); err != nil {
		t.Fatalf("mkdir Experiments: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "Libraries"), 0o755); err != nil {
		t.Fatalf("mkdir Libraries: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "Applications"), 0o755); err != nil {
		t.Fatalf("mkdir Applications: %v", err)
	}

	stdout, stderr, err := executeCLIInDir(root, "new", "experiment", "ProbeLab")
	if err != nil {
		t.Fatalf("oct new experiment failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, filepath.Join("Experiments", "ProbeLab"))
	assertNewFilesExist(t, filepath.Join(root, "Experiments", "ProbeLab"), "manifest.oct")

	stdout, stderr, err = executeCLIInDir(root, "new", "library", "SignalTools")
	if err != nil {
		t.Fatalf("oct new library failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, filepath.Join("Libraries", "SignalTools"))
	assertNewFilesExist(t, filepath.Join(root, "Libraries", "SignalTools"), "manifest.oct")

	stdout, stderr, err = executeCLIInDir(root, "new", "wrapper-library", "OpenCV")
	if err != nil {
		t.Fatalf("oct new wrapper-library failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, filepath.Join("Libraries", "OpenCV"))
	assertNewFilesExist(t, filepath.Join(root, "Libraries", "OpenCV"), "manifest.oct")

	stdout, stderr, err = executeCLIInDir(root, "new", "app", "MyApp")
	if err != nil {
		t.Fatalf("oct new app failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, filepath.Join("Applications", "MyApp"))
	assertNewFilesExist(t, filepath.Join(root, "Applications", "MyApp"), "manifest.oct")
}

func TestNewExplicitPathOverridesCollectionDefault(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Libraries"), 0o755); err != nil {
		t.Fatalf("mkdir Libraries: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "Applications"), 0o755); err != nil {
		t.Fatalf("mkdir Applications: %v", err)
	}
	stdout, stderr, err := executeCLIInDir(root, "new", "library", "SignalTools", "Custom/SignalTools")
	if err != nil {
		t.Fatalf("oct new explicit path failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, "Created library package SignalTools at Custom/SignalTools")
	assertNewFilesExist(t, filepath.Join(root, "Custom", "SignalTools"), "manifest.oct")

	stdout, stderr, err = executeCLIInDir(root, "new", "application", "MyApp", "Custom/MyApp")
	if err != nil {
		t.Fatalf("oct new application explicit path failed: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	assertOutputContains(t, stdout, "Created application package MyApp at Custom/MyApp")
	assertNewFilesExist(t, filepath.Join(root, "Custom", "MyApp"), "manifest.oct")
}

func TestNewUsageErrors(t *testing.T) {
	root := t.TempDir()
	cases := [][]string{
		{"new"},
		{"new", "library"},
		{"new", "unknown", "Name"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, err := executeCLIInDir(root, args...)
			if err == nil {
				t.Fatalf("expected usage failure, stdout=%q stderr=%q", stdout, stderr)
			}
			if !strings.Contains(stderr, "usage: oct new <experiment|library|wrapper-library|application|app> <Name>") {
				t.Fatalf("expected usage error, got stderr=%q", stderr)
			}
		})
	}
}

func TestNewInvalidNameError(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, err := executeCLIInDir(root, "new", "library", "signal_tools")
	if err == nil {
		t.Fatalf("expected invalid name failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "invalid package name \"signal_tools\"") {
		t.Fatalf("expected invalid name in stderr, got %q", stderr)
	}
}

func TestNewTargetExistsError(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "SignalTools"), 0o755); err != nil {
		t.Fatalf("mkdir existing target: %v", err)
	}
	stdout, stderr, err := executeCLIInDir(root, "new", "library", "SignalTools")
	if err == nil {
		t.Fatalf("expected target exists failure, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Fatalf("expected already exists error, got %q", stderr)
	}
}

func assertNewFilesExist(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			t.Fatalf("expected file %s, info=%v err=%v", path, info, err)
		}
	}
}

func assertFileContains(t *testing.T, path string, snippets ...string) {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	content := string(bytes)
	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected %s to contain %q, got:\n%s", path, snippet, content)
		}
	}
}

func assertGeneratedPackageTestsPass(t *testing.T, root string, name string) {
	t.Helper()
	packageDir := filepath.Join(root, name)
	for _, execution := range []string{"interpreted", "compiled"} {
		t.Run(name+"_"+execution, func(t *testing.T) {
			stdout, stderr, err := executeCLIInDir(root, "test", packageDir, "--execution", execution)
			if err != nil {
				t.Fatalf("oct test %s failed: err=%v stderr=%q stdout=%q", execution, err, stderr, stdout)
			}
			if !strings.Contains(stdout, "PASS "+name+".") {
				t.Fatalf("expected PASS for %s, got stdout=%q", name, stdout)
			}
		})
	}
}
