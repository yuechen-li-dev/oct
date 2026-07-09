package lower

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestModuleLowersVectorAddToVDMIR(t *testing.T) {
	mir := lowerSource(t, `namespace Prometheus.Kernels;
record VectorParams { Count: u32; }
shader VectorAdd {
resources {
    A: readonly array<f32>;
    B: readonly array<f32>;
    C: readwrite array<f32>;
}
fn PickScale(useDouble: bool) -> f32 {
    let scale: f32 = when utility {
        case 2.0 when useDouble score 2.0
        case 1.0 when true score 1.0
        else 1.0
    };
    return scale;
}
stage compute [numthreads(16, 16, 1)] fn CS(params: VectorParams) -> void {
    let index: u32 = DispatchThreadID.x;
    if index < params.Count {
        let scale: f32 = PickScale(false);
        C[index] = (A[index] + B[index]) * scale;
    }
    return;
}
}`)

	if got := len(mir.Resources); got != 3 {
		t.Fatalf("len(Resources) = %d, want 3", got)
	}
	if mir.Resources[0].Access != vdmir.ResourceReadOnly || mir.Resources[2].Access != vdmir.ResourceReadWrite {
		t.Fatalf("resource access = %#v", mir.Resources)
	}
	if got := len(mir.EntryPoints); got != 1 {
		t.Fatalf("len(EntryPoints) = %d, want 1", got)
	}
	entry := mir.EntryPoints[0]
	if entry.EmittedName != "VectorAdd_CS" || entry.NumThreadsX != 16 || entry.NumThreadsY != 16 || entry.NumThreadsZ != 1 {
		t.Fatalf("entry = %#v", entry)
	}
	if !entry.Builtins[0].Referenced {
		t.Fatalf("DispatchThreadID should be marked referenced: %#v", entry.Builtins)
	}
	cs := findFunction(t, mir, "VectorAdd_CS")
	letIndex, ok := cs.Body.Statements[0].(vdmir.LetStmt)
	if !ok {
		t.Fatalf("first statement = %T, want LetStmt", cs.Body.Statements[0])
	}
	field, ok := letIndex.Value.(vdmir.FieldAccessExpr)
	if !ok {
		t.Fatalf("index init = %T, want FieldAccessExpr", letIndex.Value)
	}
	target, ok := field.Target.(vdmir.VarRefExpr)
	if !ok || target.Kind != vdmir.VarBuiltin || target.Name != "DispatchThreadID" || field.Field != "x" {
		t.Fatalf("builtin field access = %#v / %#v", target, field)
	}
	pick := findFunction(t, mir, "VectorAdd_PickScale")
	pickLet, ok := pick.Body.Statements[0].(vdmir.LetStmt)
	if !ok {
		t.Fatalf("pick first stmt = %T, want LetStmt", pick.Body.Statements[0])
	}
	if _, ok := pickLet.Value.(vdmir.WhenUtilityExpr); !ok {
		t.Fatalf("pick let value = %T, want WhenUtilityExpr", pickLet.Value)
	}
}

func TestModuleLowersComputeThreadResourceBundleAndWith(t *testing.T) {
	mir := lowerSource(t, `namespace Prometheus.Kernels;
stream ComputeThread {
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
fn Adjust(tile: Tile, value: f32) -> Tile {
    return tile with { Acc0: tile.Acc0 + value };
}
stage compute [numthreads(16, 1, 1)] fn CS(thread: ComputeThread, params: Params) -> Tile {
    let tile: Tile = TileZero();
    return tile with { Acc0: A[thread.DispatchId.x] };
}
fn TileZero() -> Tile {
    let tile: Tile;
    tile.Acc0 = 0.0;
    return tile;
}
}`)
	if got := len(mir.Streams); got != 2 {
		t.Fatalf("len(Streams) = %d, want 2", got)
	}
	if got := len(mir.Resources); got != 2 {
		t.Fatalf("len(Resources) = %d, want 2", got)
	}
	if mir.Resources[0].BundleName != "VectorAddIO" {
		t.Fatalf("resource bundle = %#v", mir.Resources[0])
	}
	entry := mir.EntryPoints[0]
	if len(entry.ThreadParams) != 1 || entry.ThreadParams[0].ParamName != "thread" {
		t.Fatalf("thread params = %#v", entry.ThreadParams)
	}
	if !entry.Builtins[0].Referenced {
		t.Fatalf("DispatchThreadID should be referenced via ComputeThread: %#v", entry.Builtins)
	}
	cs := findFunction(t, mir, "VectorAdd_CS")
	ret, ok := cs.Body.Statements[1].(vdmir.ReturnStmt)
	if !ok {
		t.Fatalf("return stmt = %T, want ReturnStmt", cs.Body.Statements[1])
	}
	if _, ok := ret.Value.(vdmir.WithExpr); !ok {
		t.Fatalf("return value = %T, want WithExpr", ret.Value)
	}
}

func TestVDMIRDumpIsDeterministic(t *testing.T) {
	mir := lowerSource(t, `namespace Prometheus.Kernels;
record VectorParams { Count: u32; }
shader VectorAdd {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
stage compute [numthreads(16, 16, 1)] fn CS(params: VectorParams) -> void {
    let index: u32 = DispatchThreadID.x;
    return;
}
}`)
	first := vdmir.Dump(mir)
	second := vdmir.Dump(mir)
	if first != second {
		t.Fatalf("dump is not deterministic")
	}
	for _, want := range []string{
		"vdmir module Prometheus.Kernels",
		"resource readonly A: array<f32>",
		"resource readwrite C: array<f32>",
		"entry compute VectorAdd_CS numthreads(16,16,1)",
		"builtin DispatchThreadID: uint3 semantic SV_DispatchThreadID referenced=true",
	} {
		if !strings.Contains(first, want) {
			t.Fatalf("dump missing %q:\n%s", want, first)
		}
	}
}

func TestModuleLowersWorkgroupsAndBarriersToVDMIR(t *testing.T) {
	mir := lowerSource(t, `stream ComputeThread {
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
if idx < params.Count {
Tile[local] = A[idx];
}
WorkgroupMemoryBarrierWithSync();
if idx < params.Count {
C[idx] = Tile[local];
}
return;
}
}`)
	if got := len(mir.Workgroups); got != 1 {
		t.Fatalf("len(Workgroups) = %d, want 1", got)
	}
	if mir.Workgroups[0].Name != "Tile" || mir.Workgroups[0].Length != 256 {
		t.Fatalf("workgroup = %#v", mir.Workgroups[0])
	}
	cs := findFunction(t, mir, "TileCopy_CS")
	exprStmt, ok := cs.Body.Statements[3].(vdmir.ExprStmt)
	if !ok {
		t.Fatalf("stmt[3] = %T, want ExprStmt", cs.Body.Statements[3])
	}
	if intrinsic, ok := exprStmt.Value.(vdmir.IntrinsicCallExpr); !ok || intrinsic.Intrinsic != vdmir.IntrinsicWorkgroupMemoryBarrierWithSync {
		t.Fatalf("expr stmt = %#v, want WorkgroupMemoryBarrierWithSync intrinsic", exprStmt.Value)
	}
}

func TestModuleMonomorphizesTemplateShaderToConcreteVDMIR(t *testing.T) {
	mir := lowerSource(t, `stream ComputeThread {
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
let tileElements: u32 = C.TILE_SIZE;
if idx < params.Count {
Tile[local] = A[idx];
}
WorkgroupMemoryBarrierWithSync();
[unroll]
for i in 0u..1u {
if idx < params.Count {
C[idx] = Tile[local];
}
}
return;
}
}
compile TileCopy<Tile16x16> as TileCopy16x16;`)
	if got := len(mir.EntryPoints); got != 1 {
		t.Fatalf("len(EntryPoints) = %d, want 1", got)
	}
	entry := mir.EntryPoints[0]
	if entry.EmittedName != "TileCopy16x16_CS" || entry.NumThreadsX != 16 || entry.NumThreadsY != 16 || entry.NumThreadsZ != 1 {
		t.Fatalf("entry = %#v", entry)
	}
	if len(entry.ConfigValues) != 3 {
		t.Fatalf("len(ConfigValues) = %d, want 3", len(entry.ConfigValues))
	}
	if len(entry.Metadata) != 0 {
		t.Fatalf("len(Metadata) = %d, want 0", len(entry.Metadata))
	}
	if got := len(mir.Workgroups); got != 1 {
		t.Fatalf("len(Workgroups) = %d, want 1", got)
	}
	if mir.Workgroups[0].Length != 256 {
		t.Fatalf("workgroup length = %d, want 256", mir.Workgroups[0].Length)
	}
	if findFunctionOptional(mir, "TileCopy_CS").EmittedName != "" {
		t.Fatalf("template entry should not be emitted")
	}
	cs := findFunction(t, mir, "TileCopy16x16_CS")
	letTileElements, ok := cs.Body.Statements[2].(vdmir.LetStmt)
	if !ok {
		t.Fatalf("stmt[2] = %T, want LetStmt", cs.Body.Statements[2])
	}
	lit, ok := letTileElements.Value.(vdmir.LiteralExpr)
	if !ok || lit.Value != "256u" {
		t.Fatalf("tileElements value = %#v, want 256u literal", letTileElements.Value)
	}
	loop, ok := cs.Body.Statements[5].(vdmir.ForRangeStmt)
	if !ok || loop.LoopHint != vdmir.LoopHintUnroll {
		t.Fatalf("stmt[5] = %#v, want unrolled for loop", cs.Body.Statements[5])
	}
}

func TestModuleLowersDispatchMetadataFromConfigConvention(t *testing.T) {
	mir := lowerSource(t, `stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
record Params { M: u32; N: u32; K: u32; }
concept KernelConfig {
THREADS_X: u32;
THREADS_Y: u32;
OUTPUTS_PER_INVOCATION_M: u32;
OUTPUTS_PER_INVOCATION_N: u32;
TILE_M: u32;
TILE_N: u32;
UNROLL_K: u32;
}
config Rect: KernelConfig {
THREADS_X: 4u;
THREADS_Y: 2u;
OUTPUTS_PER_INVOCATION_M: 3u;
OUTPUTS_PER_INVOCATION_N: 5u;
TILE_M: 12u;
TILE_N: 10u;
UNROLL_K: 7u;
}
template<C: KernelConfig>
shader Kernel {
stage compute [numthreads(C.THREADS_X, C.THREADS_Y, 1u)] fn CS(thread: ComputeThread, params: Params) -> void {
return;
}
}
compile Kernel<Rect> as KernelRect;`)
	entry := mir.EntryPoints[0]
	if got := len(entry.Metadata); got != 5 {
		t.Fatalf("len(Metadata) = %d, want 5", got)
	}
	if got := entry.Metadata[0]; got.Name != "OUTPUTS_PER_INVOCATION_M" || got.Value != 3 {
		t.Fatalf("metadata[0] = %#v", got)
	}
	if got := entry.Metadata[4]; got.Name != "UNROLL_K" || got.Value != 7 {
		t.Fatalf("metadata[4] = %#v", got)
	}
	if got := len(entry.ConfigValues); got != 7 {
		t.Fatalf("len(ConfigValues) = %d, want 7", got)
	}
	if got := entry.ConfigValues[0]; got.Name != "OUTPUTS_PER_INVOCATION_M" || got.Value != 3 {
		t.Fatalf("config[0] = %#v", got)
	}
	if got := entry.ConfigValues[6]; got.Name != "UNROLL_K" || got.Value != 7 {
		t.Fatalf("config[6] = %#v", got)
	}
}

func TestModuleLowersLoopHintsAndExplicitBindings(t *testing.T) {
	mir := lowerSource(t, `shader S {
resources {
[binding(2)] A: readonly array<f32>;
C: readwrite array<f32>;
}
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
[unroll]
for i in 0u..1u {
return;
}
return;
}
}`)
	if mir.Resources[0].Binding.Binding != 2 || !mir.Resources[0].Binding.Explicit {
		t.Fatalf("resource[0].Binding = %#v", mir.Resources[0].Binding)
	}
	if mir.Resources[1].Binding.Binding != 0 || mir.Resources[1].Binding.Explicit {
		t.Fatalf("resource[1].Binding = %#v", mir.Resources[1].Binding)
	}
	cs := findFunction(t, mir, "S_CS")
	loop, ok := cs.Body.Statements[0].(vdmir.ForRangeStmt)
	if !ok {
		t.Fatalf("stmt[0] = %T, want ForRangeStmt", cs.Body.Statements[0])
	}
	if loop.LoopHint != vdmir.LoopHintUnroll {
		t.Fatalf("loop hint = %q, want unroll", loop.LoopHint)
	}
}

func lowerSource(t *testing.T, text string) vdmir.Module {
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
	mir, err := Module(module)
	if err != nil {
		t.Fatalf("Module() error = %v", err)
	}
	return mir
}

func findFunction(t *testing.T, mir vdmir.Module, emittedName string) vdmir.Function {
	t.Helper()
	for _, fn := range mir.Functions {
		if fn.EmittedName == emittedName {
			return fn
		}
	}
	t.Fatalf("function %s not found", emittedName)
	return vdmir.Function{}
}

func findFunctionOptional(mir vdmir.Module, emittedName string) vdmir.Function {
	for _, fn := range mir.Functions {
		if fn.EmittedName == emittedName {
			return fn
		}
	}
	return vdmir.Function{}
}
