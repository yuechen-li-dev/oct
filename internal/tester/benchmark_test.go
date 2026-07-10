package tester

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
)

func TestBenchmarkMetadataFromOutputDefaultsToCPUPath(t *testing.T) {
	metadata := benchmarkMetadataFromOutput("[[19 22] [43 50]]\n0\n")
	if metadata.BackendRequested != "cpu" || metadata.BackendUsed != "cpu" {
		t.Fatalf("expected cpu defaults, got requested=%q used=%q", metadata.BackendRequested, metadata.BackendUsed)
	}
	if metadata.Status != "ok" {
		t.Fatalf("expected ok status, got %q", metadata.Status)
	}
	if metadata.Environment != "not_applicable" {
		t.Fatalf("expected not_applicable environment, got %q", metadata.Environment)
	}
	if !metadata.Correctness {
		t.Fatalf("expected default correctness true")
	}
	if metadata.DetailCode != 0 || metadata.DetailName != "cpu" {
		t.Fatalf("expected default cpu detail, got code=%d name=%q", metadata.DetailCode, metadata.DetailName)
	}
	if metadata.ReportedWallNs != 0 {
		t.Fatalf("expected zero reported wall for cpu default, got %d", metadata.ReportedWallNs)
	}
}

func TestBenchmarkMetadataFromOutputCapturesPrometheusTruth(t *testing.T) {
	output := "backend_requested=prometheus backend_used=cpu status=fallback(prometheus_unavailable) correctness=true detail_code=6104 detail_name=fallback_to_direct vulkan_env=vulkan_wsl_dzn wall=0ns\n"
	metadata := benchmarkMetadataFromOutput(output)
	if metadata.BackendRequested != "prometheus" {
		t.Fatalf("expected prometheus requested backend, got %q", metadata.BackendRequested)
	}
	if metadata.BackendUsed != "cpu" {
		t.Fatalf("expected cpu used backend, got %q", metadata.BackendUsed)
	}
	if metadata.Status != "fallback(prometheus_unavailable)" {
		t.Fatalf("expected explicit fallback status, got %q", metadata.Status)
	}
	if metadata.Environment != "vulkan_wsl_dzn" {
		t.Fatalf("expected vulkan_wsl_dzn environment, got %q", metadata.Environment)
	}
	if metadata.DetailCode != 6104 || metadata.DetailName != "fallback_to_direct" {
		t.Fatalf("expected parsed detail info, got code=%d name=%q", metadata.DetailCode, metadata.DetailName)
	}
	if metadata.ReportedWallNs != 0 {
		t.Fatalf("expected parsed wall of 0ns, got %d", metadata.ReportedWallNs)
	}
}

func TestBenchmarkRunnerCleanupAfterCompileFailure(t *testing.T) {
	root := t.TempDir()
	artifactRoot := t.TempDir()
	t.Setenv(testArtifactRootEnv, artifactRoot)
	benchPath := filepath.Join(root, "bench.octest")
	if err := os.WriteFile(benchPath, []byte("package Main\n[Benchmark]\nfn Bad() -> Void { Missing() }\n"), 0o644); err != nil {
		t.Fatalf("write benchmark fixture: %v", err)
	}
	program, err := project.LoadForTest(benchPath)
	if err != nil {
		t.Fatalf("load benchmark fixture: %v", err)
	}
	_, _, err = executeBenchmarkCompiled(program, benchmarkCase{pkg: "Main", filePath: benchPath, name: "Bad"}, nil)
	if err == nil {
		t.Fatalf("expected benchmark compile failure")
	}
	entries, readErr := os.ReadDir(artifactRoot)
	if readErr != nil {
		t.Fatalf("read artifact root: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected owned benchmark scope cleanup after compile failure, found %v", entries)
	}
}
