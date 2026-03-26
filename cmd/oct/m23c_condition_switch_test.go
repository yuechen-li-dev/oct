package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM23cConditionSwitchExpressions(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantStdout  string
		wantMessage string
	}{
		{
			name: "basic condition switch",
			source: "fn Main() -> Int {\n" +
				"    let x = 7\n" +
				"    return switch {\n" +
				"        case x > 10 => 1\n" +
				"        case x > 5 => 2\n" +
				"        else => 3\n" +
				"    }\n" +
				"}\n",
			wantStdout: "2\n",
		},
		{
			name: "first true case wins",
			source: "fn Main() -> Int {\n" +
				"    return switch {\n" +
				"        case true => 1\n" +
				"        case true => 2\n" +
				"        else => 3\n" +
				"    }\n" +
				"}\n",
			wantStdout: "1\n",
		},
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
			name: "dimension consistent results",
			source: "fn Main() -> Int<m> {\n" +
				"    return switch {\n" +
				"        case true => 1m\n" +
				"        else => 2m\n" +
				"    }\n" +
				"}\n",
			wantStdout: "1m\n",
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
		{
			name: "arithmetic integration",
			source: "fn Main() -> Int {\n" +
				"    let x = 4\n" +
				"    return (switch {\n" +
				"        case x > 0 => 10\n" +
				"        else => 20\n" +
				"    }) + 1\n" +
				"}\n",
			wantStdout: "11\n",
		},
		{
			name: "laziness only selected result arm evaluates",
			source: "fn Main() -> Int {\n" +
				"    return switch {\n" +
				"        case true => 1\n" +
				"        else => 1 / 0\n" +
				"    }\n" +
				"}\n",
			wantStdout: "1\n",
		},
		{
			name: "laziness stops evaluating later conditions",
			source: "fn Main() -> Int {\n" +
				"    return switch {\n" +
				"        case true => 1\n" +
				"        case 1 / 0 == 0 => 2\n" +
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
			if test.wantMessage == "" {
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
				return
			}

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
		})
	}
}

func TestM23cConditionSwitchCrossPackageCoexistence(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Math", "math.oct", strings.Join([]string{
		"package Math",
		"",
		"fn Score(x: Int) -> Int {",
		"    return switch {",
		"        case x > 10 => 1",
		"        case x > 5 => 2",
		"        else => 3",
		"    }",
		"}",
	}, "\n"))
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Math",
		"",
		"fn Main() -> Int {",
		"    return Math.Score(7)",
		"}",
	}, "\n"))

	entry := filepath.Join(root, "Main", "main.oct")
	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if stdout != "2\n" {
		t.Fatalf("expected stdout %q, got %q", "2\\n", stdout)
	}
}
