package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestM11ValidProgramCorpus(t *testing.T) {
	tests := []struct {
		file       string
		wantStdout string
		isPlot     bool
	}{
		{file: "scalar_compute.oct", wantStdout: "42\n"},
		{file: "arrays_indexing.oct", wantStdout: "13\n"},
		{file: "loops_range_sum.oct", wantStdout: "0\n"},
		{file: "fallible_match.oct", wantStdout: "7\n"},
		{file: "si_dimensions.oct", wantStdout: "2.5m/s\n"},
		{file: "plot_line.oct", wantStdout: "0\n", isPlot: true},
		{file: "conditionals_switch.oct", wantStdout: "20\n"},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			source := loadCorpusFile(t, "valid", test.file)
			var expectedPlotPath string
			if test.isPlot {
				expectedPlotPath = filepath.Join(t.TempDir(), "corpus_plot.png")
				source = strings.ReplaceAll(source, "\"__PLOT_PATH__\"", strconv.Quote(expectedPlotPath))
			}

			sourcePath := writeSourceFile(t, test.file, source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err != nil {
				t.Fatalf("run command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
			}
			if stdout != test.wantStdout {
				t.Fatalf("expected stdout %q, got %q", test.wantStdout, stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if test.isPlot {
				info, statErr := os.Stat(expectedPlotPath)
				if statErr != nil {
					t.Fatalf("expected output png %s: %v", expectedPlotPath, statErr)
				}
				if info.Size() == 0 {
					t.Fatalf("expected non-empty output png %s", expectedPlotPath)
				}
			}
		})
	}
}

func TestM11InvalidProgramCorpus(t *testing.T) {
	tests := []struct {
		file        string
		wantMessage string
	}{
		{file: "dimension_mismatch.oct", wantMessage: "cannot add m and s"},
		{file: "invalid_question_usage.oct", wantMessage: "operator '?' requires fallible expression"},
		{file: "invalid_plot_dimensioned_array.oct", wantMessage: "function 'PlotScatter' does not accept dimensioned arrays"},
		{file: "non_bool_if.oct", wantMessage: "if condition must be Bool, got Int"},
		{file: "switch_type_mismatch.oct", wantMessage: "case type Bool does not match subject type Int"},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			source := loadCorpusFile(t, "invalid", test.file)
			sourcePath := writeSourceFile(t, test.file, source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err == nil {
				t.Fatalf("expected failure, got success with stdout %q", stdout)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantMessage, stderr)
			}
		})
	}
}

func loadCorpusFile(t *testing.T, class string, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "m11", class, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read corpus file %s: %v", path, err)
	}
	return string(data)
}
