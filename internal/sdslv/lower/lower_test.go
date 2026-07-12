package lower

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestSdslvInlineHlslRejectsUnsupportedLoweringTarget(t *testing.T) {
	result, err := lex.Analyze(source.File{Path: "inline.sdslv", Text: `fn F() -> void { HLSL { GroupMemoryBarrierWithGroupSync(); } return; }`})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Module(module); err != nil {
		t.Fatal(err)
	}
	_, err = ModuleForTarget(module, "MSL")
	if err == nil || !strings.Contains(err.Error(), "contains inline HLSL and cannot be lowered to target MSL") {
		t.Fatalf("want target constraint diagnostic, got %v", err)
	}
}

func TestSdslvTensorAssignLowersToVDMIRWithCanonicalOrder(t *testing.T) {
tokens, err := lex.Analyze(source.File{Path: "tensor.sdslv", Text: `fn Dot(A: array<array<f32, 3u>, 2u>, B: array<f32, 3u>) -> void {
  var C: array<f32, 2u> = Fill(0.0);
  tensor C[i] = Sum[k](A[i, k] * B[k]);
  return;
}`})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Module(module); err != nil {
		t.Fatal(err)
	}
	mir, err := Module(module)
	if err != nil {
		t.Fatal(err)
	}
	assign, ok := mir.Functions[0].Body.Statements[1].(vdmir.TensorAssign)
	if !ok {
		t.Fatalf("statement = %T, want TensorAssign", mir.Functions[0].Body.Statements[1])
	}
	if len(assign.FreeIndices) != 1 || assign.FreeIndices[0].Name != "i" || assign.FreeIndices[0].Extent != 2 {
		t.Fatalf("free indices = %#v", assign.FreeIndices)
	}
	if !assign.SourceSpan.Known() || !assign.FreeIndices[0].Span.Known() {
		t.Fatalf("tensor source spans were not preserved: %#v", assign)
	}
	reduction, ok := assign.Value.(vdmir.TensorReductionExpr)
	if !ok || len(reduction.Indices) != 1 || reduction.Indices[0].Name != "k" || reduction.Indices[0].Extent != 3 {
		t.Fatalf("reduction = %#v", assign.Value)
	}
	if !reduction.Span.Known() || !reduction.Indices[0].Span.Known() {
		t.Fatalf("reduction source spans were not preserved: %#v", reduction)
	}
}

func TestSdslvTensorCompoundAssignmentRetainsSingularUpdate(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "tensor_add.sdslv", Text: `shader S {
workgroup A: tile<f32, 2u, 2u>;
workgroup B: tile<f32, 2u, 2u>;
stage compute [numthreads(1, 1, 1)] fn Add() -> void {
  tensor A[i, j] += B[i, j];
  return;
}}`})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Module(module); err != nil {
		t.Fatal(err)
	}
	mir, err := Module(module)
	if err != nil {
		t.Fatal(err)
	}
	assign, ok := mir.Functions[0].Body.Statements[0].(vdmir.TensorAssign)
	if !ok || assign.AssignmentKind != vdmir.TensorAssignAdd || assign.AliasPolicy != vdmir.TensorAliasNoDestinationRead {
		t.Fatalf("assign = %#v", assign)
	}
}

func TestSdslvFixedArrayIndexCarriesCompilerOwnedRowMajorShape(t *testing.T) {
tokens, err := lex.Analyze(source.File{Path: "rank4.sdslv", Text: `fn Rank4(A: array<array<array<array<f32, 5u>, 4u>, 3u>, 2u>) -> void {
  var B: array<array<array<array<f32, 5u>, 4u>, 3u>, 2u> = Fill(0.0);
  tensor B[i, j, k, l] = A[i, j, k, l];
  return;
}`})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Module(module); err != nil {
		t.Fatal(err)
	}
	mir, err := Module(module)
	if err != nil {
		t.Fatal(err)
	}
	assign := mir.Functions[0].Body.Statements[1].(vdmir.TensorAssign)
	destination, ok := assign.Destination.(vdmir.IndexNExpr)
	if !ok {
		t.Fatalf("destination = %T, want IndexNExpr", assign.Destination)
	}
	want := []uint32{2, 3, 4, 5}
	if destination.Layout != vdmir.IndexLayoutRowMajorLinear || !slices.Equal(destination.Extents, want) {
		t.Fatalf("destination layout = %q %#v, want row-major %#v", destination.Layout, destination.Extents, want)
	}
	rhs, ok := assign.Value.(vdmir.IndexNExpr)
	if !ok || rhs.Layout != vdmir.IndexLayoutRowMajorLinear || !slices.Equal(rhs.Extents, want) {
		t.Fatalf("RHS = %#v, want row-major rank-4 IndexNExpr", assign.Value)
	}
}

func TestSdslvNdarrayLowersToDistinctTypeAndSourceOrderedLiteral(t *testing.T) {
tokens, err := lex.Analyze(source.File{Path: "ndarray.sdslv", Text: `fn F() -> void {
  let A: ndarray<u32, [2u, 3u]> = [1u, 2u, 3u, 4u, 5u, 6u];
  var B: ndarray<u32, [2u, 3u]> = Fill(0u);
  tensor B[i, j] = A[i, j];
  return;
}`})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Module(module); err != nil {
		t.Fatal(err)
	}
	mir, err := Module(module)
	if err != nil {
		t.Fatal(err)
	}
	localA := mir.Functions[0].Locals[0]
	if localA.Type.Kind != vdmir.TypeNDArray || !slices.Equal(localA.Type.Shape, []uint32{2, 3}) || localA.Type.ArraySize != 6 {
		t.Fatalf("local A type = %#v", localA.Type)
	}
	letA := mir.Functions[0].Body.Statements[0].(vdmir.LetStmt)
	literal, ok := letA.Value.(vdmir.NDArrayLiteral)
	if !ok || len(literal.Elements) != 6 {
		t.Fatalf("literal = %#v", letA.Value)
	}
	for i, want := range []uint32{1, 2, 3, 4, 5, 6} {
		value, ok := literal.Elements[i].(vdmir.LiteralExpr)
		if !ok || value.Value != fmt.Sprintf("%du", want) {
			t.Fatalf("element[%d] = %#v, want %d", i, literal.Elements[i], want)
		}
	}
	assign := mir.Functions[0].Body.Statements[2].(vdmir.TensorAssign)
	destination := assign.Destination.(vdmir.IndexNExpr)
	if destination.Layout != vdmir.IndexLayoutRowMajorLinear || !slices.Equal(destination.Extents, []uint32{2, 3}) {
		t.Fatalf("destination = %#v", destination)
	}
}

func TestSdslvLowersFixedShapeConstruction(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "construct.sdslv", Text: `fn F() -> void {
  let A: ndarray<u32, [2u]> = Fill(9u);
  let B: ndarray<u32, [2u, 3u]> = Generate[i, j](i * 3u + j);
  return;
}`})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Module(module); err != nil {
		t.Fatal(err)
	}
	mir, err := Module(module)
	if err != nil {
		t.Fatal(err)
	}
	stmts := mir.Functions[0].Body.Statements
	if _, ok := stmts[0].(vdmir.LetStmt).Value.(vdmir.FillConstruct); !ok {
		t.Fatalf("Fill MIR = %#v", stmts[0])
	}
	generated, ok := stmts[1].(vdmir.LetStmt).Value.(vdmir.GenerateConstruct)
	if !ok || !slices.Equal(generated.Binders, []string{"i", "j"}) {
		t.Fatalf("Generate MIR = %#v", stmts[1])
	}
}

func TestSdslvAssertCallsLowerToDedicatedVDMIRStatements(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "asserts.sdslvtest", Text: `[Fact]
fn Assertions() -> void {
  Assert.True(true, "embedded SDSL-V fixture must preserve its asserted invariant");
  Assert.False(false, "embedded SDSL-V fixture must preserve its asserted invariant");
  Assert.Equal(1u, 1u, "embedded SDSL-V fixture must preserve its asserted invariant");
  Assert.NotEqual(1, 2, "embedded SDSL-V fixture must preserve its asserted invariant");
  Assert.Near(1.0, 1.0, 0.1, "embedded SDSL-V fixture must preserve its asserted invariant");
}`})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	tests, diagnostics := validate.ValidatedTests(module)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
	mir, err := ModuleForTests(module, tests, "HLSL")
	if err != nil {
		t.Fatal(err)
	}
	fn := findFunction(t, mir, "Assertions")
	if len(fn.Body.Statements) != 5 {
		t.Fatalf("statements=%#v", fn.Body.Statements)
	}
	want := []vdmir.AssertKind{vdmir.AssertTrue, vdmir.AssertFalse, vdmir.AssertEqual, vdmir.AssertNotEqual, vdmir.AssertNear}
	for i, kind := range want {
		op, ok := fn.Body.Statements[i].(vdmir.AssertStmt)
		if !ok || op.Kind != kind || op.LexicalIndex != i || !op.CallSpan.Known() {
			t.Fatalf("assert[%d]=%#v", i, fn.Body.Statements[i])
		}
	}
	near := fn.Body.Statements[4].(vdmir.AssertStmt)
	if near.Expected == nil || near.Actual == nil || near.Tolerance == nil || near.ValueKind != vdmir.AssertValueFloat {
		t.Fatalf("near=%#v", near)
	}
	if dump := vdmir.Dump(mir); !strings.Contains(dump, "assert Assert.Near lexical=4") {
		t.Fatalf("VD-MIR dump omits dedicated assert operation:\n%s", dump)
	}
}

func TestSdslvAssertOperandsRemainSingleVDMIRExpressionsInSourceOrder(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "once.sdslvtest", Text: `[Fact]
[TestInputUInt(10u, 20u)]
fn Once() -> void {
  let counter: u32 = 0u;
  Assert.Equal(HLSL<u32>(counter) { counter = counter + 1u; return 7u; }, HLSL<u32>(counter) { counter = counter + 1u; return 7u; }, "embedded SDSL-V fixture must preserve its asserted invariant");
  Assert.Near(HLSL<f32>(counter) { counter = counter + 1u; return 1.0; }, HLSL<f32>(counter) { counter = counter + 1u; return 1.0; }, HLSL<f32>(counter) { counter = counter + 1u; return 0.0; }, "embedded SDSL-V fixture must preserve its asserted invariant");
  Assert.Equal(20u, read TestInput.UInt[1u] when 1u < TestInput.Length else 99u, "embedded SDSL-V fixture must preserve its asserted invariant");
}`})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	tests, diagnostics := validate.ValidatedTests(module)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
	mir, err := ModuleForTests(module, tests, "HLSL")
	if err != nil {
		t.Fatal(err)
	}
	fn := findFunction(t, mir, "Once")
	if got := len(fn.Body.Statements); got != 4 {
		t.Fatalf("statements=%#v", fn.Body.Statements)
	}
	equal := fn.Body.Statements[1].(vdmir.AssertStmt)
	if _, ok := equal.Expected.(vdmir.ForeignShaderExpr); !ok {
		t.Fatalf("Equal expected = %T, want ForeignShaderExpr", equal.Expected)
	}
	if _, ok := equal.Actual.(vdmir.ForeignShaderExpr); !ok {
		t.Fatalf("Equal actual = %T, want ForeignShaderExpr", equal.Actual)
	}
	near := fn.Body.Statements[2].(vdmir.AssertStmt)
	if _, ok := near.Expected.(vdmir.ForeignShaderExpr); !ok {
		t.Fatalf("Near expected = %T, want ForeignShaderExpr", near.Expected)
	}
	if _, ok := near.Actual.(vdmir.ForeignShaderExpr); !ok {
		t.Fatalf("Near actual = %T, want ForeignShaderExpr", near.Actual)
	}
	if _, ok := near.Tolerance.(vdmir.ForeignShaderExpr); !ok {
		t.Fatalf("Near tolerance = %T, want ForeignShaderExpr", near.Tolerance)
	}
	guarded := fn.Body.Statements[3].(vdmir.AssertStmt)
	if _, ok := guarded.Actual.(vdmir.GuardedReadExpr); !ok {
		t.Fatalf("guarded actual = %T, want GuardedReadExpr", guarded.Actual)
	}
	if equal.LexicalIndex != 0 || near.LexicalIndex != 1 || guarded.LexicalIndex != 2 {
		t.Fatalf("lexical order changed: %d %d %d", equal.LexicalIndex, near.LexicalIndex, guarded.LexicalIndex)
	}
}

func TestModuleForTestsLowersTestInputToHiddenIndexedResource(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "input.sdslvtest", Text: `[Fact]
[TestInputInt(-7, 9)]
fn ReadInput() -> void {
  let index: u32 = 1u;
  let valid: bool = index < TestInput.Length;
  let value: i32 = read TestInput.Int[index] when valid else 0;
  Assert.Equal(9, value, "embedded SDSL-V fixture must preserve its asserted invariant");
}`})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	tests, diagnostics := validate.ValidatedTests(module)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
	mir, err := ModuleForTests(module, tests, "HLSL")
	if err != nil {
		t.Fatal(err)
	}
	fn := findFunction(t, mir, "ReadInput")
	letValue := fn.Body.Statements[2].(vdmir.LetStmt)
	guarded, ok := letValue.Value.(vdmir.GuardedReadExpr)
	if !ok {
		t.Fatalf("value = %T, want GuardedReadExpr", letValue.Value)
	}
	target, ok := guarded.Target.(vdmir.IndexExpr)
	if !ok {
		t.Fatalf("guarded target = %T, want IndexExpr", guarded.Target)
	}
	resource, ok := target.Target.(vdmir.VarRefExpr)
	if !ok || resource.Name != vdmir.TestInputResourceName || target.Type().Kind != vdmir.TypeI32 {
		t.Fatalf("target = %#v", target)
	}
	if dump := vdmir.Dump(mir); !strings.Contains(dump, "resource readonly __sdslv_test_input: array<u32>") || !strings.Contains(dump, "let valid: bool = (index < 2u)") {
		t.Fatalf("dump missing hidden test input path:\n%s", dump)
	}
}

func TestModuleForTestsLowersTensorBodiesInsideSdslvTests(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "tensor_suite.sdslvtest", Text: `[Fact]
[TestInputUInt(5u, 7u, 11u)]
fn GuardedTensor() -> void {
  var indices: array<u32, 4u> = Fill(0u);
  indices[0u] = 1u;
  indices[1u] = 2u;
  indices[2u] = 3u;
  indices[3u] = 0u;
  var output: array<u32, 4u> = Fill(0u);
  tensor output[i] = read TestInput.UInt[indices[i]] when indices[i] < TestInput.Length else 99u;
  Assert.Equal(7u, output[0u], "embedded SDSL-V fixture must preserve its asserted invariant");
}`})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	tests, diagnostics := validate.ValidatedTests(module)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
	mir, err := ModuleForTests(module, tests, "HLSL")
	if err != nil {
		t.Fatal(err)
	}
	fn := findFunction(t, mir, "GuardedTensor")
	if got := len(fn.Body.Statements); got != 8 {
		t.Fatalf("statements=%#v", fn.Body.Statements)
	}
	if _, ok := fn.Body.Statements[6].(vdmir.TensorAssign); !ok {
		t.Fatalf("statement[6]=%T, want TensorAssign", fn.Body.Statements[6])
	}
	if _, ok := fn.Body.Statements[7].(vdmir.AssertStmt); !ok {
		t.Fatalf("statement[7]=%T, want AssertStmt", fn.Body.Statements[7])
	}
	if !slices.ContainsFunc(mir.Resources, func(r vdmir.Resource) bool { return r.Name == vdmir.TestInputResourceName }) {
		t.Fatalf("hidden TestInput resource missing: %#v", mir.Resources)
	}
}

func TestModuleLowersM35aVectorComponentsAndIntrinsics(t *testing.T) {
	mir := lowerSource(t, `fn F(bits: u32, raw: u32, value: f32, fv4: float4) -> void {
let lane: f32 = fv4.z;
let unpacked: float2 = Unpack<F16x2>(bits);
let repacked: u32 = Pack<F16x2>(unpacked);
let dotValue: f32 = Dot(unpacked, Unpack<F16x2>(repacked));
let bitPattern: u32 = Bitcast<u32>(value);
let asFloat: f32 = Bitcast<f32>(bitPattern);
let converted: f32 = Convert<f32>(raw);
return;
}`)
	fn := findFunction(t, mir, "F")
	stmts := fn.Body.Statements
	lane := stmts[0].(vdmir.LetStmt)
	extract, ok := lane.Value.(vdmir.VectorExtractExpr)
	if !ok || extract.Component != "z" || extract.Type().Kind != vdmir.TypeF32 {
		t.Fatalf("lane extract = %#v", lane.Value)
	}
	unpack := stmts[1].(vdmir.LetStmt).Value.(vdmir.IntrinsicCallExpr)
	if unpack.Intrinsic != vdmir.IntrinsicUnpackF16x2 || unpack.Type().Kind != vdmir.TypeFloat2 || unpack.TypeArgument.Name != "F16x2" {
		t.Fatalf("unpack = %#v", unpack)
	}
	pack := stmts[2].(vdmir.LetStmt).Value.(vdmir.IntrinsicCallExpr)
	if pack.Intrinsic != vdmir.IntrinsicPackF16x2 || pack.Type().Kind != vdmir.TypeU32 || pack.TypeArgument.Name != "F16x2" {
		t.Fatalf("pack = %#v", pack)
	}
	dotValue := stmts[3].(vdmir.LetStmt).Value.(vdmir.IntrinsicCallExpr)
	if dotValue.Intrinsic != vdmir.IntrinsicDot || dotValue.Type().Kind != vdmir.TypeF32 || len(dotValue.Arguments) != 2 {
		t.Fatalf("dot = %#v", dotValue)
	}
	bitcast := stmts[4].(vdmir.LetStmt).Value.(vdmir.IntrinsicCallExpr)
	if bitcast.Intrinsic != vdmir.IntrinsicBitcast || bitcast.Type().Kind != vdmir.TypeU32 || bitcast.TypeArgument.Name != "u32" {
		t.Fatalf("bitcast = %#v", bitcast)
	}
	convert := stmts[6].(vdmir.LetStmt).Value.(vdmir.IntrinsicCallExpr)
	if convert.Intrinsic != vdmir.IntrinsicConvert || convert.Type().Kind != vdmir.TypeF32 || convert.TypeArgument.Name != "f32" {
		t.Fatalf("convert = %#v", convert)
	}
}

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
	field, ok := letIndex.Value.(vdmir.VectorExtractExpr)
	if !ok {
		t.Fatalf("index init = %T, want VectorExtractExpr", letIndex.Value)
	}
	target, ok := field.Target.(vdmir.VarRefExpr)
	if !ok || target.Kind != vdmir.VarBuiltin || target.Name != "DispatchThreadID" || field.Component != "x" {
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

func TestModuleLowersBoardValuesThroughVDMIR(t *testing.T) {
	mir := lowerSource(t, `board LoadCoord {
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
	if got := len(mir.Boards); got != 1 {
		t.Fatalf("len(Boards) = %d, want 1", got)
	}
	helper := findFunction(t, mir, "MakeLoadCoord")
	ret, ok := helper.Body.Statements[1].(vdmir.ReturnStmt)
	if !ok {
		t.Fatalf("helper stmt[1] = %T, want ReturnStmt", helper.Body.Statements[1])
	}
	if _, ok := ret.Value.(vdmir.BoardConstructExpr); !ok {
		t.Fatalf("helper return = %T, want BoardConstructExpr", ret.Value)
	}
	cs := findFunction(t, mir, "S_CS")
	letP, ok := cs.Body.Statements[2].(vdmir.LetStmt)
	if !ok {
		t.Fatalf("CS stmt[2] = %T, want LetStmt", cs.Body.Statements[2])
	}
	if letP.Type.Kind != vdmir.TypeBoard || letP.Type.Name != "LoadCoord" {
		t.Fatalf("board local type = %#v", letP.Type)
	}
	if !strings.Contains(vdmir.Dump(mir), "board LoadCoord") {
		t.Fatalf("VD-MIR dump missing board declaration:\n%s", vdmir.Dump(mir))
	}
}

func TestModuleLowersDeriveToOrderedVDMIR(t *testing.T) {
	mir := lowerSource(t, `record LoadFacts {
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
	fn := findFunction(t, mir, "Make")
	ret, ok := fn.Body.Statements[0].(vdmir.ReturnStmt)
	if !ok {
		t.Fatalf("stmt[0] = %T, want ReturnStmt", fn.Body.Statements[0])
	}
	derive, ok := ret.Value.(vdmir.DeriveExpr)
	if !ok {
		t.Fatalf("return value = %T, want DeriveExpr", ret.Value)
	}
	if len(derive.Fields) != 3 {
		t.Fatalf("len(derive.Fields) = %d, want 3", len(derive.Fields))
	}
	if derive.Fields[0].TempName == derive.Fields[1].TempName || derive.Fields[1].TempName == derive.Fields[2].TempName {
		t.Fatalf("derive temp names must be unique: %#v", derive.Fields)
	}
	rowExpr, ok := derive.Fields[1].Value.(vdmir.BinaryExpr)
	if !ok {
		t.Fatalf("row value = %T, want BinaryExpr", derive.Fields[1].Value)
	}
	ref, ok := rowExpr.Left.(vdmir.VarRefExpr)
	if !ok || ref.Name != derive.Fields[0].TempName {
		t.Fatalf("row should reference first derive temp, got %#v", rowExpr.Left)
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
    return Tile { Acc0: 0.0; };
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

func TestModuleLowersTileAndMatrixViewsToVDMIR(t *testing.T) {
	mir := lowerSource(t, `shader S {
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
	if got := len(mir.Workgroups); got != 1 {
		t.Fatalf("len(Workgroups) = %d, want 1", got)
	}
	if !mir.Workgroups[0].IsTile || mir.Workgroups[0].Rows != 16 || mir.Workgroups[0].Cols != 8 || mir.Workgroups[0].Length != 128 {
		t.Fatalf("workgroup = %#v, want tile 16x8", mir.Workgroups[0])
	}
	cs := findFunction(t, mir, "S_CS")
	letView, ok := cs.Body.Statements[0].(vdmir.LetStmt)
	if !ok {
		t.Fatalf("stmt[0] = %T, want LetStmt", cs.Body.Statements[0])
	}
	if _, ok := letView.Value.(vdmir.RowMajorViewExpr); !ok {
		t.Fatalf("let value = %T, want RowMajorViewExpr", letView.Value)
	}
	assign := cs.Body.Statements[2].(vdmir.AssignStmt)
	if _, ok := assign.Target.(vdmir.Index2DExpr); !ok {
		t.Fatalf("assign target = %T, want Index2DExpr", assign.Target)
	}
}

func TestModuleLowersRegTileToVDMIR(t *testing.T) {
	mir := lowerSource(t, `concept MicroConfig {
Outputs: {
M: u32 = 2u;
N: u32 = 2u;
};
}
config Micro2x2: MicroConfig {}
template<C: MicroConfig>
shader S {
resources { C: readwrite array<f32>; }
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let RM: u32 = C.Outputs.M;
comptime let RN: u32 = C.Outputs.N;
var Acc: reg_tile<f32, RM, RN> = reg_tile_zero();
let CView: matrix_view<f32> = row_major(C, 2u, 2u);
Acc[1u, 0u] = Acc[1u, 0u] + 3.0;
CView[1u, 0u] = Acc[1u, 0u];
return;
}
}
compile S<Micro2x2> as S2x2;`)
	cs := findFunction(t, mir, "S2x2_CS")
	letAcc, ok := cs.Body.Statements[0].(vdmir.LetStmt)
	if !ok {
		t.Fatalf("stmt[0] = %T, want LetStmt", cs.Body.Statements[0])
	}
	if letAcc.Type.Kind != vdmir.TypeRegTile || letAcc.Type.Rows != 2 || letAcc.Type.Cols != 2 {
		t.Fatalf("let type = %#v, want 2x2 reg_tile", letAcc.Type)
	}
	if _, ok := letAcc.Value.(vdmir.RegTileZeroExpr); !ok {
		t.Fatalf("let value = %T, want RegTileZeroExpr", letAcc.Value)
	}
	assign := cs.Body.Statements[2].(vdmir.AssignStmt)
	if _, ok := assign.Target.(vdmir.Index2DExpr); !ok {
		t.Fatalf("assign target = %T, want Index2DExpr", assign.Target)
	}
}

func TestModuleLowersGuardedMemoryAccessToVDMIR(t *testing.T) {
	mir := lowerSource(t, `shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
stage compute [numthreads(1, 1, 1)] fn CS(row: u32, col: u32, guard: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
let CView: matrix_view<f32> = row_major(C, 4u, 4u);
let value: f32 = read AView[row, col] when guard else 0.0;
write CView[row, col] = value when row < 4u and col < 4u;
return;
}
}`)
	cs := findFunction(t, mir, "S_CS")
	letValue, ok := cs.Body.Statements[2].(vdmir.LetStmt)
	if !ok {
		t.Fatalf("stmt[2] = %T, want LetStmt", cs.Body.Statements[2])
	}
	guardedRead, ok := letValue.Value.(vdmir.GuardedReadExpr)
	if !ok {
		t.Fatalf("let value = %T, want GuardedReadExpr", letValue.Value)
	}
	if _, ok := guardedRead.Target.(vdmir.Index2DExpr); !ok {
		t.Fatalf("guarded read target = %T, want Index2DExpr", guardedRead.Target)
	}
	if got := vdmir.FormatExpr(guardedRead); !strings.Contains(got, "guarded_read") {
		t.Fatalf("FormatExpr(guardedRead) = %q, want normalized guarded_read form", got)
	}
	writeStmt, ok := cs.Body.Statements[3].(vdmir.GuardedWriteStmt)
	if !ok {
		t.Fatalf("stmt[3] = %T, want GuardedWriteStmt", cs.Body.Statements[3])
	}
	if _, ok := writeStmt.Target.(vdmir.Index2DExpr); !ok {
		t.Fatalf("guarded write target = %T, want Index2DExpr", writeStmt.Target)
	}
}

func TestModuleLowersGuardWhenToIfChain(t *testing.T) {
	mir := lowerSource(t, `shader S {
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
	cs := findFunction(t, mir, "S_CS")
	ifStmt, ok := cs.Body.Statements[2].(vdmir.IfStmt)
	if !ok {
		t.Fatalf("stmt[2] = %T, want IfStmt", cs.Body.Statements[2])
	}
	if _, ok := ifStmt.ThenBody.Statements[0].(vdmir.GuardedWriteStmt); !ok {
		t.Fatalf("then stmt = %T, want GuardedWriteStmt", ifStmt.ThenBody.Statements[0])
	}
	if ifStmt.ElseBody == nil || len(ifStmt.ElseBody.Statements) != 1 {
		t.Fatalf("missing nested else-if block: %#v", ifStmt.ElseBody)
	}
	nested, ok := ifStmt.ElseBody.Statements[0].(vdmir.IfStmt)
	if !ok {
		t.Fatalf("else stmt = %T, want nested IfStmt", ifStmt.ElseBody.Statements[0])
	}
	if _, ok := nested.Condition.(vdmir.UnaryExpr); !ok {
		t.Fatalf("nested condition = %T, want unary not", nested.Condition)
	}
	if nested.ElseBody == nil {
		t.Fatalf("nested if missing final else")
	}
}

func TestModuleLowersFlowStateBlocksSequentially(t *testing.T) {
	mir := lowerSource(t, `board LoadCoord {
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
	cs := findFunction(t, mir, "S_CS")
	flowBlock, ok := cs.Body.Statements[2].(vdmir.BlockStmt)
	if !ok {
		t.Fatalf("stmt[2] = %T, want BlockStmt", cs.Body.Statements[2])
	}
	if got := len(flowBlock.Body.Statements); got != 3 {
		t.Fatalf("len(flow block statements) = %d, want 3", got)
	}
	if _, ok := flowBlock.Body.Statements[0].(vdmir.LetStmt); !ok {
		t.Fatalf("flow stmt[0] = %T, want first expanded state let", flowBlock.Body.Statements[0])
	}
	if _, ok := flowBlock.Body.Statements[2].(vdmir.ExprStmt); !ok {
		t.Fatalf("flow stmt[2] = %T, want barrier expr stmt from second state", flowBlock.Body.Statements[2])
	}
	if len(mir.Flows) != 1 {
		t.Fatalf("len(mir.Flows) = %d, want 1", len(mir.Flows))
	}
	flow := mir.Flows[0]
	if flow.Name != "TileLoad" || flow.Entry != 0 || flow.MaxStackDepth != 0 || flow.HasPushPop || flow.HasGoto {
		t.Fatalf("legacy flow metadata = %#v", flow)
	}
	if len(flow.States) != 2 || flow.States[0].Terminator.Kind != vdmir.FlowTerminatorFallthrough || flow.States[0].Terminator.Target != 1 {
		t.Fatalf("flow states = %#v", flow.States)
	}
	if !flow.States[1].HasWorkgroupBarrier {
		t.Fatalf("barrier metadata missing: %#v", flow.States[1])
	}
	dump := vdmir.Dump(mir)
	for _, banned := range []string{"flow ", "state "} {
		if strings.Contains(dump, banned) {
			t.Fatalf("VDMIR should not retain source-level %q:\n%s", banned, dump)
		}
	}
}

func TestModuleLowersM31aTransitionsToFlowProgram(t *testing.T) {
	text := `fn F() -> void { flow F { state A { push Shared; } state B { finish; } state Shared { pop; } } }`
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
	fn := findFunction(t, mir, "F")
	flowBlock, ok := fn.Body.Statements[0].(vdmir.BlockStmt)
	if !ok || len(flowBlock.Body.Statements) != 1 {
		t.Fatalf("flow body = %#v", fn.Body.Statements)
	}
	flowStmt, ok := flowBlock.Body.Statements[0].(vdmir.FlowStmt)
	if !ok {
		t.Fatalf("flow statement = %T, want FlowStmt", flowBlock.Body.Statements[0])
	}
	if !flowStmt.Flow.HasPushPop || flowStmt.Flow.MaxStackDepth != 1 || flowStmt.Flow.States[0].Terminator.ReturnTo != 1 {
		t.Fatalf("flow program = %#v", flowStmt.Flow)
	}
}

func TestModuleLowersFlowBoundMutableBoards(t *testing.T) {
	mir := lowerSource(t, `board LoadCoord {
linear: u32;
row: u32;
valid: bool;
}
shader S {
resources { A: readonly array<f32>; }
workgroup Tile: tile<f32, 4u, 4u>;
stage compute [numthreads(1, 1, 1)] fn CS(flag: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
flow TileLoad {
board Load: LoadCoord = LoadCoord { linear: 0u; row: 0u; valid: false; };
state Compute {
comptime for lane in 0u..2u {
Load.linear = lane;
Load.row = Load.linear;
when {
case flag -> {
Load.valid = true;
}
else -> {
Load.valid = false;
}
}
Tile[Load.row, Load.row] = read AView[Load.row, Load.row] when Load.valid else 0.0;
}
}
}
return;
}
}`)
	cs := findFunction(t, mir, "S_CS")
	flowBlock, ok := cs.Body.Statements[1].(vdmir.BlockStmt)
	if !ok {
		t.Fatalf("stmt[1] = %T, want BlockStmt", cs.Body.Statements[1])
	}
	if _, ok := flowBlock.Body.Statements[0].(vdmir.LetStmt); !ok {
		t.Fatalf("flow stmt[0] = %T, want flow board local let", flowBlock.Body.Statements[0])
	}
	dump := vdmir.Dump(mir)
	for _, want := range []string{
		"let Load: board LoadCoord = LoadCoord { linear: 0u, row: 0u, valid: false }",
		"assign Load.linear = 0u",
		"assign Load.row = Load.linear",
		"assign Load.valid = true",
	} {
		if !strings.Contains(dump, want) {
			t.Fatalf("VD-MIR dump missing %q:\n%s", want, dump)
		}
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
TILE_K: u32;
UNROLL_K: u32;
}
config Rect: KernelConfig {
THREADS_X: 4u;
THREADS_Y: 2u;
OUTPUTS_PER_INVOCATION_M: 3u;
OUTPUTS_PER_INVOCATION_N: 5u;
TILE_M: 12u;
TILE_N: 10u;
TILE_K: 8u;
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
	if got := len(entry.Metadata); got != 6 {
		t.Fatalf("len(Metadata) = %d, want 6", got)
	}
	if got := entry.Metadata[0]; got.Name != "OUTPUTS_PER_INVOCATION_M" || got.Value != 3 {
		t.Fatalf("metadata[0] = %#v", got)
	}
	if got := entry.Metadata[4]; got.Name != "TILE_K" || got.Value != 8 {
		t.Fatalf("metadata[4] = %#v", got)
	}
	if got := entry.Metadata[5]; got.Name != "UNROLL_K" || got.Value != 7 {
		t.Fatalf("metadata[5] = %#v", got)
	}
	if got := len(entry.ConfigValues); got != 8 {
		t.Fatalf("len(ConfigValues) = %d, want 8", got)
	}
	if got := entry.ConfigValues[0]; got.Name != "OUTPUTS_PER_INVOCATION_M" || got.Value != 3 {
		t.Fatalf("config[0] = %#v", got)
	}
	if got := entry.ConfigValues[6]; got.Name != "TILE_N" || got.Value != 10 {
		t.Fatalf("config[6] = %#v", got)
	}
	if got := entry.ConfigValues[7]; got.Name != "UNROLL_K" || got.Value != 7 {
		t.Fatalf("config[7] = %#v", got)
	}
}

func TestModuleLowersStructuredConfigDefaultsAndCanonicalNames(t *testing.T) {
	mir := lowerSource(t, `stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
record Params { Count: u32; }
concept KernelConfig {
Threads: {
X: u32;
Y: u32;
};
OutputsPerInvocation: {
M: u32 = 1u;
N: u32 = 1u;
};
Tile: {
M: u32 = Threads.X * OutputsPerInvocation.M;
N: u32 = Threads.Y * OutputsPerInvocation.N;
K: u32;
};
Unroll: {
K: u32 = Tile.K;
};
}
config Rect: KernelConfig {
Threads.X => 4u;
Threads.Y => 2u;
Tile.K => 8u;
}
template<C: KernelConfig>
shader Kernel {
stage compute [numthreads(C.Threads.X, C.Threads.Y, 1u)] fn CS(thread: ComputeThread, params: Params) -> void {
let tileM: u32 = C.Tile.M;
let unrollK: u32 = C.Unroll.K;
return;
}
}
compile Kernel<Rect> as KernelRect;`)
	entry := mir.EntryPoints[0]
	if got := len(entry.Metadata); got != 6 {
		t.Fatalf("len(Metadata) = %d, want 6", got)
	}
	if got := entry.Metadata[0]; got.Name != "OUTPUTS_PER_INVOCATION_M" || got.Value != 1 {
		t.Fatalf("metadata[0] = %#v", got)
	}
	if got := entry.Metadata[5]; got.Name != "UNROLL_K" || got.Value != 8 {
		t.Fatalf("metadata[5] = %#v", got)
	}
	if got := len(entry.ConfigValues); got != 8 {
		t.Fatalf("len(ConfigValues) = %d, want 8", got)
	}
	if got := entry.ConfigValues[0]; got.Name != "OUTPUTS_PER_INVOCATION_M" || got.Value != 1 {
		t.Fatalf("config[0] = %#v", got)
	}
	if got := entry.ConfigValues[6]; got.Name != "TILE_N" || got.Value != 2 {
		t.Fatalf("config[6] = %#v", got)
	}
	if got := entry.ConfigValues[7]; got.Name != "UNROLL_K" || got.Value != 8 {
		t.Fatalf("config[7] = %#v", got)
	}
	cs := findFunction(t, mir, "KernelRect_CS")
	tileM := cs.Body.Statements[0].(vdmir.LetStmt)
	if lit, ok := tileM.Value.(vdmir.LiteralExpr); !ok || lit.Value != "4u" {
		t.Fatalf("tileM value = %#v, want 4u literal", tileM.Value)
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

func TestModuleLowersReductionExpressionsToVDMIR(t *testing.T) {
	mir := lowerSource(t, `fn Reduce(values: array<f32>) -> f32 {
let total: f32 = sum i in 0u..4u { values[i] };
let productValue: f32 = product j in 1..4 step 2 { values[j] };
return total + productValue;
}`)
	fn := findFunction(t, mir, "Reduce")
	total := fn.Body.Statements[0].(vdmir.LetStmt)
	sumExpr, ok := total.Value.(vdmir.ReductionExpr)
	if !ok {
		t.Fatalf("stmt[0] value = %T, want ReductionExpr", total.Value)
	}
	if sumExpr.Op != vdmir.ReductionSum || sumExpr.Name != "i" {
		t.Fatalf("sum expr = %#v", sumExpr)
	}
	if sumExpr.IndexType.Kind != vdmir.TypeU32 {
		t.Fatalf("sum index type = %#v, want u32", sumExpr.IndexType)
	}
	product := fn.Body.Statements[1].(vdmir.LetStmt)
	productExpr := product.Value.(vdmir.ReductionExpr)
	if productExpr.Op != vdmir.ReductionProduct {
		t.Fatalf("product expr = %#v", productExpr)
	}
	if got := vdmir.FormatExpr(productExpr); !strings.Contains(got, "step 2") {
		t.Fatalf("formatted product reduction missing step: %s", got)
	}
}

func TestModuleLowersSemanticBooleanOperatorsToVDMIRLogicalOps(t *testing.T) {
	mir := lowerSource(t, `shader Demo {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let a: bool = true;
let b: bool = false;
let c: bool = a and not b or false;
return;
}
}`)
	fn := findFunction(t, mir, "Demo_CS")
	letStmt := fn.Body.Statements[2].(vdmir.LetStmt)
	orExpr, ok := letStmt.Value.(vdmir.BinaryExpr)
	if !ok || orExpr.Operator != "||" {
		t.Fatalf("let value = %#v, want logical or", letStmt.Value)
	}
	andExpr, ok := orExpr.Left.(vdmir.BinaryExpr)
	if !ok || andExpr.Operator != "&&" {
		t.Fatalf("or left = %#v, want logical and", orExpr.Left)
	}
	notExpr, ok := andExpr.Right.(vdmir.UnaryExpr)
	if !ok || notExpr.Operator != "!" {
		t.Fatalf("and right = %#v, want logical not", andExpr.Right)
	}
}

func TestModuleLowersReductionLoopHintsToVDMIR(t *testing.T) {
	mir := lowerSource(t, `fn Reduce(values: array<f32>) -> f32 {
let total: f32 = [unroll] sum i in 0u..4u { values[i] };
let productValue: f32 = [loop] product j in 0u..4u { values[j] };
return total + productValue;
}`)
	fn := findFunction(t, mir, "Reduce")
	total := fn.Body.Statements[0].(vdmir.LetStmt).Value.(vdmir.ReductionExpr)
	if total.LoopHint != vdmir.LoopHintUnroll {
		t.Fatalf("sum loop hint = %q, want unroll", total.LoopHint)
	}
	product := fn.Body.Statements[1].(vdmir.LetStmt).Value.(vdmir.ReductionExpr)
	if product.LoopHint != vdmir.LoopHintLoop {
		t.Fatalf("product loop hint = %q, want loop", product.LoopHint)
	}
	if got := vdmir.FormatExpr(total); !strings.Contains(got, "[unroll]sum") {
		t.Fatalf("formatted reduction = %s, want loop hint", got)
	}
}

func TestModuleSpecializesTemplateConstantsInsideReductionExpressions(t *testing.T) {
	mir := lowerSource(t, `concept TileConfig {
TILE_K: u32;
}
config Tile4: TileConfig {
TILE_K: 4u;
}
template<C: TileConfig>
shader S {
fn Reduce(values: array<f32>) -> f32 {
return sum i in 0u..C.TILE_K { values[i] };
}
}
compile S<Tile4> as S4;`)
	fn := findFunction(t, mir, "S4_Reduce")
	ret := fn.Body.Statements[0].(vdmir.ReturnStmt)
	reduction := ret.Value.(vdmir.ReductionExpr)
	if got := vdmir.FormatExpr(reduction); !strings.Contains(got, "0u..4u") {
		t.Fatalf("specialized reduction = %s, want concrete bound", got)
	}
}

func TestModuleLowersPayloadEnumsAndMatchToVDMIR(t *testing.T) {
	mir := lowerSource(t, `enum LoadValue {
Zero;
Value { X: f32; }
}
fn Resolve(v: LoadValue) -> f32 {
let initial: LoadValue = LoadValue.Value { X: 1.0 };
let out: f32 = match v {
LoadValue.Zero => 0.0
LoadValue.Value(payload) => payload.X
};
return out;
}`)
	if got := len(mir.Enums); got != 1 {
		t.Fatalf("len(Enums) = %d, want 1", got)
	}
	if !mir.Enums[0].Variants[1].HasPayload || mir.Enums[0].Variants[1].Payload[0].Name != "X" {
		t.Fatalf("enum payload variant = %#v", mir.Enums[0].Variants[1])
	}
	resolve := findFunction(t, mir, "Resolve")
	initial := resolve.Body.Statements[0].(vdmir.LetStmt)
	if _, ok := initial.Value.(vdmir.EnumConstructExpr); !ok {
		t.Fatalf("initial value = %T, want EnumConstructExpr", initial.Value)
	}
	out := resolve.Body.Statements[1].(vdmir.LetStmt)
	matchExpr, ok := out.Value.(vdmir.MatchExpr)
	if !ok {
		t.Fatalf("out value = %T, want MatchExpr", out.Value)
	}
	if got := len(matchExpr.Arms); got != 2 {
		t.Fatalf("len(Arms) = %d, want 2", got)
	}
	if matchExpr.Arms[1].BindingName != "payload" || matchExpr.Arms[1].BindingType.Name != "LoadValue_ValuePayload" {
		t.Fatalf("payload arm = %#v", matchExpr.Arms[1])
	}
}

func TestModuleExpandsComptimeLetAndIfBeforeVDMIR(t *testing.T) {
	mir := lowerSource(t, `concept TileConfig {
Tile: { M: u32; N: u32; };
UseFastPath: bool = true;
}
config Tile16: TileConfig {
Tile.M => 16u;
Tile.N => 16u;
}
template<C: TileConfig>
shader Demo {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let TileElements: u32 = C.Tile.M * C.Tile.N;
comptime if C.UseFastPath {
static assert TileElements == 256u;
let selected: u32 = TileElements;
} else {
static assert false;
let dropped: u32 = 0u;
}
return;
}
}
compile Demo<Tile16> as Demo16;`)
	fn := findFunction(t, mir, "Demo16_CS")
	if got := len(fn.Body.Statements); got != 2 {
		t.Fatalf("len(statements) = %d, want selected let and return", got)
	}
	letStmt, ok := fn.Body.Statements[0].(vdmir.LetStmt)
	if !ok || letStmt.Name != "selected" {
		t.Fatalf("stmt[0] = %#v, want selected let", fn.Body.Statements[0])
	}
	lit, ok := letStmt.Value.(vdmir.LiteralExpr)
	if !ok || lit.Value != "256u" {
		t.Fatalf("selected value = %#v, want 256u literal", letStmt.Value)
	}
	dump := vdmir.Dump(mir)
	if strings.Contains(dump, "comptime") || strings.Contains(dump, "dropped") {
		t.Fatalf("VDMIR should not contain comptime or dropped branch:\n%s", dump)
	}
}

func TestModuleExpandsComptimeMatchBeforeVDMIR(t *testing.T) {
	mir := lowerSource(t, `concept TileConfig {
Tile: { K: u32; };
UseFastPath: bool = true;
}
config Tile16: TileConfig {
Tile.K => 16u;
UseFastPath => true;
}
template<C: TileConfig>
shader Demo {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let K: u32 = C.Tile.K;
comptime match K {
8u => {
static assert false;
let dropped: u32 = 8u;
}
16u => {
comptime let LocalK: u32 = 16u;
static assert LocalK == C.Tile.K;
let selected: u32 = LocalK;
}
else => {
static assert false;
let fallback: u32 = 0u;
}
}
comptime match C.UseFastPath {
true => {
let fast: u32 = 1u;
}
false => {
static assert false;
let slow: u32 = 0u;
}
}
return;
}
}
compile Demo<Tile16> as Demo16;`)
	fn := findFunction(t, mir, "Demo16_CS")
	if got := len(fn.Body.Statements); got != 3 {
		t.Fatalf("len(statements) = %d, want selected let, fast let, return", got)
	}
	selected, ok := fn.Body.Statements[0].(vdmir.LetStmt)
	if !ok || selected.Name != "selected" {
		t.Fatalf("stmt[0] = %#v, want selected let", fn.Body.Statements[0])
	}
	if lit, ok := selected.Value.(vdmir.LiteralExpr); !ok || lit.Value != "16u" {
		t.Fatalf("selected value = %#v, want 16u literal", selected.Value)
	}
	fast, ok := fn.Body.Statements[1].(vdmir.LetStmt)
	if !ok || fast.Name != "fast" {
		t.Fatalf("stmt[1] = %#v, want fast let", fn.Body.Statements[1])
	}
	dump := vdmir.Dump(mir)
	for _, banned := range []string{"comptime", "dropped", "fallback", "slow"} {
		if strings.Contains(dump, banned) {
			t.Fatalf("VDMIR should not contain %q:\n%s", banned, dump)
		}
	}
}

func TestModuleExpandsComptimeWhenUtilityBeforeVDMIR(t *testing.T) {
	mir := lowerSource(t, `concept TileConfig {
Tile: { K: u32; };
UseVectorizedLoad: bool = true;
BaseScore: u32 = 90u;
}
config Tile16: TileConfig {
Tile.K => 16u;
UseVectorizedLoad => true;
BaseScore => 90u;
}
template<C: TileConfig>
shader Demo {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let K: u32 = C.Tile.K;
comptime when utility {
case IneligibleHigh when K == 8u score 1000 {
static assert false;
let dropped: u32 = 8u;
}
case Vector4 when C.UseVectorizedLoad and K == 16u score C.BaseScore + 10u {
comptime let LocalK: u32 = K;
static assert LocalK == 16u;
let selected: u32 = LocalK;
}
case Scalar score 10 {
static assert false;
let scalar: u32 = 1u;
}
else {
static assert false;
let fallback: u32 = 0u;
}
}
return;
}
}
compile Demo<Tile16> as Demo16;`)
	fn := findFunction(t, mir, "Demo16_CS")
	if got := len(fn.Body.Statements); got != 2 {
		t.Fatalf("len(statements) = %d, want selected let and return", got)
	}
	selected, ok := fn.Body.Statements[0].(vdmir.LetStmt)
	if !ok || selected.Name != "selected" {
		t.Fatalf("stmt[0] = %#v, want selected let", fn.Body.Statements[0])
	}
	if lit, ok := selected.Value.(vdmir.LiteralExpr); !ok || lit.Value != "16u" {
		t.Fatalf("selected value = %#v, want 16u literal", selected.Value)
	}
	dump := vdmir.Dump(mir)
	for _, banned := range []string{"comptime", "dropped", "scalar", "fallback"} {
		if strings.Contains(dump, banned) {
			t.Fatalf("VDMIR should not contain %q:\n%s", banned, dump)
		}
	}
}

func TestModuleExpandsComptimeWhenUtilityElseBeforeVDMIR(t *testing.T) {
	mir := lowerSource(t, `shader Demo {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime when utility {
case Never when false score 100 {
static assert false;
let dropped: u32 = 1u;
}
else {
let selected_else: u32 = 2u;
}
}
return;
}
}`)
	fn := findFunction(t, mir, "Demo_CS")
	selected, ok := fn.Body.Statements[0].(vdmir.LetStmt)
	if !ok || selected.Name != "selected_else" {
		t.Fatalf("stmt[0] = %#v, want else let", fn.Body.Statements[0])
	}
}

func TestModuleAllowsLowerScoreComptimeWhenUtilityTie(t *testing.T) {
	mir := lowerSource(t, `shader Demo {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime when utility {
case LowA score 10 { let lowA: u32 = 1u; }
case LowB score 10 { let lowB: u32 = 2u; }
case Winner score 20 { let selected: u32 = 3u; }
}
return;
}
}`)
	fn := findFunction(t, mir, "Demo_CS")
	selected, ok := fn.Body.Statements[0].(vdmir.LetStmt)
	if !ok || selected.Name != "selected" {
		t.Fatalf("stmt[0] = %#v, want selected let", fn.Body.Statements[0])
	}
}

func TestModuleRejectsComptimeRuntimeDependencies(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "runtime parameter field",
			src: `record Params { M: u32; }
shader S {
stage compute [numthreads(1, 1, 1)] fn CS(params: Params) -> void {
comptime let X: u32 = params.M;
return;
}
}`,
			want: "comptime expression cannot reference runtime parameter `params.M`",
		},
		{
			name: "thread builtin",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime if GroupThreadID.x == 0u { return; }
return;
}
}`,
			want: "comptime expression cannot reference thread builtin `GroupThreadID.x`",
		},
		{
			name: "resource bundle",
			src: `stream IO { A: readonly array<f32>; }
shader S {
resources IO;
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let y: f32 = A[0u];
return;
}
}`,
			want: "comptime expression cannot reference resource `A`",
		},
		{
			name: "runtime local",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let x: u32 = 1u;
comptime let y: u32 = x;
return;
}
}`,
			want: "comptime expression cannot reference runtime local `x`",
		},
		{
			name: "comptime match runtime parameter field",
			src: `record Params { M: u32; }
shader S {
stage compute [numthreads(1, 1, 1)] fn CS(params: Params) -> void {
comptime match params.M {
1u => { return; }
else => { return; }
}
return;
}
}`,
			want: "comptime expression cannot reference runtime parameter `params.M`",
		},
		{
			name: "comptime match thread builtin",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime match GroupThreadID.x {
0u => { return; }
else => { return; }
}
return;
}
}`,
			want: "comptime expression cannot reference thread builtin `GroupThreadID.x`",
		},
		{
			name: "comptime when guard runtime parameter field",
			src: `record Params { M: u32; }
shader S {
stage compute [numthreads(1, 1, 1)] fn CS(params: Params) -> void {
comptime when utility {
case Bad when params.M == 1u score 10 { return; }
else { return; }
}
return;
}
}`,
			want: "comptime when guard cannot reference runtime parameter `params.M`",
		},
		{
			name: "comptime when score runtime local",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let x: u32 = 1u;
comptime when utility {
case Bad score x { return; }
else { return; }
}
return;
}
}`,
			want: "comptime when score cannot reference runtime local `x`",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: tc.src})
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
			_, err = Module(module)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Module() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleRejectsComptimeMatchErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "duplicate integer arm",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime match 16u {
16u => { return; }
16u => { return; }
else => { return; }
}
return;
}
}`,
			want: "duplicate comptime match arm for 16u",
		},
		{
			name: "integer requires else",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime match 16u {
16u => { return; }
}
return;
}
}`,
			want: "comptime match over integer requires else arm",
		},
		{
			name: "bool one arm requires else",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime match true {
true => { return; }
}
return;
}
}`,
			want: "bool comptime match requires else arm",
		},
		{
			name: "unsupported pattern expression",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime match 16u {
8u + 8u => { return; }
else => { return; }
}
return;
}
}`,
			want: "comptime match arm pattern must be compile-time literal",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: tc.src})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			module, err := parse.BuildModule(tokens)
			if err != nil {
				t.Fatalf("BuildModule() error = %v", err)
			}
			validateErr := validate.Module(module)
			if validateErr != nil && strings.Contains(validateErr.Error(), tc.want) {
				return
			}
			if validateErr != nil {
				t.Fatalf("validate.Module() error = %v, want %q", validateErr, tc.want)
			}
			_, err = Module(module)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Module() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleRejectsComptimeWhenUtilityErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "no eligible case and no else",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime when utility {
case Never when false score 10 { return; }
}
return;
}
}`,
			want: "no comptime when utility case qualified and no else block provided",
		},
		{
			name: "tied highest score",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime when utility {
case A score 10 { return; }
case B score 10 { return; }
else { return; }
}
return;
}
}`,
			want: "ambiguous comptime when utility cases A and B have tied score 10",
		},
		{
			name: "duplicate labels",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime when utility {
case Same score 10 { return; }
case Same score 11 { return; }
else { return; }
}
return;
}
}`,
			want: "duplicate comptime when utility case label Same",
		},
		{
			name: "guard non bool",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime when utility {
case Bad when 1u score 10 { return; }
else { return; }
}
return;
}
}`,
			want: "comptime when guard must be compile-time bool",
		},
		{
			name: "score non numeric",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime when utility {
case Bad score true { return; }
else { return; }
}
return;
}
}`,
			want: "comptime when score must be compile-time numeric",
		},
		{
			name: "selected static assert fires",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime when utility {
case Selected score 10 { static assert false; }
case Lower score 1 { return; }
}
return;
}
}`,
			want: "failed static assert false",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: tc.src})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			module, err := parse.BuildModule(tokens)
			if err != nil {
				t.Fatalf("BuildModule() error = %v", err)
			}
			validateErr := validate.Module(module)
			if validateErr != nil && strings.Contains(validateErr.Error(), tc.want) {
				return
			}
			if validateErr != nil && !strings.Contains(tc.want, "duplicate comptime when utility case label") && !strings.Contains(tc.want, "guard must") && !strings.Contains(tc.want, "score must") {
				t.Fatalf("validate.Module() error = %v, want %q", validateErr, tc.want)
			}
			_, err = Module(module)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Module() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleEvaluatesSelectedComptimeStaticAssert(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime if true {
static assert false;
} else {
static assert true;
}
return;
}
}`})
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
	_, err = Module(module)
	if err == nil || !strings.Contains(err.Error(), "failed static assert false") {
		t.Fatalf("Module() error = %v, want selected static assert failure", err)
	}
}

func TestModuleExpandsComptimeForBeforeVDMIR(t *testing.T) {
	mir := lowerSource(t, `concept MicroConfig {
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
	dump := vdmir.Dump(mir)
	if strings.Contains(dump, "comptime") {
		t.Fatalf("VDMIR should not contain comptime for:\n%s", dump)
	}
	for _, want := range []string{
		"assign Acc[0u, 0u] = (Acc[0u, 0u] + 1.0)",
		"assign Acc[1u, 1u] = (Acc[1u, 1u] + 1.0)",
		"assign CView[(row + 0u), (col + 1u)] = Acc[0u, 1u]",
		"assign CView[(row + 1u), (col + 0u)] = Acc[1u, 0u]",
	} {
		if !strings.Contains(dump, want) {
			t.Fatalf("VDMIR missing %q:\n%s", want, dump)
		}
	}
}

func TestModuleAllowsZeroIterationComptimeFor(t *testing.T) {
	mir := lowerSource(t, `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime for i in 2u..2u {
let dropped: u32 = i;
}
return;
}
}`)
	dump := vdmir.Dump(mir)
	if strings.Contains(dump, "dropped") {
		t.Fatalf("zero-iteration comptime for should emit nothing:\n%s", dump)
	}
}

func TestModuleRejectsComptimeForErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "runtime bound",
			src: `record Params { M: u32; }
shader S {
stage compute [numthreads(1, 1, 1)] fn CS(params: Params) -> void {
comptime for i in 0u..params.M { return; }
return;
}
}`,
			want: "comptime for bounds must be compile-time integers",
		},
		{
			name: "start greater than end",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime for i in 3u..2u { return; }
return;
}
}`,
			want: "comptime for range start must be <= end",
		},
		{
			name: "negative bound",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime for i in -1..2u { return; }
return;
}
}`,
			want: "comptime for bounds must be non-negative in SDSL-V M16",
		},
		{
			name: "expansion limit",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime for i in 0u..17u {
comptime for j in 0u..17u {
let x: u32 = i + j;
}
}
return;
}
}`,
			want: "comptime for expansion exceeds M16 limit of 256 iterations",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: tc.src})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			module, err := parse.BuildModule(tokens)
			if err != nil {
				t.Fatalf("BuildModule() error = %v", err)
			}
			if err := validate.Module(module); err != nil {
				if strings.Contains(err.Error(), tc.want) {
					return
				}
				t.Fatalf("validate.Module() error = %v", err)
			}
			_, err = Module(module)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Module() error = %v, want %q", err, tc.want)
			}
		})
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
