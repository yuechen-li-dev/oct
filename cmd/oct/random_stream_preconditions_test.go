//go:build integration

package main

import (
	"strings"
	"testing"
)

// A violated precondition in Random v2 is a non-recoverable runtime failure,
// which an .octest cannot assert and an .octfail (compile-time only) cannot
// express. This runtime-boundary check runs a fixture whose facts each violate
// one precondition and requires both execution lanes to stop every fact with
// the same text: a runtime error from internal/octrandom for a native
// builtin, and a failed Assert.True for the Oct library layer above it.
func TestRandomStreamPreconditionsStopBothLanesWithTheSameError(t *testing.T) {
	fixture := repoPath(t, "testdata", "random_stream_preconditions")
	const negativeIndex = "runtime error: random: index must be >= 0"
	want := map[string]string{
		"ChildRejectsNegativeIndex":       negativeIndex,
		"UnitRejectsNegativeIndex":        negativeIndex,
		"BetweenRejectsNegativeIndex":     negativeIndex,
		"IntBetweenRejectsNegativeIndex":  negativeIndex,
		"NormalRejectsNegativeIndex":      negativeIndex,
		"BetweenRejectsReversedBounds":    "runtime error: random: Between requires lo <= hi",
		"IntBetweenRejectsReversedBounds": "runtime error: random: IntBetween requires lo <= hi",
		"NormalRejectsNegativeStddev":     "runtime error: random: Normal requires stddev >= 0",

		"ChanceRejectsProbabilityAboveOne": "assertion failed: Chance requires p in [0, 1]",
		"ChanceRejectsNegativeProbability": "assertion failed: Chance requires p in [0, 1]",
		"ExponentialRejectsZeroRate":       "assertion failed: Exponential requires rate > 0",
		"UnitsRejectsNegativeCount":        "assertion failed: Units requires count >= 0",
		"NormalsRejectsNegativeCount":      "assertion failed: Normals requires count >= 0",
		"SpikeRejectsNegativeAmplitude":    "assertion failed: Spike requires amplitude >= 0",
		"SpikeRejectsProbabilityAboveOne":  "assertion failed: Chance requires p in [0, 1]",
		"FlipCoinsRejectsNegativeCount":    "assertion failed: FlipCoins requires count >= 0",
		"RollDieRejectsOneSide":            "assertion failed: RollDie requires sides >= 2",
		"RollDiceRejectsNegativeCount":     "assertion failed: RollDice requires count >= 0",
		"RollDiceRejectsOneSide":           "assertion failed: RollDice requires sides >= 2",
		"RollWithAdvantageRejectsOneSide":  "assertion failed: RollDice requires sides >= 2",
		"FlipCoinRejectsNegativeIndex":     negativeIndex,
		"RollDiceRejectsNegativeIndex":     negativeIndex,
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
				if !strings.HasSuffix(line, ": "+message) {
					t.Errorf("fact %s failed with the wrong error:\n%s\nwant suffix %q", fact, line, message)
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
