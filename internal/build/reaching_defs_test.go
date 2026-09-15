package build

import (
	"reflect"
	"strings"
	"testing"
)

func TestReachingDefinitionsStraightLineRedefinitionAndUsePrecision(t *testing.T) {
	fn := MIRFunction{Package: "Example", Name: "Straight", Blocks: []MIRBlock{{
		Label: "entry",
		Statements: []MIRStmt{
			MIRAssign{Target: "x", Value: MIRLiteral{Type: "Int", Value: "1"}},
			MIRAssign{Target: "y", Value: MIRBinary{Op: "+", Left: MIRLocal{Name: "x"}, Right: MIRLiteral{Type: "Int", Value: "2"}}},
			MIRAssign{Target: "x", Value: MIRBinary{Op: "*", Left: MIRLocal{Name: "y"}, Right: MIRLiteral{Type: "Int", Value: "3"}}},
		},
		Terminator: MIRReturn{Value: MIRLocal{Name: "x"}},
	}}}
	analysis, err := AnalyzeReachingDefinitions(fn)
	if err != nil {
		t.Fatal(err)
	}
	if got := localsOf(analysis.Out["entry"]); !reflect.DeepEqual(got, []string{"y", "x"}) {
		t.Fatalf("OUT locals = %#v, want y then latest x", got)
	}
	uses := analysis.UseDefs.UsesByBlock["entry"]
	if got := analysis.AtUse[uses[0].Site]; len(got) != 1 || got[0].Statement != 0 {
		t.Fatalf("x use in stmt1 reaches %#v, want stmt0 x", got)
	}
	if got := analysis.AtUse[uses[len(uses)-1].Site]; len(got) != 1 || got[0].Statement != 2 {
		t.Fatalf("return x reaches %#v, want stmt2 x", got)
	}
	if got := analysis.DefUses[analysis.UseDefs.Definitions[0]]; len(got) != 1 || got[0] != uses[0].Site {
		t.Fatalf("first x def-use chain = %#v", got)
	}
}

func TestReachingDefinitionsChooseJoin(t *testing.T) {
	fn := findMIRFunction(t, loadWasmComputeMIR(t), "Choose")
	analysis, err := AnalyzeReachingDefinitions(fn)
	if err != nil {
		t.Fatal(err)
	}
	returnUse, ok := findUse(analysis, func(site UseSite) bool { return site.Operand == "return" })
	if !ok {
		t.Fatal("Choose return use not found")
	}
	reaching := analysis.AtUse[returnUse]
	if len(reaching) != 2 || reaching[0].Local != reaching[1].Local {
		t.Fatalf("definitions at Choose return = %#v, want two definitions of x", reaching)
	}
	if reaching[0].Statement == reaching[1].Statement && reaching[0].Block == reaching[1].Block {
		t.Fatalf("definitions do not have distinct MIR positions: %#v", reaching)
	}
}

func TestReachingDefinitionsSumToRequiresLoopFixedPoint(t *testing.T) {
	fn := findMIRFunction(t, loadWasmComputeMIR(t), "SumTo")
	analysis, err := AnalyzeReachingDefinitions(fn)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.BlocksProcessed <= len(analysis.CFG.Reachable) {
		t.Fatalf("processed %d blocks for %d-block loop, want reconsideration after backedge facts", analysis.BlocksProcessed, len(analysis.CFG.Reachable))
	}
	header := "b1"
	for _, local := range []string{"__oct_user_1", "__oct_user_2"} {
		count := 0
		for _, def := range analysis.In[header] {
			if def.Local == local {
				count++
			}
		}
		if count != 2 {
			t.Fatalf("IN[%s] has %d definitions of %s, want entry and loop-carried definitions: %#v", header, count, local, analysis.In[header])
		}
	}
}

func TestReachingDefinitionsParametersAreSyntheticEntryDefinitions(t *testing.T) {
	fn := MIRFunction{Package: "Example", Name: "Identity", Params: []MIRField{{Name: "x", Type: "Int"}}, Blocks: []MIRBlock{{
		Label: "entry", Terminator: MIRReturn{Value: MIRLocal{Name: "x"}},
	}}}
	analysis, err := AnalyzeReachingDefinitions(fn)
	if err != nil {
		t.Fatal(err)
	}
	if got := analysis.In["entry"]; len(got) != 1 || got[0].Kind != DefinitionParameter || got[0].Local != "x" {
		t.Fatalf("entry IN = %#v, want synthetic parameter x", got)
	}
	use := analysis.UseDefs.Uses[0].Site
	if !reflect.DeepEqual(analysis.AtUse[use], analysis.In["entry"]) {
		t.Fatalf("parameter use reaches %#v, want %#v", analysis.AtUse[use], analysis.In["entry"])
	}
}

func TestReachingDefinitionsCapturesUseTheirMIRParameterIdentity(t *testing.T) {
	fn := MIRFunction{Package: "Example", Name: "Worker", CaptureEnv: []MIRCapture{{Name: "threshold", Parameter: "__oct_capture_0", Type: "Int"}}, Blocks: []MIRBlock{{
		Label: "entry", Terminator: MIRReturn{Value: MIRLocal{Name: "__oct_capture_0"}},
	}}}
	analysis, err := AnalyzeReachingDefinitions(fn)
	if err != nil {
		t.Fatal(err)
	}
	if got := analysis.In["entry"]; len(got) != 1 || got[0].Kind != DefinitionCapture || got[0].Local != "__oct_capture_0" {
		t.Fatalf("entry IN = %#v, want synthetic capture parameter", got)
	}
	use := analysis.UseDefs.Uses[0].Site
	if len(analysis.AtUse[use]) != 1 || analysis.AtUse[use][0].Local != use.Local {
		t.Fatalf("capture use %q reaches %#v", use.Local, analysis.AtUse[use])
	}
}

func TestReachingDefinitionsIgnoresUnreachableBlocks(t *testing.T) {
	fn := MIRFunction{Package: "Example", Name: "Disconnected", Blocks: []MIRBlock{
		{Label: "entry", Statements: []MIRStmt{MIRAssign{Target: "x", Value: MIRLiteral{Type: "Int", Value: "1"}}}, Terminator: MIRReturn{Value: MIRLocal{Name: "x"}}},
		{Label: "orphan", Statements: []MIRStmt{MIRAssign{Target: "x", Value: MIRLiteral{Type: "Int", Value: "2"}}}, Terminator: MIRReturn{Value: MIRLocal{Name: "x"}}},
	}}
	analysis, err := AnalyzeReachingDefinitions(fn)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(analysis.Unreachable, []string{"orphan"}) {
		t.Fatalf("Unreachable = %#v", analysis.Unreachable)
	}
	if _, exists := analysis.In["orphan"]; exists {
		t.Fatal("unreachable block unexpectedly has IN facts")
	}
	for site := range analysis.AtUse {
		if site.Block == "orphan" {
			t.Fatal("unreachable use unexpectedly has reaching facts")
		}
	}
}

func TestReachingDefinitionsRejectsMalformedCFGAndIsDeterministic(t *testing.T) {
	bad := MIRFunction{Package: "P", Name: "Bad", Blocks: []MIRBlock{{Label: "entry", Terminator: MIRJump{Target: "missing"}}}}
	if _, err := AnalyzeReachingDefinitions(bad); err == nil || !strings.Contains(err.Error(), "missing target") {
		t.Fatalf("malformed CFG error = %v", err)
	}
	fn := findMIRFunction(t, loadWasmComputeMIR(t), "Choose")
	first, err := DumpReachingDefinitions(fn)
	if err != nil {
		t.Fatal(err)
	}
	second, err := DumpReachingDefinitions(fn)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("reaching-definitions dump is not deterministic")
	}
}

func localsOf(set DefinitionSet) []string {
	locals := make([]string, len(set))
	for i, def := range set {
		locals[i] = def.Local
	}
	return locals
}

func findUse(analysis ReachingDefinitions, match func(UseSite) bool) (UseSite, bool) {
	for _, use := range analysis.UseDefs.Uses {
		if match(use.Site) {
			return use.Site, true
		}
	}
	return UseSite{}, false
}
