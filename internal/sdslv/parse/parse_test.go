package parse

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestBuildModuleParsesComputeShader(t *testing.T) {
	module := parseTestModule(t, `namespace Prometheus.Kernels;
type F32 = f32;
record Params { Count: u32; }
shader S { stage compute [numthreads(16, 16, 1)] fn CS(params: Params) -> void { return; } }`)
	if module.Namespace != "Prometheus.Kernels" {
		t.Fatalf("namespace = %q", module.Namespace)
	}
	shader, ok := module.Decls[2].(ast.ShaderDecl)
	if !ok {
		t.Fatalf("decl[2] = %T, want ShaderDecl", module.Decls[2])
	}
	if shader.Methods[0].Stage != "compute" {
		t.Fatalf("method = %#v", shader.Methods[0])
	}
	if lit, ok := shader.Methods[0].NumThreads.X.(ast.IntegerLiteral); !ok || lit.Value != "16" {
		t.Fatalf("numthreads x = %#v", shader.Methods[0].NumThreads.X)
	}
}

func TestBuildModuleParsesStreamsResourceBundleAndWith(t *testing.T) {
	module := parseTestModule(t, `stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
stream VectorAddIO {
A: readonly array<f32>;
C: readwrite array<f32>;
}
record Surface { Roughness: f32; }
shader S {
resources VectorAddIO;
stage compute [numthreads(16, 1, 1)] fn CS(thread: ComputeThread, s: Surface) -> Surface {
return s with { Roughness: 0.5, };
}
}`)
	if _, ok := module.Decls[0].(ast.StreamDecl); !ok {
		t.Fatalf("decl[0] = %T, want StreamDecl", module.Decls[0])
	}
	if _, ok := module.Decls[1].(ast.StreamDecl); !ok {
		t.Fatalf("decl[1] = %T, want StreamDecl", module.Decls[1])
	}
	shader, ok := module.Decls[3].(ast.ShaderDecl)
	if !ok {
		t.Fatalf("decl[3] = %T, want ShaderDecl", module.Decls[3])
	}
	if shader.ResourceBundleName != "VectorAddIO" {
		t.Fatalf("ResourceBundleName = %q", shader.ResourceBundleName)
	}
	ret, ok := shader.Methods[0].Body.Statements[0].(ast.ReturnStmt)
	if !ok {
		t.Fatalf("first stmt = %T, want ReturnStmt", shader.Methods[0].Body.Statements[0])
	}
	if _, ok := ret.Value.(ast.WithExpr); !ok {
		t.Fatalf("return value = %T, want WithExpr", ret.Value)
	}
}

func TestBuildModuleParsesShaderWorkgroups(t *testing.T) {
	module := parseTestModule(t, `shader TileCopy {
workgroup TileA: array<f32, 256>;
workgroup TileB: array<uint4, 16>;
stage compute [numthreads(16, 16, 1)] fn CS() -> void { return; }
}`)
	shader, ok := module.Decls[0].(ast.ShaderDecl)
	if !ok {
		t.Fatalf("decl[0] = %T, want ShaderDecl", module.Decls[0])
	}
	if got := len(shader.Workgroups); got != 2 {
		t.Fatalf("len(Workgroups) = %d, want 2", got)
	}
	if shader.Workgroups[0].Name != "TileA" || !shader.Workgroups[0].Type.HasArraySize {
		t.Fatalf("first workgroup = %#v", shader.Workgroups[0])
	}
	if lit, ok := shader.Workgroups[0].Type.ArraySize.(ast.IntegerLiteral); !ok || lit.Value != "256" {
		t.Fatalf("first workgroup size = %#v", shader.Workgroups[0].Type.ArraySize)
	}
}

func TestBuildModuleParsesTileMatrixViewsAnd2DIndexing(t *testing.T) {
	module := parseTestModule(t, `shader TileCopy {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
workgroup Tile: tile<f32, 16, 8>;
stage compute [numthreads(16, 1, 1)] fn CS() -> void {
let View: matrix_view<f32> = row_major(A, 16u, 8u);
Tile[1u, 2u] = View[1u, 2u];
return;
}
}`)
	shader := module.Decls[0].(ast.ShaderDecl)
	if shader.Workgroups[0].Type.Name != "tile" || !shader.Workgroups[0].Type.HasTileShape {
		t.Fatalf("workgroup type = %#v, want tile shape", shader.Workgroups[0].Type)
	}
	letStmt := shader.Methods[0].Body.Statements[0].(ast.LetStmt)
	if letStmt.Type.Name != "matrix_view" || len(letStmt.Type.Args) != 1 {
		t.Fatalf("let type = %#v, want matrix_view<f32>", letStmt.Type)
	}
	assign := shader.Methods[0].Body.Statements[1].(ast.AssignStmt)
	target := assign.Target.(ast.IndexExpr)
	if !target.HasSecond {
		t.Fatalf("assignment target = %#v, want 2D index", target)
	}
}

func TestBuildModuleParsesRegTileAndZeroInitializer(t *testing.T) {
	module := parseTestModule(t, `shader RegTileBasic {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let Acc: reg_tile<f32, 2u, 2u> = reg_tile_zero();
Acc[0u, 1u] = Acc[0u, 1u] + 1.0;
return;
}
}`)
	shader := module.Decls[0].(ast.ShaderDecl)
	letStmt := shader.Methods[0].Body.Statements[0].(ast.LetStmt)
	if letStmt.Type.Name != "reg_tile" || !letStmt.Type.HasTileShape {
		t.Fatalf("let type = %#v, want reg_tile shape", letStmt.Type)
	}
	call, ok := letStmt.Value.(ast.CallExpr)
	if !ok {
		t.Fatalf("let value = %T, want CallExpr", letStmt.Value)
	}
	callee, ok := call.Callee.(ast.IdentifierExpr)
	if !ok || callee.Name != "reg_tile_zero" {
		t.Fatalf("callee = %#v, want reg_tile_zero", call.Callee)
	}
}

func TestBuildModuleParsesGuardedReadAndWrite(t *testing.T) {
	module := parseTestModule(t, `shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
stage compute [numthreads(1, 1, 1)] fn CS(row: u32, col: u32, ok: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
let CView: matrix_view<f32> = row_major(C, 4u, 4u);
let value: f32 = read AView[row, col] when ok and not false else 0.0;
write CView[row, col] = value when ok or false;
return;
}
}`)
	shader := module.Decls[0].(ast.ShaderDecl)
	letStmt := shader.Methods[0].Body.Statements[2].(ast.LetStmt)
	guardedRead, ok := letStmt.Value.(ast.GuardedReadExpr)
	if !ok {
		t.Fatalf("let value = %T, want GuardedReadExpr", letStmt.Value)
	}
	if _, ok := guardedRead.Target.(ast.IndexExpr); !ok {
		t.Fatalf("guarded read target = %T, want IndexExpr", guardedRead.Target)
	}
	if cond, ok := guardedRead.Condition.(ast.BinaryExpr); !ok || cond.Operator != "and" {
		t.Fatalf("guarded read condition = %#v, want semantic and", guardedRead.Condition)
	}
	writeStmt, ok := shader.Methods[0].Body.Statements[3].(ast.GuardedWriteStmt)
	if !ok {
		t.Fatalf("stmt[3] = %T, want GuardedWriteStmt", shader.Methods[0].Body.Statements[3])
	}
	if _, ok := writeStmt.Target.(ast.IndexExpr); !ok {
		t.Fatalf("guarded write target = %T, want IndexExpr", writeStmt.Target)
	}
	if cond, ok := writeStmt.Condition.(ast.BinaryExpr); !ok || cond.Operator != "or" {
		t.Fatalf("guarded write condition = %#v, want semantic or", writeStmt.Condition)
	}
}

func TestBuildModuleParsesGuardWhen(t *testing.T) {
	module := parseTestModule(t, `shader S {
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
	body := module.Decls[0].(ast.ShaderDecl).Methods[0].Body.Statements
	whenStmt, ok := body[2].(ast.GuardWhenStmt)
	if !ok {
		t.Fatalf("stmt[2] = %T, want GuardWhenStmt", body[2])
	}
	if got := len(whenStmt.Cases); got != 2 {
		t.Fatalf("len(Cases) = %d, want 2", got)
	}
	if _, ok := whenStmt.Cases[0].Body.Statements[0].(ast.GuardedWriteStmt); !ok {
		t.Fatalf("first case stmt = %T, want GuardedWriteStmt", whenStmt.Cases[0].Body.Statements[0])
	}
	if whenStmt.ElseBody == nil {
		t.Fatalf("missing else body")
	}
}

func TestBuildModuleRejectsStatefulShaderFlowActions(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "goto",
			src:  `shader S { stage compute [numthreads(1,1,1)] fn CS(flag: bool) -> void { when { case flag -> { goto Done } } return; } }`,
			want: "goto is only valid inside a flow state",
		},
		{
			name: "state outside flow",
			src:  `shader S { stage compute [numthreads(1,1,1)] fn CS() -> void { state Start { return; } return; } }`,
			want: "state blocks are only valid inside flow blocks in SDSL-V M22",
		},
		{
			name: "when policy",
			src:  `fn F(flag: bool) -> u32 { return when policy { hysteresis: 1 min_commit: 1 } { case 1u when flag score 2 else 0u }; }`,
			want: "SDSL-V M23 does not support when policy",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: tc.src})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			_, err = BuildModule(tokens)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBuildModuleParsesFlowStateBlocks(t *testing.T) {
	module := parseTestModule(t, `board LoadCoord {
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
	body := module.Decls[2].(ast.ShaderDecl).Methods[0].Body.Statements
	flowStmt, ok := body[2].(ast.FlowStmt)
	if !ok {
		t.Fatalf("stmt[2] = %T, want FlowStmt", body[2])
	}
	if flowStmt.Name != "TileLoad" || len(flowStmt.States) != 2 {
		t.Fatalf("flow = %#v", flowStmt)
	}
	if flowStmt.States[0].Name != "Load" || flowStmt.States[1].Name != "Sync" {
		t.Fatalf("states = %#v", flowStmt.States)
	}
	if _, ok := flowStmt.States[0].Body.Statements[0].(ast.ComptimeForStmt); !ok {
		t.Fatalf("state body stmt = %T, want ComptimeForStmt", flowStmt.States[0].Body.Statements[0])
	}
}

func TestSdslvFlowParsesPushPopGotoFinishAndPreservesOrder(t *testing.T) {
	module := parseTestModule(t, `fn F() -> void {
flow Example {
state A { push Shared; }
state B { goto Done; }
state Shared { pop; }
state Done { finish; }
}
}`)
	fn := module.Decls[0].(ast.FunctionDecl)
	flowStmt := fn.Body.Statements[0].(ast.FlowStmt)
	if got := []string{flowStmt.States[0].Name, flowStmt.States[1].Name, flowStmt.States[2].Name, flowStmt.States[3].Name}; strings.Join(got, ",") != "A,B,Shared,Done" {
		t.Fatalf("state order = %#v", got)
	}
	if !flowStmt.States[0].NameSpan.Known() {
		t.Fatalf("state name span missing: %#v", flowStmt.States[0])
	}
	push := flowStmt.States[0].Body.Statements[0].(ast.PushFlowStateStmt)
	if push.Target != "Shared" || !push.Span.Known() || !push.TargetSpan.Known() {
		t.Fatalf("push = %#v", push)
	}
	if got := flowStmt.States[1].Body.Statements[0].(ast.GotoFlowStateStmt).Target; got != "Done" {
		t.Fatalf("goto target = %q", got)
	}
	if _, ok := flowStmt.States[2].Body.Statements[0].(ast.PopFlowStateStmt); !ok {
		t.Fatalf("shared stmt = %T, want PopFlowStateStmt", flowStmt.States[2].Body.Statements[0])
	}
	if _, ok := flowStmt.States[3].Body.Statements[0].(ast.FinishFlowStmt); !ok {
		t.Fatalf("done stmt = %T, want FinishFlowStmt", flowStmt.States[3].Body.Statements[0])
	}
}

func TestBuildModuleParsesFlowBoardDeclarations(t *testing.T) {
	module := parseTestModule(t, `board LoadCoord {
row: u32;
}
shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
flow TileLoad {
board Load: LoadCoord = LoadCoord { row: 0u; };
state Compute {
Load.row = 1u;
}
}
return;
}
}`)
	body := module.Decls[1].(ast.ShaderDecl).Methods[0].Body.Statements
	flowStmt, ok := body[0].(ast.FlowStmt)
	if !ok {
		t.Fatalf("stmt[0] = %T, want FlowStmt", body[0])
	}
	if len(flowStmt.Boards) != 1 {
		t.Fatalf("len(Boards) = %d, want 1", len(flowStmt.Boards))
	}
	if flowStmt.Boards[0].Name != "Load" || flowStmt.Boards[0].Type.Name != "LoadCoord" {
		t.Fatalf("board = %#v", flowStmt.Boards[0])
	}
	assign, ok := flowStmt.States[0].Body.Statements[0].(ast.AssignStmt)
	if !ok {
		t.Fatalf("state stmt = %T, want AssignStmt", flowStmt.States[0].Body.Statements[0])
	}
	if _, ok := assign.Target.(ast.FieldAccessExpr); !ok {
		t.Fatalf("assign target = %T, want FieldAccessExpr", assign.Target)
	}
}

func TestBuildModuleParsesBoardDeclarationLiteralAndFieldAccess(t *testing.T) {
	module := parseTestModule(t, `board LoadCoord {
linear: u32;
row: u32;
col: u32;
}
fn Make(linear: u32, tileK: u32) -> LoadCoord {
return LoadCoord { linear: linear; row: linear / tileK; col: linear % tileK; };
}
shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let p: LoadCoord = Make(1u, 16u);
let row: u32 = p.row;
return;
}
}`)
	if _, ok := module.Decls[0].(ast.BoardDecl); !ok {
		t.Fatalf("decl[0] = %T, want BoardDecl", module.Decls[0])
	}
	fn := module.Decls[1].(ast.FunctionDecl)
	ret := fn.Body.Statements[0].(ast.ReturnStmt)
	if _, ok := ret.Value.(ast.BoardLiteralExpr); !ok {
		t.Fatalf("return value = %T, want BoardLiteralExpr", ret.Value)
	}
	shader := module.Decls[2].(ast.ShaderDecl)
	letRow := shader.Methods[0].Body.Statements[1].(ast.LetStmt)
	if _, ok := letRow.Value.(ast.FieldAccessExpr); !ok {
		t.Fatalf("row init = %T, want FieldAccessExpr", letRow.Value)
	}
}

func TestBuildModuleParsesDeriveExpr(t *testing.T) {
	module := parseTestModule(t, `record LoadFacts {
linear: u32;
row: u32;
col: u32;
}
fn F(localThreadLinear: u32, lane: u32, tileK: u32) -> LoadFacts {
let load: LoadFacts = derive {
linear = localThreadLinear * 4u + lane;
row = linear / tileK;
col = linear % tileK;
};
return load;
}`)
	fn := module.Decls[1].(ast.FunctionDecl)
	letLoad := fn.Body.Statements[0].(ast.LetStmt)
	derive, ok := letLoad.Value.(ast.DeriveExpr)
	if !ok {
		t.Fatalf("let value = %T, want DeriveExpr", letLoad.Value)
	}
	if len(derive.Fields) != 3 {
		t.Fatalf("len(derive.Fields) = %d, want 3", len(derive.Fields))
	}
	if derive.Fields[1].Name != "row" || derive.Fields[2].Name != "col" {
		t.Fatalf("derive fields = %#v", derive.Fields)
	}
}

func TestBuildModuleRejectsWorkgroupOutsideShader(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: `workgroup Tile: array<f32, 16>;`})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	_, err = BuildModule(tokens)
	if err == nil {
		t.Fatalf("BuildModule() error = nil, want rejection")
	}
}

func TestBuildModuleParsesTemplateConceptConfigAndCompile(t *testing.T) {
	module := parseTestModule(t, `concept TileConfig {
THREADS_X: u32;
THREADS_Y: u32;
TILE_SIZE: u32;
}
config Tile16x16: TileConfig {
THREADS_X: 16;
THREADS_Y: 16;
TILE_SIZE: 256;
}
template<C: TileConfig>
shader TileCopy {
workgroup Tile: array<f32, C.TILE_SIZE>;
stage compute [numthreads(C.THREADS_X, C.THREADS_Y, 1)] fn CS() -> void {
let tileElements: u32 = C.TILE_SIZE;
return;
}
}
compile TileCopy<Tile16x16> as TileCopy16x16;`)
	if _, ok := module.Decls[0].(ast.ConceptDecl); !ok {
		t.Fatalf("decl[0] = %T, want ConceptDecl", module.Decls[0])
	}
	if _, ok := module.Decls[1].(ast.ConfigDecl); !ok {
		t.Fatalf("decl[1] = %T, want ConfigDecl", module.Decls[1])
	}
	shader, ok := module.Decls[2].(ast.ShaderDecl)
	if !ok {
		t.Fatalf("decl[2] = %T, want ShaderDecl", module.Decls[2])
	}
	if shader.Template == nil || shader.Template.Name != "C" || shader.Template.ConceptName != "TileConfig" {
		t.Fatalf("shader template = %#v", shader.Template)
	}
	if _, ok := shader.Workgroups[0].Type.ArraySize.(ast.FieldAccessExpr); !ok {
		t.Fatalf("workgroup size = %T, want FieldAccessExpr", shader.Workgroups[0].Type.ArraySize)
	}
	compileDecl, ok := module.Decls[3].(ast.CompileDecl)
	if !ok {
		t.Fatalf("decl[3] = %T, want CompileDecl", module.Decls[3])
	}
	if compileDecl.ShaderName != "TileCopy" || compileDecl.ConfigName != "Tile16x16" || compileDecl.AliasName != "TileCopy16x16" {
		t.Fatalf("compile decl = %#v", compileDecl)
	}
}

func TestBuildModuleParsesRequirementsStaticAssertsAndAttributes(t *testing.T) {
	module := parseTestModule(t, `stream IO {
[binding(0)] A: readonly array<f32>;
[binding(3)] C: readwrite array<f32>;
}
concept TileConfig {
TILE_SIZE: u32;
require TILE_SIZE > 0u;
}
config Tile16: TileConfig {
TILE_SIZE: 16u;
require TILE_SIZE <= 1024u;
}
template<C: TileConfig>
shader TileCopy {
resources IO;
static assert C.TILE_SIZE <= 1024u;
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
[unroll]
for i in 0u..C.TILE_SIZE {
return;
}
return;
}
}
compile TileCopy<Tile16> as TileCopy16;`)
	concept := module.Decls[1].(ast.ConceptDecl)
	if got := len(concept.Requirements); got != 1 {
		t.Fatalf("len(Requirements) = %d, want 1", got)
	}
	config := module.Decls[2].(ast.ConfigDecl)
	if got := len(config.Requirements); got != 1 {
		t.Fatalf("len(config.Requirements) = %d, want 1", got)
	}
	shader := module.Decls[3].(ast.ShaderDecl)
	if got := len(shader.StaticAsserts); got != 1 {
		t.Fatalf("len(StaticAsserts) = %d, want 1", got)
	}
	loop, ok := shader.Methods[0].Body.Statements[0].(ast.ForStmt)
	if !ok {
		t.Fatalf("stmt[0] = %T, want ForStmt", shader.Methods[0].Body.Statements[0])
	}
	if got := len(loop.Attributes); got != 1 {
		t.Fatalf("len(loop.Attributes) = %d, want 1", got)
	}
	stream := module.Decls[0].(ast.StreamDecl)
	if got := len(stream.Fields[0].Attributes); got != 1 {
		t.Fatalf("len(field.Attributes) = %d, want 1", got)
	}
}

func TestBuildModuleParsesComptimeLetAndIf(t *testing.T) {
	module := parseTestModule(t, `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let TileElements: u32 = 16u * 16u;
comptime if TileElements == 256u {
static assert TileElements == 256u;
let x: u32 = TileElements;
} else {
let x: u32 = 0u;
}
return;
}
}`)
	shader := module.Decls[0].(ast.ShaderDecl)
	body := shader.Methods[0].Body.Statements
	if _, ok := body[0].(ast.ComptimeLetStmt); !ok {
		t.Fatalf("stmt[0] = %T, want ComptimeLetStmt", body[0])
	}
	ctIf, ok := body[1].(ast.ComptimeIfStmt)
	if !ok {
		t.Fatalf("stmt[1] = %T, want ComptimeIfStmt", body[1])
	}
	if ctIf.ElseBody == nil {
		t.Fatalf("comptime if missing else body")
	}
	if _, ok := ctIf.ThenBody.Statements[0].(ast.StaticAssertStmt); !ok {
		t.Fatalf("then stmt[0] = %T, want StaticAssertStmt", ctIf.ThenBody.Statements[0])
	}
}

func TestBuildModuleParsesNestedComptimeIf(t *testing.T) {
	module := parseTestModule(t, `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime if true {
comptime if false {
return;
} else {
return;
}
}
return;
}
}`)
	shader := module.Decls[0].(ast.ShaderDecl)
	outer := shader.Methods[0].Body.Statements[0].(ast.ComptimeIfStmt)
	if _, ok := outer.ThenBody.Statements[0].(ast.ComptimeIfStmt); !ok {
		t.Fatalf("nested stmt = %T, want ComptimeIfStmt", outer.ThenBody.Statements[0])
	}
}

func TestBuildModuleParsesComptimeMatch(t *testing.T) {
	module := parseTestModule(t, `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime match 16u {
8u => {
return;
}
16u => {
comptime match true {
true => { let x: u32 = 1u; }
false => { let x: u32 = 0u; }
}
}
else => {
static assert false;
}
}
return;
}
}`)
	shader := module.Decls[0].(ast.ShaderDecl)
	matchStmt, ok := shader.Methods[0].Body.Statements[0].(ast.ComptimeMatchStmt)
	if !ok {
		t.Fatalf("stmt[0] = %T, want ComptimeMatchStmt", shader.Methods[0].Body.Statements[0])
	}
	if got := len(matchStmt.Arms); got != 3 {
		t.Fatalf("len(Arms) = %d, want 3", got)
	}
	if !matchStmt.Arms[2].IsElse {
		t.Fatalf("third arm should be else")
	}
	if _, ok := matchStmt.Arms[1].Body.Statements[0].(ast.ComptimeMatchStmt); !ok {
		t.Fatalf("nested stmt = %T, want ComptimeMatchStmt", matchStmt.Arms[1].Body.Statements[0])
	}
}

func TestBuildModuleParsesComptimeWhenUtility(t *testing.T) {
	module := parseTestModule(t, `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime when utility {
case UseVector4 when true score 100 {
let selected: u32 = 4u;
}
case Policy.UseScalar score 10 {
comptime when utility {
case Nested score 1 { return; }
else { return; }
}
}
else {
static assert false;
}
}
return;
}
}`)
	shader := module.Decls[0].(ast.ShaderDecl)
	whenStmt, ok := shader.Methods[0].Body.Statements[0].(ast.ComptimeWhenUtilityStmt)
	if !ok {
		t.Fatalf("stmt[0] = %T, want ComptimeWhenUtilityStmt", shader.Methods[0].Body.Statements[0])
	}
	if got := len(whenStmt.Cases); got != 2 {
		t.Fatalf("len(Cases) = %d, want 2", got)
	}
	if whenStmt.Cases[0].Label != "UseVector4" || whenStmt.Cases[0].Condition == nil {
		t.Fatalf("first case = %#v, want label and guard", whenStmt.Cases[0])
	}
	if whenStmt.Cases[1].Label != "Policy.UseScalar" || whenStmt.Cases[1].Condition != nil {
		t.Fatalf("second case = %#v, want qualified unguarded label", whenStmt.Cases[1])
	}
	if whenStmt.ElseBody == nil {
		t.Fatalf("missing else body")
	}
	if _, ok := whenStmt.Cases[1].Body.Statements[0].(ast.ComptimeWhenUtilityStmt); !ok {
		t.Fatalf("nested stmt = %T, want ComptimeWhenUtilityStmt", whenStmt.Cases[1].Body.Statements[0])
	}
}

func TestBuildModuleParsesComptimeFor(t *testing.T) {
	module := parseTestModule(t, `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime for i in 0u..4u {
let x: u32 = i;
comptime for j in 0u..i {
let y: u32 = j;
}
}
return;
}
}`)
	shader := module.Decls[0].(ast.ShaderDecl)
	loop, ok := shader.Methods[0].Body.Statements[0].(ast.ComptimeForStmt)
	if !ok {
		t.Fatalf("stmt[0] = %T, want ComptimeForStmt", shader.Methods[0].Body.Statements[0])
	}
	if loop.Name != "i" {
		t.Fatalf("loop.Name = %q, want i", loop.Name)
	}
	if _, ok := loop.Body.Statements[1].(ast.ComptimeForStmt); !ok {
		t.Fatalf("nested stmt = %T, want ComptimeForStmt", loop.Body.Statements[1])
	}
}

func TestBuildModuleParsesSemanticBooleanOperatorsAndPrecedence(t *testing.T) {
	module := parseTestModule(t, `fn F(A: bool, B: bool, C: bool, X: u32, Y: u32) -> bool {
let v0: bool = not A and B;
let v1: bool = A or B and C;
let v2: bool = X == Y and C;
return v0 or v1 or v2;
}`)
	fn := module.Decls[0].(ast.FunctionDecl)
	v0 := fn.Body.Statements[0].(ast.LetStmt).Value.(ast.BinaryExpr)
	if v0.Operator != "and" {
		t.Fatalf("v0 operator = %q, want and", v0.Operator)
	}
	if unary, ok := v0.Left.(ast.UnaryExpr); !ok || unary.Operator != "not" {
		t.Fatalf("v0 left = %#v, want unary not", v0.Left)
	}
	v1 := fn.Body.Statements[1].(ast.LetStmt).Value.(ast.BinaryExpr)
	if v1.Operator != "or" {
		t.Fatalf("v1 operator = %q, want or", v1.Operator)
	}
	if rhs, ok := v1.Right.(ast.BinaryExpr); !ok || rhs.Operator != "and" {
		t.Fatalf("v1 right = %#v, want nested and", v1.Right)
	}
	v2 := fn.Body.Statements[2].(ast.LetStmt).Value.(ast.BinaryExpr)
	if v2.Operator != "and" {
		t.Fatalf("v2 operator = %q, want and", v2.Operator)
	}
	if lhs, ok := v2.Left.(ast.BinaryExpr); !ok || lhs.Operator != "==" {
		t.Fatalf("v2 left = %#v, want comparison on left", v2.Left)
	}
}

func TestBuildModuleParsesSemanticBooleanOperatorsInComptimeExpressions(t *testing.T) {
	module := parseTestModule(t, `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let UseFast: bool = true and not false;
comptime if UseFast or false {
static assert UseFast and not false;
}
return;
}
}`)
	body := module.Decls[0].(ast.ShaderDecl).Methods[0].Body.Statements
	comptimeLet := body[0].(ast.ComptimeLetStmt)
	if bin, ok := comptimeLet.Value.(ast.BinaryExpr); !ok || bin.Operator != "and" {
		t.Fatalf("comptime let value = %#v, want semantic and", comptimeLet.Value)
	}
	ctIf := body[1].(ast.ComptimeIfStmt)
	if bin, ok := ctIf.Condition.(ast.BinaryExpr); !ok || bin.Operator != "or" {
		t.Fatalf("comptime if condition = %#v, want semantic or", ctIf.Condition)
	}
}

func TestBuildModuleRejectsCStyleLogicalNot(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: `fn F(A: bool) -> bool { return !A; }`})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	_, err = BuildModule(tokens)
	if err == nil || !strings.Contains(err.Error(), "use `not` instead of `!`") {
		t.Fatalf("BuildModule() error = %v, want logical not diagnostic", err)
	}
}

func TestAnalyzeRejectsCStyleLogicalAndOr(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want string
	}{
		{src: `fn F(A: bool, B: bool) -> bool { return A && B; }`, want: "use `and` instead of `&&`"},
		{src: `fn F(A: bool, B: bool) -> bool { return A || B; }`, want: "use `or` instead of `||`"},
	} {
		_, err := lex.Analyze(source.File{Path: "test.sdslv", Text: tc.src})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("Analyze() error = %v, want %q", err, tc.want)
		}
	}
}

func TestBuildModuleParsesStructuredConceptsDefaultsAndFatArrowConfig(t *testing.T) {
	module := parseTestModule(t, `concept TileConfig {
Threads: {
X: u32;
Y: u32;
};
Tile: {
K: u32 = Threads.X * Threads.Y;
Padding: {
K: u32! = 0u;
};
};
}
config Tile16: TileConfig {
Threads.X => 4u;
Threads.Y => 4u;
}`)
	concept := module.Decls[0].(ast.ConceptDecl)
	if got := len(concept.Members); got != 2 {
		t.Fatalf("len(Members) = %d, want 2", got)
	}
	threads, ok := concept.Members[0].(ast.ConceptGroup)
	if !ok || threads.Name != "Threads" || len(threads.Members) != 2 {
		t.Fatalf("threads group = %#v", concept.Members[0])
	}
	tile, ok := concept.Members[1].(ast.ConceptGroup)
	if !ok || tile.Name != "Tile" || len(tile.Members) != 2 {
		t.Fatalf("tile group = %#v", concept.Members[1])
	}
	field, ok := tile.Members[0].(ast.ConceptField)
	if !ok || field.Name != "K" || field.DefaultValue == nil {
		t.Fatalf("tile field = %#v", tile.Members[0])
	}
	padding, ok := tile.Members[1].(ast.ConceptGroup)
	if !ok || padding.Name != "Padding" {
		t.Fatalf("padding group = %#v", tile.Members[1])
	}
	paddingField := padding.Members[0].(ast.ConceptField)
	if !paddingField.Type.ZeroAllowed {
		t.Fatalf("padding type = %#v, want ZeroAllowed", paddingField.Type)
	}
	config := module.Decls[1].(ast.ConfigDecl)
	if got := len(config.Fields); got != 2 {
		t.Fatalf("len(config.Fields) = %d, want 2", got)
	}
	if config.Fields[0].Path != "Threads.X" || config.Fields[0].Style != ast.ConfigAssignmentFatArrow {
		t.Fatalf("config field[0] = %#v", config.Fields[0])
	}
}

func TestBuildModuleRejectsU32BangOutsideConceptConfigField(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: `record Bad { Count: u32!; }`})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	_, err = BuildModule(tokens)
	if err == nil || !strings.Contains(err.Error(), "u32! is only valid for concept/config fields") {
		t.Fatalf("BuildModule() error = %v, want u32! placement diagnostic", err)
	}
}

func TestBuildModuleParsesPayloadEnumAndMatch(t *testing.T) {
	module := parseTestModule(t, `enum LoadValue {
Zero;
Value { X: f32; }
}
fn Resolve(value: LoadValue) -> f32 {
let loaded: LoadValue = LoadValue.Value { X: 1.0 };
return match value {
LoadValue.Zero => 0.0
LoadValue.Value(payload) => payload.X
};
}`)
	enumDecl, ok := module.Decls[0].(ast.EnumDecl)
	if !ok {
		t.Fatalf("decl[0] = %T, want EnumDecl", module.Decls[0])
	}
	if got := len(enumDecl.Variants); got != 2 {
		t.Fatalf("len(Variants) = %d, want 2", got)
	}
	if !enumDecl.Variants[1].Payload || enumDecl.Variants[1].Fields[0].Name != "X" {
		t.Fatalf("payload variant = %#v", enumDecl.Variants[1])
	}
	fn := module.Decls[1].(ast.FunctionDecl)
	letStmt := fn.Body.Statements[0].(ast.LetStmt)
	if _, ok := letStmt.Value.(ast.EnumConstructExpr); !ok {
		t.Fatalf("let value = %T, want EnumConstructExpr", letStmt.Value)
	}
	ret := fn.Body.Statements[1].(ast.ReturnStmt)
	matchExpr, ok := ret.Value.(ast.MatchExpr)
	if !ok {
		t.Fatalf("return value = %T, want MatchExpr", ret.Value)
	}
	if got := len(matchExpr.Arms); got != 2 {
		t.Fatalf("len(Arms) = %d, want 2", got)
	}
	if matchExpr.Arms[1].BindingName != "payload" {
		t.Fatalf("binding = %q, want payload", matchExpr.Arms[1].BindingName)
	}
}

func TestBuildModuleRejectsAttributeOnNonForStatement(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
[unroll]
return;
}
}`})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	_, err = BuildModule(tokens)
	if err == nil {
		t.Fatalf("BuildModule() error = nil, want rejection")
	}
}

func TestBuildModuleParsesReductionExpressions(t *testing.T) {
	module := parseTestModule(t, `fn Reduce(values: array<f32>) -> f32 {
let total: f32 = sum i in 0u..4u { values[i] };
let productValue: f32 = product j in 1..4 step 2 { total };
return max k in 0u..4u { values[k] };
}`)
	fn := module.Decls[0].(ast.FunctionDecl)
	letSum := fn.Body.Statements[0].(ast.LetStmt)
	sumExpr, ok := letSum.Value.(ast.ReductionExpr)
	if !ok {
		t.Fatalf("let value = %T, want ReductionExpr", letSum.Value)
	}
	if sumExpr.Op != ast.ReductionSum || sumExpr.Name != "i" {
		t.Fatalf("sum expr = %#v", sumExpr)
	}
	letProduct := fn.Body.Statements[1].(ast.LetStmt)
	productExpr := letProduct.Value.(ast.ReductionExpr)
	if productExpr.Op != ast.ReductionProduct {
		t.Fatalf("product op = %v, want product", productExpr.Op)
	}
	if lit, ok := productExpr.Step.(ast.IntegerLiteral); !ok || lit.Value != "2" {
		t.Fatalf("product step = %#v, want int literal 2", productExpr.Step)
	}
	ret := fn.Body.Statements[2].(ast.ReturnStmt)
	maxExpr, ok := ret.Value.(ast.ReductionExpr)
	if !ok || maxExpr.Op != ast.ReductionMax {
		t.Fatalf("return value = %#v, want max ReductionExpr", ret.Value)
	}
}

func TestBuildModuleParsesReductionAttributes(t *testing.T) {
	module := parseTestModule(t, `fn Reduce(values: array<f32>) -> f32 {
let total: f32 = [unroll] sum i in 0u..4u { values[i] };
let productValue: f32 = [loop] product j in 0u..4u { values[j] };
return [unroll] max k in 0u..4u { values[k] };
}`)
	fn := module.Decls[0].(ast.FunctionDecl)
	total := fn.Body.Statements[0].(ast.LetStmt).Value.(ast.ReductionExpr)
	if got := len(total.Attributes); got != 1 || total.Attributes[0].Name != "unroll" {
		t.Fatalf("sum attributes = %#v, want [unroll]", total.Attributes)
	}
	product := fn.Body.Statements[1].(ast.LetStmt).Value.(ast.ReductionExpr)
	if got := len(product.Attributes); got != 1 || product.Attributes[0].Name != "loop" {
		t.Fatalf("product attributes = %#v, want [loop]", product.Attributes)
	}
	maxExpr := fn.Body.Statements[2].(ast.ReturnStmt).Value.(ast.ReductionExpr)
	if got := len(maxExpr.Attributes); got != 1 || maxExpr.Attributes[0].Name != "unroll" {
		t.Fatalf("max attributes = %#v, want [unroll]", maxExpr.Attributes)
	}
}

func TestBuildModuleRejectsReductionAttributeInNestedExpression(t *testing.T) {
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: `fn F(values: array<f32>) -> f32 {
return 1.0 + [unroll] sum i in 0u..4u { values[i] };
}`})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	_, err = BuildModule(tokens)
	if err == nil || !strings.Contains(err.Error(), "reduction attributes are only supported before direct reduction expressions") {
		t.Fatalf("BuildModule() error = %v, want reduction attribute placement diagnostic", err)
	}
}

func TestBuildModuleAllowsMaxKeywordAsCallCalleeWhenNotReduction(t *testing.T) {
	module := parseTestModule(t, `fn Reduce(a: f32, b: f32) -> f32 { return max(a, b); }`)
	fn := module.Decls[0].(ast.FunctionDecl)
	ret := fn.Body.Statements[0].(ast.ReturnStmt)
	call, ok := ret.Value.(ast.CallExpr)
	if !ok {
		t.Fatalf("return value = %T, want CallExpr", ret.Value)
	}
	callee, ok := call.Callee.(ast.IdentifierExpr)
	if !ok || callee.Name != "max" {
		t.Fatalf("callee = %#v, want identifier max", call.Callee)
	}
}

func parseTestModule(t *testing.T, text string) ast.Module {
	t.Helper()
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: text})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	module, err := BuildModule(tokens)
	if err != nil {
		t.Fatalf("BuildModule() error = %v", err)
	}
	return module
}
