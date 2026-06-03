package main

import (
	"os"
	"strings"
	"testing"
)

func TestGeneratedGoHardeningM23BuildsRepresentativeShapes(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "go keyword local and range loop",
			source: strings.Join([]string{
				"fn Main() -> Int {",
				"    let range = 0",
				"    var out = range",
				"    for i in 0..2 {",
				"        out = out + i",
				"    }",
				"    return out",
				"}",
			}, "\n") + "\n",
		},
		{
			name: "dimensioned fallible result type name",
			source: strings.Join([]string{
				"fn Twice(x: Float<m>) -> Float<m> ! Error {",
				"    return x * 2.0",
				"}",
				"",
				"fn Main() -> Float<m> ! Error {",
				"    return Twice(2m) ?",
				"}",
			}, "\n") + "\n",
		},
		{
			name: "int literal coerces to dimensioned float argument",
			source: strings.Join([]string{
				"fn NeedsMeters(x: Float<m>) -> Float<m> {",
				"    return x",
				"}",
				"",
				"fn Main() -> Float<m> {",
				"    return NeedsMeters(1m)",
				"}",
			}, "\n") + "\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("build", sourcePath)
			if err != nil {
				t.Fatalf("expected build success, got err=%v stdout=%q stderr=%q", err, stdout, stderr)
			}
			if !strings.Contains(stdout, "build succeeded: ") {
				t.Fatalf("expected build success output, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if _, statErr := os.Stat(sourcePath + ".octbin"); statErr != nil {
				t.Fatalf("expected artifact on build success, stat err = %v", statErr)
			}
		})
	}
}
