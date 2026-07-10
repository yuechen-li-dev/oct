package validate

import (
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
	"strings"
	"testing"
)

func TestSdslvInlineHlslChecksCaptureScope(t *testing.T) {
	mustInlineHLSLError(t, "let x: f32 = HLSL<f32>(missing) { return missing; };", "not in scope")
}
func TestSdslvInlineHlslRejectsDuplicateCaptures(t *testing.T) {
	mustInlineHLSLError(t, "let x: f32 = HLSL<f32>(a, a) { return a; };", "duplicate inline HLSL capture")
}
func TestSdslvInlineHlslRejectsUnsupportedCaptureType(t *testing.T) {
	mustInlineHLSLError(t, "let x: R = HLSL<R>(r) { return r; };", "cannot capture R")
}
func TestSdslvInlineHlslChecksDeclaredResultType(t *testing.T) {
	mustInlineHLSLError(t, "let x: R = HLSL<R> { return 1; };", "result type R")
}
func TestSdslvInlineHlslRejectsResourceDeclaration(t *testing.T) {
	mustInlineHLSLError(t, "HLSL { Texture2D<float> X; }", "forbidden interface")
}
func TestSdslvInlineHlslRejectsRegisterDeclaration(t *testing.T) {
	mustInlineHLSLError(t, "HLSL { float X : register(t0); }", "forbidden interface")
}
func TestSdslvInlineHlslRejectsEntryPointDeclaration(t *testing.T) {
	mustInlineHLSLError(t, "HLSL { [numthreads(1,1,1)] void X() {} }", "forbidden interface")
}
func TestSdslvInlineHlslRejectsPreprocessorDirective(t *testing.T) {
	mustInlineHLSLError(t, "HLSL { #include \"x\" }", "forbidden interface")
}

func mustInlineHLSLError(t *testing.T, body, want string) {
	t.Helper()
	src := "record R { x: f32; } fn F(a: f32, r: R) -> void { " + body + " return; }"
	r, e := lex.Analyze(source.File{Path: "inline.sdslv", Text: src})
	if e != nil {
		t.Fatal(e)
	}
	m, e := parse.BuildModule(r)
	if e != nil {
		t.Fatal(e)
	}
	e = Module(m)
	if e == nil || !strings.Contains(e.Error(), want) {
		t.Fatalf("want %q, got %v", want, e)
	}
}
