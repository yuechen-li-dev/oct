package validate

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestModuleRejectsDuplicateRecordFields(t *testing.T) {
	err := validateSource(`record Params { M: u32; M: u32; }`)
	if err == nil || !strings.Contains(err.Error(), "duplicate record field") {
		t.Fatalf("error = %v, want duplicate record field", err)
	}
}

func TestModuleRejectsUnknownTypes(t *testing.T) {
	err := validateSource(`record Params { M: Missing; }`)
	if err == nil || !strings.Contains(err.Error(), "unknown type Missing") {
		t.Fatalf("error = %v, want unknown type", err)
	}
}

func TestModuleRejectsBadReturnType(t *testing.T) {
	err := validateSource(`fn F() -> u32 { return 1.0; }`)
	if err == nil || !strings.Contains(err.Error(), "return type mismatch") {
		t.Fatalf("error = %v, want return type mismatch", err)
	}
}

func TestModuleRejectsUnsupportedStage(t *testing.T) {
	err := validateSource(`shader S { stage vertex fn VS() -> void { return; } }`)
	if err == nil || !strings.Contains(err.Error(), "not implemented in GoOct SDSL-V M0") {
		t.Fatalf("error = %v, want M0 diagnostic", err)
	}
}

func TestModuleRejectsDuplicateStreamFields(t *testing.T) {
	err := validateSource(`stream ComputeThread { GroupIndex: u32; GroupIndex: u32; }`)
	if err == nil || !strings.Contains(err.Error(), "duplicate stream field") {
		t.Fatalf("error = %v, want duplicate stream field", err)
	}
}

func TestModuleRejectsUnknownWithField(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn F(s: Surface) -> Surface { return s with { Missing: 0.5 }; }`)
	if err == nil || !strings.Contains(err.Error(), "unknown with field Missing") {
		t.Fatalf("error = %v, want unknown with field", err)
	}
}

func TestModuleRejectsDuplicateEnumVariant(t *testing.T) {
	err := validateSource(`enum LoadValue { Zero; Zero; }`)
	if err == nil || !strings.Contains(err.Error(), "duplicate enum variant") {
		t.Fatalf("error = %v, want duplicate enum variant", err)
	}
}

func TestModuleRejectsDuplicateEnumPayloadField(t *testing.T) {
	err := validateSource(`enum LoadValue { Value { X: f32; X: f32; } }`)
	if err == nil || !strings.Contains(err.Error(), "duplicate enum payload field") {
		t.Fatalf("error = %v, want duplicate enum payload field", err)
	}
}

func TestModuleRejectsEnumPayloadConstructionShapeErrors(t *testing.T) {
	err := validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F() -> LoadValue { return LoadValue.Value; }`)
	if err == nil || !strings.Contains(err.Error(), "requires payload construction") {
		t.Fatalf("error = %v, want payload construction diagnostic", err)
	}
	err = validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F() -> LoadValue { return LoadValue.Value { Y: 1.0 }; }`)
	if err == nil || !strings.Contains(err.Error(), "unknown payload field Y") {
		t.Fatalf("error = %v, want unknown payload field diagnostic", err)
	}
	err = validateSource(`enum Bounds { Tail { Rows: u32; Cols: u32; } }
fn F() -> Bounds { return Bounds.Tail { Rows: 1u }; }`)
	if err == nil || !strings.Contains(err.Error(), "missing payload field Cols") {
		t.Fatalf("error = %v, want missing payload field diagnostic", err)
	}
}

func TestModuleRejectsEnumMatchErrors(t *testing.T) {
	err := validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 {
return match v {
LoadValue.Zero => 0.0
};
}`)
	if err == nil || !strings.Contains(err.Error(), "match missing arm") {
		t.Fatalf("error = %v, want missing arm diagnostic", err)
	}
	err = validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 {
return match v {
LoadValue.Zero => 0.0
LoadValue.Zero => 1.0
};
}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate match arm") {
		t.Fatalf("error = %v, want duplicate match arm diagnostic", err)
	}
	err = validateSource(`enum LoadValue { Zero; Value { X: f32; } }
enum Other { Zero; }
fn F(v: LoadValue) -> f32 {
return match v {
Other.Zero => 0.0
LoadValue.Value(payload) => payload.X
};
}`)
	if err == nil || !strings.Contains(err.Error(), "does not match subject enum") {
		t.Fatalf("error = %v, want wrong enum diagnostic", err)
	}
}

func TestModuleRejectsEnumMatchTypeAndScopeErrors(t *testing.T) {
	err := validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 {
return match v {
LoadValue.Zero => 0.0
LoadValue.Value(payload) => true
};
}`)
	if err == nil || !strings.Contains(err.Error(), "uniform type") {
		t.Fatalf("error = %v, want arm type mismatch diagnostic", err)
	}
	err = validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 {
let out: f32 = match v {
LoadValue.Zero => payload.X
LoadValue.Value(payload) => payload.X
};
return out;
}`)
	if err == nil || !strings.Contains(err.Error(), "unknown identifier payload") {
		t.Fatalf("error = %v, want payload scope diagnostic", err)
	}
	err = validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 {
let out: f32 = match v {
LoadValue.Zero(payload) => 0.0
LoadValue.Value(bound) => bound.X
};
return out;
}`)
	if err == nil || !strings.Contains(err.Error(), "must not bind payload") {
		t.Fatalf("error = %v, want simple variant binding diagnostic", err)
	}
}

func TestModuleRejectsNestedMatchPlacement(t *testing.T) {
	err := validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 { return G(match v { LoadValue.Zero => 0.0 LoadValue.Value(payload) => payload.X }); }
fn G(x: f32) -> f32 { return x; }`)
	if err == nil || !strings.Contains(err.Error(), "match expression is only supported as a direct let initializer, assignment RHS, or return value") {
		t.Fatalf("error = %v, want match placement diagnostic", err)
	}
}

func TestModuleRejectsDuplicateWithField(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn F(s: Surface) -> Surface { return s with { Roughness: 0.5, Roughness: 1.0 }; }`)
	if err == nil || !strings.Contains(err.Error(), "duplicate with field Roughness") {
		t.Fatalf("error = %v, want duplicate with field", err)
	}
}

func TestModuleRejectsWrongWithFieldType(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn F(s: Surface) -> Surface { return s with { Roughness: true }; }`)
	if err == nil || !strings.Contains(err.Error(), "with field Roughness expects f32, got bool") {
		t.Fatalf("error = %v, want wrong with field type", err)
	}
}

func TestModuleRejectsRecordParameterFieldAssignment(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn Bad(s: Surface) -> Surface { s.Roughness = 0.5; return s; }`)
	if err == nil || !strings.Contains(err.Error(), "use with instead") {
		t.Fatalf("error = %v, want immutable record parameter diagnostic", err)
	}
}

func TestModuleRejectsStreamParameterFieldAssignment(t *testing.T) {
	err := validateSource(`stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
fn Bad(t: ComputeThread) -> u32 { t.DispatchId.x = 1u; return 0u; }`)
	if err == nil || !strings.Contains(err.Error(), "immutable stream parameter t") {
		t.Fatalf("error = %v, want immutable stream parameter diagnostic", err)
	}
}

func TestModuleAllowsLocalRecordFieldAssignment(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn Good(s: Surface) -> Surface {
let local: Surface = s;
local.Roughness = 0.5;
return local;
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleResolvesNamedResourceBundleStream(t *testing.T) {
	err := validateSource(`stream VectorAddIO {
A: readonly array<f32>;
C: readwrite array<f32>;
}
record Params { Count: u32; }
shader VectorAdd {
resources VectorAddIO;
stage compute [numthreads(16, 1, 1)] fn CS(params: Params) -> void { return; }
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsWithInUnsupportedNestedPosition(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn G(s: Surface) -> Surface { return F(s with { Roughness: 0.5 }); }
fn F(s: Surface) -> Surface { return s; }`)
	if err == nil || !strings.Contains(err.Error(), "with expression is only supported as a direct let initializer, assignment RHS, or return value") {
		t.Fatalf("error = %v, want with placement diagnostic", err)
	}
}

func TestModuleAllowsWorkgroupArrayUseInComputeShader(t *testing.T) {
	err := validateSource(`stream ComputeThread {
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
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsRuntimeSizedWorkgroupArray(t *testing.T) {
	err := validateSource(`shader S {
workgroup Tile: array<f32>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}`)
	if err == nil || !strings.Contains(err.Error(), "fixed-size array") {
		t.Fatalf("error = %v, want fixed-size workgroup diagnostic", err)
	}
}

func TestModuleRejectsDuplicateWorkgroupNames(t *testing.T) {
	err := validateSource(`shader S {
workgroup Tile: array<f32, 16>;
workgroup Tile: array<f32, 16>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate shader workgroup") {
		t.Fatalf("error = %v, want duplicate workgroup diagnostic", err)
	}
}

func TestModuleRejectsInvalidWorkgroupElementType(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
shader S {
workgroup Tile: array<Surface, 16>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}`)
	if err == nil || !strings.Contains(err.Error(), "element type") {
		t.Fatalf("error = %v, want invalid workgroup element diagnostic", err)
	}
}

func TestModuleRejectsBarrierWrongArgCount(t *testing.T) {
	err := validateSource(`shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
WorkgroupBarrier(1u);
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "expects 0 arguments") {
		t.Fatalf("error = %v, want barrier arg count diagnostic", err)
	}
}

func TestModuleRejectsBarrierInValuePosition(t *testing.T) {
	err := validateSource(`shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let x: void = WorkgroupBarrier();
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "may only be used as an expression statement") {
		t.Fatalf("error = %v, want barrier placement diagnostic", err)
	}
}

func TestModuleRejectsWrongWorkgroupAssignmentType(t *testing.T) {
	err := validateSource(`shader S {
workgroup Tile: array<f32, 16>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
Tile[0u] = true;
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "assignment type mismatch") {
		t.Fatalf("error = %v, want workgroup assignment mismatch", err)
	}
}

func TestModuleAllowsTemplateConfigCompileSpecialization(t *testing.T) {
	err := validateSource(`concept TileCopyConfig {
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
workgroup Tile: array<f32, C.TILE_SIZE>;
stage compute [numthreads(C.THREADS_X, C.THREADS_Y, 1u)] fn CS() -> void {
let tileElements: u32 = C.TILE_SIZE;
return;
}
}
compile TileCopy<Tile16x16> as TileCopy16x16;`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleAllowsSumAndProductReductions(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = sum i in 0u..4u { values[i] };
let productValue: f32 = product j in 1..4 step 2 { values[j] };
return total + productValue;
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsReductionValidationErrors(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = sum i in 0.0..4u { values[i] };
return total;
}`)
	if err == nil || !strings.Contains(err.Error(), "sum reduction bounds must be integer") {
		t.Fatalf("error = %v, want integer bounds diagnostic", err)
	}
	err = validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = sum i in 0u..4u step 0 { values[i] };
return total;
}`)
	if err == nil || !strings.Contains(err.Error(), "sum reduction step must be a positive integer literal") {
		t.Fatalf("error = %v, want positive step diagnostic", err)
	}
	err = validateSource(`fn Reduce() -> f32 {
let total: f32 = sum i in 0u..4u { true };
return total;
}`)
	if err == nil || !strings.Contains(err.Error(), "sum reduction body must be numeric") {
		t.Fatalf("error = %v, want numeric body diagnostic", err)
	}
}

func TestModuleRejectsNestedReductionPlacement(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
return 1.0 + sum i in 0u..4u { values[i] };
}`)
	if err == nil || !strings.Contains(err.Error(), "reduction expression is only supported as a direct let initializer, assignment RHS, or return value") {
		t.Fatalf("error = %v, want reduction placement diagnostic", err)
	}
}

func TestModuleRejectsReductionIndexOutOfScopeAndDeferredMax(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = sum i in 0u..4u { values[i] };
return i;
}`)
	if err == nil || !strings.Contains(err.Error(), "unknown identifier i") {
		t.Fatalf("error = %v, want index scope diagnostic", err)
	}
	err = validateSource(`fn Reduce(values: array<f32>) -> f32 {
return max i in 0u..4u { values[i] };
}`)
	if err == nil || !strings.Contains(err.Error(), "max reduction is reserved but not yet implemented") {
		t.Fatalf("error = %v, want deferred max diagnostic", err)
	}
}

func TestModuleRejectsConfigMissingField(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; TILE_N: u32; }
config Tile16: TileConfig { TILE_M: 16u; }`)
	if err == nil || !strings.Contains(err.Error(), "config field TILE_N missing") {
		t.Fatalf("error = %v, want missing config field", err)
	}
}

func TestModuleRejectsConfigExtraField(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; }
config Tile16: TileConfig { TILE_M: 16u; TILE_N: 16u; }`)
	if err == nil || !strings.Contains(err.Error(), "unknown config field Tile16.TILE_N") {
		t.Fatalf("error = %v, want extra config field", err)
	}
}

func TestModuleRejectsUnknownTemplateField(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; }
template<C: TileConfig>
shader TileCopy {
workgroup Tile: array<f32, 16>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let x: u32 = C.TILE_N;
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "unknown template field TILE_N") {
		t.Fatalf("error = %v, want unknown template field", err)
	}
}

func TestModuleRejectsCompileTargetNotTemplate(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; }
config Tile16: TileConfig { TILE_M: 16u; }
shader TileCopy { stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; } }
compile TileCopy<Tile16> as TileCopy16;`)
	if err == nil || !strings.Contains(err.Error(), "must be a template shader") {
		t.Fatalf("error = %v, want compile target template diagnostic", err)
	}
}

func TestModuleRejectsCompileConfigWrongConcept(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; }
concept OtherConfig { TILE_M: u32; }
config Tile16: OtherConfig { TILE_M: 16u; }
template<C: TileConfig>
shader TileCopy { stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; } }
compile TileCopy<Tile16> as TileCopy16;`)
	if err == nil || !strings.Contains(err.Error(), "does not satisfy concept") {
		t.Fatalf("error = %v, want wrong concept diagnostic", err)
	}
}

func TestModuleRejectsCompileAliasCollision(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; }
config Tile16: TileConfig { TILE_M: 16u; }
template<C: TileConfig>
shader TileCopy { stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; } }
record TileCopy16 {}
compile TileCopy<Tile16> as TileCopy16;`)
	if err == nil || !strings.Contains(err.Error(), "collides with top-level declaration") {
		t.Fatalf("error = %v, want alias collision diagnostic", err)
	}
}

func TestModuleValidatesConceptRequirementsAndStaticAsserts(t *testing.T) {
	err := validateSource(`stream IO {
[binding(0)] A: readonly array<f32>;
[binding(1)] C: readwrite array<f32>;
}
concept TileConfig {
THREADS_X: u32;
THREADS_Y: u32;
TILE_SIZE: u32;
require THREADS_X > 0u;
require TILE_SIZE == THREADS_X * THREADS_Y;
}
config Tile16x16: TileConfig {
THREADS_X: 16u;
THREADS_Y: 16u;
TILE_SIZE: 256u;
}
template<C: TileConfig>
shader TileCopy {
resources IO;
static assert C.TILE_SIZE <= 1024u;
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
[unroll]
for i in 0u..1u { return; }
return;
}
}
compile TileCopy<Tile16x16> as TileCopy16x16;`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsFailedConceptRequirement(t *testing.T) {
	err := validateSource(`concept TileConfig {
TILE_SIZE: u32;
require TILE_SIZE > 0u;
}
config Bad: TileConfig {
TILE_SIZE: 0u;
}`)
	if err == nil || !strings.Contains(err.Error(), "failed requirement") {
		t.Fatalf("error = %v, want failed requirement", err)
	}
}

func TestModuleRejectsStaticAssertFailure(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_SIZE: u32; }
config Tile16: TileConfig { TILE_SIZE: 2048u; }
template<C: TileConfig>
shader TileCopy {
static assert C.TILE_SIZE <= 1024u;
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}
compile TileCopy<Tile16> as TileCopy16;`)
	if err == nil || !strings.Contains(err.Error(), "failed static assert") {
		t.Fatalf("error = %v, want failed static assert", err)
	}
}

func TestModuleRejectsConflictingLoopAttributes(t *testing.T) {
	err := validateSource(`shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
[unroll][loop]
for i in 0u..1u { return; }
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "both [unroll] and [loop]") {
		t.Fatalf("error = %v, want loop attribute conflict", err)
	}
}

func TestModuleRejectsDuplicateExplicitBindings(t *testing.T) {
	err := validateSource(`shader S {
resources {
[binding(0)] A: readonly array<f32>;
[binding(0)] C: readwrite array<f32>;
}
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate explicit binding 0") {
		t.Fatalf("error = %v, want duplicate binding diagnostic", err)
	}
}

func TestModuleRejectsUnknownAttribute(t *testing.T) {
	err := validateSource(`shader S {
resources {
[mystery] A: readonly array<f32>;
}
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}`)
	if err == nil || !strings.Contains(err.Error(), "unknown attribute") {
		t.Fatalf("error = %v, want unknown attribute diagnostic", err)
	}
}

func validateSource(text string) error {
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: text})
	if err != nil {
		return err
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return err
	}
	return Module(module)
}
