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
