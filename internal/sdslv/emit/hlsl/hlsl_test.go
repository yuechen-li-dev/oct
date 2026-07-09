package hlsl

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lower"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestEmitComputeShaderHLSLFromVDMIR(t *testing.T) {
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
		"[[vk::binding(0, 0)]] Buffer<float> A;",
		"[[vk::binding(1, 0)]] RWBuffer<float> C;",
		"[[vk::push_constant]] ConstantBuffer<Params> params;",
		"[numthreads(16, 16, 1)]",
		"void VectorAdd_CS(uint3 DispatchThreadID : SV_DispatchThreadID",
		"C[index] = A[index];",
	} {
		if !strings.Contains(first, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, first)
		}
	}
}

func TestEmitWhenUtilityFromVDMIR(t *testing.T) {
	text := `shader Picker {
fn Pick(flag: bool) -> f32 {
let scale: f32 = when utility {
case 2.0 when flag score 2.0
case 1.0 when true score 1.0
else 1.0
};
return scale;
}
}`
	out := emitSource(t, text)
	if !strings.Contains(out, "float __sdslv_scale_score = -3.402823466e+38F;") {
		t.Fatalf("when utility score scratch missing:\n%s", out)
	}
	if !strings.Contains(out, "if (flag && (2.0 > __sdslv_scale_score))") {
		t.Fatalf("when utility first case missing:\n%s", out)
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
	mir, err := lower.Module(module)
	if err != nil {
		t.Fatalf("lower.Module() error = %v", err)
	}
	out, err := Emit(mir)
	if err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	return out
}
