//go:build integration

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// [Interpreted("reason")] and [Compiled("reason")] restrict a test to one
// lane. These cases fix what the runner reports in each execution mode. The
// fixtures under testdata/lane_attributes are synthetic; the language contract
// is Language/Testing/LaneAttributes.
func TestLaneAttributesSelectTheLane(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "lane_attributes", "restricted")
	cases := []struct {
		mode string
		want []string
	}{
		{"interpreted", []string{
			"PASS LaneFixture.Both",
			"PASS LaneFixture.OnlyInterpreted",
			"SKIP LaneFixture.OnlyCompiled (lanes.octest): compiled only: fixture: compiled only",
			"SKIP LaneFixture.OnlyCompiledTheory[0] (lanes.octest): compiled only: fixture: compiled theory",
			"SKIP LaneFixture.OnlyCompiledTheory[1] (lanes.octest): compiled only: fixture: compiled theory",
			"Execution summary: compiled: 0 interpreted fallback: 0",
			"Result: 2 passed, 0 failed, 3 skipped",
		}},
		{"compiled", []string{
			"PASS LaneFixture.Both",
			"PASS LaneFixture.OnlyCompiled",
			"PASS LaneFixture.OnlyCompiledTheory[0]",
			"PASS LaneFixture.OnlyCompiledTheory[1]",
			"SKIP LaneFixture.OnlyInterpreted (lanes.octest): interpreted only: fixture: interpreted only",
			"Execution summary: compiled: 4 interpreted fallback: 0",
			"Result: 4 passed, 0 failed, 1 skipped",
		}},
		// In auto every test runs, each in the lane it belongs to. The
		// interpreted-only fact is not counted as a fallback: the compiled
		// lane was never asked to build it.
		{"auto", []string{
			"PASS LaneFixture.Both",
			"PASS LaneFixture.OnlyInterpreted",
			"PASS LaneFixture.OnlyCompiled",
			"PASS LaneFixture.OnlyCompiledTheory[1]",
			"Execution summary: compiled: 4 interpreted fallback: 0",
			"Result: 5 passed, 0 failed, 0 skipped",
		}},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			stdout, stderr, err := executeCLIArgs("test", root, "--execution", c.mode)
			if err != nil {
				t.Fatalf("lane fixture failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
			}
			for _, want := range c.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("missing %q in:\n%s", want, stdout)
				}
			}
			if strings.Contains(stdout, "falling back to interpreted") {
				t.Errorf("a lane-restricted test was reported as a fallback:\n%s", stdout)
			}
		})
	}
}

// A [Compiled] test that the compiled lane cannot build fails. It does not
// fall back to the interpreter the way an unrestricted test does under auto.
func TestCompiledLaneAttributeDoesNotFallBack(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "lane_attributes", "no_fallback")
	stdout, _, err := executeCLIArgs("test", root, "--execution", "auto")
	if err == nil {
		t.Fatalf("a [Compiled] test that cannot be built passed:\n%s", stdout)
	}
	for _, want := range []string{
		"FAIL LaneFixture.CompiledFactThatCannotBeBuilt (compiled_required.octest): compiled execution required:",
		"INFO LaneFixture.UnrestrictedFactFallsBack (unrestricted_falls_back.octest): compiled unsupported, falling back to interpreted:",
		"PASS LaneFixture.UnrestrictedFactFallsBack",
		"Execution summary: compiled: 0 interpreted fallback: 1",
		"Result: 1 passed, 1 failed, 0 skipped",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q in:\n%s", want, stdout)
		}
	}

	stdout, _, err = executeCLIArgs("test", root, "--execution", "interpreted")
	if err != nil {
		t.Fatalf("interpreted run failed: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "SKIP LaneFixture.CompiledFactThatCannotBeBuilt") || !strings.Contains(stdout, "Result: 1 passed, 0 failed, 1 skipped") {
		t.Errorf("the interpreted lane did not skip the [Compiled] test:\n%s", stdout)
	}
}
