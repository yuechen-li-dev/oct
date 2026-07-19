package zimage

import (
	"strings"
	"testing"
)

func TestNoiseRefiner0PayloadDiagnosticsNameSetupAndGuide(t *testing.T) {
	tests := []struct {
		name  string
		paths NoiseRefiner0PayloadPaths
		want  string
	}{
		{"cache unset", NoiseRefiner0PayloadPaths{OracleRoot: t.TempDir()}, "OCT_EVT2_CACHE is unset"},
		{"oracle unset", NoiseRefiner0PayloadPaths{CacheRoot: t.TempDir()}, "OCT_EVT2_ORACLE is unset"},
		{"cache absent", NoiseRefiner0PayloadPaths{CacheRoot: "missing-cache", OracleRoot: t.TempDir()}, "OCT_EVT2_CACHE directory absent"},
		{"oracle absent", NoiseRefiner0PayloadPaths{CacheRoot: t.TempDir(), OracleRoot: "missing-oracle"}, "OCT_EVT2_ORACLE directory absent"},
		{"manifest absent", NoiseRefiner0PayloadPaths{CacheRoot: t.TempDir(), OracleRoot: t.TempDir()}, "cache manifest absent"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadNoiseRefiner0PayloadBundle(test.paths)
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), NoiseRefiner0PayloadGuide) {
				t.Fatalf("error = %v, want %q and guide", err, test.want)
			}
		})
	}
}
