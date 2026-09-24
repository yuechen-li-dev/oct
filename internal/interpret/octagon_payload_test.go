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
	for run := 0; run < 100; run++ {
		if err := WriteOctagon(path, outer); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "Outer.Wrapped(Inner.Value(42))\n" {
			t.Fatalf("unexpected Octagon payload on run %d: %q", run+1, data)
		}
	}
}
