package build

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildCFGFromWasmCompute(t *testing.T) {
	module := loadWasmComputeMIR(t)
	tests := []struct {
		name         string
		entry        string
		blockOrder   []string
		successors   map[string][]string
		predecessors map[string][]string
		reachable    []string
	}{
		{
			name:       "Max",
			entry:      "entry",
			blockOrder: []string{"entry", "b1", "b2", "b3"},
			successors: map[string][]string{
				"entry": {"b1", "b2"},
				"b1":    {},
				"b2":    {"b3"},
				"b3":    {},
			},
			predecessors: map[string][]string{
				"entry": {},
				"b1":    {"entry"},
				"b2":    {"entry"},
				"b3":    {"b2"},
			},
			reachable: []string{"entry", "b1", "b2", "b3"},
		},
		{
			name:       "SumTo",
			entry:      "entry",
			blockOrder: []string{"entry", "b1", "b2", "b3"},
			successors: map[string][]string{
				"entry": {"b1"},
				"b1":    {"b2", "b3"},
				"b2":    {"b1"},
				"b3":    {},
			},
			predecessors: map[string][]string{
				"entry": {},
				"b1":    {"entry", "b2"},
				"b2":    {"b1"},
				"b3":    {"b1"},
			},
			reachable: []string{"entry", "b1", "b2", "b3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := BuildCFG(findMIRFunction(t, module, tt.name))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Entry != tt.entry {
				t.Fatalf("Entry = %q, want %q", cfg.Entry, tt.entry)
			}
			if !reflect.DeepEqual(cfg.BlockOrder, tt.blockOrder) {
				t.Fatalf("BlockOrder = %#v, want %#v", cfg.BlockOrder, tt.blockOrder)
			}
			if !reflect.DeepEqual(cfg.Successors, tt.successors) {
				t.Fatalf("Successors = %#v, want %#v", cfg.Successors, tt.successors)
			}
			if !reflect.DeepEqual(cfg.Predecessors, tt.predecessors) {
				t.Fatalf("Predecessors = %#v, want %#v", cfg.Predecessors, tt.predecessors)
			}
			if !reflect.DeepEqual(cfg.Reachable, tt.reachable) {
				t.Fatalf("Reachable = %#v, want %#v", cfg.Reachable, tt.reachable)
			}
		})
	}
}

func TestBuildCFGMarksDisconnectedBlockUnreachable(t *testing.T) {
	fn := MIRFunction{Package: "Example", Name: "Disconnected", Blocks: []MIRBlock{
		{Label: "entry", Terminator: MIRReturn{}},
		{Label: "orphan", Terminator: MIRFail{}},
	}}
	cfg, err := BuildCFG(fn)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"entry"}; !reflect.DeepEqual(cfg.Reachable, want) {
		t.Fatalf("Reachable = %#v, want %#v", cfg.Reachable, want)
	}
}

func TestBuildCFGDeduplicatesBranchToOneTarget(t *testing.T) {
	fn := MIRFunction{Package: "Example", Name: "OneTarget", Blocks: []MIRBlock{
		{Label: "entry", Terminator: MIRBranch{TrueTarget: "exit", FalseTarget: "exit"}},
		{Label: "exit", Terminator: MIRReturn{}},
	}}
	cfg, err := BuildCFG(fn)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"exit"}; !reflect.DeepEqual(cfg.Successors["entry"], want) {
		t.Fatalf("Successors[entry] = %#v, want %#v", cfg.Successors["entry"], want)
	}
	if want := []string{"entry"}; !reflect.DeepEqual(cfg.Predecessors["exit"], want) {
		t.Fatalf("Predecessors[exit] = %#v, want %#v", cfg.Predecessors["exit"], want)
	}
}

func TestBuildCFGRejectsInvalidMIR(t *testing.T) {
	tests := []struct {
		name string
		fn   MIRFunction
		want string
	}{
		{name: "no blocks", fn: MIRFunction{Package: "P", Name: "F"}, want: "has no MIR blocks"},
		{name: "empty label", fn: MIRFunction{Package: "P", Name: "F", Blocks: []MIRBlock{{Terminator: MIRReturn{}}}}, want: "empty label"},
		{name: "duplicate label", fn: MIRFunction{Package: "P", Name: "F", Blocks: []MIRBlock{{Label: "entry", Terminator: MIRReturn{}}, {Label: "entry", Terminator: MIRReturn{}}}}, want: "duplicate MIR block label entry"},
		{name: "missing terminator", fn: MIRFunction{Package: "P", Name: "F", Blocks: []MIRBlock{{Label: "entry"}}}, want: "has no terminator"},
		{name: "missing jump target", fn: MIRFunction{Package: "P", Name: "F", Blocks: []MIRBlock{{Label: "entry", Terminator: MIRJump{Target: "missing"}}}}, want: "references missing target missing"},
		{name: "missing branch target", fn: MIRFunction{Package: "P", Name: "F", Blocks: []MIRBlock{{Label: "entry", Terminator: MIRBranch{TrueTarget: "entry", FalseTarget: "missing"}}}}, want: "references missing target missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildCFG(tt.fn)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildCFG error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func loadWasmComputeMIR(t *testing.T) MIRModule {
	t.Helper()
	module, _, err := LoadMIR(wasmComputeExamplePath())
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func wasmComputeExamplePath() string {
	return "../../Examples/WasmCompute"
}

func findMIRFunction(t *testing.T, module MIRModule, name string) MIRFunction {
	t.Helper()
	for _, fn := range module.Functions {
		if fn.Package == "WasmCompute" && fn.Name == name {
			return fn
		}
	}
	t.Fatalf("MIR function WasmCompute.%s not found", name)
	return MIRFunction{}
}
