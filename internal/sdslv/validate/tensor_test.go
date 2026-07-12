package validate

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestSdslvTensorInfersReductionExtentFromSourcesWithProvenance(t *testing.T) {
	module := mustTensorModule(t, `fn F(
    A: array<array<f32, 4u>, 2u>,
    B: array<array<f32, 3u>, 4u>
) -> void {
    var C: array<array<f32, 3u>, 2u> = Fill(0.0);
    tensor C[i, j] = Sum[k](A[i, k] * B[k, j]);
    return;
}`)
	assigns, diags := ValidatedTensorAssignments(module)
	if len(diags) != 0 {
		t.Fatalf("ValidatedTensorAssignments() diagnostics = %v", diags)
	}
	if len(assigns) != 1 {
		t.Fatalf("ValidatedTensorAssignments() len = %d, want 1", len(assigns))
	}
	assign := assigns[0]
	if len(assign.FreeIndices) != 2 || assign.FreeIndices[0].Extent != 2 || assign.FreeIndices[1].Extent != 3 {
		t.Fatalf("free indices = %#v", assign.FreeIndices)
	}
	if len(assign.Reductions) != 1 || len(assign.Reductions[0].Indices) != 1 {
		t.Fatalf("reductions = %#v", assign.Reductions)
	}
	reduction := assign.Reductions[0].Indices[0]
	if reduction.Extent != 4 {
		t.Fatalf("reduction extent = %d, want 4", reduction.Extent)
	}
	if len(reduction.Provenance) != 2 {
		t.Fatalf("reduction provenance = %#v, want 2 source-axis entries", reduction.Provenance)
	}
	if reduction.Provenance[0].SourceValue != "A" || reduction.Provenance[1].SourceValue != "B" {
		t.Fatalf("reduction provenance sources = %#v", reduction.Provenance)
	}
	if len(assign.LoopOrder) != 2 || assign.LoopOrder[0] != "i" || assign.LoopOrder[1] != "j" {
		t.Fatalf("loop order = %#v", assign.LoopOrder)
	}
}

func TestSdslvTensorRejectsInPlaceTranspose(t *testing.T) {
err := validateSource(`fn F() -> void {
    var A: array<array<f32, 2u>, 2u> = Fill(0.0);
    tensor A[i, j] = A[j, i];
    return;
}`)
	if err == nil || !containsAll(err.Error(), "SDSL-V3216", "unsafe destination alias/remapping") {
		t.Fatalf("error = %v", err)
	}
}

func TestSdslvTensorRejectsFreeIndexScalarUse(t *testing.T) {
err := validateSource(`fn F() -> void {
    var A: array<f32, 4u> = Fill(0.0);
    tensor A[i] = i;
    return;
}`)
	if err == nil || !containsAll(err.Error(), "SDSL-V3218", "free index `i` cannot be used as a scalar value") {
		t.Fatalf("error = %v", err)
	}
}

func TestSdslvValidatedTensorAssignIsLoweringReady(t *testing.T) {
	module := mustTensorModule(t, `shader S {
    workgroup TileA: tile<f32, 2u, 4u>;
    workgroup TileB: tile<f32, 4u, 2u>;

    stage compute [numthreads(1, 1, 1)] fn CS() -> void {
        var Acc: reg_tile<f32, 2u, 2u> = reg_tile_zero();
        let localRow: u32 = 0u;
        let localCol: u32 = 0u;
        tensor Acc[oi, oj] += Sum[kk](
            TileA[localRow + oi, kk] * TileB[kk, localCol + oj]
        );
        return;
    }
}`)
	assigns, diags := ValidatedTensorAssignments(module)
	if len(diags) != 0 {
		t.Fatalf("ValidatedTensorAssignments() diagnostics = %v", diags)
	}
	assign := assigns[0]
	if assign.AssignmentKind != ast.TensorAssignAdd {
		t.Fatalf("assignment kind = %q, want +=", assign.AssignmentKind)
	}
	if assign.AliasPolicy != "no-destination-read" {
		t.Fatalf("alias policy = %q, want no-destination-read", assign.AliasPolicy)
	}
	if len(assign.Reductions) != 1 || assign.Reductions[0].ResultType.Name != "f32" {
		t.Fatalf("reductions = %#v", assign.Reductions)
	}
	if assign.DestinationElementType.Name != "f32" || assign.ResultType.Name != "f32" {
		t.Fatalf("types = dest:%#v rhs:%#v", assign.DestinationElementType, assign.ResultType)
	}
}

func TestSdslvTensorValidationHonorsSdslvTestSourceAndTestInput(t *testing.T) {
	module := mustTensorModule(t, `[Fact]
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
}`)
	module.Source.Path = "guarded_tensor.sdslvtest"
	assigns, diags := ValidatedTensorAssignments(module)
	if len(diags) != 0 {
		t.Fatalf("ValidatedTensorAssignments() diagnostics = %v", diags)
	}
	if len(assigns) != 1 || len(assigns[0].FreeIndices) != 1 || assigns[0].FreeIndices[0].Extent != 4 {
		t.Fatalf("assigns = %#v", assigns)
	}
}

func mustTensorModule(t *testing.T, text string) ast.Module {
	t.Helper()
	tokens, err := lex.Analyze(source.File{Path: "tensor_test.sdslv", Text: text})
	if err != nil {
		t.Fatalf("lex.Analyze() error = %v", err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatalf("parse.BuildModule() error = %v", err)
	}
	return module
}

func containsAll(text string, wants ...string) bool {
	for _, want := range wants {
		if !strings.Contains(text, want) {
			return false
		}
	}
	return true
}
