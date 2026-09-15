package project

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVerilogProfileIsSingleEntryPackageAuthority(t *testing.T) {
	root := filepath.Join("..", "..", "Language", "Profiles", "VerilogM0", "ownership")
	program, err := Load(filepath.Join(root, "single"))
	if err != nil {
		t.Fatal(err)
	}
	if program.Profile != "Verilog" || program.Packages["Main"].Profile != "Verilog" {
		t.Fatalf("unexpected program profile: %+v", program)
	}
	if len(program.Packages["Main"].Functions) != 2 {
		t.Fatalf("unprofiled sibling did not inherit compilation-unit profile")
	}

	_, err = Load(filepath.Join(root, "duplicate"))
	if err == nil || !strings.Contains(err.Error(), "duplicate profile declaration 'Verilog'") {
		t.Fatalf("duplicate profile error = %v", err)
	}

	_, err = Load(filepath.Join(root, "nonentry"))
	if err == nil || !strings.Contains(err.Error(), "cannot select the backend") {
		t.Fatalf("non-entry profile error = %v", err)
	}
}
