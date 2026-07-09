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
