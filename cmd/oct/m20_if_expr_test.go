package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM20IfExpressions(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantStdout  string
		wantMessage string
	}{
		{
			name: "basic if expression",
			source: "fn Main() -> Int {\n" +
				"    let x = if true { 1 } else { 2 }\n" +
				"    return x\n" +
				"}\n",
			wantStdout: "1\n",
		},
		{
			name: "false branch",
			source: "fn Main() -> Int {\n" +
				"    return if false { 1 } else { 2 }\n" +
				"}\n",
			wantStdout: "2\n",
		},
		{
			name: "arithmetic context",
			source: "fn Main() -> Int {\n" +
				"    return (if true { 1 } else { 2 }) + 3\n" +
				"}\n",
			wantStdout: "4\n",
		},
		{
			name: "bool condition required",
			source: "fn Main() -> Int {\n" +
				"    return if 1 { 1 } else { 2 }\n" +
				"}\n",
			wantMessage: "if expression condition must be Bool",
		},
		{
			name: "missing else rejected",
			source: "fn Main() -> Int {\n" +
				"    let x = if true { 1 }\n" +
				"    return x\n" +
				"}\n",
			wantMessage: "if expression requires else branch",
		},
		{
			name: "dimension consistent branches",
			source: "fn Main() -> Int<m> {\n" +
				"    return if true { 1m } else { 2m }\n" +
				"}\n",
			wantStdout: "1m\n",
		},
		{
			name: "dimension mismatch rejected",
			source: "fn Main() -> Int<m> {\n" +
				"    return if true { 1m } else { 2s }\n" +
				"}\n",
			wantMessage: "if expression branches must have matching types",
		},
		{
			name: "record branches",
			source: "record Point {\n" +
				"    X: Int\n" +
				"    Y: Int\n" +
				"}\n" +
				"fn Main() -> Point {\n" +
				"    return if true {\n" +
				"        Point { X: 1 Y: 2 }\n" +
				"    } else {\n" +
				"        Point { X: 3 Y: 4 }\n" +
				"    }\n" +
				"}\n",
			wantStdout: "Point{X: 1, Y: 2}\n",
		},
		{
			name: "branch type mismatch rejected",
			source: "fn Main() -> Int {\n" +
				"    return if true { 1 } else { 2.0 }\n" +
				"}\n",
			wantMessage: "if expression branches must have matching types",
		},
		{
			name: "non selected branch is not evaluated",
			source: "fn Main() -> Int {\n" +
				"    return if true { 1 } else { 1 / 0 }\n" +
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

func TestM20CrossPackageIfExpression(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Geometry", "geometry.oct", strings.Join([]string{
		"package Geometry",
		"record Point {",
		"    X: Int",
		"    Y: Int",
		"}",
		"fn Make(flag: Bool) -> Point {",
		"    return if flag { Point { X: 1 Y: 2 } } else { Point { X: 3 Y: 4 } }",
		"}",
	}, "\n"))
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Geometry",
		"",
		"fn Main() -> Int {",
		"    let p = Geometry.Make(true)",
		"    return p.X",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if stdout != "1\n" {
		t.Fatalf("expected stdout %q, got %q", "1\\n", stdout)
	}
}
