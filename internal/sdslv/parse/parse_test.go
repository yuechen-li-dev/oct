package parse

import (
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
