package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM23eDecisionLadderShapePolicy(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantStdout  string
		wantMessage string
	}{
		{
			name: "rejects decision ladder",
			source: "fn Main() -> Int {\n" +
				"    if true {\n" +
				"        return 1\n" +
				"    } else {\n" +
				"        if false {\n" +
				"            return 2\n" +
				"        } else {\n" +
				"            return 3\n" +
				"        }\n" +
				"    }\n" +
				"}\n",
			wantMessage: "nested decision-ladder `if/else` is not allowed in Oct by design. Use a condition-switch expression instead:",
		},
		{
			name: "allows nested if in then branch",
			source: "fn Main() -> Int {\n" +
				"    if true {\n" +
				"        if true {\n" +
				"            return 1\n" +
				"        }\n" +
				"        return 2\n" +
				"    }\n" +
				"    return 3\n" +
				"}\n",
			wantStdout: "1\n",
		},
		{
			name: "allows nested control in while",
			source: "fn Main() -> Int {\n" +
				"    var x = 0\n" +
				"    while x < 1 {\n" +
				"        if true {\n" +
				"            if false {\n" +
				"                return 2\n" +
				"            }\n" +
				"            x = 1\n" +
				"        }\n" +
				"    }\n" +
				"    return x\n" +
				"}\n",
			wantStdout: "1\n",
		},
		{
			name: "condition switch replacement succeeds",
			source: "fn Main() -> Int {\n" +
				"    return switch {\n" +
				"        case true => 1\n" +
				"        case false => 2\n" +
				"        else => 3\n" +
				"    }\n" +
				"}\n",
			wantStdout: "1\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)

			if test.wantMessage != "" {
				if err == nil {
					t.Fatalf("expected run failure, got success with stdout %q", stdout)
				}
				if !strings.Contains(stderr, test.wantMessage) {
					t.Fatalf("expected run stderr to contain %q, got %q", test.wantMessage, stderr)
				}
				buildStdout, buildStderr, buildErr := executeCLI("build", sourcePath)
				if buildErr == nil {
					t.Fatalf("expected build failure, got success with stdout %q", buildStdout)
				}
				if !strings.Contains(buildStderr, test.wantMessage) {
					t.Fatalf("expected build stderr to contain %q, got %q", test.wantMessage, buildStderr)
				}
				if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
					t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("run failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if stdout != test.wantStdout {
				t.Fatalf("expected stdout %q, got %q", test.wantStdout, stdout)
			}

			buildStdout, buildStderr, buildErr := executeCLI("build", sourcePath)
			if buildErr != nil {
				t.Fatalf("build failed: %v\nstdout:%s\nstderr:%s", buildErr, buildStdout, buildStderr)
			}
			if buildStderr != "" {
				t.Fatalf("expected empty build stderr, got %q", buildStderr)
			}
			if _, statErr := os.Stat(sourcePath + ".octbin"); statErr != nil {
				t.Fatalf("expected artifact on build success, stat err = %v", statErr)
			}
		})
	}
}

func TestM23eProofSignalUsesConditionSwitch(t *testing.T) {
	signalPath := filepath.Join("..", "..", "Packages", "Signal", "signal.oct")
	bytes, err := os.ReadFile(signalPath)
	if err != nil {
		t.Fatalf("read signal proof package: %v", err)
	}
	src := string(bytes)
	if !strings.Contains(src, "let shape = switch {") {
		t.Fatalf("expected signal proof package to use condition-switch shape selection")
	}
	if strings.Contains(src, "} else {\n            if") {
		t.Fatalf("expected signal proof package to avoid nested decision-ladder if/else")
	}
}
