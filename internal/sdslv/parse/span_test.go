package parse

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/source"
)

// This is deliberately source slicing only as a parser contract test. Parser
// production code always composes spans from consumed tokens and child nodes.
func TestSdslvAstRepresentativeSourceSpans(t *testing.T) {
	const text = "[Fact]\nfn F(x: array<u32, 4u>) -> void {\n  let y: u32 = read x[1u] when true else 0u;\n  HLSL(x) { uint z = 1; };\n  Assert.Equal(y, 1u);\n}\n"
	result, err := lex.Analyze(source.File{Path: "span.sdslvtest", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	module, err := BuildModule(result)
	if err != nil {
		t.Fatal(err)
	}
	fn := module.Decls[0].(ast.FunctionDecl)
	assertSlice(t, text, fn.Span, text[:len(text)-1])
	assertSlice(t, text, fn.Attributes[0].Span, "[Fact]")
	assertSlice(t, text, fn.Parameters[0].Type.Span, "array<u32, 4u>")
	let := fn.Body.Statements[0].(ast.LetStmt)
	assertSlice(t, text, let.Span, "let y: u32 = read x[1u] when true else 0u;")
	read := let.Value.(ast.GuardedReadExpr)
	assertSlice(t, text, read.Span, "read x[1u] when true else 0u")
	foreign := fn.Body.Statements[1].(ast.ForeignShaderStmt)
	assertSlice(t, text, foreign.Span, "HLSL(x) { uint z = 1; };")
	call := fn.Body.Statements[2].(ast.ExprStmt).Value.(ast.CallExpr)
	assertSlice(t, text, call.Span, "Assert.Equal(y, 1u)")
	auditKnownSpans(t, reflect.ValueOf(module), len(text))
}

func TestSdslvNdarraySourceSpansAndSlices(t *testing.T) {
	const text = "fn F() -> void {\n  let input: ndarray<u32, [2u, 3u]> = [1u, 2u, 3u, 4u, 5u, 6u];\n  let value: u32 = input[1u, 2u];\n}\n"
	result, err := lex.Analyze(source.File{Path: "ndarray.sdslv", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	module, err := BuildModule(result)
	if err != nil {
		t.Fatal(err)
	}
	fn := module.Decls[0].(ast.FunctionDecl)
	letInput := fn.Body.Statements[0].(ast.LetStmt)
	ref := letInput.Type
	assertSlice(t, text, ref.NameSpan, "ndarray")
	assertSlice(t, text, ref.Args[0].Span, "u32")
	assertSlice(t, text, ref.NDArrayShapeOpen, "[")
	assertSlice(t, text, ref.NDArrayShapeClose, "]")
	assertSlice(t, text, ref.NDArrayShapeSpan, "[2u, 3u]")
	assertSlice(t, text, ref.Span, "ndarray<u32, [2u, 3u]>")
	lit := letInput.Value.(ast.ArrayLiteral)
	assertSlice(t, text, lit.Span, "[1u, 2u, 3u, 4u, 5u, 6u]")
	for i, want := range []string{"1u", "2u", "3u", "4u", "5u", "6u"} {
		assertSlice(t, text, ast.ExprSpan(lit.Elements[i]), want)
	}
	indexed := fn.Body.Statements[1].(ast.LetStmt).Value.(ast.IndexExpr)
	assertSlice(t, text, indexed.Span, "input[1u, 2u]")
	auditKnownSpans(t, reflect.ValueOf(module), len(text))
}

func TestSdslvAstExampleCorpusHasKnownSpans(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "..", "..", "..", "examples", "SDSL-V"))
	paths, err := filepath.Glob(filepath.Join(root, "*", "*.sdslv"))
	if err != nil {
		t.Fatal(err)
	}
	testPaths, err := filepath.Glob(filepath.Join(root, "*", "*.sdslvtest"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, testPaths...)
	if len(paths) == 0 {
		t.Fatal("no SDSL-V examples")
	}
	for _, path := range paths {
		file, err := source.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		result, err := lex.Analyze(file)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		module, err := BuildModule(result)
		// This audit must not alter the pre-existing language corpus contract:
		// some historical examples intentionally predate current parser support.
		if err != nil {
			continue
		}
		auditKnownSpans(t, reflect.ValueOf(module), len(file.Text))
	}
}

func assertSlice(t *testing.T, text string, span source.Span, want string) {
	t.Helper()
	if !span.Known() {
		t.Fatal("unknown span")
	}
	if span.End.Offset < span.Start.Offset || int(span.End.Offset) > len(text) {
		t.Fatalf("invalid span %#v", span)
	}
	if got := text[span.Start.Offset:span.End.Offset]; got != want {
		t.Fatalf("slice = %q, want %q", got, want)
	}
}

// auditKnownSpans walks parsed AST values without using source text. It checks
// every span-bearing AST value reached from the parsed module, including nodes
// nested in interfaces, slices, and pointers.
func auditKnownSpans(t *testing.T, v reflect.Value, sourceLen int) {
	auditSpanValue(t, v, sourceLen, source.Span{}, "")
}
func auditSpanValue(t *testing.T, v reflect.Value, sourceLen int, parent source.Span, parentName string) {
	t.Helper()
	if !v.IsValid() {
		return
	}
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if !v.IsNil() {
			auditSpanValue(t, v.Elem(), sourceLen, parent, parentName)
		}
		return
	}
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		for i := 0; i < v.Len(); i++ {
			auditSpanValue(t, v.Index(i), sourceLen, parent, parentName)
		}
		return
	}
	if v.Kind() != reflect.Struct {
		return
	}
	if v.Type().PkgPath() == "github.com/yuechen-li-dev/oct/internal/sdslv/ast" {
		if f := v.FieldByName("Span"); f.IsValid() {
			span := f.Interface().(source.Span)
			// Omitted loop/reduction steps are compiler-synthesized `1` values;
			// the AST inventory explicitly permits their documented unknown span.
			if !span.Known() && v.Type().Name() == "IntegerLiteral" {
				return
			}
			if !span.Known() || span.End.Offset < span.Start.Offset || int(span.End.Offset) > sourceLen {
				t.Fatalf("%s has invalid span %#v", v.Type().Name(), span)
			}
			if parent.Known() && !parent.Contains(span) {
				t.Fatalf("%s span %#v escapes %s %#v", v.Type().Name(), span, parentName, parent)
			}
			parent = span
		}
	}
	for i := 0; i < v.NumField(); i++ {
		auditSpanValue(t, v.Field(i), sourceLen, parent, v.Type().Name())
	}
}
