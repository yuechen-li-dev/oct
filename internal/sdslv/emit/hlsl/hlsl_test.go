package hlsl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lower"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestCooperativeMatrixEmitterOwnsExactHLSLIsland(t *testing.T) {
	first := emitSource(t, cooperativeMatrixEmitterTestSource)
	second := emitSource(t, cooperativeMatrixEmitterTestSource)
	if first != second {
		t.Fatal("cooperative HLSL emission is not deterministic")
	}
	for _, want := range []string{
		`vk::ext_extension("SPV_KHR_cooperative_matrix")`,
		"OpTypeCooperativeMatrix", // deliberately absent: HLSL owns typed declarations, not textual SPIR-V
		"[[vk::ext_instruction(4457)]]",
		"[[vk::ext_instruction(4459)]]",
		"[[vk::ext_instruction(4458)]]",
		"groupshared float16_t __sdslv_cooperative_tile_a[256]",
		"[numthreads(32, 1, 1)]",
	} {
		present := strings.Contains(first, want)
		if want == "OpTypeCooperativeMatrix" {
			if present {
				t.Fatalf("emitter leaked direct SPIR-V text:\n%s", first)
			}
		} else if !present {
			t.Fatalf("HLSL missing %q:\n%s", want, first)
		}
	}
}

const cooperativeMatrixEmitterTestSource = `stream ComputeThread { DispatchId:uint3; GroupId:uint3; GroupThreadId:uint3; GroupIndex:u32; }
stream IO { [binding(0)] A:readonly array<u32>; [binding(1)] B:readonly array<u32>; [binding(2)] C:readwrite array<f32>; }
record P { m:u32; n:u32; k:u32; }
shader S { resources IO; stage compute [numthreads(32,1,1)] fn CS(thread:ComputeThread,p:P)->void {
CooperativeMatMul<F16F32M16N16K16Subgroup>(A,B,C,thread.GroupId,thread.GroupIndex,p.m,p.n,p.k); return; } }`

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
		"[[vk::binding(0, 0)]] StructuredBuffer<float> A;",
		"[[vk::binding(1, 0)]] RWStructuredBuffer<float> C;",
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

func TestSdslvTensorUsesSharedEmitterLoopsAndAccumulator(t *testing.T) {
	text := `fn Dot(A: array<array<f32, 3u>, 2u>, B: array<f32, 3u>) -> void {
  var C: array<f32, 2u> = Fill(0.0);
  tensor C[i] = Sum[k](A[i, k] * B[k]);
  return;
}`
	output := emitSource(t, text)
	for _, want := range []string{
		"for (uint __sdslv_tensor_free_0 = 0u; __sdslv_tensor_free_0 < 2u;",
		"float A[6]",
		"float __sdslv_tensor_accumulator_4 = 0.0;",
		"for (uint __sdslv_tensor_reduce_0 = 0u; __sdslv_tensor_reduce_0 < 3u;",
		"A[((__sdslv_tensor_free_0 * 3u) + __sdslv_tensor_reduce_0)]",
		"C[__sdslv_tensor_offset_2] = __sdslv_tensor_rhs_3;",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, output)
		}
	}
}

func TestSdslvFixedArraysUseRankGeneralRowMajorLinearStorage(t *testing.T) {
	out := emitSource(t, `fn Layout(
  A1: array<f32, 2u>,
  A2: array<array<f32, 3u>, 2u>,
  A3: array<array<array<f32, 4u>, 3u>, 2u>,
  A4: array<array<array<array<f32, 5u>, 4u>, 3u>, 2u>
) -> void {
  var B1: array<f32, 2u> = Fill(0.0);
  var B2: array<array<f32, 3u>, 2u> = Fill(0.0);
  var B3: array<array<array<f32, 4u>, 3u>, 2u> = Fill(0.0);
  var B4: array<array<array<array<f32, 5u>, 4u>, 3u>, 2u> = Fill(0.0);
  tensor B1[i] = A1[i];
  tensor B2[i, j] = A2[i, j];
  tensor B3[i, j, k] = A3[i, j, k];
  tensor B4[i, j, k, l] = A4[i, j, k, l];
  return;
}`)
	for _, want := range []string{
		"float A1[2]", "float A2[6]", "float A3[24]", "float A4[120]",
		"A1[__sdslv_tensor_free_0]",
		"A2[((__sdslv_tensor_free_0 * 3u) + __sdslv_tensor_free_1)]",
		"A3[((((__sdslv_tensor_free_0 * 3u) + __sdslv_tensor_free_1) * 4u) + __sdslv_tensor_free_2)]",
		"A4[((((((__sdslv_tensor_free_0 * 3u) + __sdslv_tensor_free_1) * 4u) + __sdslv_tensor_free_2) * 5u) + __sdslv_tensor_free_3)]",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("row-major HLSL missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "][") {
		t.Fatalf("fixed tensor indexing depends on nested HLSL brackets:\n%s", out)
	}
}

func TestSdslvNdarrayUsesSharedFixedShapeEmitterAndSourceOrder(t *testing.T) {
	out := emitSource(t, `fn Copy() -> void {
  let input: ndarray<u32, [2u, 2u, 2u, 3u]> = [
    1u, 2u, 4u, 5u, 2u, 3u,
    4u, 5u, 7u, 8u, 5u, 6u,
    4u, 5u, 7u, 8u, 5u, 6u,
    8u, 9u, 11u, 12u, 9u, 10u
  ];
  var output: ndarray<u32, [2u, 2u, 2u, 3u]> = Fill(0u);
  tensor output[b, h, i, j] = input[b, h, i, j];
  return;
}`)
	for _, want := range []string{
		"uint input[24];",
		"input[0] = 1u;",
		"input[1] = 2u;",
		"input[23] = 10u;",
		"input[((((((__sdslv_tensor_free_0 * 2u) + __sdslv_tensor_free_1) * 2u) + __sdslv_tensor_free_2) * 3u) + __sdslv_tensor_free_3)]",
		"uint output[24];",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("ndarray HLSL missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "uint input[2][2][2][3]") || strings.Contains(out, "][") {
		t.Fatalf("ndarray emitter fell back to nested HLSL array semantics:\n%s", out)
	}
}

func TestSdslvFixedShapeConstructionUsesFlatRowMajorStorage(t *testing.T) {
	out := emitSource(t, `fn F(seed: u32) -> void {
  let A: ndarray<u32, [2u, 2u]> = Fill(seed);
  let B: ndarray<u32, [2u, 3u]> = Generate[i, j](i * 3u + j);
  return;
}`)
	for _, want := range []string{
		"uint A[4];", "uint __sdslv_fill_0 = seed;", "A[0] = __sdslv_fill_0;", "A[3] = __sdslv_fill_0;",
		"uint B[6];", "for (uint i = 0u; i < 2u; ++i)", "for (uint j = 0u; j < 3u; ++j)",
		"uint __sdslv_tensor_index_", "uint __sdslv_tensor_offset_", "B[__sdslv_tensor_offset_",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("construction HLSL missing %q:\n%s", want, out)
		}
	}
}

func TestSdslvFillMaterializesGuardedReadOnceAndReusesResult(t *testing.T) {
	out := emitSource(t, `fn F(index: u32) -> void {
  let input: ndarray<u32, [3u]> = [5u, 7u, 11u];
  let values: ndarray<u32, [2u, 2u]> = Fill(read input[index] when index < 3u else 99u);
  return;
}`)
	if strings.Count(out, "uint __sdslv_guarded_read_") != 1 {
		t.Fatalf("Fill should materialize one guarded-read temporary:\n%s", out)
	}
	if strings.Count(out, "uint __sdslv_fill_") != 1 {
		t.Fatalf("Fill should materialize one fill temporary:\n%s", out)
	}
	if strings.Count(out, "values[") != 5 || strings.Count(out, "__sdslv_fill_") != 5 {
		t.Fatalf("Fill temporary should be declared once and reused by four stores:\n%s", out)
	}
}

func TestSdslvGenerateMaterializesDestinationOncePerCoordinate(t *testing.T) {
	out := emitSource(t, `fn F(seed: u32) -> void {
  let values: ndarray<u32, [2u, 3u]> = Generate[i, j](seed + i * 10u + j);
  return;
}`)
	if strings.Count(out, "uint __sdslv_tensor_offset_") != 1 {
		t.Fatalf("Generate should materialize one destination offset per element site:\n%s", out)
	}
	if strings.Count(out, "values[__sdslv_tensor_offset_") != 1 {
		t.Fatalf("Generate should reuse the materialized destination address:\n%s", out)
	}
}

func TestSdslvGenerateMaterializesInlineHLSLAndCallsWithoutDuplication(t *testing.T) {
	out := emitSource(t, `fn AddBias(value: u32, bias: u32) -> u32 { return value + bias; }
fn F(bias: u32) -> void {
  let values: ndarray<u32, [2u, 2u]> = Generate[i, j](AddBias(HLSL<u32>(i, j) { return i * 10u + j; }, bias));
  return;
}`)
	if strings.Count(out, "BEGIN INLINE HLSL") != 1 {
		t.Fatalf("Generate body duplicated inline HLSL:\n%s", out)
	}
	if strings.Count(out, "= AddBias(") != 1 {
		t.Fatalf("Generate body duplicated ordinary call lowering:\n%s", out)
	}
}

func TestSdslvFixedShapeConstructionRejectsMalformedMetadata(t *testing.T) {
	u32Type := vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}
	badShape := vdmir.Type{Kind: vdmir.TypeArray, Name: "array", Element: &u32Type}
	_, err := Emit(vdmir.Module{
		Functions: []vdmir.Function{{
			Name:        "F",
			EmittedName: "F",
			ReturnType:  vdmir.Type{Kind: vdmir.TypeVoid, Name: "void"},
			Body: vdmir.Block{Statements: []vdmir.Stmt{
				vdmir.LetStmt{Name: "values", Type: badShape, Value: vdmir.FillConstruct{ExprType: badShape, Value: vdmir.LiteralExpr{ExprType: u32Type, Kind: vdmir.LiteralInteger, Value: "7u"}}},
			}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "malformed fixed-shape Fill metadata") {
		t.Fatalf("Emit() err = %v, want malformed Fill metadata rejection", err)
	}

	goodShape := vdmir.Type{Kind: vdmir.TypeNDArray, Name: "ndarray", Element: &u32Type, Shape: []uint32{2, 2}, ArraySize: 4}
	_, err = Emit(vdmir.Module{
		Functions: []vdmir.Function{{
			Name:        "G",
			EmittedName: "G",
			ReturnType:  vdmir.Type{Kind: vdmir.TypeVoid, Name: "void"},
			Body: vdmir.Block{Statements: []vdmir.Stmt{
				vdmir.LetStmt{Name: "values", Type: goodShape, Value: vdmir.GenerateConstruct{
					ExprType: goodShape,
					Binders:  []string{"i"},
					Body:     vdmir.LiteralExpr{ExprType: u32Type, Kind: vdmir.LiteralInteger, Value: "1u"},
				}},
			}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "malformed fixed-shape Generate metadata") {
		t.Fatalf("Emit() err = %v, want malformed Generate metadata rejection", err)
	}
}

func TestSdslvTensorCompoundAssignmentMaterializesDestinationOnce(t *testing.T) {
	out := emitSource(t, `fn Add(B: array<f32, 2u>) -> void {
  var A: array<f32, 2u> = Fill(0.0);
  tensor A[i] += B[i];
  return;
}`)
	if strings.Count(out, "uint __sdslv_tensor_offset_2 =") != 1 {
		t.Fatalf("destination offset was not materialized exactly once:\n%s", out)
	}
	if strings.Count(out, "A[__sdslv_tensor_offset_2]") != 2 {
		t.Fatalf("materialized destination was not reused for one read/one write:\n%s", out)
	}
	old := strings.Index(out, "tensor_old")
	rhs := strings.Index(out, "tensor_rhs")
	write := strings.LastIndex(out, "A[__sdslv_tensor_offset_2] =")
	if old < 0 || rhs < old || write < rhs {
		t.Fatalf("compound tensor prelude order is not old/read, RHS, write:\n%s", out)
	}
}

func TestSdslvTensorBodiesUseSharedExpressionMaterialization(t *testing.T) {
	guarded := emitSource(t, `fn Guarded(A: array<array<f32, 2u>, 2u>, M: u32) -> void {
  var B: array<f32, 2u> = Fill(0.0);
  tensor B[i] = Sum[k](read A[i, k] when i < M else 0.0);
  return;
}`)
	if strings.Count(guarded, "float __sdslv_guarded_read_") != 1 || strings.Contains(guarded, "unsupported guarded read") {
		t.Fatalf("guarded tensor body did not use shared materialization:\n%s", guarded)
	}
	inline := emitSource(t, `fn Inline(A: array<f32, 2u>) -> void {
  var B: array<f32, 2u> = Fill(0.0);
  tensor B[i] = A[i] + HLSL<f32> { return 2.0; };
  return;
}`)
	if strings.Count(inline, "BEGIN INLINE HLSL") != 1 || strings.Contains(inline, "inline HLSL expressions require") {
		t.Fatalf("inline HLSL tensor body did not use shared materialization:\n%s", inline)
	}
	if strings.Contains(guarded+inline, "requires statement context") {
		t.Fatalf("supported tensor expression emitted a placeholder:\n%s\n%s", guarded, inline)
	}
}

func TestSdslvTensorAffineBaseCallMaterializesOnce(t *testing.T) {
	out := emitSource(t, `fn Base() -> u32 { return 0u; }
fn Affine(A: array<f32, 2u>) -> void {
  var B: array<f32, 2u> = Fill(0.0);
  tensor B[i] = A[Base() + i];
  return;
}`)
	if strings.Count(out, "= Base();") != 1 {
		t.Fatalf("affine base call was not evaluated once per tensor body:\n%s", out)
	}
}

func TestSdslvTensorLoopOrderFollowsSourceOrder(t *testing.T) {
	out := emitSource(t, `fn Ordered(
  Input: array<array<array<array<u32, 2u>, 2u>, 2u>, 2u>
) -> void {
  var Output: array<array<u32, 2u>, 2u> = Fill(0u);
  tensor Output[y, x] = Sum[ky, kx](Input[y, x, ky, kx]);
  return;
}`)
	freeY := strings.Index(out, "for (uint __sdslv_tensor_free_0 = 0u; __sdslv_tensor_free_0 < 2u;")
	freeX := strings.Index(out, "for (uint __sdslv_tensor_free_1 = 0u; __sdslv_tensor_free_1 < 2u;")
	reduceKY := strings.Index(out, "for (uint __sdslv_tensor_reduce_0 = 0u; __sdslv_tensor_reduce_0 < 2u;")
	reduceKX := strings.Index(out, "for (uint __sdslv_tensor_reduce_1 = 0u; __sdslv_tensor_reduce_1 < 2u;")
	if freeY < 0 || freeX <= freeY || reduceKY <= freeX || reduceKX <= reduceKY {
		t.Fatalf("tensor loop order is not free y/x then reduction ky/kx:\n%s", out)
	}
}

func TestSdslvSharedMaterializationPreservesLeftToRightOrderAndHygiene(t *testing.T) {
	out := emitSource(t, `fn Ordered() -> f32 {
  return HLSL<f32> { return 1.0; } + HLSL<f32> { return 2.0; };
}`)
	left := strings.Index(out, "__sdslv_inline_hlsl_0 = 1.0")
	right := strings.Index(out, "__sdslv_inline_hlsl_1 = 2.0")
	if left < 0 || right <= left {
		t.Fatalf("shared preludes are not hygienic and left-to-right:\n%s", out)
	}
	if strings.Count(out, "BEGIN INLINE HLSL") != 2 || strings.Contains(out, "inline HLSL expressions require") {
		t.Fatalf("foreign operands were duplicated or emitted as placeholders:\n%s", out)
	}
	calls := emitSource(t, `fn Left() -> u32 { return 1u; }
fn Right() -> u32 { return 2u; }
fn OrderedCalls() -> u32 { return Left() + Right(); }`)
	leftCall := strings.Index(calls, "uint __sdslv_operand_0 = Left();")
	rightCall := strings.Index(calls, "uint __sdslv_operand_1 = Right();")
	if leftCall < 0 || rightCall <= leftCall {
		t.Fatalf("call operands were not materialized left-to-right:\n%s", calls)
	}
}

func TestSdslvFlowDispatcherShapes(t *testing.T) {
	legacy := emitSource(t, `shader S { stage compute [numthreads(1,1,1)] fn CS() -> void { flow F { state A { let x: u32 = 1u; } state B { } } } }`)
	if strings.Contains(legacy, "flow dispatcher") || strings.Contains(legacy, "return_stack") {
		t.Fatalf("legacy flow gained runtime machinery:\n%s", legacy)
	}
	goTo := emitSource(t, `shader S { stage compute [numthreads(1,1,1)] fn CS() -> void { flow F { state A { goto Done; } state Done { finish; } } } }`)
	for _, want := range []string{"flow dispatcher test.sdslv:", "switch (__flow_F_0_state)", "__flow_F_0_state = 1u;"} {
		if !strings.Contains(goTo, want) {
			t.Fatalf("goto flow missing %q:\n%s", want, goTo)
		}
	}
	if strings.Contains(goTo, "return_stack") || strings.Contains(goTo, "stack_top") {
		t.Fatalf("goto-only flow emitted stack:\n%s", goTo)
	}
	push := emitSource(t, `shader S { stage compute [numthreads(1,1,1)] fn CS() -> void { flow F { state A { push Shared; } state Done { finish; } state Shared { pop; } } } }`)
	for _, want := range []string{"uint __flow_F_0_return_stack[1];", "__flow_F_0_return_stack[__flow_F_0_stack_top] = 1u;", "__flow_F_0_stack_top -= 1u;"} {
		if !strings.Contains(push, want) {
			t.Fatalf("push/pop flow missing %q:\n%s", want, push)
		}
	}
}

func TestSdslvFlowDispatcherEmitsSourceMarkers(t *testing.T) {
	text := `shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow F {
state A { push Shared; }
state Resume { goto Done; }
state Done { finish; }
state Shared { pop; }
}
}
}`
	out := emitSource(t, text)
	for _, want := range []string{
		"// flow dispatcher test.sdslv:",
		"// flow stack test.sdslv:",
		"// flow state test.sdslv:4:7 A",
		"// flow terminator 4:11 push target=3u return=1u",
		"// flow stack push 4:11",
		"// flow state test.sdslv:5:7 Resume",
		"// flow terminator 5:16 goto",
		"// flow state test.sdslv:6:7 Done",
		"// flow terminator 6:14 finish",
		"// flow state test.sdslv:7:7 Shared",
		"// flow terminator 7:16 pop",
		"// flow stack pop 7:16",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("flow marker missing %q:\n%s", want, out)
		}
	}
}

func TestSdslvFlowDispatcherRejectsMalformedMetadata(t *testing.T) {
	makeModule := func(flow vdmir.Flow) vdmir.Module {
		return vdmir.Module{
			Functions: []vdmir.Function{{
				Name: "CS",
				Body: vdmir.Block{Statements: []vdmir.Stmt{vdmir.FlowStmt{Flow: flow}}},
			}},
			EntryPoints: []vdmir.ComputeEntryPoint{{
				ShaderName:   "S",
				FunctionName: "CS",
				EmittedName:  "S_CS",
				NumThreadsX:  1,
				NumThreadsY:  1,
				NumThreadsZ:  1,
			}},
		}
	}
	baseState := func(id int, kind vdmir.FlowTerminatorKind) vdmir.FlowState {
		return vdmir.FlowState{
			ID:   id,
			Name: fmt.Sprintf("S%d", id),
			Body: vdmir.Block{},
			Terminator: vdmir.FlowTerminator{
				Kind: kind,
			},
		}
	}
	cases := []struct {
		name string
		flow vdmir.Flow
		want string
	}{
		{
			name: "duplicate state id",
			flow: vdmir.Flow{
				Name:       "Dup",
				Entry:      0,
				States:     []vdmir.FlowState{baseState(0, vdmir.FlowTerminatorFinish), baseState(0, vdmir.FlowTerminatorFinish)},
				SourceSpan: source.Span{Start: source.Position{Line: 1, Column: 1}},
			},
			want: "duplicate or invalid state ID 0",
		},
		{
			name: "invalid target id",
			flow: vdmir.Flow{
				Name:  "BadTarget",
				Entry: 0,
				States: []vdmir.FlowState{{
					ID:   0,
					Name: "A",
					Body: vdmir.Block{},
					Terminator: vdmir.FlowTerminator{
						Kind:   vdmir.FlowTerminatorGoto,
						Target: 7,
					},
				}},
			},
			want: "invalid target state ID 7",
		},
		{
			name: "missing push return",
			flow: vdmir.Flow{
				Name:          "MissingReturn",
				Entry:         0,
				HasPushPop:    true,
				MaxStackDepth: 1,
				States: []vdmir.FlowState{{
					ID:   0,
					Name: "A",
					Body: vdmir.Block{},
					Terminator: vdmir.FlowTerminator{
						Kind:     vdmir.FlowTerminatorPush,
						Target:   1,
						ReturnTo: 9,
					},
				}, baseState(1, vdmir.FlowTerminatorFinish)},
			},
			want: "invalid push terminator",
		},
		{
			name: "pop with zero stack contract",
			flow: vdmir.Flow{
				Name:  "PopZero",
				Entry: 0,
				States: []vdmir.FlowState{{
					ID:   0,
					Name: "A",
					Body: vdmir.Block{},
					Terminator: vdmir.FlowTerminator{
						Kind: vdmir.FlowTerminatorPop,
					},
				}},
			},
			want: "pop without return-stack contract",
		},
		{
			name: "inconsistent max stack depth",
			flow: vdmir.Flow{
				Name:       "DepthZero",
				Entry:      0,
				HasPushPop: true,
				States:     []vdmir.FlowState{baseState(0, vdmir.FlowTerminatorFinish)},
			},
			want: "push/pop requires positive MaxStackDepth",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Emit(makeModule(tc.flow))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Emit() err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSdslvInlineHlslEmitsSourceMarkers(t *testing.T) {
	out := emitSource(t, `shader S { stage compute [numthreads(1, 1, 1)] fn CS() -> void {
HLSL { GroupMemoryBarrierWithGroupSync(); }
let lane: u32 = HLSL<u32> { return WaveGetLaneIndex(); };
return;
} }`)
	for _, want := range []string{"BEGIN INLINE HLSL test.sdslv:", "GroupMemoryBarrierWithGroupSync();", "WaveGetLaneIndex()", "END INLINE HLSL"} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
}

func TestEmitBoardValuesAsHLSLStructs(t *testing.T) {
	out := emitSource(t, `board LoadCoord {
linear: u32;
row: u32;
col: u32;
}
fn MakeLoadCoord(localThreadLinear: u32, lane: u32, tileK: u32) -> LoadCoord {
let linear: u32 = localThreadLinear * 4u + lane;
return LoadCoord { linear: linear; row: linear / tileK; col: linear % tileK; };
}
shader S {
resources { A: readonly array<f32>; }
workgroup Tile: tile<f32, 16u, 16u>;
stage compute [numthreads(1, 1, 1)] fn CS(fullTile: bool) -> void {
let localThreadLinear: u32 = GroupThreadID.x;
let AView: matrix_view<f32> = row_major(A, 16u, 16u);
comptime for lane in 0u..1u {
let p: LoadCoord = MakeLoadCoord(localThreadLinear, lane, 16u);
when {
case fullTile -> { Tile[p.row, p.col] = AView[p.row, p.col]; }
else -> { Tile[p.row, p.col] = read AView[p.row, p.col] when p.row < 16u and p.col < 16u else 0.0; }
}
}
return;
}
}`)
	for _, want := range []string{
		"struct LoadCoord",
		"uint linear_;",
		"LoadCoord MakeLoadCoord(",
		"LoadCoord __sdslv_board_0;",
		"__sdslv_board_0.row = (linear_ / tileK);",
		"LoadCoord p__ct0 = MakeLoadCoord(localThreadLinear, 0u, 16u);",
		"Tile[((p__ct0.row) * (16)) + (p__ct0.col)]",
		"if (((p__ct0.row < 16u) && (p__ct0.col < 16u)))",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
}

func TestEmitDeriveAsOrderedTemps(t *testing.T) {
	out := emitSource(t, `record LoadFacts {
linear: u32;
row: u32;
col: u32;
}
fn Make(localThreadLinear: u32, lane: u32, tileK: u32) -> LoadFacts {
return derive {
linear = localThreadLinear * 4u + lane;
row = linear / tileK;
col = linear % tileK;
};
}`)
	for _, want := range []string{
		"uint __sdslv_derive_linear_0 = ((localThreadLinear * 4u) + lane);",
		"uint __sdslv_derive_row_1 = (__sdslv_derive_linear_0 / tileK);",
		"uint __sdslv_derive_col_2 = (__sdslv_derive_linear_0 % tileK);",
		"LoadFacts __sdslv_derive_result_0;",
		"__sdslv_derive_result_0.linear_ = __sdslv_derive_linear_0;",
		"__sdslv_derive_result_0.row = __sdslv_derive_row_1;",
		"return __sdslv_derive_result_0;",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
}

func TestEmitTileAndMatrixView2DIndexing(t *testing.T) {
	hlsl := emitSource(t, `shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
workgroup Tile: tile<f32, 16, 8>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let AView: matrix_view<f32> = row_major(A, 16u, 8u);
let CView: matrix_view<f32> = row_major(C, 16u, 8u);
Tile[0u, 1u] = AView[0u, 1u];
CView[0u, 1u] = Tile[0u, 1u];
return;
}
}`)
	for _, want := range []string{
		"groupshared float Tile[16 * 8];",
		"Tile[((0u) * (8)) + (1u)] = A[((0u) * (8u)) + (1u)];",
		"C[((0u) * (8u)) + (1u)] = Tile[((0u) * (8)) + (1u)];",
	} {
		if !strings.Contains(hlsl, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, hlsl)
		}
	}
	if strings.Contains(hlsl, "AView") || strings.Contains(hlsl, "CView") {
		t.Fatalf("matrix view aliases should not emit as HLSL locals:\n%s", hlsl)
	}
}

func TestEmitRegTileToLocalArrayHLSL(t *testing.T) {
	hlsl := emitSource(t, `shader RegTileBasic {
resources { C: readwrite array<f32>; }
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
var Acc: reg_tile<f32, 2u, 2u> = reg_tile_zero();
let CView: matrix_view<f32> = row_major(C, 2u, 2u);
Acc[0u, 0u] = Acc[0u, 0u] + 1.0;
Acc[1u, 1u] = Acc[1u, 1u] + 4.0;
CView[1u, 1u] = Acc[1u, 1u];
return;
}
}`)
	for _, want := range []string{
		"float Acc[4];",
		"Acc[0] = 0.0;",
		"Acc[3] = 0.0;",
		"Acc[((0u) * (2)) + (0u)] = (Acc[((0u) * (2)) + (0u)] + 1.0);",
		"C[((1u) * (2u)) + (1u)] = Acc[((1u) * (2)) + (1u)];",
	} {
		if !strings.Contains(hlsl, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, hlsl)
		}
	}
	if strings.Contains(hlsl, "reg_tile") {
		t.Fatalf("HLSL should not mention reg_tile:\n%s", hlsl)
	}
}

func TestEmitGuardedMemoryAccessToSafeHLSL(t *testing.T) {
	hlsl := emitSource(t, `shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
stage compute [numthreads(1, 1, 1)] fn CS(row: u32, col: u32, guard: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
let CView: matrix_view<f32> = row_major(C, 4u, 4u);
let value: f32 = read AView[row, col] when guard and not false else 0.0;
write CView[row, col] = value when row < 4u and col < 4u;
return;
}
}`)
	for _, want := range []string{
		"float __sdslv_guarded_read_0 = 0.0;",
		"if ((guard && !false))",
		"__sdslv_guarded_read_0 = A[((row) * (4u)) + (col)];",
		"float value = __sdslv_guarded_read_0;",
		"if (((row < 4u) && (col < 4u)))",
		"C[((row) * (4u)) + (col)] = value;",
	} {
		if !strings.Contains(hlsl, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, hlsl)
		}
	}
	for _, banned := range []string{"read AView", "write CView"} {
		if strings.Contains(hlsl, banned) {
			t.Fatalf("HLSL should not contain source guarded spelling %q:\n%s", banned, hlsl)
		}
	}
}

func TestEmitGuardedReadAssignmentMaterializesValueBeforeTileStore(t *testing.T) {
	hlsl := emitSource(t, `shader S {
resources { A: readonly array<f32>; }
workgroup Tile: tile<f32, 4u, 4u>;
stage compute [numthreads(1, 1, 1)] fn CS(row: u32, col: u32, guard: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
Tile[row, col] = read AView[row, col] when guard else 7.0;
return;
}
}`)
	for _, want := range []string{
		"float __sdslv_guarded_read_0 = 7.0;",
		"if (guard)",
		"__sdslv_guarded_read_0 = A[((row) * (4u)) + (col)];",
		"Tile[((row) * (4)) + (col)] = __sdslv_guarded_read_0;",
	} {
		if !strings.Contains(hlsl, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, hlsl)
		}
	}
	if strings.Contains(hlsl, "if (guard)\n    {\n        Tile[") {
		t.Fatalf("guarded read must not lower as a conditional tile store:\n%s", hlsl)
	}
}

func TestEmitGuardWhenToIfElseIfHLSL(t *testing.T) {
	hlsl := emitSource(t, `shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
stage compute [numthreads(1, 1, 1)] fn CS(row: u32, col: u32, full: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
let CView: matrix_view<f32> = row_major(C, 4u, 4u);
when {
case full -> {
write CView[row, col] = AView[row, col] when row < 4u and col < 4u;
}
case not full -> {
let value: f32 = read AView[row, col] when row < 4u and col < 4u else 0.0;
write CView[row, col] = value when true;
}
else -> {
return;
}
}
return;
}
}`)
	for _, want := range []string{
		"if (full)",
		"else if (!full)",
		"else",
		"float __sdslv_guarded_read_0 = 0.0;",
		"float value = __sdslv_guarded_read_0;",
		"C[((row) * (4u)) + (col)] = value;",
	} {
		if !strings.Contains(hlsl, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, hlsl)
		}
	}
	if strings.Contains(hlsl, "when {") || strings.Contains(hlsl, "case full") {
		t.Fatalf("HLSL should not contain source guard when spelling:\n%s", hlsl)
	}
}

func TestEmitFlowStateBlocksAsOrdinaryStructuredHLSL(t *testing.T) {
	hlsl := emitSource(t, `board LoadCoord {
row: u32;
col: u32;
}
fn Make(row: u32, col: u32) -> LoadCoord {
return LoadCoord { row: row; col: col; };
}
shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
workgroup Tile: tile<f32, 4u, 4u>;
stage compute [numthreads(1, 1, 1)] fn CS(fullTile: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
let CView: matrix_view<f32> = row_major(C, 4u, 4u);
flow TileLoad {
state Load {
comptime for lane in 0u..1u {
let p: LoadCoord = Make(lane, lane);
when {
case fullTile -> {
Tile[p.row, p.col] = AView[p.row, p.col];
}
else -> {
write CView[p.row, p.col] = read AView[p.row, p.col] when true else 0.0 when true;
}
}
}
}
state Sync {
WorkgroupMemoryBarrierWithSync();
}
}
return;
}
}`)
	for _, want := range []string{
		"if (fullTile)",
		"GroupMemoryBarrierWithGroupSync();",
	} {
		if !strings.Contains(hlsl, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, hlsl)
		}
	}
	for _, banned := range []string{"flow TileLoad", "state Load", "state Sync"} {
		if strings.Contains(hlsl, banned) {
			t.Fatalf("HLSL should not contain %q:\n%s", banned, hlsl)
		}
	}
}

func TestEmitComptimeSelectedBranchOnly(t *testing.T) {
	hlsl := emitSource(t, `shader Demo {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let UseFastPath: bool = true;
comptime if UseFastPath {
let selected: u32 = 7u;
} else {
let dropped: u32 = 99u;
}
return;
}
}`)
	if strings.Contains(hlsl, "comptime") || strings.Contains(hlsl, "dropped") || strings.Contains(hlsl, "99u") {
		t.Fatalf("HLSL should contain only selected branch:\n%s", hlsl)
	}
	if !strings.Contains(hlsl, "uint selected = 7u;") {
		t.Fatalf("HLSL missing selected branch:\n%s", hlsl)
	}
}

func TestEmitComptimeMatchSelectedBranchOnly(t *testing.T) {
	hlsl := emitSource(t, `shader Demo {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let TileK: u32 = 16u;
comptime match TileK {
8u => {
let dropped: u32 = 8u;
}
16u => {
comptime let LocalK: u32 = 16u;
let selected: u32 = LocalK;
}
else => {
let fallback: u32 = 0u;
}
}
return;
}
}`)
	for _, banned := range []string{"comptime", "dropped", "fallback", "8u", "0u"} {
		if strings.Contains(hlsl, banned) {
			t.Fatalf("HLSL should not contain %q:\n%s", banned, hlsl)
		}
	}
	if !strings.Contains(hlsl, "uint selected = 16u;") {
		t.Fatalf("HLSL missing selected branch:\n%s", hlsl)
	}
}

func TestEmitComptimeWhenUtilitySelectedCaseOnly(t *testing.T) {
	hlsl := emitSource(t, `shader Demo {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let TileK: u32 = 16u;
comptime when utility {
case Vector4 when TileK == 16u score 100 {
comptime let LocalK: u32 = TileK;
let selected: u32 = LocalK;
}
case Scalar score 10 {
let dropped: u32 = 1u;
}
else {
let fallback: u32 = 0u;
}
}
return;
}
}`)
	for _, banned := range []string{"comptime", "dropped", "fallback", "1u", "0u"} {
		if strings.Contains(hlsl, banned) {
			t.Fatalf("HLSL should not contain %q:\n%s", banned, hlsl)
		}
	}
	if !strings.Contains(hlsl, "uint selected = 16u;") {
		t.Fatalf("HLSL missing selected branch:\n%s", hlsl)
	}
}

func TestEmitComptimeForExpandedOnly(t *testing.T) {
	hlsl := emitSource(t, `concept MicroConfig {
Outputs: { M: u32 = 2u; N: u32 = 2u; };
}
config Micro2x2: MicroConfig {}
template<C: MicroConfig>
shader Demo {
resources { C: readwrite array<f32>; }
stage compute [numthreads(1, 1, 1)] fn CS(row: u32, col: u32) -> void {
var Acc: reg_tile<f32, C.Outputs.M, C.Outputs.N> = reg_tile_zero();
let CView: matrix_view<f32> = row_major(C, 4u, 4u);
comptime for i in 0u..C.Outputs.M {
comptime for j in 0u..C.Outputs.N {
Acc[i, j] = Acc[i, j] + 1.0;
CView[row + i, col + j] = Acc[i, j];
}
}
return;
}
}
compile Demo<Micro2x2> as Demo2x2;`)
	if strings.Contains(hlsl, "comptime") {
		t.Fatalf("HLSL should not mention comptime:\n%s", hlsl)
	}
	for _, want := range []string{
		"Acc[((0u) * (2)) + (0u)] = (Acc[((0u) * (2)) + (0u)] + 1.0);",
		"Acc[((1u) * (2)) + (1u)] = (Acc[((1u) * (2)) + (1u)] + 1.0);",
		"C[(((row + 0u)) * (4u)) + ((col + 1u))] = Acc[((0u) * (2)) + (1u)];",
		"C[(((row + 1u)) * (4u)) + ((col + 0u))] = Acc[((1u) * (2)) + (0u)];",
	} {
		if !strings.Contains(hlsl, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, hlsl)
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

func TestEmitSemanticBooleanOperatorsAsHLSLPunctuation(t *testing.T) {
	out := emitSource(t, `shader Demo {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let a: bool = true;
let b: bool = false;
let c: bool = a and not b or false;
if c and not false {
return;
}
return;
}
}`)
	for _, want := range []string{
		"bool c = ((a && !b) || false);",
		"if ((c && !false))",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, " and ") || strings.Contains(out, " or ") {
		t.Fatalf("HLSL should use punctuation operators:\n%s", out)
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
let tile: Tile = Tile { Acc0: 0.0; };
return tile with { Acc0: A[thread.DispatchId.x] };
}
}`
	out := emitSource(t, text)
	for _, want := range []string{
		"struct ComputeThread",
		"struct Tile",
		"[[vk::binding(0, 0)]] StructuredBuffer<float> A;",
		"[[vk::binding(1, 0)]] RWStructuredBuffer<float> C;",
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
		"[[vk::binding(2, 0)]] StructuredBuffer<float> A;",
		"[[vk::binding(0, 0)]] RWStructuredBuffer<float> C;",
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
var sink: f32 = 0.0;
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

func TestSdslvM35aIntrinsicsEmitNativeHLSL(t *testing.T) {
	out := emitSource(t, `fn F(bits: u32, raw: u32, value: f32, fv4: float4) -> void {
  let lane: f32 = fv4.z;
  let unpacked: float2 = Unpack<F16x2>(bits);
  let repacked: u32 = Pack<F16x2>(unpacked);
  let dotValue: f32 = Dot(unpacked, Unpack<F16x2>(repacked));
  let bitPattern: u32 = Bitcast<u32>(value);
  let asFloat: f32 = Bitcast<f32>(bitPattern);
  let converted: f32 = Convert<f32>(raw);
  return;
}`)
	for _, want := range []string{
		"float lane = fv4.z;",
		"float2 unpacked = f16tof32(uint2((bits & 65535u), (bits >> 16u)));",
		"uint repacked = (f32tof16(unpacked).x | (f32tof16(unpacked).y << 16u));",
		"float dotValue = dot(unpacked, f16tof32(uint2((repacked & 65535u), (repacked >> 16u))));",
		"uint bitPattern = asuint(value);",
		"float asFloat = asfloat(bitPattern);",
		"float converted = float(raw);",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HLSL missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "BEGIN INLINE HLSL") {
		t.Fatalf("M35a intrinsic lowering fell back to inline HLSL:\n%s", out)
	}
}

func TestSdslvPackF16x2MaterializesOperandExactlyOnce(t *testing.T) {
	out := emitSource(t, `fn MakePair(seed: f32) -> float2 {
  return HLSL<float2>(seed) {
    return float2(seed, seed + 1.0);
  };
}
fn F(seed: f32) -> u32 {
  return Pack<F16x2>(MakePair(seed));
}`)
	if strings.Count(out, "MakePair(seed)") != 1 {
		t.Fatalf("Pack<F16x2> duplicated its operand:\n%s", out)
	}
	if strings.Count(out, "f32tof16(__sdslv_operand_") != 2 {
		t.Fatalf("Pack<F16x2> did not reuse a single materialized operand:\n%s", out)
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
