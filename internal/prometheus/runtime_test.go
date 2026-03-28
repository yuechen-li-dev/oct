package prometheus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/octagon"
)

func TestRunSGEMMCPUPathPassesStarterCorrectness(t *testing.T) {
	report, err := RunStarterCorpus(BackendCPU)
	if err != nil {
		t.Fatalf("RunStarterCorpus(cpu) failed: %v", err)
	}
	if len(report.Runs) != len(StarterCorpus) {
		t.Fatalf("expected %d runs, got %d", len(StarterCorpus), len(report.Runs))
	}
	for _, run := range report.Runs {
		if run.RequestedBackend != BackendCPU || run.UsedBackend != BackendCPU {
			t.Fatalf("expected cpu->cpu, got requested=%s used=%s", run.RequestedBackend, run.UsedBackend)
		}
		if run.Status.Kind != RunStatusOK {
			t.Fatalf("expected ok status, got %s", run.Status.String())
		}
		if !run.Correctness.Pass {
			t.Fatalf("expected correctness pass, got %#v", run.Correctness)
		}
	}
}

func TestRunSGEMMPrometheusUnavailableFallsBackExplicitly(t *testing.T) {
	t.Setenv("PROMETHEUS_FORCE_UNAVAILABLE", "1")
	run, err := RunSGEMM(SGEMMRequest{
		Backend: BackendPrometheus,
		Shape:   Shape{M: 4, N: 4, K: 4},
		A:       deterministicMatrix(4, 4),
		B:       deterministicMatrix(4, 4),
	})
	if err != nil {
		t.Fatalf("expected fallback, got err=%v", err)
	}
	if run.Status.Kind != RunStatusFallback || run.Status.FallbackReason != "prometheus_unavailable" {
		t.Fatalf("expected fallback(prometheus_unavailable), got %s", run.Status.String())
	}
	if run.UsedBackend != BackendCPU {
		t.Fatalf("expected cpu fallback backend, got %s", run.UsedBackend)
	}
}

func TestRunSGEMMPrometheusSubmitFailureReturnsExplicitErrorWithoutFallback(t *testing.T) {
	t.Setenv("PROMETHEUS_FORCE_SUBMIT_FAILURE", "1")
	run, err := RunSGEMM(SGEMMRequest{
		Backend: BackendPrometheus,
		Shape:   Shape{M: 4, N: 4, K: 4},
		A:       deterministicMatrix(4, 4),
		B:       deterministicMatrix(4, 4),
	})
	if err == nil {
		t.Fatalf("expected explicit submit failure")
	}
	if run.UsedBackend != BackendPrometheus {
		t.Fatalf("expected used backend prometheus, got %s", run.UsedBackend)
	}
	if run.Status.Kind != RunStatusError || run.Status.ErrorStage != StageSubmit {
		t.Fatalf("expected error(submit,*), got %s", run.Status.String())
	}
}

func TestWriteOctagonReportIncludesRequiredFields(t *testing.T) {
	report, err := RunStarterCorpus(BackendCPU)
	if err != nil {
		t.Fatalf("RunStarterCorpus(cpu) failed: %v", err)
	}
	out := filepath.Join(t.TempDir(), "prometheus-report.octagon")
	if err := WriteOctagonReport(out, report); err != nil {
		t.Fatalf("WriteOctagonReport failed: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	text := string(body)
	for _, required := range []string{"BackendRequested", "BackendUsed", "Status", "CorrectnessPass", "MaxAbsError", "MaxRelError", "WallTimeNs"} {
		if !strings.Contains(text, required) {
			t.Fatalf("report missing field token %q", required)
		}
	}
	if _, err := octagon.Load(out); err != nil {
		t.Fatalf("expected report to be loadable octagon: %v", err)
	}
}
