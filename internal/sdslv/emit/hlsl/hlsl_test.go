package hlsl

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestEmitComputeShaderHLSL(t *testing.T) {
	text := `record Params { Count: u32; }
shader VectorAdd {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
stage compute [numthreads(16, 16, 1)] fn CS(params: Params) -> void {
let index: u32 = DispatchThreadID.x;
if index < params.Count { C[index] = A[index]; }
return;
}
}`
	first := emitSource(t, text)
	second := emitSource(t, text)
	if first != second {
		t.Fatalf("emission is not deterministic")
	}
	for _, want := range []string{
		"Buffer<float> A;",
		"RWBuffer<float> C;",
		"[numthreads(16, 16, 1)]",
		"void VectorAdd_CS(Params params, uint3 DispatchThreadID : SV_DispatchThreadID",
		"C[index] = A[index];",
	} {
		if !strings.Contains(first, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, first)
		}
	}
}

func emitSource(t *testing.T, text string) string {
	t.Helper()
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: text})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatalf("BuildModule() error = %v", err)
	}
	if err := validate.Module(module); err != nil {
		t.Fatalf("validate.Module() error = %v", err)
	}
	out, err := Emit(module)
	if err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	return out
}
