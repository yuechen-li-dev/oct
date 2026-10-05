package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

// The typechecker and the sidecar builtin table describe the same builtins in
// two places: the typechecker says what a call means, the table says how the
// compiled lane carries it to a sidecar. This is a host-side check that the
// two agree for every entry. Each builtin is called from a function whose
// parameters, result and fallibility are taken from the table; a wrong
// parameter type, result type or fallibility is a typecheck error, and the
// lowered call must then name the sidecar the table names.
//
// What the builtins do is not checked here. That is the business of the
// library tests under Libraries/, which run in both lanes.
func TestSidecarBuiltinTableAgreesWithTypechecker(t *testing.T) {
	entries := builtin.SidecarBuiltins()
	if len(entries) == 0 {
		t.Fatal("the sidecar builtin table is empty")
	}
	for _, entry := range entries {
		t.Run(entry.Name, func(t *testing.T) {
			if !builtin.IsName(entry.Name) {
				t.Fatalf("%s is in the sidecar table and is not a builtin", entry.Name)
			}
			params := make([]string, len(entry.Params))
			args := make([]string, len(entry.Params))
			for i, param := range entry.Params {
				params[i] = fmt.Sprintf("a%d: %s", i, param.Oct)
				args[i] = fmt.Sprintf("a%d", i)
				if param.Handle != "" && param.Oct != "Int" {
					t.Fatalf("argument %d is a handle and is carried as %s; a handle is an Int", i+1, param.Oct)
				}
			}
			signature, call := entry.Result.Oct, fmt.Sprintf("%s(%s)", entry.Name, strings.Join(args, ", "))
			if entry.Fallible {
				signature += " ! Error"
				call += "?"
			}
			source := fmt.Sprintf("package Main\n\nfn Probe(%s) -> %s {\n    return %s\n}\n\nfn main() -> Int {\n    return 0\n}\n", strings.Join(params, ", "), signature, call)
			path := filepath.Join(t.TempDir(), "main.oct")
			if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			program, err := project.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := typecheck.CheckProgram(program); err != nil {
				t.Fatalf("the table and the typechecker disagree about %s: %v\n%s", entry.Name, err, source)
			}
			module, err := lowerProgram(program, compileOptions{})
			if err != nil {
				t.Fatalf("lowering %s: %v\n%s", entry.Name, err, source)
			}
			generated, err := emitGo(module)
			if err != nil {
				t.Fatalf("emitting %s: %v", entry.Name, err)
			}
			// A handle is an Int in the program and a typed handle on the
			// wire: sent with its type, and checked for it when it comes back.
			for i, param := range entry.Params {
				if param.Handle == "" {
					continue
				}
				sent := fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: %q, HandleType: %q, HandleID: ", entry.Family, param.Handle)
				if !strings.Contains(generated, sent) {
					t.Fatalf("argument %d is not sent as a %s handle of family %s", i+1, param.Handle, entry.Family)
				}
			}
			if entry.Result.Handle != "" {
				checked := fmt.Sprintf("__octOctxiliaryValidateHandle(__value, %q, %q)", entry.Family, entry.Result.Handle)
				if !strings.Contains(generated, checked) || !strings.Contains(generated, "__value.HandleID") {
					t.Fatalf("the result is not checked and read as a %s handle of family %s", entry.Result.Handle, entry.Family)
				}
			}
			var calls []MIRGenericOctxiliaryCall
			for _, function := range module.Functions {
				if function.Name != "Probe" {
					continue
				}
				for _, block := range function.Blocks {
					for _, statement := range block.Statements {
						if call, ok := statement.(MIRGenericOctxiliaryCall); ok {
							calls = append(calls, call)
						}
					}
				}
			}
			if len(calls) != 1 {
				t.Fatalf("expected one sidecar call in Probe, got %d", len(calls))
			}
			got := calls[0]
			if got.SidecarCommand != entry.Sidecar || got.Family != entry.Family || got.WireName != entry.Name {
				t.Fatalf("lowered to %s %s.%s, want %s %s.%s", got.SidecarCommand, got.Family, got.WireName, entry.Sidecar, entry.Family, entry.Name)
			}
			if got.RetType != entry.Result.Oct || got.Fallible != entry.Fallible || got.RetHandle != entry.Result.Handle {
				t.Fatalf("lowered result %s fallible=%t handle=%q, want %s fallible=%t handle=%q", got.RetType, got.Fallible, got.RetHandle, entry.Result.Oct, entry.Fallible, entry.Result.Handle)
			}
			for i, param := range entry.Params {
				if got.ArgHandles[i] != param.Handle {
					t.Fatalf("argument %d lowered with handle %q, want %q", i+1, got.ArgHandles[i], param.Handle)
				}
			}
		})
	}
}
