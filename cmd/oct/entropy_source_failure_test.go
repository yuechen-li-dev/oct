package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octrandom"
)

type failingEntropySource struct{}

func (failingEntropySource) Read([]byte) (int, error) {
	return 0, errors.New("injected source failure")
}

// A failure of the operating system's random source is the only Error an
// Entropy builtin returns, and the real source cannot be made to fail. These
// runtime-boundary checks run a fixture against a source that always fails:
// the failure must reach the program as an ordinary Error that Assert.Error,
// match and ? all see, while a violated precondition stays a runtime error.
//
// The interpreted lane runs in this process, so the test replaces the source
// directly. The compiled lane is in entropy_source_failure_compiled_test.go.
func TestEntropySourceFailureIsAnOrdinaryErrorInterpreted(t *testing.T) {
	restore := octrandom.SetEntropySourceForTest(failingEntropySource{})
	defer restore()
	checkEntropySourceFailure(t, "interpreted")
}

func checkEntropySourceFailure(t *testing.T, execution string) {
	t.Helper()
	fixture := repoPath(t, "testdata", "entropy_source_failure")
	stdout, stderr, err := executeCLIArgs("test", fixture, "--execution", execution)
	if err == nil {
		t.Fatalf("expected the two failing facts to fail the run\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	const prefix = "EntropySourceFailure."
	wantPass := []string{
		"EveryBuiltinReturnsAnError",
		"AnEmptyPayloadNeedsNoEntropy",
		"MatchTakesTheErrArm",
		"TheErrorPropagates",
	}
	wantFail := map[string]string{
		"UnwrapStopsWithTheSourceFailure":   "entropy: the operating system random source failed: injected source failure",
		"APreconditionIsStillARuntimeError": "runtime error: entropy: IntBetween requires lo <= hi",
	}

	passed := map[string]bool{}
	failed := map[string]string{}
	for _, line := range strings.Split(stdout+"\n"+stderr, "\n") {
		if rest, ok := strings.CutPrefix(line, "PASS "+prefix); ok {
			fact, _, _ := strings.Cut(rest, " ")
			passed[fact] = true
		}
		if rest, ok := strings.CutPrefix(line, "FAIL "+prefix); ok {
			fact, _, _ := strings.Cut(rest, " ")
			failed[fact] = line
		}
	}
	for _, fact := range wantPass {
		if !passed[fact] {
			t.Errorf("fact %s did not pass\nstdout:\n%s", fact, stdout)
		}
	}
	for fact, message := range wantFail {
		line, ok := failed[fact]
		if !ok {
			t.Errorf("fact %s did not fail\nstdout:\n%s", fact, stdout)
			continue
		}
		if !strings.HasSuffix(line, ": "+message) {
			t.Errorf("fact %s failed with the wrong error:\n%s\nwant suffix %q", fact, line, message)
		}
		if execution == "compiled" && !strings.Contains(line, "compiled test run failed") {
			t.Errorf("fact %s did not run as a compiled test:\n%s", fact, line)
		}
	}
	if len(passed) != len(wantPass) || len(failed) != len(wantFail) {
		t.Errorf("%d facts passed and %d failed, want %d and %d\nstdout:\n%s", len(passed), len(failed), len(wantPass), len(wantFail), stdout)
	}
}
