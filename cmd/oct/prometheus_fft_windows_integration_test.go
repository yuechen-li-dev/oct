//go:build windows && cgo && native

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPrometheusFFTRealGpuContractBothLanes(t *testing.T) {
	requirePrometheusIntegration(t)
	t.Setenv("OCT_PROMETHEUS_REACTOR", windowsReactorDLL(t))
	for _, mode := range []string{"interpreted", "compiled"} {
		t.Run(mode, func(t *testing.T) {
			target := filepath.Join(windowsRepoRoot(t), "Language", "Prometheus", "gpu")
			output, stderr, err := executeCLIArgs("test", target, "--execution", mode)
			if err != nil || !strings.Contains(output, "Result: 3 passed, 0 failed, 0 skipped") {
				t.Fatalf("real GPU contract failed: %v\n%s\n%s", err, output, stderr)
			}
			if !strings.Contains(output, "interpreted fallback: 0") {
				t.Fatalf("unexpected interpreter fallback:\n%s", output)
			}
		})
	}
}
