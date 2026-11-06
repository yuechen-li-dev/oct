//go:build integration

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// The Oct files own numerical and no-fallback semantics. This test owns CLI
// orchestration and gives each fixture the native environment it requires.
func TestPrometheusFFTCorpusEnvironmentLanes(t *testing.T) {
	t.Setenv("OCT_PROMETHEUS_REACTOR", filepath.Join(t.TempDir(), "missing-reactor"))
	for _, suite := range []struct{ directory, summary string }{
		{"valid", "Result: 3 passed, 0 failed, 0 skipped"},
		{"unavailable", "Result: 2 passed, 0 failed, 0 skipped"},
	} {
		for _, mode := range []string{"interpreted", "compiled"} {
			t.Run(suite.directory+"/"+mode, func(t *testing.T) {
				target := filepath.Join("..", "..", "Language", "Prometheus", suite.directory)
				output, stderr, err := executeCLIArgs("test", target, "--execution", mode)
				if err != nil || !strings.Contains(output, suite.summary) {
					t.Fatalf("contract failed: %v\n%s\n%s", err, output, stderr)
				}
				if mode == "compiled" && !strings.Contains(output, "interpreted fallback: 0") {
					t.Fatalf("compiled corpus unexpectedly fell back:\n%s", output)
				}
			})
		}
	}
}
