//go:build windows && cgo

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
	"github.com/yuechen-li-dev/oct/internal/octagon"
)

func TestWindowsBenchRetainedFamiliesUsePrometheusBackend(t *testing.T) {
	requirePrometheusIntegration(t)
	root := windowsRepoRoot(t)
	experiment := filepath.Join(root, "Experiments", "PrometheusSgemmAlgorithmLab", "M4")
	reactor := windowsReactorDLL(t)
	t.Setenv("OCT_PROMETHEUS_REACTOR", reactor)

	cases := []string{
		"PrometheusSgemmAlgorithmLab.M4d_S16x16_SingleCallRep",
		"PrometheusSgemmAlgorithmLab.M4d_S16x16_BlockedRep",
		"PrometheusSgemmAlgorithmLab.M4d_K16x16x512_KDecompositionRep",
	}

	for _, filter := range cases {
		out := filepath.Join(t.TempDir(), strings.ReplaceAll(filter, ".", "_")+".octagon")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if err := cli.Execute([]string{"bench", experiment, "--filter", filter, "--octagon-out", out}, &stdout, &stderr); err != nil {
			t.Fatalf("expected benchmark success for %s, got err=%v stderr=%q stdout=%q", filter, err, stderr.String(), stdout.String())
		}

		text := stdout.String()
		if !strings.Contains(text, "backend_requested=prometheus backend_used=prometheus status=ok") {
			t.Fatalf("expected truthful Prometheus success for %s, got %q", filter, text)
		}
		if !strings.Contains(text, "vulkan_env=windows_native_vulkan") {
			t.Fatalf("expected windows native Vulkan environment for %s, got %q", filter, text)
		}

		body, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("read octagon report for %s: %v", filter, err)
		}
		octText := string(body)
		if !strings.Contains(octText, "BackendUsed: \"prometheus\"") || !strings.Contains(octText, "Environment: \"windows_native_vulkan\"") {
			t.Fatalf("expected octagon report to preserve prometheus backend/environment for %s, got %q", filter, octText)
		}
		if _, err := octagon.Load(out); err != nil {
			t.Fatalf("expected loadable .octagon report for %s, got %v", filter, err)
		}
	}
}
