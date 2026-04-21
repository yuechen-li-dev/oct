package prometheus

import (
	"errors"
	"math"
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
					reactorSymbolProbe:      reactorProbe(func(reactorRuntimeHandle) (reactorCaps, error) { return reactorCaps{Available: false}, nil }),
					reactorSymbolSGEMM: reactorSGEMM(func(reactorRuntimeHandle, int, int, int, []float32, []float32) ([]float32, reactorCallStatus, error) {
						return nil, reactorCallStatus{}, nil
					}),
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

func TestRunSGEMMPrometheusPathCallsNativeSGEMMAndStaysCorrect(t *testing.T) {
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	called := false
	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			"/tmp/reactor.so": {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func() (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
					reactorSymbolDestroy:    reactorDestroy(func(reactorRuntimeHandle) {}),
					reactorSymbolProbe: reactorProbe(func(reactorRuntimeHandle) (reactorCaps, error) {
						return reactorCaps{Available: true, BackendType: 1}, nil
					}),
					reactorSymbolSGEMM: reactorSGEMM(func(_ reactorRuntimeHandle, m, n, k int, a, b []float32) ([]float32, reactorCallStatus, error) {
						called = true
						return cpuSGEMM(m, n, k, a, b), reactorCallStatus{}, nil
					}),
				},
			},
		}}}
	}
	t.Setenv(reactorEnvVar, "/tmp/reactor.so")

	run, err := RunSGEMM(SGEMMRequest{
		Backend: BackendPrometheus,
		Shape:   Shape{M: 3, N: 5, K: 7},
		A:       deterministicMatrix(3, 7),
		B:       deterministicMatrix(7, 5),
	})
	if err != nil {
		t.Fatalf("expected successful prometheus run, got err=%v", err)
	}
	if !called {
		t.Fatalf("expected native sgemm path to be called")
	}
	if run.UsedBackend != BackendPrometheus {
		t.Fatalf("expected prometheus backend, got %s", run.UsedBackend)
	}
	if run.Status.Kind != RunStatusOK || !run.Correctness.Pass {
		t.Fatalf("expected ok+correctness, got status=%s correctness=%#v", run.Status.String(), run.Correctness)
	}
}

func TestRunSGEMMPrometheusNativeSGEMMFailureIsExplicit(t *testing.T) {
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			"/tmp/reactor.so": {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func() (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
					reactorSymbolDestroy:    reactorDestroy(func(reactorRuntimeHandle) {}),
					reactorSymbolProbe: reactorProbe(func(reactorRuntimeHandle) (reactorCaps, error) {
						return reactorCaps{Available: true, BackendType: 1}, nil
					}),
					reactorSymbolSGEMM: reactorSGEMM(func(_ reactorRuntimeHandle, _, _, _ int, _, _ []float32) ([]float32, reactorCallStatus, error) {
						return nil, reactorCallStatus{StageCode: 3, DetailCode: -77}, errors.New("native submit failed")
					}),
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
		t.Fatalf("expected native sgemm failure")
	}
	if run.UsedBackend != BackendPrometheus {
		t.Fatalf("expected prometheus backend on native failure, got %s", run.UsedBackend)
	}
	if run.Status.Kind != RunStatusError || run.Status.ErrorStage != StageSubmit || run.Status.ErrorCode != -77 {
		t.Fatalf("expected explicit submit error, got %s", run.Status.String())
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
	for _, required := range []string{"CPUTimeNs", "VulkanTimeNs", "VulkanEnv"} {
		if !strings.Contains(text, required) {
			t.Fatalf("report missing timing/env field token %q", required)
		}
	}
	if _, err := octagon.Load(out); err != nil {
		t.Fatalf("expected report to be loadable octagon: %v", err)
	}
}

func TestRunSGEMMReportsSoftwareVulkanEnvironment(t *testing.T) {
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			"/tmp/reactor.so": {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func() (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
					reactorSymbolDestroy:    reactorDestroy(func(reactorRuntimeHandle) {}),
					reactorSymbolProbe: reactorProbe(func(reactorRuntimeHandle) (reactorCaps, error) {
						return reactorCaps{Available: true, BackendType: 3}, nil
					}),
					reactorSymbolSGEMM: reactorSGEMM(func(_ reactorRuntimeHandle, m, n, k int, a, b []float32) ([]float32, reactorCallStatus, error) {
						return cpuSGEMM(m, n, k, a, b), reactorCallStatus{}, nil
					}),
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
	if err != nil {
		t.Fatalf("expected successful prometheus run, got err=%v", err)
	}
	if run.VulkanEnv != "software_vulkan_llvmpipe_or_cpu" {
		t.Fatalf("expected software Vulkan environment note, got %q", run.VulkanEnv)
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

func TestCompareAgainstOracleRejectsNaNAndInf(t *testing.T) {
	expected := []float32{1, 2, 3}
	actual := []float32{1, float32(math.NaN()), float32(math.Inf(1))}
	res := compareAgainstOracle(expected, actual)
	if res.Pass {
		t.Fatalf("expected compareAgainstOracle to fail for non-finite values")
	}
	if res.FailingCount != 2 {
		t.Fatalf("expected 2 non-finite failures, got %d", res.FailingCount)
	}
	if res.FirstFailingIndex != 1 {
		t.Fatalf("expected first failing index 1, got %d", res.FirstFailingIndex)
	}
}

func TestRunSGEMMReportingInvariantsNoFalseFallbackOrSuccess(t *testing.T) {
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			"/tmp/reactor.so": {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func() (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
					reactorSymbolDestroy:    reactorDestroy(func(reactorRuntimeHandle) {}),
					reactorSymbolProbe: reactorProbe(func(reactorRuntimeHandle) (reactorCaps, error) {
						return reactorCaps{Available: true, BackendType: 2}, nil
					}),
					reactorSymbolSGEMM: reactorSGEMM(func(_ reactorRuntimeHandle, _, _, _ int, _, _ []float32) ([]float32, reactorCallStatus, error) {
						return nil, reactorCallStatus{StageCode: 3, DetailCode: -501}, errors.New("forced submit failure")
					}),
				},
			},
		}}}
	}
	t.Setenv(reactorEnvVar, "/tmp/reactor.so")

	run, err := RunSGEMM(SGEMMRequest{
		Backend: BackendPrometheus,
		Shape:   Shape{M: 3, N: 5, K: 7},
		A:       deterministicMatrix(3, 7),
		B:       deterministicMatrix(7, 5),
	})
	if err == nil {
		t.Fatalf("expected forced native failure")
	}
	if run.Status.Kind != RunStatusError || run.Status.ErrorStage != StageSubmit || run.Status.ErrorCode != -501 {
		t.Fatalf("expected explicit submit error status, got %s", run.Status.String())
	}
	if run.UsedBackend != BackendPrometheus {
		t.Fatalf("expected used backend to remain prometheus, got %s", run.UsedBackend)
	}
	if run.Status.Kind == RunStatusFallback {
		t.Fatalf("prometheus execution failure must not be reported as fallback")
	}
}
