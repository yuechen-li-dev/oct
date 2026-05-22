package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStringErgonomics(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		wantStdout   string
		wantMessage  string
		wantBuildErr string
	}{
		{
			name: "basic string concatenation",
			source: "fn Main() -> String {\n" +
				"    return \"hello\" + \" world\"\n" +
				"}\n",
			wantStdout:   "hello world\n",
			wantBuildErr: "",
		},
		{
			name: "concatenation in Print",
			source: "fn Main() -> Int {\n" +
				"    Print(\"a\" + \"b\")\n" +
				"    return 0\n" +
				"}\n",
			wantStdout:   "ab\n0\n",
			wantBuildErr: "",
		},
		{
			name: "if expression string branches",
			source: "fn Main() -> String {\n" +
				"    return if true { \"yes\" } else { \"no\" }\n" +
				"}\n",
			wantStdout:   "yes\n",
			wantBuildErr: "",
		},
		{
			name: "switch expression string result",
			source: "fn Main() -> String {\n" +
				"    return switch \"a\" {\n" +
				"        case \"a\" => \"alpha\"\n" +
				"        else => \"other\"\n" +
				"    }\n" +
				"}\n",
			wantStdout:   "alpha\n",
			wantBuildErr: "",
		},
		{
			name: "mixed string and int rejected",
			source: "fn Main() -> String {\n" +
				"    return \"value: \" + 1\n" +
				"}\n",
			wantMessage:  `operator "+" not defined for String and Int`,
			wantBuildErr: `operator "+" not defined for String and Int`,
		},
		{
			name: "mixed int and string rejected",
			source: "fn Main() -> String {\n" +
				"    return 1 + \"x\"\n" +
				"}\n",
			wantMessage:  `operator "+" not defined for Int and String`,
			wantBuildErr: `operator "+" not defined for Int and String`,
		},
		{
			name: "Len supports String",
			source: "fn Main() -> Int {\n" +
				"    return Len(\"abc\")\n" +
				"}\n",
			wantStdout:   "3\n",
			wantBuildErr: "",
		},
		{
			name: "numeric addition preserved",
			source: "fn Main() -> Int {\n" +
				"    return 41 + 1\n" +
				"}\n",
			wantStdout:   "42\n",
			wantBuildErr: "",
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
				if test.wantBuildErr == "" {
					if buildErr != nil {
						t.Fatalf("build failed: %v\nstdout:%s\nstderr:%s", buildErr, buildStdout, buildStderr)
					}
					if buildStderr != "" {
						t.Fatalf("expected empty build stderr, got %q", buildStderr)
					}
					if _, statErr := os.Stat(sourcePath + ".octbin"); statErr != nil {
						t.Fatalf("expected artifact on build success, stat err = %v", statErr)
					}
				} else {
					if buildErr == nil {
						t.Fatalf("expected build failure, got success with stdout %q", buildStdout)
					}
					if buildStdout != "" {
						t.Fatalf("expected empty build stdout, got %q", buildStdout)
					}
					if !strings.Contains(buildStderr, test.wantBuildErr) {
						t.Fatalf("expected build stderr to contain %q, got %q", test.wantBuildErr, buildStderr)
					}
					if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
						t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
					}
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
			wantBuildErr := test.wantBuildErr
			if wantBuildErr == "" {
				wantBuildErr = test.wantMessage
			}
			if !strings.Contains(buildStderr, wantBuildErr) {
				t.Fatalf("expected build stderr to contain %q, got %q", wantBuildErr, buildStderr)
			}
			if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
				t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
			}
		})
	}
}

func TestCrossPackageStringConcat(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Utils", "utils.oct", strings.Join([]string{
		"package Utils",
		"fn Prefix(value: String) -> String {",
		"    return \"value: \" + value",
		"}",
	}, "\n"))
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Utils",
		"",
		"fn Main() -> String {",
		"    return Utils.Prefix(\"ok\")",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if stdout != "value: ok\n" {
		t.Fatalf("expected stdout %q, got %q", "value: ok\\n", stdout)
	}
}
