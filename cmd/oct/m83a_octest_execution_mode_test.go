package main

import (
	"bytes"
	"strings"
	"testing"

	"oct/internal/cli"
)

func TestM83aOctestExecutesInterpreterOnlySurface(t *testing.T) {
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

func TestM83aCompiledBuildStillUnsupportedForEquivalentFlow(t *testing.T) {
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
	if err == nil {
		t.Fatalf("expected compiled build failure, got success with stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "compiled mode does not yet support while") {
		t.Fatalf("expected explicit compiled boundary diagnostic, got %q", stderr)
	}
}
