package tester

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/build"
)

// A runtime .octfail is decided per lane. These cases fix which lanes run in
// each execution mode and what each lane outcome means; the lanes themselves
// are stand-ins, so no Oct program is involved.
func TestCheckRuntimeOctFailDecidesPerLane(t *testing.T) {
	fails := func(message string) func() (string, bool, error) {
		return func() (string, bool, error) { return message, true, nil }
	}
	completes := func() (string, bool, error) { return "", false, nil }
	rejected := func() (string, bool, error) { return "", false, errors.New("type error") }

	cases := []struct {
		name                  string
		mode                  string
		interpreted, compiled func() (string, bool, error)
		wantRan               string
		wantActual            string
	}{
		{"auto, both fail as expected", "auto", fails("runtime error: boom"), fails("runtime error: boom"), "interpreted,compiled", ""},
		{"auto, interpreted completes", "auto", completes, fails("boom"), "interpreted", "interpreted: the program ran to completion"},
		{"auto, compiled completes", "auto", fails("boom"), completes, "interpreted,compiled", "compiled: the program ran to completion"},
		{"auto, compiled fails differently", "auto", fails("boom"), fails("other"), "interpreted,compiled", "compiled: other"},
		{"auto, interpreted fails differently", "auto", fails("other"), fails("boom"), "interpreted", "interpreted: other"},
		{"auto, source rejected", "auto", rejected, rejected, "interpreted", "interpreted: did not compile: type error"},
		{"interpreted only", "interpreted", fails("boom"), completes, "interpreted", ""},
		{"compiled only", "compiled", completes, fails("boom"), "compiled", ""},
		{"compiled only, completes", "compiled", fails("boom"), completes, "compiled", "compiled: the program ran to completion"},
		{"compiled only, source rejected", "compiled", fails("boom"), rejected, "compiled", "compiled: did not compile: type error"},
	}
	for _, c := range cases {
		var ran []string
		lane := func(name string, run func() (string, bool, error)) octFailLane {
			return octFailLane{name: name, run: func() (string, bool, error) {
				ran = append(ran, name)
				return run()
			}}
		}
		actual, err := checkRuntimeOctFail([]string{"boom"}, c.mode, []octFailLane{lane("interpreted", c.interpreted), lane("compiled", c.compiled)})
		if got := strings.Join(ran, ","); got != c.wantRan {
			t.Errorf("%s: lanes run = %q, want %q", c.name, got, c.wantRan)
		}
		if (err == nil) != (c.wantActual == "") {
			t.Errorf("%s: err = %v, want failure %v", c.name, err, c.wantActual != "")
		}
		if actual != c.wantActual {
			t.Errorf("%s: actual = %q, want %q", c.name, actual, c.wantActual)
		}
	}
}

// The exported parser serves corpora that only compile and take one
// expectation. Any other fixture handed to one of them is an error, not a
// fixture that "failed to fail".
func TestParseOctFailFixtureServesOnlyThePlainForm(t *testing.T) {
	for _, content := range []string{
		"expect runtime error: \"a\"\nbody\n",
		"expect artifact error: \"a\"\nbody\n",
		"expect error: \"a\"\nexpect error: \"b\"\nbody\n",
	} {
		if _, _, err := ParseOctFailFixture(content); err == nil {
			t.Errorf("%q was accepted as a plain compile-time expectation", content)
		}
	}
	if expected, source, err := ParseOctFailFixture("expect error: \"a\"\nbody\n"); err != nil || expected != "a" || source != "body\n" {
		t.Fatalf("compile-time header: got (%q, %q, %v)", expected, source, err)
	}
}

// Every expected text must appear in the one failure.
func TestOctFailRequiresEveryExpectedText(t *testing.T) {
	failure := errors.New("instantiating A -> B from b.oct: return is Float<m^2>")
	if actual, err := judgeCompileTimeOctFail([]string{"A -> B", "b.oct", "Float<m^2>"}, failure); err != nil {
		t.Errorf("all texts present: got (%q, %v)", actual, err)
	}
	if _, err := judgeCompileTimeOctFail([]string{"A -> B", "c.oct"}, failure); err == nil || !strings.Contains(err.Error(), `"c.oct"`) {
		t.Errorf("a missing text was not reported: %v", err)
	}
	fails := func() (string, bool, error) { return "runtime error: boom at 3", true, nil }
	lanes := []octFailLane{{name: "interpreted", run: fails}}
	if actual, err := checkRuntimeOctFail([]string{"boom", "at 3"}, "interpreted", lanes); err != nil {
		t.Errorf("all runtime texts present: got (%q, %v)", actual, err)
	}
	if _, err := checkRuntimeOctFail([]string{"boom", "at 4"}, "interpreted", lanes); err == nil || !strings.Contains(err.Error(), `"at 4"`) {
		t.Errorf("a missing runtime text was not reported: %v", err)
	}
}

// Two contracts once expected "operator + not defined" and passed on the Go
// compiler's complaint about generated code. That can no longer happen.
func TestCompileTimeOctFailIsNotSatisfiedByAGoBuildFailure(t *testing.T) {
	rejected := errors.New("function Main: operator + not defined for String and Int")
	if actual, err := judgeCompileTimeOctFail([]string{"operator + not defined"}, rejected); err != nil || actual != rejected.Error() {
		t.Errorf("a compiler rejection with the expected text: got (%q, %v)", actual, err)
	}
	if _, err := judgeCompileTimeOctFail([]string{"some other text"}, rejected); err == nil {
		t.Errorf("a compiler rejection without the expected text was accepted")
	}
	if _, err := judgeCompileTimeOctFail([]string{"anything"}, nil); err == nil {
		t.Errorf("a source that compiled was accepted")
	}
	goBuild := fmt.Errorf("%w: exit status 1: invalid operation: operator + not defined on xs (variable of type []int)", build.ErrGeneratedProgramDidNotBuild)
	actual, err := judgeCompileTimeOctFail([]string{"operator + not defined"}, goBuild)
	if err == nil {
		t.Fatalf("a Go build failure satisfied the contract")
	}
	if !strings.Contains(actual, "the generated program did not build") {
		t.Errorf("the report does not say what happened: %q", actual)
	}
}

// An artifact fixture passes only when evaluation fails with every expected
// text and leaves nothing in the output root.
func TestJudgeArtifactOctFail(t *testing.T) {
	failure := errors.New("1 artifact(s) failed")
	stdout := "Execution: build-time-interpreted\nFAIL Main.Fails (x.octest): fatal error: stops here\n"
	if actual, err := judgeArtifactOctFail([]string{"FAIL Main.Fails", "stops here"}, stdout, failure, nil); err != nil || !strings.Contains(actual, "fatal error: stops here; 1 artifact(s) failed") {
		t.Errorf("failed evaluation with every text: got (%q, %v)", actual, err)
	}
	if actual, err := judgeArtifactOctFail([]string{"stops here"}, "", nil, nil); err == nil || actual != "artifact evaluation completed" {
		t.Errorf("completed evaluation: got (%q, %v)", actual, err)
	}
	if _, err := judgeArtifactOctFail([]string{"stops here", "elsewhere"}, stdout, failure, nil); err == nil || !strings.Contains(err.Error(), `"elsewhere"`) {
		t.Errorf("a missing text was not reported: %v", err)
	}
	// The header line of the report is not part of the failure.
	if _, err := judgeArtifactOctFail([]string{"build-time-interpreted"}, stdout, failure, nil); err == nil {
		t.Errorf("text from outside the FAIL lines satisfied the contract")
	}
	actual, err := judgeArtifactOctFail([]string{"stops here"}, stdout, failure, []string{"out/a.txt"})
	if err == nil || !strings.Contains(actual, "still published out/a.txt") {
		t.Errorf("published output after a failure: got (%q, %v)", actual, err)
	}
}
