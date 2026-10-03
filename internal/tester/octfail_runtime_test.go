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
		actual, err := checkRuntimeOctFail("boom", c.mode, []octFailLane{lane("interpreted", c.interpreted), lane("compiled", c.compiled)})
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

func TestParseOctFailExpectationReadsBothHeaders(t *testing.T) {
	cases := []struct {
		content     string
		expected    string
		runtime     bool
		source      string
		errContains string
	}{
		{"expect error: \"a\"\nbody\n", "a", false, "body\n", ""},
		{"\n\nexpect runtime error: \"b c\"\nbody\n", "b c", true, "body\n", ""},
		{"expect runtime error: \"\"\nbody\n", "", false, "", "non-empty"},
		{"expect warning: \"a\"\nbody\n", "", false, "", "malformed expectation header"},
		{"expect error: \"a\"\nexpect runtime error: \"b\"\n", "", false, "", "multiple expectation headers"},
		{"expect runtime error: \"a\"\nexpect error: \"b\"\n", "", false, "", "multiple expectation headers"},
		{"\n \n", "", false, "", "missing expectation header"},
	}
	for _, c := range cases {
		expected, runtime, source, err := parseOctFailExpectation(c.content)
		if c.errContains != "" {
			if err == nil || !strings.Contains(err.Error(), c.errContains) {
				t.Errorf("%q: err = %v, want one containing %q", c.content, err, c.errContains)
			}
			continue
		}
		if err != nil || expected != c.expected || runtime != c.runtime || source != c.source {
			t.Errorf("%q: got (%q, %v, %q, %v)", c.content, expected, runtime, source, err)
		}
	}
}

// The exported parser serves corpora that only compile. A runtime fixture
// handed to one of them is an error, not a fixture that "failed to fail".
func TestParseOctFailFixtureRefusesRuntimeHeader(t *testing.T) {
	if _, _, err := ParseOctFailFixture("expect runtime error: \"a\"\nbody\n"); err == nil {
		t.Fatalf("a runtime header was accepted as a compile-time expectation")
	}
	if expected, source, err := ParseOctFailFixture("expect error: \"a\"\nbody\n"); err != nil || expected != "a" || source != "body\n" {
		t.Fatalf("compile-time header: got (%q, %q, %v)", expected, source, err)
	}
}

// Two contracts once expected "operator + not defined" and passed on the Go
// compiler's complaint about generated code. That can no longer happen.
func TestCompileTimeOctFailIsNotSatisfiedByAGoBuildFailure(t *testing.T) {
	rejected := errors.New("function Main: operator + not defined for String and Int")
	if actual, err := judgeCompileTimeOctFail("operator + not defined", rejected); err != nil || actual != rejected.Error() {
		t.Errorf("a compiler rejection with the expected text: got (%q, %v)", actual, err)
	}
	if _, err := judgeCompileTimeOctFail("some other text", rejected); err == nil {
		t.Errorf("a compiler rejection without the expected text was accepted")
	}
	if _, err := judgeCompileTimeOctFail("anything", nil); err == nil {
		t.Errorf("a source that compiled was accepted")
	}
	goBuild := fmt.Errorf("%w: exit status 1: invalid operation: operator + not defined on xs (variable of type []int)", build.ErrGeneratedProgramDidNotBuild)
	actual, err := judgeCompileTimeOctFail("operator + not defined", goBuild)
	if err == nil {
		t.Fatalf("a Go build failure satisfied the contract")
	}
	if !strings.Contains(actual, "the generated program did not build") {
		t.Errorf("the report does not say what happened: %q", actual)
	}
}
