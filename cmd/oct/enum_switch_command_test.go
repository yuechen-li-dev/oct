package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnumAwareSwitch(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		wantStdout   string
		wantMessage  string
		wantBuildErr string
	}{
		{
			name: "basic integer switch",
			source: "fn Main() -> Int {\n" +
				"    let x = 2\n" +
				"    return switch x {\n" +
				"        case 1 => 10\n" +
				"        case 2 => 20\n" +
				"        else => 0\n" +
				"    }\n" +
				"}\n",
			wantStdout: "20\n",
		},
		{
			name: "ordered condition switch first match wins",
			source: "fn Main() -> Int {\n" +
				"    let v = 7\n" +
				"    return switch {\n" +
				"        case v > 0 => 1\n" +
				"        case v > 5 => 2\n" +
				"        else => 3\n" +
				"    }\n" +
				"}\n",
			wantStdout: "1\n",
		},
		{
			name: "else fallback is used",
			source: "fn Main() -> Int {\n" +
				"    let x = 99\n" +
				"    return switch x {\n" +
				"        case 1 => 10\n" +
				"        case 2 => 20\n" +
				"        else => -1\n" +
				"    }\n" +
				"}\n",
			wantStdout: "-1\n",
		},
		{
			name: "realistic usage chooses shipping tier",
			source: "fn ShippingTier(weight: Int) -> Int {\n" +
				"    return switch {\n" +
				"        case weight < 1 => 1\n" +
				"        case weight < 5 => 2\n" +
				"        else => 3\n" +
				"    }\n" +
				"}\n" +
				"fn Main() -> Int {\n" +
				"    return ShippingTier(3)\n" +
				"}\n",
			wantStdout: "2\n",
		},
		{
			name: "basic enum switch",
			source: "enum Method { Euler RK4 }\n" +
				"fn Main() -> Int {\n" +
				"    let m = Method.Euler\n" +
				"    return switch m {\n" +
				"        case Method.Euler => 1\n" +
				"        case Method.RK4 => 4\n" +
				"    }\n" +
				"}\n",
			wantStdout: "1\n",
		},
		{
			name: "non exhaustive without else",
			source: "enum Method { Euler RK4 }\n" +
				"fn Main() -> Int {\n" +
				"    let m = Method.Euler\n" +
				"    return switch m {\n" +
				"        case Method.Euler => 1\n" +
				"    }\n" +
				"}\n",
			wantMessage:  "non-exhaustive switch over enum 'Method'; missing cases and no else",
			wantBuildErr: "non-exhaustive switch over enum 'Method'; missing cases and no else",
		},
		{
			name: "non exhaustive with else",
			source: "enum Method { Euler RK4 }\n" +
				"fn Main() -> Int {\n" +
				"    let m = Method.Euler\n" +
				"    return switch m {\n" +
				"        case Method.RK4 => 4\n" +
				"        else => 0\n" +
				"    }\n" +
				"}\n",
			wantStdout: "0\n",
		},
		{
			name: "duplicate enum case",
			source: "enum Method { Euler RK4 }\n" +
				"fn Main() -> Int {\n" +
				"    let m = Method.Euler\n" +
				"    return switch m {\n" +
				"        case Method.Euler => 1\n" +
				"        case Method.Euler => 2\n" +
				"        else => 0\n" +
				"    }\n" +
				"}\n",
			wantMessage:  "duplicate case 'Method.Euler'",
			wantBuildErr: "duplicate case 'Method.Euler'",
		},
		{
			name: "unknown variant",
			source: "enum Method { Euler RK4 }\n" +
				"fn Main() -> Int {\n" +
				"    let m = Method.Euler\n" +
				"    return switch m {\n" +
				"        case Method.Unknown => 1\n" +
				"        else => 0\n" +
				"    }\n" +
				"}\n",
			wantMessage:  "enum 'Method' has no variant 'Unknown'",
			wantBuildErr: "enum 'Method' has no variant 'Unknown'",
		},
		{
			name: "wrong enum type",
			source: "enum Method { Euler RK4 }\n" +
				"enum OtherEnum { Value }\n" +
				"fn Main() -> Int {\n" +
				"    let m = Method.Euler\n" +
				"    return switch m {\n" +
				"        case OtherEnum.Value => 1\n" +
				"        else => 0\n" +
				"    }\n" +
				"}\n",
			wantMessage:  "case type OtherEnum does not match subject type Method",
			wantBuildErr: "case type OtherEnum does not match subject type Method",
		},
		{
			name: "mixing enum and literal cases",
			source: "enum Method { Euler RK4 }\n" +
				"fn Main() -> Int {\n" +
				"    let m = Method.Euler\n" +
				"    return switch m {\n" +
				"        case Method.Euler => 1\n" +
				"        case 1 => 2\n" +
				"        else => 0\n" +
				"    }\n" +
				"}\n",
			wantMessage:  "mixing enum and non-enum case labels is not allowed",
			wantBuildErr: "mixing enum and non-enum case labels is not allowed",
		},
		{
			name: "result type mismatch",
			source: "enum Method { Euler RK4 }\n" +
				"fn Main() -> Int {\n" +
				"    let m = Method.Euler\n" +
				"    return switch m {\n" +
				"        case Method.Euler => 1\n" +
				"        case Method.RK4 => 4.0\n" +
				"    }\n" +
				"}\n",
			wantMessage:  "result type Float does not match Int",
			wantBuildErr: "result type Float does not match Int",
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
					artifactOut, artifactErr := exec.Command(sourcePath + ".octbin").CombinedOutput()
					if artifactErr != nil {
						t.Fatalf("run artifact failed: %v (%s)", artifactErr, string(artifactOut))
					}
					if string(artifactOut) != test.wantStdout {
						t.Fatalf("expected artifact stdout %q, got %q", test.wantStdout, string(artifactOut))
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

func TestCrossPackageEnumSwitch(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Physics", "physics.oct", "package Physics\nenum Method { Euler RK4 }\n")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Physics",
		"",
		"fn Main() -> Int {",
		"    let m = Physics.Method.Euler",
		"    return switch m {",
		"        case Physics.Method.Euler => 1",
		"        case Physics.Method.RK4 => 4",
		"    }",
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
