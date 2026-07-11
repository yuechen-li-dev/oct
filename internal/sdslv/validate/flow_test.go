package validate

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func parsedM31Flow(t *testing.T, text string) ast.FlowStmt {
	t.Helper()
	tokens, err := lex.Analyze(source.File{Path: "flow.sdslv", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	fn := module.Decls[0].(ast.FunctionDecl)
	return fn.Body.Statements[0].(ast.FlowStmt)
}

func validateM31Flow(t *testing.T, text string) ValidatedFlow {
	t.Helper()
	got, issues := ValidateFlow(parsedM31Flow(t, text))
	if len(issues) != 0 {
		t.Fatalf("issues: %#v", issues)
	}
	return got
}

func requireM31FlowIssue(t *testing.T, text, code string) {
	t.Helper()
	_, issues := ValidateFlow(parsedM31Flow(t, text))
	for _, issue := range issues {
		if issue.Code == code {
			return
		}
	}
	t.Fatalf("missing %s in %#v", code, issues)
}

func TestSdslvFlowAddsImplicitFallthroughAndCompletion(t *testing.T) {
	got := validateM31Flow(t, `fn F() -> void { flow F { state A { let x: u32 = 1u; } state B { } } }`)
	if got.Entry != 0 || got.MaxStackDepth != 0 || got.HasPushPop || got.HasGoto {
		t.Fatalf("metadata = %#v", got)
	}
	if got.States[0].Terminator.Kind != FlowFallthrough || got.States[0].Terminator.Target != 1 {
		t.Fatalf("A terminator = %#v", got.States[0].Terminator)
	}
	if got.States[1].Terminator.Kind != FlowFallthrough || got.States[1].Terminator.Target != FlowCompleteStateID {
		t.Fatalf("B terminator = %#v", got.States[1].Terminator)
	}
}

func TestSdslvFlowResolvesPushReturnSuccessorAndDepth(t *testing.T) {
	got := validateM31Flow(t, `fn F() -> void { flow F { state A { push Shared; } state B { finish; } state Shared { pop; } } }`)
	if got.States[0].Terminator.Kind != FlowPush || got.States[0].Terminator.Target != 2 || got.States[0].Terminator.ReturnTo != 1 {
		t.Fatalf("push terminator = %#v", got.States[0].Terminator)
	}
	if got.MaxStackDepth != 1 || !got.HasPushPop {
		t.Fatalf("metadata = %#v", got)
	}
}

func TestSdslvFlowFinalPushReturnsToCompletion(t *testing.T) {
	got := validateM31Flow(t, `fn F() -> void { flow F { state Entry { goto Final; } state Cleanup { pop; } state Final { push Cleanup; } } }`)
	if got.States[2].Terminator.ReturnTo != FlowCompleteStateID {
		t.Fatalf("return successor = %d", got.States[2].Terminator.ReturnTo)
	}
}

func TestSdslvFlowComputesMaximumStackDepthForNestedPush(t *testing.T) {
	got := validateM31Flow(t, `fn F() -> void { flow F { state A { push B; } state Done { finish; } state B { push C; } state BReturn { pop; } state C { pop; } } }`)
	if got.MaxStackDepth != 2 {
		t.Fatalf("MaxStackDepth = %d", got.MaxStackDepth)
	}
}

func TestSdslvFlowAllowsSharedSubflowWithCallerSpecificReturns(t *testing.T) {
	got := validateM31Flow(t, `fn F() -> void { flow F { state A { push Shared; } state B { push Shared; } state Done { finish; } state Shared { pop; } } }`)
	if got.MaxStackDepth != 1 {
		t.Fatalf("MaxStackDepth = %d", got.MaxStackDepth)
	}
	if len(got.States[3].ReachableDepths) != 1 || got.States[3].ReachableDepths[0] != 1 {
		t.Fatalf("Shared depths = %#v", got.States[3].ReachableDepths)
	}
}

func TestSdslvFlowAllowsGotoCycleAtEmptyStack(t *testing.T) {
	got := validateM31Flow(t, `fn F() -> void { flow F { state A { goto B; } state B { goto A; } } }`)
	if !got.HasGoto || got.MaxStackDepth != 0 {
		t.Fatalf("metadata = %#v", got)
	}
}

func TestSdslvFlowFinishTerminatesWithNonemptyStack(t *testing.T) {
	got := validateM31Flow(t, `fn F() -> void { flow F { state A { push Exit; } state Exit { finish; } } }`)
	if got.MaxStackDepth != 1 {
		t.Fatalf("MaxStackDepth = %d", got.MaxStackDepth)
	}
}

func TestSdslvFlowMarksBarrierStateAndAllowsUniformSubflow(t *testing.T) {
	got := validateM31Flow(t, `fn F() -> void { flow F { state A { push Barrier; } state Done { finish; } state Barrier { WorkgroupMemoryBarrierWithSync(); pop; } } }`)
	if !got.States[2].HasWorkgroupBarrier {
		t.Fatalf("barrier metadata missing: %#v", got.States[2])
	}
}

func TestSdslvFlowRejectsPushCycles(t *testing.T) {
	for _, text := range []string{
		`fn F() -> void { flow F { state A { push A; } } }`,
		`fn F() -> void { flow F { state A { push B; } state B { push A; } } }`,
		`fn F() -> void { flow F { state A { push B; } state B { push C; } state C { push A; } } }`,
	} {
		t.Run(text, func(t *testing.T) {
			requireM31FlowIssue(t, text, "SDSL-V3105")
		})
	}
}

func TestSdslvFlowRejectsPopUnderflowAndMixedDepth(t *testing.T) {
	requireM31FlowIssue(t, `fn F() -> void { flow F { state A { pop; } } }`, "SDSL-V3108")
	requireM31FlowIssue(t, `fn F() -> void { flow F { state A { push Shared; } state B { goto Shared; } state Shared { pop; } } }`, "SDSL-V3110")
}

func TestSdslvFlowRejectsPushedRegionEscapes(t *testing.T) {
	requireM31FlowIssue(t, `fn F() -> void { flow F { state A { push B; } state Done { finish; } state B { goto Done; } } }`, "SDSL-V3109")
	requireM31FlowIssue(t, `fn F() -> void { flow F { state A { push B; } state Done { finish; } state B { } } }`, "SDSL-V3107")
}

func TestSdslvFlowRejectsBarrierStackAmbiguity(t *testing.T) {
	requireM31FlowIssue(t, `fn F() -> void { flow F { state A { push Barrier; } state B { push Barrier; } state Done { finish; } state Barrier { WorkgroupMemoryBarrierWithSync(); pop; } } }`, "SDSL-V3114")
}

func TestSdslvFlowRejectsStatementsAfterTransitionAndNestedTransitions(t *testing.T) {
	requireM31FlowIssue(t, `fn F() -> void { flow F { state A { push B; let x: u32 = 1u; } state B { pop; } } }`, "SDSL-V3103")
	requireM31FlowIssue(t, `fn F() -> void { flow F { state A { if true { push B; } } state B { pop; } } }`, "SDSL-V3102")
}

func TestSdslvFlowReportsPushCyclePath(t *testing.T) {
	_, issues := ValidateFlow(parsedM31Flow(t, `fn F() -> void { flow F { state A { push B; } state B { push C; } state C { push A; } } }`))
	for _, issue := range issues {
		if issue.Code == "SDSL-V3105" && strings.Contains(issue.Message, "A -> B -> C -> A") {
			return
		}
	}
	t.Fatalf("missing cycle path in %#v", issues)
}
