//go:build windows && cgo

package prometheus

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func requirePrometheusIntegration(t *testing.T) {
	if os.Getenv("OCT_RUN_PROMETHEUS_INTEGRATION") != "1" {
		t.Skip("set OCT_RUN_PROMETHEUS_INTEGRATION=1 and OCT_PROMETHEUS_REACTOR=<path-to-prometheus_reactor.dll> to run real Prometheus integration tests")
	}
}

func windowsReactorDLLPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	pkgDir := filepath.Dir(file)
	candidates := []string{
		filepath.Join(pkgDir, "reactor", reactorLibraryBasename()),
		filepath.Join(pkgDir, "..", "..", "out", "prometheus", "native", reactorLibraryBasename()),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Skipf("reactor DLL not present in expected repo-local paths: %v", candidates)
	return ""
}

func TestWindowsLoaderCanOpenRealReactorDLL(t *testing.T) {
	requirePrometheusIntegration(t)
	path := windowsReactorDLLPath(t)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("reactor DLL not present at %s: %v", path, err)
	}
	lib, err := defaultReactorLoader().Open(path)
	if err != nil {
		t.Fatalf("open real reactor dll: %v", err)
	}
	defer lib.Close()

	abiAny, err := lib.Resolve(reactorSymbolABIVersion)
	if err != nil {
		t.Fatalf("resolve ABI symbol: %v", err)
	}
	abi, ok := abiAny.(reactorABI)
	if !ok {
		t.Fatalf("ABI symbol has unexpected type %T", abiAny)
	}
	if got := abi(); got != reactorExpectedABIVersion {
		t.Fatalf("unexpected ABI version: got %d want %d", got, reactorExpectedABIVersion)
	}
}

func TestWindowsRunSGEMMUsesRealReactorWhenDLLAvailable(t *testing.T) {
	requirePrometheusIntegration(t)
	path := windowsReactorDLLPath(t)
	t.Setenv(reactorEnvVar, path)

	run, err := RunSGEMM(SGEMMRequest{
		Backend: BackendPrometheus,
		Shape:   Shape{M: 4, N: 4, K: 4},
		A:       deterministicMatrix(4, 4),
		B:       deterministicMatrix(4, 4),
	})
	if err != nil {
		t.Fatalf("expected successful real reactor run, got err=%v status=%s notes=%q", err, run.Status.String(), run.Notes)
	}
	if run.UsedBackend != BackendPrometheus {
		t.Fatalf("expected prometheus backend, got %s", run.UsedBackend)
	}
	if run.Status.Kind != RunStatusOK {
		t.Fatalf("expected ok status, got %s", run.Status.String())
	}
	if !run.Correctness.Pass {
		t.Fatalf("expected correctness pass, got %#v", run.Correctness)
	}
	if run.VulkanEnv != "windows_native_vulkan" {
		t.Fatalf("expected windows_native_vulkan, got %q", run.VulkanEnv)
	}
}

func TestWindowsRepeatedPrometheusRunsStayStable(t *testing.T) {
	requirePrometheusIntegration(t)
	path := windowsReactorDLLPath(t)
	t.Setenv(reactorEnvVar, path)

	for i := 0; i < 3; i++ {
		run, err := RunSGEMM(SGEMMRequest{
			Backend: BackendPrometheus,
			Shape:   Shape{M: 4, N: 4, K: 4},
			A:       deterministicMatrix(4, 4),
			B:       deterministicMatrix(4, 4),
		})
		if err != nil {
			t.Fatalf("run %d: expected successful real reactor run, got err=%v status=%s notes=%q", i, err, run.Status.String(), run.Notes)
		}
		if run.UsedBackend != BackendPrometheus || run.Status.Kind != RunStatusOK {
			t.Fatalf("run %d: expected prometheus ok, got backend=%s status=%s", i, run.UsedBackend, run.Status.String())
		}
	}
}

func TestWindowsUnavailablePathFallsBackTruthfully(t *testing.T) {
	oldFactory := newPrometheusBridge
	t.Cleanup(func() { newPrometheusBridge = oldFactory })

	missing := filepath.Join(t.TempDir(), "missing-"+reactorLibraryBasename())
	newPrometheusBridge = func() *prometheusBridge {
		return &prometheusBridge{loader: fakeLoader{libraries: map[string]fakeLibrarySpec{}}}
	}
	t.Setenv(reactorEnvVar, missing)

	run, err := RunSGEMM(SGEMMRequest{
		Backend: BackendPrometheus,
		Shape:   Shape{M: 4, N: 4, K: 4},
		A:       deterministicMatrix(4, 4),
		B:       deterministicMatrix(4, 4),
	})
	if err != nil {
		t.Fatalf("expected fallback success, got err=%v", err)
	}
	if run.UsedBackend != BackendCPU || run.Status.String() != "fallback(prometheus_unavailable)" {
		t.Fatalf("expected truthful fallback, got backend=%s status=%s", run.UsedBackend, run.Status.String())
	}
	if !strings.Contains(run.Notes, "unavailable") {
		t.Fatalf("expected unavailable note, got %q", run.Notes)
	}
}
