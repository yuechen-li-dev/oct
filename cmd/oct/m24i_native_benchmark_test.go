package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/cli"
)

func TestOctBenchRunsDiscoveredBenchmarks(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", "package Main\n[Benchmark]\nfn Small() -> Void { let x = 1 + 1 Print(x) }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"bench", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected benchmark success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "RUN  Main.Small") || !strings.Contains(output, "PASS Main.Small") {
		t.Fatalf("expected benchmark run/pass output, got %q", output)
	}
	if !strings.Contains(output, "benchmark(s) passed") {
		t.Fatalf("expected benchmark summary output, got %q", output)
	}
}

func TestOctBenchUsesDeterministicOrdering(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Alpha", "a2.octest", "package Alpha\n[Benchmark]\nfn Beta() -> Void { return }\n")
	writeOctPkgFile(t, root, "Alpha", "a1.octest", "package Alpha\n[Benchmark]\nfn Alpha() -> Void { return }\n")
	writeOctPkgFile(t, root, "Main", "m.octest", "package Main\n[Benchmark]\nfn Omega() -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"bench", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected benchmark success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	output := stdout.String()
	alpha := strings.Index(output, "RUN  Alpha.Alpha")
	beta := strings.Index(output, "RUN  Alpha.Beta")
	omega := strings.Index(output, "RUN  Main.Omega")
	if alpha < 0 || beta < 0 || omega < 0 || !(alpha < beta && beta < omega) {
		t.Fatalf("expected deterministic package/file/function order, got %q", output)
	}
}

func TestOctBenchRejectsInvalidBenchmarkShapes(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		wantErr string
	}{
		{name: "outside octest", source: "package Main\n[Benchmark]\nfn Bad() -> Void { return }\n", wantErr: "[Benchmark] is only valid in .octest files"},
		{name: "parameterized", source: "package Main\n[Benchmark]\nfn Bad(x: Int) -> Void { return }\n", wantErr: "[Benchmark] function must not declare parameters"},
		{name: "non void", source: "package Main\n[Benchmark]\nfn Bad() -> Int { return 1 }\n", wantErr: "[Benchmark] function must return Void"},
		{name: "fact combined", source: "package Main\n[Benchmark]\n[Fact]\nfn Bad() -> Void { return }\n", wantErr: "[Benchmark] cannot be combined with [Fact]"},
		{name: "theory combined", source: "package Main\n[Benchmark]\n[Theory]\n[InlineData(1)]\nfn Bad(x: Int) -> Void { return }\n", wantErr: "[Benchmark] cannot be combined with [Theory]"},
		{name: "artifact combined", source: "package Main\n[Benchmark]\n[Artifact]\nfn Bad() -> Void { return }\n", wantErr: "[Benchmark] cannot be combined with [Artifact]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
			name := "bad.octest"
			if tc.name == "outside octest" {
				name = "bad.oct"
			}
			writeOctPkgFile(t, root, "Main", name, tc.source)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := cli.Execute([]string{"bench", root}, &stdout, &stderr)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got err=%v stderr=%q stdout=%q", tc.wantErr, err, stderr.String(), stdout.String())
			}
		})
	}
}

func TestOctBenchFailurePropagatesAndOtherCommandsUnaffected(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "suite.octest", strings.Join([]string{
		"package Main",
		"[Fact]",
		"fn FactStillRuns() -> Void { Assert.True(true, \"fact pass\") }",
		"[Artifact]",
		"fn ArtifactStillRuns() -> Void { return }",
		"[Benchmark]",
		"fn BenchFails() -> Void { let x = 1 / 0 Print(x) }",
	}, "\n")+"\n")

	var testStdout bytes.Buffer
	var testStderr bytes.Buffer
	if err := cli.Execute([]string{"test", root}, &testStdout, &testStderr); err != nil {
		t.Fatalf("expected oct test success, got err=%v stderr=%q stdout=%q", err, testStderr.String(), testStdout.String())
	}
	if !strings.Contains(testStdout.String(), "PASS Main.FactStillRuns") {
		t.Fatalf("expected fact to run, got %q", testStdout.String())
	}
	if strings.Contains(testStdout.String(), "BenchFails") {
		t.Fatalf("benchmark should not run in oct test, got %q", testStdout.String())
	}

	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if err := cli.Execute([]string{"artifact", root}, &artifactStdout, &artifactStderr); err != nil {
		t.Fatalf("expected oct artifact success, got err=%v stderr=%q stdout=%q", err, artifactStderr.String(), artifactStdout.String())
	}
	if !strings.Contains(artifactStdout.String(), "PASS Main.ArtifactStillRuns") {
		t.Fatalf("expected artifact to run, got %q", artifactStdout.String())
	}
	if strings.Contains(artifactStdout.String(), "BenchFails") {
		t.Fatalf("benchmark should not run in oct artifact, got %q", artifactStdout.String())
	}

	var benchStdout bytes.Buffer
	var benchStderr bytes.Buffer
	err := cli.Execute([]string{"bench", root}, &benchStdout, &benchStderr)
	if err == nil {
		t.Fatalf("expected benchmark command failure")
	}
	if !strings.Contains(benchStdout.String(), "FAIL Main.BenchFails") {
		t.Fatalf("expected benchmark failure output, got %q", benchStdout.String())
	}
	if !strings.Contains(benchStderr.String(), "bench failed: 1 benchmark(s) failed") {
		t.Fatalf("expected benchmark command failure summary, got %q", benchStderr.String())
	}
}

func TestOctBenchRealSignalBenchmarkExample(t *testing.T) {
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "Libraries", "Signal"), filepath.Join(root, "Signal"))
	copyDir(t, filepath.Join("..", "..", "testdata", "m24i", "valid", "Main"), filepath.Join(root, "Main"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"bench", root}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected migrated signal benchmark example to pass, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "PASS Signal.MovingAverageSmall") {
		t.Fatalf("expected real benchmark pass output, got %q", stdout.String())
	}
}

func TestOctBenchCPUProfileEmitsDeterministicArtifact(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", "package Main\n[Benchmark]\nfn Small() -> Void { let x = 1 + 1 Print(x) }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"bench", root, "--profile", "cpu"}, &stdout, &stderr); err != nil {
		t.Fatalf("expected benchmark profiling success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	profilePath := filepath.Join(root, "bench.cpu.pprof")
	info, err := os.Stat(profilePath)
	if err != nil {
		t.Fatalf("expected cpu profile artifact at %s: %v", profilePath, err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty cpu profile artifact at %s", profilePath)
	}
}

func TestOctBenchRejectsInvalidProfileMode(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", "package Main\n[Benchmark]\nfn Small() -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"bench", root, "--profile", "heap"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected invalid profile mode error")
	}
	if !strings.Contains(err.Error(), "bench --profile must be 'cpu'") {
		t.Fatalf("expected invalid profile mode error, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
}

func TestOctBenchWithoutProfileDoesNotEmitCPUArtifact(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", "package Main\n[Benchmark]\nfn Small() -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"bench", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected benchmark success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	if _, err := os.Stat(filepath.Join(root, "bench.cpu.pprof")); !os.IsNotExist(err) {
		t.Fatalf("expected no cpu profile artifact without --profile, stat err=%v", err)
	}
}

func TestOctBenchFilterRunsOnlyMatchingBenchmarks(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", strings.Join([]string{
		"package Main",
		"[Benchmark]",
		"fn HotPathA() -> Void { return }",
		"[Benchmark]",
		"fn ColdPathB() -> Void { return }",
		"[Benchmark]",
		"fn HotPathC() -> Void { return }",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"bench", root, "--filter", "HotPath"}, &stdout, &stderr); err != nil {
		t.Fatalf("expected filtered benchmark success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "RUN  Main.HotPathA") || !strings.Contains(output, "RUN  Main.HotPathC") {
		t.Fatalf("expected matching benchmarks to run, got %q", output)
	}
	if strings.Contains(output, "RUN  Main.ColdPathB") {
		t.Fatalf("expected non-matching benchmark to be skipped, got %q", output)
	}
	if !strings.Contains(output, "Result: 2 benchmark(s) passed, 0 failed") {
		t.Fatalf("expected filtered summary counts, got %q", output)
	}
}

func TestOctBenchFilterNoMatchFailsPredictably(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", "package Main\n[Benchmark]\nfn HotPath() -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"bench", root, "--filter", "NoSuchBench"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected no-match filter to fail")
	}
	if !strings.Contains(err.Error(), "no benchmarks matched filter \"NoSuchBench\"") {
		t.Fatalf("expected no-match filter error, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
}

func TestOctBenchFilterWithCPUProfileStillEmitsArtifact(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", strings.Join([]string{
		"package Main",
		"[Benchmark]",
		"fn HotPath() -> Void { let x = 1 + 1 Print(x) }",
		"[Benchmark]",
		"fn ColdPath() -> Void { let y = 2 + 2 Print(y) }",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"bench", root, "--filter", "HotPath", "--profile", "cpu"}, &stdout, &stderr); err != nil {
		t.Fatalf("expected filtered benchmark profiling success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "RUN  Main.HotPath") || strings.Contains(output, "RUN  Main.ColdPath") {
		t.Fatalf("expected only filtered benchmark to run during profiling, got %q", output)
	}

	profilePath := filepath.Join(root, "bench.cpu.pprof")
	info, err := os.Stat(profilePath)
	if err != nil {
		t.Fatalf("expected cpu profile artifact at %s: %v", profilePath, err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty cpu profile artifact at %s", profilePath)
	}
}
