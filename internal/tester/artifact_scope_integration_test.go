//go:build integration

package tester

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yuechen-li-dev/oct/internal/build"
	"github.com/yuechen-li-dev/oct/internal/project"
)

func TestCompiledOctestArtifactLifecycle(t *testing.T) {
	artifactRoot := t.TempDir()
	t.Setenv(testArtifactRootEnv, artifactRoot)
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "lifecycle.octest")
	source := `package Main

[Fact]
fn Passes() -> Void {
    Assert.True(true, "pass")
}

[Fact]
fn FailsAtRuntime() -> Void {
    Assert.True(false, "runtime failure")
}

[Fact]
fn TimesOut() -> Void {
    while true {
    }
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write lifecycle fixture: %v", err)
	}
	program, err := project.LoadForTest(sourcePath)
	if err != nil {
		t.Fatalf("load lifecycle fixture: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		err := executeCompiledTestCase(program, lifecycleTestCase(sourcePath, "Passes", time.Second), &bytes.Buffer{})
		if err != nil {
			t.Fatalf("compiled success: %v", err)
		}
		assertNoArtifactScopes(t, artifactRoot)
		assertNoGeneratedRunnerFiles(t, sourceDir)
	})

	t.Run("compile failure", func(t *testing.T) {
		err := executeCompiledTestCase(program, lifecycleTestCase(sourcePath, "MissingFunction", time.Second), &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected compile failure")
		}
		assertNoArtifactScopes(t, artifactRoot)
		assertNoGeneratedRunnerFiles(t, sourceDir)
	})

	t.Run("runtime failure", func(t *testing.T) {
		err := executeCompiledTestCase(program, lifecycleTestCase(sourcePath, "FailsAtRuntime", time.Second), &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "compiled test run failed") {
			t.Fatalf("expected runtime failure, got %v", err)
		}
		assertNoArtifactScopes(t, artifactRoot)
		assertNoGeneratedRunnerFiles(t, sourceDir)
	})

	t.Run("timeout closes Windows process handle", func(t *testing.T) {
		err := executeCompiledTestCase(program, lifecycleTestCase(sourcePath, "TimesOut", time.Millisecond), &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected timeout")
		}
		assertNoArtifactScopes(t, artifactRoot)
		assertNoGeneratedRunnerFiles(t, sourceDir)
	})

	t.Run("debug retention", func(t *testing.T) {
		t.Setenv(keepTestArtifactsEnv, "1")
		var diagnostic bytes.Buffer
		err := executeCompiledTestCase(program, lifecycleTestCase(sourcePath, "Passes", time.Second), &diagnostic)
		if err != nil {
			t.Fatalf("compiled retained run: %v", err)
		}
		if !strings.Contains(diagnostic.String(), "Retained compiled test artifacts:") {
			t.Fatalf("retained directory not reported: %q", diagnostic.String())
		}
		retained, globErr := filepath.Glob(filepath.Join(artifactRoot, "octest-run-*"))
		if globErr != nil || len(retained) != 1 {
			t.Fatalf("expected one retained scope, got %v (err=%v)", retained, globErr)
		}
		matches, globErr := filepath.Glob(filepath.Join(retained[0], "*.exe"))
		if globErr != nil || len(matches) != 1 {
			t.Fatalf("retained native harness missing: %v (err=%v)", matches, globErr)
		}
		if removeErr := os.RemoveAll(retained[0]); removeErr != nil {
			t.Fatalf("remove retained test scope: %v", removeErr)
		}
	})
}

func TestParallelCompiledOctestRunsUseDistinctOwnedScopes(t *testing.T) {
	artifactRoot := t.TempDir()
	t.Setenv(testArtifactRootEnv, artifactRoot)
	sourcePath := filepath.Join(t.TempDir(), "parallel.octest")
	if err := os.WriteFile(sourcePath, []byte("package Main\n[Fact]\nfn Passes() -> Void { Assert.True(true, \"pass\") }\n"), 0o644); err != nil {
		t.Fatalf("write parallel fixture: %v", err)
	}
	program, err := project.LoadForTest(sourcePath)
	if err != nil {
		t.Fatalf("load parallel fixture: %v", err)
	}
	const count = 2
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- executeCompiledTestCase(program, lifecycleTestCase(sourcePath, "Passes", time.Second), &bytes.Buffer{})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("parallel compiled run: %v", err)
		}
	}
	assertNoArtifactScopes(t, artifactRoot)
}

func TestUserRequestedBuildArtifactRemainsPersistent(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "main.oct")
	if err := os.WriteFile(sourcePath, []byte("package Main\nfn Main() -> Int { return 42 }\n"), 0o644); err != nil {
		t.Fatalf("write build source: %v", err)
	}
	result, err := build.Compile(sourcePath)
	if err != nil {
		t.Fatalf("user build: %v", err)
	}
	if result.ArtifactPath != build.OutputPath(sourcePath, build.ArtifactExecutable, build.HostTarget()) {
		t.Fatalf("unexpected persistent build path: %s", result.ArtifactPath)
	}
	if _, err := os.Stat(result.ArtifactPath); err != nil {
		t.Fatalf("user-requested build output was removed: %v", err)
	}
	header := make([]byte, 2)
	file, err := os.Open(result.ArtifactPath)
	if err != nil {
		t.Fatalf("open native build: %v", err)
	}
	_, err = file.Read(header)
	_ = file.Close()
	if err != nil {
		t.Fatalf("read native build header: %v", err)
	}
	if string(header) != "MZ" {
		t.Fatalf("Windows native build header = %q, want PE MZ", header)
	}
}

func TestCompiledSuiteAcrossFilesUsesOneRetainedHarness(t *testing.T) {
	artifactRoot := t.TempDir()
	t.Setenv(testArtifactRootEnv, artifactRoot)
	t.Setenv(keepTestArtifactsEnv, "1")
	root := t.TempDir()
	for name, body := range map[string]string{
		"one.octest": "package Main\n[Suite(\"Numerics\")]\n[Fact]\nfn One() -> Void { Assert.True(true, \"one\") }\n",
		"two.octest": "package Main\n[Suite(\"Numerics\")]\n[Fact]\nfn Two() -> Void { Assert.True(true, \"two\") }\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := ExecuteWithOptions(root, &out, TestOptions{Execution: "compiled"}); err != nil {
		t.Fatalf("compiled suite: %v (%s)", err, out.String())
	}
	retained, err := filepath.Glob(filepath.Join(artifactRoot, "octest-run-*"))
	if err != nil || len(retained) != 1 {
		t.Fatalf("suite should own one harness scope, got %v (err=%v)", retained, err)
	}
	if err := os.RemoveAll(retained[0]); err != nil {
		t.Fatal(err)
	}
}

func TestCompiledHarnessMetricsProveSuiteFileAndTheoryBatching(t *testing.T) {
	artifactRoot := t.TempDir()
	t.Setenv(testArtifactRootEnv, artifactRoot)
	t.Setenv("OCT_TEST_METRICS", "1")
	root := t.TempDir()
	fixtures := map[string]string{
		"suite_one.octest": "package Main\n[Suite(\"Numerics\")]\n[Fact]\nfn One() -> Void { Assert.True(true, \"one\") }\n",
		"suite_two.octest": "package Main\n[Suite(\"Numerics\")]\n[Fact]\nfn Two() -> Void { Assert.True(true, \"two\") }\n",
		"rows.octest":      "package Main\n[Theory]\n[InlineData(1)]\n[InlineData(2)]\n[InlineData(3)]\nfn Rows(value: Int) -> Void { Assert.True(value > 0, \"positive\") }\n",
	}
	for name, body := range fixtures {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := ExecuteWithOptions(root, &out, TestOptions{Execution: "compiled"}); err != nil {
		t.Fatalf("compiled metrics fixture: %v (%s)", err, out.String())
	}
	want := "Test metrics: cases=5 suite_groups=1 file_groups=1 harness_groups=2 native_compilations=2 process_launches=5 single_case_reruns=0 artifact_scopes_created=2 artifact_scopes_cleaned=2"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("metrics mismatch; want %q in:\n%s", want, out.String())
	}
	t.Log(want)
	assertNoArtifactScopes(t, artifactRoot)
}

func lifecycleTestCase(path string, name string, cycle time.Duration) testCase {
	return testCase{pkg: "Main", filePath: path, name: name, displayName: name, cycleTime: cycle}
}

func assertNoArtifactScopes(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read artifact root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("compiled test scopes leaked under %s: %v", root, entries)
	}
}

func assertNoGeneratedRunnerFiles(t *testing.T, sourceDir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(sourceDir, "zz_oct_test_runner_*"))
	if err != nil {
		t.Fatalf("glob source directory: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("generated runner was written into source directory: %v", matches)
	}
}
