package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

func TestVerilogProfileRoutesFrontendMIRToDeterministicSystemVerilog(t *testing.T) {
	sourcePath := copyVerilogFixture(t, filepath.Join("..", "..", "Language", "Profiles", "VerilogM0", "valid", "combinational.oct"))
	program, err := project.Load(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if program.Profile != "Verilog" {
		t.Fatalf("profile = %q, want Verilog", program.Profile)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	module, err := lowerProgram(program, compileOptions{allowNoEntry: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSystemVerilogLegal(module); err != nil {
		t.Fatal(err)
	}
	first, err := emitSystemVerilog(module)
	if err != nil {
		t.Fatal(err)
	}
	second, err := emitSystemVerilog(module)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("SystemVerilog emission is nondeterministic")
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "Language", "Profiles", "VerilogM0", "valid", "combinational.golden.sv"))
	if err != nil {
		t.Fatal(err)
	}
	if first != string(want) {
		t.Fatalf("SystemVerilog golden mismatch\nwant:\n%s\ngot:\n%s", want, first)
	}
	for _, fragment := range []string{
		"module Add(", "input  logic signed [63:0] A", "output logic Result",
		"64'sd42", "(A + B)", "(A - B)", "(A * B)", "(A > B)", "(!Value)", "Sum =",
	} {
		if !strings.Contains(first, fragment) {
			t.Errorf("missing %q in generated SystemVerilog", fragment)
		}
	}

	result, err := Compile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(result.ArtifactPath) != ".sv" || result.GeneratedSourcePath != result.ArtifactPath {
		t.Fatalf("unexpected Verilog build result: %+v", result)
	}
	compiled, err := os.ReadFile(result.ArtifactPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(compiled) != first {
		t.Fatal("compiled .sv artifact differs from direct MIR emission")
	}
	if strings.Contains(first, "package main") || strings.Contains(first, "func ") {
		t.Fatal("SystemVerilog output contains Go backend syntax")
	}
}

func TestVerilogProfileFocusedCapabilityDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"string", "Verilog profile does not support String values"},
		{"dynamic_array", "Verilog profile does not support dynamic array values"},
		{"builtin", "Verilog profile does not support builtin Abs in M0"},
		{"flow", "Verilog profile does not support FLOW in M0"},
		{"branch", "Verilog profile does not support branch/control-flow lowering in M0"},
		{"call", "Verilog profile does not support inter-function calls in M0"},
		{"unknown_profile", "unknown profile 'CUDA'"},
		{"duplicate_profile", "duplicate profile declaration 'Verilog'"},
		{"conflicting_profile", "conflicting profile declarations 'Verilog' and 'Go'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := filepath.Join("..", "..", "Language", "Profiles", "VerilogM0", "invalid", tc.name+".octfail")
			data, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}
			parts := strings.SplitN(string(data), "\n\n", 2)
			if len(parts) != 2 {
				t.Fatalf("malformed fixture %s", fixture)
			}
			sourcePath := filepath.Join(t.TempDir(), tc.name+".oct")
			if err := os.WriteFile(sourcePath, []byte(parts[1]), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = Compile(sourcePath)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestSystemVerilogLegalityRejectsMIRBackendValue(t *testing.T) {
	module := MIRModule{
		EntryPackage: "Main",
		Functions: []MIRFunction{{
			Package: "Main", Name: "Legacy", Return: "Int",
			Blocks: []MIRBlock{{Label: "entry", Terminator: MIRReturn{Value: MIRBackendValue{Backend: "go", Expression: "x", Type: "Int", Reason: "legacy-test"}}}},
		}},
	}
	err := CheckSystemVerilogLegal(module)
	if err == nil || !strings.Contains(err.Error(), "does not support MIRBackendValue") {
		t.Fatalf("error = %v", err)
	}
}

func TestOrdinaryProgramStillUsesGoBackend(t *testing.T) {
	fixture := filepath.Join("..", "..", "Language", "Profiles", "VerilogM0", "default", "ordinary.oct")
	sourcePath := copyVerilogFixture(t, fixture)
	result, err := Compile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(result.GeneratedSourcePath) != ".go" {
		t.Fatalf("ordinary generated source = %s, want .go", result.GeneratedSourcePath)
	}
}

func copyVerilogFixture(t *testing.T, fixture string) string {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), filepath.Base(fixture))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
