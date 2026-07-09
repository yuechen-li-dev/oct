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
