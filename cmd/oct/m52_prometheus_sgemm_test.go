package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/cli"
	"oct/internal/octagon"
)

func TestM52PrometheusSgemmCPUScenarioEmitsOctagonReport(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sgemm-report.octagon")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"prometheus-sgemm", "cpu", "--octagon-out", out}, &stdout, &stderr); err != nil {
		t.Fatalf("prometheus-sgemm cpu failed: err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "backend_requested=cpu backend_used=cpu status=ok correctness=true") {
		t.Fatalf("expected explicit cpu status line, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "cpu=") || !strings.Contains(stdout.String(), "vulkan=") || !strings.Contains(stdout.String(), "vulkan_env=") {
		t.Fatalf("expected explicit timing/env output, got %q", stdout.String())
	}
	if _, err := octagon.Load(out); err != nil {
		t.Fatalf("expected loadable .octagon report, got %v", err)
	}
}

func TestM52PrometheusSgemmPrometheusUnavailableStatusVisible(t *testing.T) {
	t.Setenv("OCT_PROMETHEUS_REACTOR", filepath.Join(t.TempDir(), "missing-reactor"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"prometheus-sgemm", "prometheus"}, &stdout, &stderr); err != nil {
		t.Fatalf("expected fallback success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "status=fallback(prometheus_unavailable)") {
		t.Fatalf("expected explicit fallback status, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "backend_requested=prometheus backend_used=cpu") {
		t.Fatalf("expected explicit backend distinction, got %q", stdout.String())
	}
}

func TestM52PrometheusSgemmPrometheusDefaultsToUnavailableUntilRuntimeReady(t *testing.T) {
	t.Setenv("OCT_PROMETHEUS_REACTOR", filepath.Join(t.TempDir(), "missing-reactor"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"prometheus-sgemm", "prometheus"}, &stdout, &stderr); err != nil {
		t.Fatalf("expected fallback success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "status=fallback(prometheus_unavailable)") {
		t.Fatalf("expected explicit fallback status, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "backend_requested=prometheus backend_used=cpu") {
		t.Fatalf("expected explicit backend distinction, got %q", stdout.String())
	}
}
