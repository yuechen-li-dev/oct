package prometheus

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"oct/internal/octagon"
)

func fakeReactorPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), reactorLibraryBasename())
}

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
		if run.DetailName != "cpu" {
			t.Fatalf("expected cpu detail name, got %q", run.DetailName)
		}
	}
}

func TestRunSGEMMPrometheusUnavailableFallsBackExplicitly(t *testing.T) {
	t.Setenv(reactorEnvVar, filepath.Join(t.TempDir(), "missing-"+reactorLibraryBasename()))
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
	if run.DetailName != "fallback_cpu" {
		t.Fatalf("expected fallback detail name, got %q", run.DetailName)
	}
}

func TestRunSGEMMPrometheusBridgeInitFailureIsExplicit(t *testing.T) {
	reactorPath := fakeReactorPath(t)
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			reactorPath: {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func(reactorCreateConfig) (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
					reactorSymbolDestroy:    reactorDestroy(func(reactorRuntimeHandle) {}),
					reactorSymbolProbe:      reactorProbe(func(reactorRuntimeHandle) (reactorCaps, error) { return reactorCaps{Available: false}, nil }),
					reactorSymbolSGEMM: reactorSGEMM(func(reactorRuntimeHandle, int, int, int, []float32, []float32) ([]float32, reactorCallStatus, error) {
						return nil, reactorCallStatus{}, nil
					}),
				},
			},
		}}}
	}
	t.Setenv(reactorEnvVar, reactorPath)

	run, err := RunSGEMM(SGEMMRequest{
		Backend: BackendPrometheus,
		Shape:   Shape{M: 4, N: 4, K: 4},
		A:       deterministicMatrix(4, 4),
		B:       deterministicMatrix(4, 4),
	})
	if err != nil {
		t.Fatalf("expected unavailable fallback success, got err=%v", err)
	}
	if run.UsedBackend != BackendCPU {
		t.Fatalf("expected cpu fallback backend on unavailable probe, got %s", run.UsedBackend)
	}
	if run.Status.Kind != RunStatusFallback || run.Status.FallbackReason != "prometheus_unavailable" {
		t.Fatalf("expected fallback(prometheus_unavailable), got %s", run.Status.String())
	}
}

func TestRunSGEMMPrometheusPathCallsNativeSGEMMAndStaysCorrect(t *testing.T) {
	reactorPath := fakeReactorPath(t)
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	called := false
	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			reactorPath: {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func(reactorCreateConfig) (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
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
	t.Setenv(reactorEnvVar, reactorPath)

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
	if run.DetailName != "not_applicable" {
		t.Fatalf("expected default native detail name when stub omits detail, got %q", run.DetailName)
	}
}

func TestRunCompiledMatMulMMUsesPrometheusBackendAndReturnsMatrix(t *testing.T) {
	reactorPath := fakeReactorPath(t)
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	called := false
	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			reactorPath: {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func(reactorCreateConfig) (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
					reactorSymbolDestroy:    reactorDestroy(func(reactorRuntimeHandle) {}),
					reactorSymbolProbe: reactorProbe(func(reactorRuntimeHandle) (reactorCaps, error) {
						return reactorCaps{Available: true, BackendType: 2}, nil
					}),
					reactorSymbolSGEMM: reactorSGEMM(func(_ reactorRuntimeHandle, m, n, k int, a, b []float32) ([]float32, reactorCallStatus, error) {
						called = true
						return cpuSGEMM(m, n, k, a, b), reactorCallStatus{}, nil
					}),
				},
			},
		}}}
	}
	t.Setenv(reactorEnvVar, reactorPath)

	out, run, err := RunCompiledMatMulMM(
		[][]float64{{1, 2}, {3, 4}},
		[][]float64{{5, 6}, {7, 8}},
	)
	if err != nil {
		t.Fatalf("expected successful compiled matmul, got err=%v", err)
	}
	if !called {
		t.Fatalf("expected compiled matmul to call native sgemm")
	}
	if run.UsedBackend != BackendPrometheus || run.Status.Kind != RunStatusOK {
		t.Fatalf("expected prometheus ok result, got backend=%s status=%s", run.UsedBackend, run.Status.String())
	}
	want := [][]float64{{19, 22}, {43, 50}}
	if len(out) != len(want) || len(out[0]) != len(want[0]) {
		t.Fatalf("unexpected output shape: %#v", out)
	}
	for row := range want {
		for col := range want[row] {
			if out[row][col] != want[row][col] {
				t.Fatalf("unexpected output at [%d][%d]: got %v want %v", row, col, out[row][col], want[row][col])
			}
		}
	}
}

func TestRunCompiledMatMulMMUnavailableFallsBackTruthfully(t *testing.T) {
	t.Setenv(reactorEnvVar, filepath.Join(t.TempDir(), "missing-"+reactorLibraryBasename()))

	out, run, err := RunCompiledMatMulMM(
		[][]float64{{1, 2}, {3, 4}},
		[][]float64{{5, 6}, {7, 8}},
	)
	if err != nil {
		t.Fatalf("expected compiled matmul fallback success, got err=%v", err)
	}
	if run.UsedBackend != BackendCPU || run.Status.String() != "fallback(prometheus_unavailable)" {
		t.Fatalf("expected truthful cpu fallback, got backend=%s status=%s", run.UsedBackend, run.Status.String())
	}
	if out[0][0] != 19 || out[1][1] != 50 {
		t.Fatalf("expected cpu fallback output, got %#v", out)
	}
}

func TestRunSGEMMPrometheusNativeSGEMMFailureIsExplicit(t *testing.T) {
	reactorPath := fakeReactorPath(t)
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			reactorPath: {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func(reactorCreateConfig) (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
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
	t.Setenv(reactorEnvVar, reactorPath)

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
	if run.DetailCode != -77 || run.DetailName != "detail_-77" {
		t.Fatalf("expected explicit detail propagation, got code=%d name=%q", run.DetailCode, run.DetailName)
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
	for _, required := range []string{"BackendRequested", "BackendUsed", "Status", "DetailCode", "DetailName", "CorrectnessPass", "MaxAbsError", "MaxRelError", "WallTimeNs"} {
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
	reactorPath := fakeReactorPath(t)
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			reactorPath: {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func(reactorCreateConfig) (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
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
	t.Setenv(reactorEnvVar, reactorPath)

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

func TestRunSGEMMReportsWindowsNativeVulkanEnvironment(t *testing.T) {
	reactorPath := fakeReactorPath(t)
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			reactorPath: {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func(reactorCreateConfig) (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
					reactorSymbolDestroy:    reactorDestroy(func(reactorRuntimeHandle) {}),
					reactorSymbolProbe: reactorProbe(func(reactorRuntimeHandle) (reactorCaps, error) {
						return reactorCaps{Available: true, BackendType: 2}, nil
					}),
					reactorSymbolSGEMM: reactorSGEMM(func(_ reactorRuntimeHandle, m, n, k int, a, b []float32) ([]float32, reactorCallStatus, error) {
						return cpuSGEMM(m, n, k, a, b), reactorCallStatus{}, nil
					}),
				},
			},
		}}}
	}
	t.Setenv(reactorEnvVar, reactorPath)

	run, err := RunSGEMM(SGEMMRequest{
		Backend: BackendPrometheus,
		Shape:   Shape{M: 4, N: 4, K: 4},
		A:       deterministicMatrix(4, 4),
		B:       deterministicMatrix(4, 4),
	})
	if err != nil {
		t.Fatalf("expected successful prometheus run, got err=%v", err)
	}
	want := "hardware_vulkan"
	if runtime.GOOS == "windows" {
		want = "windows_native_vulkan"
	}
	if run.VulkanEnv != want {
		t.Fatalf("expected %q environment note, got %q", want, run.VulkanEnv)
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
	reactorPath := fakeReactorPath(t)
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{
			reactorPath: {
				symbols: map[string]any{
					reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
					reactorSymbolCreate:     reactorCreate(func(reactorCreateConfig) (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
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
	t.Setenv(reactorEnvVar, reactorPath)

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
