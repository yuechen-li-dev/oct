package interpret

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestOctagonPayloadEnumValidation(t *testing.T) {
	intType := ast.TypeRef{Name: "Int"}
	arrayType := ast.TypeRef{Name: "Int", IsArray: true, ArrayDepth: 1}
	engine := interpreter{enums: map[string]ast.EnumDecl{
		"Payload.Choice": {Name: "Choice", Variants: []ast.EnumVariantDecl{
			{Name: "None"}, {Name: "Some", Payload: &intType}, {Name: "Items", Payload: &arrayType},
		}},
	}}
	cases := []struct {
		text      string
		wantError string
		wantKind  ValueKind
	}{
		{"Choice.None", "", ValueEnum},
		{"Choice.Some(42)", "", ValueEnum},
		{"Choice.Items([1, 2])", "", ValueEnum},
		{"Choice.Some()", "requires exactly 1 payload", ""},
		{"Choice.Some(1, 2)", "requires exactly 1 payload", ""},
		{"Choice.Some(\"wrong\")", "payload mismatch", ""},
		{"Choice.None(1)", "does not accept a payload", ""},
		{"Choice.Some", "requires exactly 1 payload", ""},
		{"Choice.Unknown(1)", "has no variant", ""},
		{"Choice.Some(1 + 2)", ".octagon does not allow arithmetic", ""},
		{"Choice.Some(Compute())", ".octagon does not allow function calls", ""},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			lexed, err := lex.Analyze(source.File{Path: "payload.octagon", Text: tc.text})
			if err != nil {
				t.Fatal(err)
			}
			expr, err := parse.BuildDataValue(lexed)
			if err == nil {
				value, materializeErr := engine.materializeOctagonValue("Payload", ast.TypeRef{Name: "Choice"}, expr)
				err = materializeErr
				if err == nil && value.Kind != tc.wantKind {
					t.Fatalf("kind = %s, want %s", value.Kind, tc.wantKind)
				}
			}
			if tc.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
		})
	}
}

func TestWriteOctagonPayloadEnumUsesDataConstructor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.octagon")
	inner := Value{Kind: ValueEnum, Enum: EnumValue{TypeName: "Inner", Variant: "Value", Payload: &Value{Kind: ValueInt, Int: 42}}}
	outer := Value{Kind: ValueEnum, Enum: EnumValue{TypeName: "Outer", Variant: "Wrapped", Payload: &inner}}
	golden, err := os.ReadFile(filepath.Join("..", "..", "Language", "Data", "Octagon", "valid", "payload_enum_nested.octagon"))
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 100; run++ {
		if err := WriteOctagon(path, outer); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != string(golden) {
			t.Fatalf("unexpected Octagon payload on run %d: %q", run+1, data)
		}
	}
}

func TestWriteOctagonColumnarCatalogMatchesConceptGolden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.octagon")
	value := Value{Kind: ValueRecord, Record: RecordValue{
		TypeName: "Catalog", FieldOrder: []string{"ID", "Active"},
		Fields: map[string]Value{
			"ID":     {Kind: ValueArray, Array: []Value{{Kind: ValueInt, Int: 1}, {Kind: ValueInt, Int: 2}, {Kind: ValueInt, Int: 3}}},
			"Active": {Kind: ValueArray, Array: []Value{{Kind: ValueBool, Bool: true}, {Kind: ValueBool, Bool: false}, {Kind: ValueBool, Bool: true}}},
		},
	}}
	golden, err := os.ReadFile(filepath.Join("..", "..", "Language", "Data", "Octagon", "Load", "valid", "concept_catalog.octagon"))
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 100; run++ {
		if err := WriteOctagon(path, value); err != nil {
			t.Fatal(err)
		}
		written, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(written) != string(golden) {
			t.Fatalf("columnar Octagon bytes differ from Concept golden on run %d: %q", run+1, written)
		}
	}
}

func TestWriteOctagonFixedArrayMatchesConceptGolden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "array.octagon")
	value := Value{Kind: ValueArray, Array: []Value{{Kind: ValueInt, Int: 1}, {Kind: ValueInt, Int: 2}, {Kind: ValueInt, Int: 3}}}
	golden, err := os.ReadFile(filepath.Join("..", "..", "Language", "Data", "Octagon", "valid", "array_of_scalars.octagon"))
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 100; run++ {
		if err := WriteOctagon(path, value); err != nil {
			t.Fatal(err)
		}
		written, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(written) != string(golden) {
			t.Fatalf("array Octagon bytes differ from Concept golden on run %d: %q", run+1, written)
		}
	}
}
