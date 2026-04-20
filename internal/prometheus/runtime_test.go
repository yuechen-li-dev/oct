package prometheus

import (
	"errors"
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
	t.Setenv(reactorEnvVar, filepath.Join(t.TempDir(), "missing-reactor.so"))
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

func TestRunSGEMMPrometheusBridgeInitFailureIsExplicit(t *testing.T) {
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			"/tmp/reactor.so": {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func() (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
					reactorSymbolDestroy:    reactorDestroy(func(reactorRuntimeHandle) {}),
					reactorSymbolProbe:      reactorProbe(func(reactorRuntimeHandle) (bool, error) { return false, nil }),
				},
			},
		}}}
	}
	t.Setenv(reactorEnvVar, "/tmp/reactor.so")

	run, err := RunSGEMM(SGEMMRequest{
		Backend: BackendPrometheus,
		Shape:   Shape{M: 4, N: 4, K: 4},
		A:       deterministicMatrix(4, 4),
		B:       deterministicMatrix(4, 4),
	})
	if err == nil {
		t.Fatalf("expected init failure")
	}
	if run.UsedBackend != BackendPrometheus {
		t.Fatalf("expected prometheus backend on explicit init failure, got %s", run.UsedBackend)
	}
	if run.Status.Kind != RunStatusError || run.Status.ErrorStage != StageInit {
		t.Fatalf("expected init error status, got %s", run.Status.String())
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

type fakeLoader struct {
	libraries map[string]fakeLibrarySpec
}

type fakeLibrarySpec struct {
	symbols map[string]any
}

func (l fakeLoader) Open(path string) (dynamicLibrary, error) {
	spec, ok := l.libraries[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &fakeLibrary{symbols: spec.symbols}, nil
}

type fakeLibrary struct {
	symbols map[string]any
}

func (l *fakeLibrary) Resolve(symbol string) (any, error) {
	v, ok := l.symbols[symbol]
	if !ok {
		return nil, errors.New("symbol missing")
	}
	return v, nil
}

func (l *fakeLibrary) Close() error { return nil }
