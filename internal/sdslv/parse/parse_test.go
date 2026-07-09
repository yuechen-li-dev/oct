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
	if shader.Methods[0].Stage != "compute" || shader.Methods[0].NumThreads.X != 16 {
		t.Fatalf("method = %#v", shader.Methods[0])
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
	if shader.Workgroups[0].Name != "TileA" || !shader.Workgroups[0].Type.HasArraySize || shader.Workgroups[0].Type.ArraySize != 256 {
		t.Fatalf("first workgroup = %#v", shader.Workgroups[0])
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
