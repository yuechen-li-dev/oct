package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func TestExecutionModeOctestExecutesInterpreterOnlySurface(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "m83a_execution_mode.octest", strings.Join([]string{
		"package Main",
		"",
		"fn ParsePositive(x: Int) -> Int ! Error {",
		"    if x > 0 {",
		"        return x",
		"    }",
		"    return error(\"x must be positive\")",
		"}",
		"",
		"[Fact]",
		"fn InterpreterPathSupportsHelpers() -> Void {",
		"    let sum = 0",
		"    while sum < 1 {",
		"        let value = Assert.LGTM(ParsePositive(1), \"expected ok result\")",
		"        Assert.Equal(1, value, \"Assert.LGTM returns ok(value)\")",
		"        Assert.Error(ParsePositive(0), \"expected error result\")",
		"        return",
		"    }",
		"}",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"test", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected oct test success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "PASS Main.InterpreterPathSupportsHelpers") {
		t.Fatalf("expected pass output for fact, got %q", stdout.String())
	}
}

func TestExecutionModeCompiledBuildSupportsEquivalentFlow(t *testing.T) {
	sourcePath := writeSourceFile(t, "m83a_compiled_mode_boundary.oct", strings.Join([]string{
		"fn Main() -> Int {",
		"    let n = 0",
		"    while n < 1 {",
		"        return 1",
		"    }",
		"    return 0",
		"}",
	}, "\n")+"\n")

	stdout, stderr, err := executeCLI("build", sourcePath)
	if err != nil {
		t.Fatalf("expected compiled build success, got err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "build succeeded: ") {
		t.Fatalf("expected build success output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
