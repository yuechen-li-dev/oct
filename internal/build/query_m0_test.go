package build

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

func TestQueryM0LowersToOrdinaryFusedFlow(t *testing.T) {
	program, err := project.LoadForTest("../../Language/Data/QueryM0/valid")
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	module, err := lowerProgram(program, compileOptions{})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{"ReadyIDs": false, "ReadyJobs": false, "NormalizedJobs": false, "ReadyTableRows": false}
	for _, flow := range module.Flows {
		if _, ok := want[flow.Name]; !ok {
			continue
		}
		want[flow.Name] = true
		if len(flow.States) != 1 || flow.States[0].Name != "Scan" {
			t.Fatalf("query %s did not fuse to one Scan state: %#v", flow.Name, flow.States)
		}
		if len(flow.Board) != 2 || flow.Board[0].Name != "Cursor" || flow.Board[1].Name != "Emitted" {
			t.Fatalf("query %s cursor board = %#v", flow.Name, flow.Board)
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("missing lowered query flow %s", name)
		}
	}

	generated, err := emitGo(module)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"type MIRIterator", "type QueryPlan", "chan ", "go func("} {
		if strings.Contains(generated, forbidden) {
			t.Fatalf("generated query introduced forbidden runtime shape %q", forbidden)
		}
	}
	if strings.Count(generated, "type __octFlow_QueryM0Valid_ReadyIDs struct") != 1 {
		t.Fatal("ReadyIDs did not emit exactly one existing FLOW state machine")
	}
}
