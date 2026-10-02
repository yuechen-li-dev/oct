//go:build integration

package main

import (
	"strings"
	"testing"
)

// A violated precondition of a Random v2 builtin is a non-recoverable runtime
// error, which an .octest cannot assert and an .octfail (compile-time only)
// cannot express. This runtime-boundary check runs a fixture whose facts each
// violate one precondition and requires both execution lanes to stop every
// fact with the same error text from internal/octrandom.
func TestRandomStreamPreconditionsStopBothLanesWithTheSameError(t *testing.T) {
	fixture := repoPath(t, "testdata", "random_stream_preconditions")
	want := map[string]string{
		"ChildRejectsNegativeIndex":       "random: index must be >= 0",
		"UnitRejectsNegativeIndex":        "random: index must be >= 0",
		"BetweenRejectsNegativeIndex":     "random: index must be >= 0",
		"IntBetweenRejectsNegativeIndex":  "random: index must be >= 0",
		"NormalRejectsNegativeIndex":      "random: index must be >= 0",
		"BetweenRejectsReversedBounds":    "random: Between requires lo <= hi",
		"IntBetweenRejectsReversedBounds": "random: IntBetween requires lo <= hi",
		"NormalRejectsNegativeStddev":     "random: Normal requires stddev >= 0",
	}

	for _, execution := range []string{"interpreted", "compiled"} {
		t.Run(execution, func(t *testing.T) {
			stdout, stderr, err := executeCLIArgs("test", fixture, "--execution", execution)
			if err == nil {
				t.Fatalf("expected the fixture to fail\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
			}
			failures := map[string]string{}
			for _, line := range strings.Split(stdout+"\n"+stderr, "\n") {
				rest, ok := strings.CutPrefix(line, "FAIL RandomStreamPreconditions.")
				if !ok {
					continue
				}
				fact, _, _ := strings.Cut(rest, " ")
				failures[fact] = line
			}
			for fact, message := range want {
				line, failed := failures[fact]
				if !failed {
					t.Errorf("fact %s did not fail\nstdout:\n%s", fact, stdout)
					continue
				}
				if !strings.HasSuffix(line, "runtime error: "+message) {
					t.Errorf("fact %s failed with the wrong error:\n%s\nwant suffix %q", fact, line, "runtime error: "+message)
				}
				if execution == "compiled" && !strings.Contains(line, "compiled test run failed") {
					t.Errorf("fact %s did not run as a compiled test:\n%s", fact, line)
				}
			}
			if len(failures) != len(want) {
				t.Errorf("%d facts failed, want %d\nstdout:\n%s", len(failures), len(want), stdout)
			}
			if strings.Contains(stdout, "PASS ") {
				t.Errorf("a fact that violates a precondition passed\nstdout:\n%s", stdout)
			}
		})
	}
}
