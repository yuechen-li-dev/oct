package main

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestScientificNotationScientificNotationRuntime(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   float64
	}{
		{
			name: "basic scientific notation",
			source: "fn Main() -> Float {\n" +
				"    return 2e3\n" +
				"}\n",
			want: 2000.0,
		},
		{
			name: "fractional exponent form",
			source: "fn Main() -> Float {\n" +
				"    return 1.5e2\n" +
				"}\n",
			want: 150.0,
		},
		{
			name: "negative exponent",
			source: "fn Main() -> Float {\n" +
				"    return 1e-3\n" +
				"}\n",
			want: 0.001,
		},
		{
			name: "uppercase E",
			source: "fn Main() -> Float {\n" +
				"    return 6E2\n" +
				"}\n",
			want: 600.0,
		},
		{
			name: "arithmetic integration",
			source: "fn Main() -> Float {\n" +
				"    return 1e2 + 2\n" +
				"}\n",
			want: 102.0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err != nil {
				t.Fatalf("run failed: %v stderr=%s", err, stderr)
			}
			got, parseErr := strconv.ParseFloat(strings.TrimSpace(stdout), 64)
			if parseErr != nil {
				t.Fatalf("expected float output, got %q parseErr=%v", stdout, parseErr)
			}
			if got != test.want {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}

func TestScientificNotationDimensionedScientificNotationBuilds(t *testing.T) {
	sourcePath := writeSourceFile(t, "m23b_dimensioned.oct", "fn Main() -> Float<kg/m/s/s> {\n    return 2.0e11kg/m/s/s\n}\n")

	stdout, stderr, err := executeCLI("build", sourcePath)
	if err != nil {
		t.Fatalf("build failed: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "build succeeded") {
		t.Fatalf("expected build success output, got %q", stdout)
	}
}

func TestScientificNotationMalformedScientificNotationRejected(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "missing exponent digits", source: "fn Main() -> Float {\n    return 2e\n}\n"},
		{name: "missing exponent digits after plus", source: "fn Main() -> Float {\n    return 2e+\n}\n"},
		{name: "missing exponent digits after minus", source: "fn Main() -> Float {\n    return 2e-\n}\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("build", sourcePath)
			if err == nil {
				t.Fatalf("expected build failure, got success with stdout=%q", stdout)
			}
			if !strings.Contains(stderr, "invalid float literal") {
				t.Fatalf("expected invalid float literal in stderr, got %q", stderr)
			}
			if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
				t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
			}
		})
	}
}

func TestScientificNotationIntegerLiteralsUnchanged(t *testing.T) {
	sourcePath := writeSourceFile(t, "m23b_int_regression.oct", "fn Main() -> Int {\n    return 42\n}\n")

	stdout, stderr, err := executeCLI("run", sourcePath)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "42\n" {
		t.Fatalf("expected int output 42, got %q", stdout)
	}
}
