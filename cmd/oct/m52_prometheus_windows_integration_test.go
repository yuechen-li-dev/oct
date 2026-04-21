//go:build windows && cgo

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"oct/internal/cli"
	"oct/internal/octagon"
)

func windowsRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func windowsReactorDLL(t *testing.T) string {
	t.Helper()
	root := windowsRepoRoot(t)
	candidates := []string{
		filepath.Join(root, "internal", "prometheus", "reactor", "prometheus_reactor.dll"),
		filepath.Join(root, "out", "prometheus", "native", "prometheus_reactor.dll"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Skipf("reactor DLL not present in expected repo-local paths: %v", candidates)
	return ""
}

func TestWindowsBenchM0UsesPrometheusBackendAndWritesOctagon(t *testing.T) {
	root := windowsRepoRoot(t)
	experiment := filepath.Join(root, "Experiments", "PrometheusBenchmarkHarness", "M0")
	reactor := windowsReactorDLL(t)
	out := filepath.Join(t.TempDir(), "m0-bench.octagon")
	t.Setenv("OCT_PROMETHEUS_REACTOR", reactor)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"bench", experiment, "--octagon-out", out}, &stdout, &stderr); err != nil {
		t.Fatalf("expected benchmark success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	text := stdout.String()
	if !strings.Contains(text, "PASS Main.MatrixMulCPUReference_M001_N001_K001") || !strings.Contains(text, "PASS Main.MatrixMulPrometheus_M128_N128_K128") {
		t.Fatalf("expected expanded CPU and Prometheus benchmark passes, got %q", text)
	}
	if !strings.Contains(text, "backend_requested=prometheus backend_used=prometheus status=ok") {
		t.Fatalf("expected truthful prometheus success line, got %q", text)
	}
	if !strings.Contains(text, "vulkan_env=windows_native_vulkan") {
		t.Fatalf("expected windows native Vulkan environment, got %q", text)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read octagon report: %v", err)
	}
	octText := string(body)
	if !strings.Contains(octText, "BackendUsed: \"prometheus\"") || !strings.Contains(octText, "Environment: \"windows_native_vulkan\"") {
		t.Fatalf("expected octagon report to preserve prometheus backend/environment, got %q", octText)
	}
	if _, err := octagon.Load(out); err != nil {
		t.Fatalf("expected loadable .octagon report, got %v", err)
	}
}
