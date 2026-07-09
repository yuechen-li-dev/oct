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

func TestEmitComputeThreadResourceBundleAndWithFromVDMIR(t *testing.T) {
	text := `stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
stream VectorAddIO {
A: readonly array<f32>;
C: readwrite array<f32>;
}
record Params { Count: u32; }
record Tile { Acc0: f32; }
shader VectorAdd {
resources VectorAddIO;
stage compute [numthreads(16, 1, 1)] fn CS(thread: ComputeThread, params: Params) -> Tile {
let tile: Tile;
tile.Acc0 = 0.0;
return tile with { Acc0: A[thread.DispatchId.x] };
}
}`
	out := emitSource(t, text)
	for _, want := range []string{
		"struct ComputeThread",
		"struct Tile",
		"[[vk::binding(0, 0)]] Buffer<float> A;",
		"[[vk::binding(1, 0)]] RWBuffer<float> C;",
		"[[vk::push_constant]] ConstantBuffer<Params> params;",
		"Tile VectorAdd_CS(uint3 DispatchThreadID : SV_DispatchThreadID, uint3 GroupThreadID : SV_GroupThreadID, uint3 GroupID : SV_GroupID, uint GroupIndex : SV_GroupIndex)",
		"ComputeThread thread;",
		"thread.DispatchId = DispatchThreadID;",
		"Tile __sdslv_with_0 = tile;",
		"__sdslv_with_0.Acc0 = A[thread.DispatchId.x];",
		"return __sdslv_with_0;",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
}

func TestEmitWorkgroupAndBarrierHLSLFromVDMIR(t *testing.T) {
	text := `stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
stream TileCopyIO {
A: readonly array<f32>;
C: readwrite array<f32>;
}
record Params { Count: u32; }
shader TileCopy {
resources TileCopyIO;
workgroup Tile: array<f32, 256>;
stage compute [numthreads(16, 16, 1)] fn CS(thread: ComputeThread, params: Params) -> void {
let idx: u32 = thread.DispatchId.x;
let local: u32 = thread.GroupIndex;
if idx < params.Count { Tile[local] = A[idx]; }
WorkgroupBarrier();
WorkgroupMemoryBarrier();
WorkgroupMemoryBarrierWithSync();
if idx < params.Count { C[idx] = Tile[local]; }
return;
}
}`
	out := emitSource(t, text)
	for _, want := range []string{
		"groupshared float Tile[256];",
		"uint3 GroupThreadID : SV_GroupThreadID",
		"uint3 GroupID : SV_GroupID",
		"uint GroupIndex : SV_GroupIndex",
		"GroupMemoryBarrierWithGroupSync();",
		"GroupMemoryBarrier();",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
}

func TestEmitMonomorphizedTemplateShaderHLSLFromVDMIR(t *testing.T) {
	text := `stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
stream TileCopyIO {
A: readonly array<f32>;
C: readwrite array<f32>;
}
record Params { Count: u32; }
concept TileCopyConfig {
THREADS_X: u32;
THREADS_Y: u32;
TILE_SIZE: u32;
}
config Tile16x16: TileCopyConfig {
THREADS_X: 16u;
THREADS_Y: 16u;
TILE_SIZE: 256u;
}
template<C: TileCopyConfig>
shader TileCopy {
resources TileCopyIO;
workgroup Tile: array<f32, C.TILE_SIZE>;
stage compute [numthreads(C.THREADS_X, C.THREADS_Y, 1u)] fn CS(thread: ComputeThread, params: Params) -> void {
let idx: u32 = thread.DispatchId.x;
let local: u32 = thread.GroupIndex;
if idx < params.Count { Tile[local] = A[idx]; }
WorkgroupMemoryBarrierWithSync();
[unroll]
for i in 0u..1u {
if idx < params.Count { C[idx] = Tile[local]; }
}
return;
}
}
compile TileCopy<Tile16x16> as TileCopy16x16;`
	out := emitSource(t, text)
	for _, want := range []string{
		"[numthreads(16, 16, 1)]",
		"groupshared float Tile[256];",
		"[unroll]",
		"void TileCopy16x16_CS(",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "void TileCopy_CS(") {
		t.Fatalf("template entry point should not be emitted:\n%s", out)
	}
}

func TestEmitLoopHintsAndExplicitBindingsFromVDMIR(t *testing.T) {
	text := `shader S {
resources {
[binding(2)] A: readonly array<f32>;
C: readwrite array<f32>;
}
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
[loop]
for i in 0u..1u {
return;
}
return;
}
}`
	out := emitSource(t, text)
	for _, want := range []string{
		"[[vk::binding(2, 0)]] Buffer<float> A;",
		"[[vk::binding(0, 0)]] RWBuffer<float> C;",
		"[loop]",
		"for (uint i = 0u; i < 1u; i += 1)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
}

func TestEmitStructuredConfigTemplateSpecializationToHLSL(t *testing.T) {
	text := `stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
stream TileCopyIO {
A: readonly array<f32>;
C: readwrite array<f32>;
}
record Params { Count: u32; }
concept TileCopyConfig {
Threads: {
X: u32;
Y: u32;
};
Tile: {
M: u32 = Threads.X;
N: u32 = Threads.Y;
K: u32;
};
}
config Tile16: TileCopyConfig {
Threads.X => 16u;
Threads.Y => 8u;
Tile.K => 4u;
}
template<C: TileCopyConfig>
shader TileCopy {
resources TileCopyIO;
workgroup Tile: array<f32, C.Tile.M * C.Tile.N>;
stage compute [numthreads(C.Threads.X, C.Threads.Y, 1u)] fn CS(thread: ComputeThread, params: Params) -> void {
let tileK: u32 = C.Tile.K;
return;
}
}
compile TileCopy<Tile16> as TileCopy16;`
	out := emitSource(t, text)
	for _, want := range []string{
		"[numthreads(16, 8, 1)]",
		"groupshared float Tile[128];",
		"uint tileK = 4u;",
		"void TileCopy16_CS(",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
}

func TestEmitPayloadEnumsAndMatchFromVDMIR(t *testing.T) {
	text := `enum LoadValue {
Zero;
Value { X: f32; }
}
fn Resolve(v: LoadValue) -> f32 {
let initial: LoadValue = LoadValue.Zero;
let out: f32 = match v {
LoadValue.Zero => 0.0
LoadValue.Value(payload) => payload.X
};
return out;
}`
	out := emitSource(t, text)
	for _, want := range []string{
		"static const int LoadValue_Zero = 0;",
		"static const int LoadValue_Value = 1;",
		"struct LoadValue_ValuePayload",
		"struct LoadValue",
		"LoadValue __sdslv_make_LoadValue_Zero()",
		"LoadValue __sdslv_make_LoadValue_Value(float X)",
		"LoadValue __sdslv_match_subject_",
		"if (__sdslv_match_subject_0.Tag == LoadValue_Zero)",
		"LoadValue_ValuePayload payload = __sdslv_match_subject_0.Value;",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
}

func TestEmitReductionsFromVDMIR(t *testing.T) {
	text := `shader S {
resources {
A: readonly array<f32>;
C: readwrite array<f32>;
}
fn Reduce(values: array<f32>) -> f32 {
return product i in 0u..4u { values[i] };
}
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let total: f32 = sum i in 0u..4u { A[i] };
let productValue: f32 = product j in 0u..4u { A[j] };
let sink: f32 = 0.0;
sink = sum k in 0u..4u { A[k] };
C[0u] = total + productValue + sink + Reduce(A);
return;
}
}`
	out := emitSource(t, text)
	for _, want := range []string{
		"float total = 0.0;",
		"for (uint i = 0u; i < 4u; i += 1)",
		"total = total + (A[i]);",
		"float productValue = 1.0;",
		"productValue = productValue * (A[j]);",
		"float __sdslv_reduce_1 = 0.0;",
		"sink = __sdslv_reduce_1;",
		"float __sdslv_reduce_0 = 1.0;",
		"return __sdslv_reduce_0;",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
}

func TestEmitReductionLoopHintsFromVDMIR(t *testing.T) {
	text := `shader S {
resources {
A: readonly array<f32>;
C: readwrite array<f32>;
}
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let total: f32 = [unroll] sum i in 0u..4u { A[i] };
let productValue: f32 = [loop] product j in 0u..4u { A[j] };
C[0u] = total + productValue;
return;
}
}`
	out := emitSource(t, text)
	for _, want := range []string{
		"[unroll]",
		"for (uint i = 0u; i < 4u; i += 1)",
		"[loop]",
		"for (uint j = 0u; j < 4u; j += 1)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
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
