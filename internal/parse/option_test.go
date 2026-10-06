package parse

import (
	"testing"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// The four spellings of an Option value parse to one shape, which
// ast.AsOptionConstruction reads back. A spelling without a type argument
// carries the slot the typechecker fills.
func TestBuildFileParsesOptionConstructions(t *testing.T) {
	file := parseSource(t, `fn Main() -> Int {
    let a: Option<Int> = Option.None
    let b: Option<Int> = Option.Some(1)
    let c = Option<Float<m>>.None
    let d = Option<Int[]>.Some([1, 2])
    return 0
}`)
	cases := []struct {
		variant   string
		inferred  bool
		payload   string
		arguments int
	}{
		{ast.OptionNoneVariant, true, "", 0},
		{ast.OptionSomeVariant, true, "", 1},
		{ast.OptionNoneVariant, false, "Float", 0},
		{ast.OptionSomeVariant, false, "Int", 1},
	}
	statements := file.Functions[0].Body.Statements
	for index, want := range cases {
		value := statements[index].(ast.LetStmt).Value
		construction, ok := ast.AsOptionConstruction(value)
		if !ok {
			t.Fatalf("statement %d: %#v is not an Option construction", index, value)
		}
		if construction.Variant != want.variant || construction.Payload.Inferred != want.inferred || construction.Payload.Name != want.payload || len(construction.Arguments) != want.arguments {
			t.Fatalf("statement %d: got variant %q inferred %v payload %q with %d argument(s), want %+v", index, construction.Variant, construction.Payload.Inferred, construction.Payload.Name, len(construction.Arguments), want)
		}
		if construction.Resolved == want.inferred {
			t.Fatalf("statement %d: Resolved = %v for a construction whose type argument was written = %v", index, construction.Resolved, !want.inferred)
		}
	}
	if payload := statements[2].(ast.LetStmt).Value.(ast.CallExpr).TypeArguments[0]; !payload.HasUnit {
		t.Fatalf("Option<Float<m>>.None lost the dimension of its type argument: %#v", payload)
	}
	if payload := statements[3].(ast.LetStmt).Value.(ast.CallExpr).TypeArguments[0]; payload.ArrayDepth != 1 {
		t.Fatalf("Option<Int[]>.Some lost the array depth of its type argument: %#v", payload)
	}
}

func TestBuildFileKeepsOptionCaseLabelsAsEnumLabels(t *testing.T) {
	file := parseSource(t, `fn Main(value: Option<Int>) -> Bool {
    return switch value {
        case Option.None => true
        else => false
    }
}`)
	label := file.Functions[0].Body.Statements[0].(ast.ReturnStmt).Value.(ast.SwitchExpr).Cases[0].Match
	access, ok := label.(ast.FieldAccessExpr)
	if !ok || access.Field != ast.OptionNoneVariant {
		t.Fatalf("case label = %#v, want the field access Option.None", label)
	}
}

func TestBuildFileRejectsMalformedOptionSyntax(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { let a: Option<Int> = Option.None(1)\n return 0 }", "`Option.None` takes no payload; write it without parentheses")
	assertParseErrorContains(t, "fn Main() -> Int { let a = Option<Int>.None()\n return 0 }", "`Option.None` takes no payload; write it without parentheses")
	assertParseErrorContains(t, "fn Main() -> Int { let a = Option<Int, Float>.None\n return 0 }", "Option takes one type argument, got 2")
	assertParseErrorContains(t, "record Option { Value: Int }", "`Option` is the builtin type `Option<T>`; a record cannot be named Option")
	assertParseErrorContains(t, "enum Option { None }", "`Option` is the builtin type `Option<T>`; a enum cannot be named Option")
	assertParseErrorContains(t, "fn Option() -> Int { return 0 }", "`Option` is the builtin type `Option<T>`; a function cannot be named Option")
	assertParseErrorContains(t, "concept Option = Int", "`Option` is the builtin type `Option<T>`; a concept cannot be named Option")
	assertParseErrorContains(t, "flow Option() -> Int { state Run { return 0 } }", "`Option` is the builtin type `Option<T>`; a flow cannot be named Option")
	assertParseErrorContains(t, "package Option\nfn Main() -> Int { return 0 }", "`Option` is the builtin type `Option<T>`; a package cannot be named Option")
}

// `Option < limit` is still a comparison: only a type argument that a variant
// follows opens an Option construction.
func TestBuildFileLeavesComparisonsWithAnOptionNamedOperandAlone(t *testing.T) {
	file := parseSource(t, `fn Main(Option: Int, limit: Int) -> Bool {
    return Option < limit
}`)
	value := file.Functions[0].Body.Statements[0].(ast.ReturnStmt).Value
	if binary, ok := value.(ast.BinaryExpr); !ok || binary.Operator != "<" {
		t.Fatalf("`Option < limit` parsed as %#v, want a comparison", value)
	}
}

// `Option` is resolved by scope, as `vector` is: where a parameter or a local
// has the name, `Option.Field` reads it.
func TestBuildFileResolvesOptionByScope(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		constructed bool
	}{
		{"nothing bound", "fn F() -> Option<Int> { return Option.None }", true},
		{"parameter", "fn F(Option: Holder) -> Int { return Option.None }", false},
		{"let, after it", "fn F() -> Int { let Option = Make() return Option.None }", false},
		{"binding ended with its block", "fn F(flag: Bool) -> Option<Int> { if flag { let Option = Make() } return Option.None }", true},
		{"another function's parameter", "fn A(Option: Holder) -> Int { return 0 }\nfn F() -> Option<Int> { return Option.None }", true},
		{"written type argument while bound", "fn F(Option: Int) -> Bool { return Option < 3 }", false},
	}
	for _, c := range cases {
		file := parseSource(t, c.source)
		function := file.Functions[len(file.Functions)-1]
		last := function.Body.Statements[len(function.Body.Statements)-1].(ast.ReturnStmt).Value
		if _, constructed := ast.AsOptionConstruction(last); constructed != c.constructed {
			t.Errorf("%s: `%s` parsed as a construction = %v, want %v", c.name, c.source, constructed, c.constructed)
		}
	}
}
