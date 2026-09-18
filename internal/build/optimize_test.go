package build

import (
	"reflect"
	"testing"
)

func TestFoldValueScalarOperations(t *testing.T) {
	tests := []struct {
		name  string
		value MIRValue
		want  string
	}{
		{"integer", MIRBinary{Op: "+", Left: mirInt("2"), Right: mirInt("3"), Type: "Int"}, "5"},
		{"float", MIRBinary{Op: "*", Left: MIRLiteral{Type: "Float", Value: "1.5"}, Right: MIRLiteral{Type: "Float", Value: "2"}, Type: "Float"}, "3"},
		{"bool", MIRBinary{Op: "&&", Left: mirBool(true), Right: mirBool(false), Type: "Bool"}, "false"},
		{"comparison", MIRBinary{Op: ">", Left: mirInt("7"), Right: mirInt("5"), Type: "Bool"}, "true"},
		{"nested", MIRBinary{Op: "*", Left: MIRBinary{Op: "+", Left: mirInt("2"), Right: mirInt("3"), Type: "Int"}, Right: mirInt("4"), Type: "Int"}, "20"},
		{"conversion", MIRConvert{TargetType: "Float", Value: mirInt("5")}, "5"},
		{"integer division", MIRIntrinsicValue{Kind: "safe-divide", Type: "Int", Args: []MIRValue{mirInt("9"), mirInt("2")}}, "4"},
		{"euclidean modulo", MIRIntrinsicValue{Kind: "euclidean-modulo", Type: "Int", Args: []MIRValue{mirInt("-5"), mirInt("3")}}, "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := FoldValue(tt.value)
			if !changed || dumpMIRValue(got) != tt.want {
				t.Fatalf("FoldValue() = %s, changed=%t; want %s", dumpMIRValue(got), changed, tt.want)
			}
		})
	}
}

func TestFoldValueFailsClosed(t *testing.T) {
	tests := []MIRValue{
		MIRIntrinsicValue{Kind: "safe-divide", Type: "Int", Args: []MIRValue{mirInt("1"), mirInt("0")}},
		MIRIntrinsicValue{Kind: "euclidean-modulo", Type: "Int", Args: []MIRValue{mirInt("1"), mirInt("0")}},
		MIRIntrinsicValue{Kind: "safe-divide", Type: "Int", Args: []MIRValue{mirInt("-9223372036854775808"), mirInt("-1")}},
		MIRBackendValue{Backend: "go", Expression: "effectful()", Type: "Int", Reason: "test"},
		MIRBinary{Op: "*", Left: MIRLocal{Name: "effectful", Type: "Int"}, Right: mirInt("0"), Type: "Int"},
	}
	for _, value := range tests {
		got, changed := FoldValue(value)
		if changed || !reflect.DeepEqual(got, value) {
			t.Fatalf("unsafe/unsupported value changed: %#v -> %#v", value, got)
		}
	}
}

func TestAnalyzeConstantsStraightLineAndJoins(t *testing.T) {
	straight := MIRFunction{Package: "Book", Name: "Straight", Return: "Int", Locals: scalarLocals("x", "y"), Blocks: []MIRBlock{{
		Label: "entry", Statements: []MIRStmt{
			MIRAssign{Target: "x", Value: mirInt("5")},
			MIRAssign{Target: "y", Value: MIRBinary{Op: "+", Left: MIRLocal{Name: "x", Type: "Int"}, Right: mirInt("1"), Type: "Int"}},
		}, Terminator: MIRReturn{Value: MIRLocal{Name: "y", Type: "Int"}},
	}}}
	analysis, err := AnalyzeConstants(straight)
	if err != nil {
		t.Fatal(err)
	}
	assertKnownInt(t, analysis.BeforeStatement["entry"][1]["x"], 5)
	assertKnownInt(t, analysis.Out["entry"]["y"], 6)

	same := joinFunction("5", "5")
	analysis, err = AnalyzeConstants(same)
	if err != nil {
		t.Fatal(err)
	}
	assertKnownInt(t, analysis.In["join"]["x"], 5)

	different := joinFunction("5", "7")
	analysis, err = AnalyzeConstants(different)
	if err != nil {
		t.Fatal(err)
	}
	if got := analysis.In["join"]["x"].Kind; got != ConstantOverdefined {
		t.Fatalf("different join state = %v, want Overdefined", got)
	}
}

func TestAnalyzeConstantsParametersAndCapturesStartOverdefined(t *testing.T) {
	fn := MIRFunction{
		Package: "Book", Name: "Inputs",
		Params:     []MIRField{{Name: "parameter", Type: "Int"}},
		CaptureEnv: []MIRCapture{{Name: "sourceName", Parameter: "captureParameter", Type: "Int"}},
		Return:     "Int",
		Blocks:     []MIRBlock{{Label: "entry", Terminator: MIRReturn{Value: MIRLocal{Name: "captureParameter", Type: "Int"}}}},
	}
	analysis, err := AnalyzeConstants(fn)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.In["entry"]["parameter"].Kind != ConstantOverdefined || analysis.In["entry"]["captureParameter"].Kind != ConstantOverdefined {
		t.Fatalf("runtime inputs must start overdefined: %+v", analysis.In["entry"])
	}
}

func TestAnalyzeConstantsLoopCarriedMutationBecomesOverdefined(t *testing.T) {
	fn := MIRFunction{Package: "Book", Name: "Loop", Params: []MIRField{{Name: "again", Type: "Bool"}}, Return: "Int", Locals: scalarLocals("x"), Blocks: []MIRBlock{
		{Label: "entry", Statements: []MIRStmt{MIRAssign{Target: "x", Value: mirInt("0")}}, Terminator: MIRJump{Target: "loop"}},
		{Label: "loop", Statements: []MIRStmt{MIRAssign{Target: "x", Value: MIRBinary{Op: "+", Left: MIRLocal{Name: "x", Type: "Int"}, Right: mirInt("1"), Type: "Int"}}}, Terminator: MIRBranch{Cond: MIRLocal{Name: "again", Type: "Bool"}, TrueTarget: "loop", FalseTarget: "done"}},
		{Label: "done", Terminator: MIRReturn{Value: MIRLocal{Name: "x", Type: "Int"}}},
	}}
	analysis, err := AnalyzeConstants(fn)
	if err != nil {
		t.Fatal(err)
	}
	if got := analysis.In["loop"]["x"].Kind; got != ConstantOverdefined {
		t.Fatalf("loop-header x = %v, want Overdefined", got)
	}
}

func TestOptimizeFunctionPropagationFoldingAndBranches(t *testing.T) {
	for _, condition := range []bool{true, false} {
		fn := MIRFunction{Package: "Book", Name: "Pipeline", Return: "Int", Locals: scalarLocals("x", "y", "cond"), Blocks: []MIRBlock{
			{Label: "entry", Statements: []MIRStmt{
				MIRAssign{Target: "x", Value: mirInt("2")},
				MIRAssign{Target: "y", Value: MIRBinary{Op: "+", Left: MIRLocal{Name: "x", Type: "Int"}, Right: mirInt("3"), Type: "Int"}},
				MIRAssign{Target: "cond", Value: MIRBinary{Op: "==", Left: MIRLocal{Name: "y", Type: "Int"}, Right: mirInt("5"), Type: "Bool"}},
			}, Terminator: MIRBranch{Cond: func() MIRValue {
				if condition {
					return MIRLocal{Name: "cond", Type: "Bool"}
				}
				return mirBool(false)
			}(), TrueTarget: "yes", FalseTarget: "no"}},
			{Label: "yes", Terminator: MIRReturn{Value: MIRLocal{Name: "y", Type: "Int"}}},
			{Label: "no", Terminator: MIRReturn{Value: mirInt("0")}},
		}}
		optimized, stats, err := OptimizeFunction(fn)
		if err != nil {
			t.Fatal(err)
		}
		entry := optimized.Blocks[0]
		if got := dumpMIRValue(entry.Statements[1].(MIRAssign).Value); got != "5" {
			t.Fatalf("propagation then fold = %s, want 5", got)
		}
		jump, ok := entry.Terminator.(MIRJump)
		if !ok {
			t.Fatalf("terminator = %T, want MIRJump", entry.Terminator)
		}
		want := "yes"
		if !condition {
			want = "no"
		}
		if jump.Target != want {
			t.Fatalf("jump target = %s, want %s", jump.Target, want)
		}
		if stats.LocalReadsPropagated == 0 || stats.ValuesFolded == 0 || stats.BranchesFolded != 1 {
			t.Fatalf("unexpected stats: %+v", stats)
		}
	}
}

func TestOptimizeMIRIsIdempotentAndDeterministic(t *testing.T) {
	module := MIRModule{EntryPackage: "Book", Functions: []MIRFunction{joinFunction("5", "5")}}
	first, _, err := OptimizeMIR(module)
	if err != nil {
		t.Fatal(err)
	}
	second, secondStats, err := OptimizeMIR(first)
	if err != nil {
		t.Fatal(err)
	}
	third, _, err := OptimizeMIR(module)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || dumpMIR(first) != dumpMIR(third) {
		t.Fatal("optimizer is not idempotent and deterministic")
	}
	if secondStats.ValuesFolded != 0 || secondStats.LocalReadsPropagated != 0 || secondStats.BranchesFolded != 0 {
		t.Fatalf("second optimization changed MIR: %+v", secondStats)
	}
}

func joinFunction(left, right string) MIRFunction {
	return MIRFunction{Package: "Book", Name: "Join", Params: []MIRField{{Name: "flag", Type: "Bool"}}, Return: "Int", Locals: scalarLocals("x", "y"), Blocks: []MIRBlock{
		{Label: "entry", Terminator: MIRBranch{Cond: MIRLocal{Name: "flag", Type: "Bool"}, TrueTarget: "left", FalseTarget: "right"}},
		{Label: "left", Statements: []MIRStmt{MIRAssign{Target: "x", Value: mirInt(left)}}, Terminator: MIRJump{Target: "join"}},
		{Label: "right", Statements: []MIRStmt{MIRAssign{Target: "x", Value: mirInt(right)}}, Terminator: MIRJump{Target: "join"}},
		{Label: "join", Statements: []MIRStmt{MIRAssign{Target: "y", Value: MIRBinary{Op: "+", Left: MIRLocal{Name: "x", Type: "Int"}, Right: mirInt("1"), Type: "Int"}}}, Terminator: MIRReturn{Value: MIRLocal{Name: "y", Type: "Int"}}},
	}}
}

func scalarLocals(names ...string) []MIRField {
	result := make([]MIRField, len(names))
	for i, name := range names {
		typ := "Int"
		if name == "cond" {
			typ = "Bool"
		}
		result[i] = MIRField{Name: name, Type: typ}
	}
	return result
}

func assertKnownInt(t *testing.T, state ConstantState, want int64) {
	t.Helper()
	if state.Kind != ConstantKnown || state.Constant.Kind != "Int" || state.Constant.Int != want {
		t.Fatalf("state = %+v, want Const(%d)", state, want)
	}
}
