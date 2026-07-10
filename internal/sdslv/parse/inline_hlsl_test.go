package parse

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestSdslvInlineHlslParsesStatementBlock(t *testing.T) {
	inlineHLSLModule(t, "HLSL { GroupMemoryBarrierWithGroupSync(); }")
}
func TestSdslvInlineHlslParsesTypedExpression(t *testing.T) {
	inlineHLSLModule(t, "let lane: u32 = HLSL<u32> { return WaveGetLaneIndex(); };")
}
func TestSdslvInlineHlslParsesExplicitCaptures(t *testing.T) {
	inlineHLSLModule(t, "let y: f32 = HLSL<f32>(x) { return x; };")
}
func TestSdslvInlineHlslPreservesNestedBraces(t *testing.T) {
	inlineHLSLModule(t, "HLSL { if (true) { GroupMemoryBarrierWithGroupSync(); } }")
}
func TestSdslvInlineHlslPreservesCommentsAndStrings(t *testing.T) {
	inlineHLSLModule(t, "HLSL { // }\n string s = \"}\"; }")
}
func TestSdslvInlineHlslRejectsUnterminatedBlock(t *testing.T) {
	_, err := lex.Analyze(source.File{Path: "inline.sdslv", Text: "fn F() -> void { HLSL { if (true) { }"})
	if err == nil || !strings.Contains(err.Error(), "unterminated inline HLSL block") {
		t.Fatalf("want clear unterminated diagnostic, got %v", err)
	}
}

func inlineHLSLModule(t *testing.T, body string) ast.Module {
	t.Helper()
	input := "fn F(x: f32) -> void { " + body + " return; }"
	result, err := lex.Analyze(source.File{Path: "inline.sdslv", Text: input})
	if err != nil {
		t.Fatal(err)
	}
	module, err := BuildModule(result)
	if err != nil {
		t.Fatal(err)
	}
	return module
}
