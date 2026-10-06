package typecheck

import (
	"testing"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// An `Option.None` or `Option.Some(value)` takes its type argument from the
// site, and the typechecker writes it into the construction: the interpreter
// and the compiled lane read it there instead of working it out again. This
// checks that hand-over, which no `.octest` can observe directly.
func TestCheckWritesTheTypeArgumentOfAnOptionConstruction(t *testing.T) {
	file := parseSource(t, `record Reading { Level: Float<m> }

fn Take(value: Option<Reading[]>) -> Int {
    return 0
}

fn Main() -> Int {
    let a: Option<Float<m>> = Option.Some(2.0m)
    let b: Option<Option<Int>> = Option.Some(Option.None)
    let c = Take(Option.None)
    let d = Option<Bool>.None
    return 0
}`)
	if err := Check(file); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	statements := file.Functions[1].Body.Statements
	payloadOf := func(expr ast.Expr) ast.TypeRef {
		t.Helper()
		construction, ok := ast.AsOptionConstruction(expr)
		if !ok {
			t.Fatalf("%#v is not an Option construction", expr)
		}
		if !construction.Resolved {
			t.Fatalf("the type argument of %#v was not filled in", expr)
		}
		return construction.Payload
	}

	a := payloadOf(statements[0].(ast.LetStmt).Value)
	if a.Name != "Float" || !a.HasUnit || a.Dimension.String() != "m" || !a.Inferred {
		t.Fatalf("Option.Some(2.0m) in an Option<Float<m>> got type argument %#v", a)
	}

	outer := statements[1].(ast.LetStmt).Value
	b := payloadOf(outer)
	if !ast.IsOptionType(b) || b.TypeArguments[0].Name != "Int" {
		t.Fatalf("the outer Option.Some in an Option<Option<Int>> got type argument %#v", b)
	}
	inner := payloadOf(outer.(ast.CallExpr).Arguments[0])
	if inner.Name != "Int" {
		t.Fatalf("the inner Option.None in an Option<Option<Int>> got type argument %#v", inner)
	}

	c := payloadOf(statements[2].(ast.LetStmt).Value.(ast.CallExpr).Arguments[0])
	if c.Name != "Reading" || c.ArrayDepth != 1 || !c.IsArray {
		t.Fatalf("Option.None for a parameter Option<Reading[]> got type argument %#v", c)
	}

	d := payloadOf(statements[3].(ast.LetStmt).Value)
	if d.Name != "Bool" || d.Inferred {
		t.Fatalf("a written type argument was changed: %#v", d)
	}
}

// A second check of the same file finds the slot filled and fills it again
// from the site; it does not mistake the earlier answer for a written type.
func TestCheckResolvesAnOptionConstructionAgainOnASecondPass(t *testing.T) {
	file := parseSource(t, `fn Main() -> Int {
    let a: Option<Float> = Option.Some(1)
    return 0
}`)
	for pass := 1; pass <= 2; pass++ {
		if err := Check(file); err != nil {
			t.Fatalf("pass %d: Check returned error: %v", pass, err)
		}
		construction, _ := ast.AsOptionConstruction(file.Functions[0].Body.Statements[0].(ast.LetStmt).Value)
		if construction.Payload.Name != "Float" || !construction.Payload.Inferred {
			t.Fatalf("pass %d: type argument %#v, want an inferred Float", pass, construction.Payload)
		}
	}
}

// The dots in the name of an option type belong to its payload type; the
// name is not a package-qualified one.
func TestSplitQualifiedTypeNameLeavesOptionNamesWhole(t *testing.T) {
	for _, name := range []string{"Option<Sensors.Sample>", "Option<Option<Sensors.Sample>>", "Option<Int>"} {
		if pkg, local, qualified := splitQualifiedTypeName(name); qualified {
			t.Errorf("splitQualifiedTypeName(%q) = %q, %q; an option name is not qualified", name, pkg, local)
		}
	}
	if pkg, local, qualified := splitQualifiedTypeName("Sensors.Sample"); !qualified || pkg != "Sensors" || local != "Sample" {
		t.Errorf("splitQualifiedTypeName(\"Sensors.Sample\") = %q, %q, %v", pkg, local, qualified)
	}
}
