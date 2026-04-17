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
