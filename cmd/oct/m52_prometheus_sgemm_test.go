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
	if _, err := octagon.Load(out); err != nil {
		t.Fatalf("expected loadable .octagon report, got %v", err)
	}
}

func TestM52PrometheusSgemmPrometheusUnavailableStatusVisible(t *testing.T) {
	t.Setenv("PROMETHEUS_FORCE_UNAVAILABLE", "1")
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
	t.Setenv("PROMETHEUS_STUB_RUNTIME_READY", "")
	t.Setenv("PROMETHEUS_FORCE_UNAVAILABLE", "")
	t.Setenv("PROMETHEUS_FORCE_SUBMIT_FAILURE", "")
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

func TestM52PrometheusSgemmPrometheusSubmitFailureNoHiddenFallback(t *testing.T) {
	t.Setenv("PROMETHEUS_FORCE_SUBMIT_FAILURE", "1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"prometheus-sgemm", "prometheus"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected explicit prometheus submit failure")
	}
	if strings.Contains(stdout.String(), "backend_used=cpu") {
		t.Fatalf("expected no cpu fallback after dispatch failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "prometheus-sgemm failed") {
		t.Fatalf("expected surfaced command error, got %q", stderr.String())
	}
}
