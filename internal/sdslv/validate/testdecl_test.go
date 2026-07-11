package validate

import (
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func validatedTestFixture(t *testing.T) []ValidatedTestDecl {
	t.Helper()
	text := "[Fact]\n[TestInputUInt(7u, 11u)]\nfn FactCase() -> void { Assert.True(true); }\n[Theory]\n[InlineData(1u)]\n[InlineData(2u)]\n[WorkgroupSize(32, 1, 1)]\n[DispatchGroups(2, 1, 1)]\nfn Rows(v: u32) -> void { Assert.Equal(v, v); }\n"
	tokens, err := lex.Analyze(source.File{Path: "fixture.sdslvtest", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	tests, diagnostics := ValidatedTests(module)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics: %#v", diagnostics)
	}
	return tests
}

func TestSdslvValidatedTestDeclContainsFactKind(t *testing.T) {
	tests := validatedTestFixture(t)
	if len(tests) != 2 || tests[0].Kind != TestKindFact || tests[0].Function.Name != "FactCase" {
		t.Fatalf("validated tests: %#v", tests)
	}
}

func TestSdslvValidatedTestDeclContainsCanonicalTestInput(t *testing.T) {
	d := validatedTestFixture(t)[0]
	if d.TestInput.Kind != TestInputKindUInt || d.TestInput.ElementCount != 2 || len(d.TestInput.PayloadWords) != 2 || d.TestInput.PayloadWords[1] != 11 {
		t.Fatalf("test input = %#v", d.TestInput)
	}
}
func TestSdslvValidatedTestDeclContainsTheoryRows(t *testing.T) {
	d := validatedTestFixture(t)[1]
	if d.Kind != TestKindTheory || len(d.InlineRows) != 2 || d.InlineRows[1].Values[0].Text != "2u" {
		t.Fatalf("rows: %#v", d.InlineRows)
	}
}
func TestSdslvValidatedTestDeclContainsLaunchMetadata(t *testing.T) {
	d := validatedTestFixture(t)[1]
	if d.Launch.WorkgroupSize != [3]uint32{32, 1, 1} || d.Launch.DispatchGroups != [3]uint32{2, 1, 1} {
		t.Fatalf("launch: %#v", d.Launch)
	}
}
func TestSdslvValidatedTestDeclContainsAssertCallsAndExactSpans(t *testing.T) {
	d := validatedTestFixture(t)[1]
	if len(d.AssertCalls) != 1 || d.AssertCalls[0].Kind != "Assert.Equal" || !d.FunctionSpan.Known() || !d.InlineRows[0].ValueSpans[0].Known() || !d.AssertCalls[0].OperandSpans[0].Known() {
		t.Fatalf("declaration lacks lowering metadata: %#v", d)
	}
}
func TestSdslvStableIdsDeriveFromValidatedTests(t *testing.T) {
	tests := validatedTestFixture(t)
	cases := ValidatedTestCases(tests, "examples/SDSL-V/M29/fixture.sdslvtest")
	if len(cases) != 3 || cases[1].StableID == cases[2].StableID || cases[1].DisplayName != "Rows[0]" {
		t.Fatalf("canonical cases: %#v", cases)
	}
	// Reversing a grouping/projection cannot change an already-derived ID.
	again := ValidatedTestCases(tests, "examples/SDSL-V/M29/fixture.sdslvtest")
	if cases[0].StableID != again[0].StableID || cases[1].StableID != again[1].StableID {
		t.Fatal("stable IDs depend on downstream ordering")
	}
}
