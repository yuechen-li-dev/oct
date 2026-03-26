package main

import (
	"os"
	"strings"
	"testing"
)

func TestM23cConditionSwitchInvalidCasesStayHostOwned(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{
			name: "else required",
			source: "fn Main() -> Int {\n" +
				"    return switch {\n" +
				"        case true => 1\n" +
				"    }\n" +
				"}\n",
			wantMessage: "condition switch requires else arm",
		},
		{
			name: "bool only case conditions",
			source: "fn Main() -> Int {\n" +
				"    return switch {\n" +
				"        case 1 => 1\n" +
				"        else => 2\n" +
				"    }\n" +
				"}\n",
			wantMessage: "condition switch case must be Bool",
		},
		{
			name: "exact result type matching",
			source: "fn Main() -> Int {\n" +
				"    return switch {\n" +
				"        case true => 1\n" +
				"        else => 2.0\n" +
				"    }\n" +
				"}\n",
			wantMessage: "condition switch result arms must have matching types",
		},
		{
			name: "dimension mismatch rejected",
			source: "fn Main() -> Int<m> {\n" +
				"    return switch {\n" +
				"        case true => 1m\n" +
				"        else => 2s\n" +
				"    }\n" +
				"}\n",
			wantMessage: "condition switch result arms must have matching dimensions",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("build", sourcePath)
			if err == nil {
				t.Fatalf("expected build failure, got success with stdout %q", stdout)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("expected build stderr to contain %q, got %q", test.wantMessage, stderr)
			}
			if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
				t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
			}
		})
	}
}
