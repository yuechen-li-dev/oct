package tester

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, root string, rel string, contents string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestAssertFallibleHelpersPass(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "helper_pass.octest", `package Main

fn MightFail(x: Int) -> Int ! Error {
    if x > 0 {
        return x + 1
    }
    return error("x must be positive")
}

[Fact]
fn UsesHelpers() -> Void {
    let value = Assert.LGTM(MightFail(4), "should succeed")
    Assert.Equal(5, value, "lgtm should return value")
    Assert.Error(MightFail(0), "should reject non-positive input")
}
`)

	var out bytes.Buffer
	if err := Execute(root, &out); err != nil {
		t.Fatalf("expected pass, got %v (%s)", err, out.String())
	}
}

func TestAssertLGTMFailureIncludesUnderlyingError(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "helper_lgtm_fail.octest", `package Main

fn AlwaysErr() -> Int ! Error {
    return error("boom")
}

[Fact]
fn FailsWithMessage() -> Void {
    let value = Assert.LGTM(AlwaysErr(), "should succeed")
    Assert.Equal(0, value, "unreachable")
}
`)

	var out bytes.Buffer
	err := Execute(root, &out)
	if err == nil {
		t.Fatalf("expected failure, got pass (%s)", out.String())
	}
	log := out.String()
	if !strings.Contains(log, "assertion failed: should succeed") {
		t.Fatalf("expected assertion failure message, got %q", log)
	}
	if !strings.Contains(log, "underlying error: boom") {
		t.Fatalf("expected underlying error text, got %q", log)
	}
}

func TestAssertErrorFailureOnSuccess(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "helper_error_fail.octest", `package Main

fn AlwaysOk() -> Int ! Error {
    return 7
}

[Fact]
fn FailsWhenOk() -> Void {
    Assert.Error(AlwaysOk(), "expected failure")
}
`)

	var out bytes.Buffer
	err := Execute(root, &out)
	if err == nil {
		t.Fatalf("expected failure, got pass (%s)", out.String())
	}
	if !strings.Contains(out.String(), "assertion failed: expected failure") {
		t.Fatalf("expected assertion failed output, got %q", out.String())
	}
}

func TestZeroAssertFactFails(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "zero_fact.octest", `package Main

[Fact]
fn EmptyFact() -> Void {
    let x = 1
}
`)
	var out bytes.Buffer
	err := Execute(root, &out)
	if err == nil {
		t.Fatalf("expected failure for zero-assert fact, got pass (%s)", out.String())
	}
	if !strings.Contains(out.String(), "test completed with zero assertions") {
		t.Fatalf("expected zero-assert failure message, got %q", out.String())
	}
}

func TestAssertedFactPasses(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "asserted_fact.octest", `package Main

[Fact]
fn AssertedFact() -> Void {
    Assert.Equal(1 + 1, 2, "math works")
}
`)
	var out bytes.Buffer
	if err := Execute(root, &out); err != nil {
		t.Fatalf("expected pass, got %v (%s)", err, out.String())
	}
}

func TestZeroAssertTheoryRowFails(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "zero_theory.octest", `package Main

[Theory]
[InlineData(1)]
fn EmptyTheory(x: Int) -> Void {
    let y = x + 1
}
`)
	var out bytes.Buffer
	err := Execute(root, &out)
	if err == nil {
		t.Fatalf("expected failure for zero-assert theory row, got pass (%s)", out.String())
	}
	log := out.String()
	if !strings.Contains(log, "EmptyTheory[0]") || !strings.Contains(log, "test completed with zero assertions") {
		t.Fatalf("expected row-specific zero-assert failure, got %q", log)
	}
}

func TestAssertedTheoryRowsPass(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "asserted_theory.octest", `package Main

[Theory]
[InlineData(1)]
[InlineData(2)]
fn Positive(x: Int) -> Void {
    Assert.True(x > 0, "inline row is positive")
}
`)
	var out bytes.Buffer
	if err := Execute(root, &out); err != nil {
		t.Fatalf("expected pass, got %v (%s)", err, out.String())
	}
}

func TestSkipTestFactReportsSkip(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "skip_fact.octest", `package Main

[Fact]
fn SkippedFact() -> Void {
    SkipTest("known unsupported capability")
}
`)
	var out bytes.Buffer
	if err := Execute(root, &out); err != nil {
		t.Fatalf("expected pass with skip, got %v (%s)", err, out.String())
	}
	log := out.String()
	if !strings.Contains(log, "SKIP Main.SkippedFact") || !strings.Contains(log, "Result: 0 passed, 0 failed, 1 skipped") {
		t.Fatalf("expected skip output and summary, got %q", log)
	}
}

func TestSkipTestIsTerminal(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "skip_terminal.octest", `package Main

[Fact]
fn SkipStopsExecution() -> Void {
    SkipTest("stop here")
    Assert.True(false, "unreachable")
}
`)
	var out bytes.Buffer
	if err := Execute(root, &out); err != nil {
		t.Fatalf("expected skip, got %v (%s)", err, out.String())
	}
	if strings.Contains(out.String(), "FAIL") {
		t.Fatalf("expected terminal skip not failure, got %q", out.String())
	}
}

func TestSkipTestTheoryRowOnlySkipsCurrentRow(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "skip_theory.octest", `package Main

[Theory]
[InlineData(0)]
[InlineData(1)]
fn MaybeSkip(x: Int) -> Void {
    if x == 0 {
        SkipTest("zero case unsupported")
    }
    Assert.True(x > 0, "positive row")
}
`)
	var out bytes.Buffer
	if err := Execute(root, &out); err != nil {
		t.Fatalf("expected mixed pass/skip, got %v (%s)", err, out.String())
	}
	log := out.String()
	if !strings.Contains(log, "SKIP Main.MaybeSkip[0]") || !strings.Contains(log, "PASS Main.MaybeSkip[1]") || !strings.Contains(log, "Result: 1 passed, 0 failed, 1 skipped") {
		t.Fatalf("expected row skip behavior, got %q", log)
	}
}

func TestSkipTestValidationAndLaneRestrictions(t *testing.T) {
	cases := []struct {
		name    string
		rel     string
		source  string
		wantErr string
	}{
		{name: "no args", rel: "bad_no_args.octest", source: "package Main\n[Fact]\nfn Bad() -> Void { SkipTest() }\n", wantErr: "function 'SkipTest' expects 1 arguments, got 0"},
		{name: "empty literal", rel: "bad_empty.octest", source: "package Main\n[Fact]\nfn Bad() -> Void { SkipTest(\"\") }\n", wantErr: "SkipTest reason must be non-empty"},
		{name: "wrong type", rel: "bad_type.octest", source: "package Main\n[Fact]\nfn Bad() -> Void { SkipTest(123) }\n", wantErr: "function 'SkipTest' argument 1 expects String, got Int"},
		{name: "non test file", rel: "bad_prod.oct", source: "package Main\nfn Bad() -> Void { SkipTest(\"nope\") }\n", wantErr: "SkipTest is only available in .octest files"},
		{name: "benchmark lane", rel: "bad_bench.octest", source: "package Main\n[Benchmark]\nfn Bad() -> Void { SkipTest(\"nope\") }\n", wantErr: "SkipTest is only available in [Fact] or [Theory] tests"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, tc.rel, tc.source)
			var out bytes.Buffer
			err := Execute(root, &out)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got err=%v out=%q", tc.wantErr, err, out.String())
			}
		})
	}
}
