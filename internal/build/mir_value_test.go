package build

import (
	"reflect"
	"strings"
	"testing"
)

func TestMIRValueLoweringIsStructuredAndBackendNeutral(t *testing.T) {
	tests := []struct {
		name, expression, typ string
		want                  any
	}{
		{"binary arithmetic", "(a + b)", "Int", MIRBinary{}},
		{"comparison", "(a >= b)", "Bool", MIRBinary{}},
		{"conversion", "float64(x)", "Float", MIRConvert{}},
		{"indexing", "items[i]", "Int", MIRIndex{}},
		{"field access", "row.Value", "Int", MIRFieldAccess{}},
		{"clone", "__octClone(items)", "Int[]", MIRClone{}},
		{"range", "__octRange{Start: first, End: last, Step: step}", "Range", MIRRangeValue{}},
		{"array conversion", "__octIntArrayToFloat(items)", "Float[]", MIRArrayConvert{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := lowerMIRValue(tc.expression, tc.typ)
			if reflect.TypeOf(got) != reflect.TypeOf(tc.want) {
				t.Fatalf("%s lowered to %T, want %T", tc.expression, got, tc.want)
			}
			if _, raw := got.(MIRBackendValue); raw {
				t.Fatalf("ordinary expression retained backend text: %#v", got)
			}
		})
	}
}

func TestValueBearingMIRFieldsUseMIRValues(t *testing.T) {
	left, right := mirLocal("left", "Int"), mirLocal("right", "Int")
	condition := MIRBinary{Op: ">", Left: left, Right: right, Type: "Bool"}
	result := MIRBinary{Op: "+", Left: left, Right: right, Type: "Int"}
	block := MIRBlock{
		Statements: []MIRStmt{
			MIRAssign{Target: "sum", Value: result},
			MIRCall{Target: "called", Callee: "Main.Add", Args: []MIRValue{left, right}, ArgTypes: []string{"Int", "Int"}, RetType: "Int"},
		},
		Terminator: MIRBranch{Cond: condition, TrueTarget: "yes", FalseTarget: "no"},
	}
	if _, ok := block.Statements[0].(MIRAssign).Value.(MIRBinary); !ok {
		t.Fatalf("assignment value is not structured: %#v", block.Statements[0])
	}
	if _, ok := block.Statements[1].(MIRCall).Args[0].(MIRLocal); !ok {
		t.Fatalf("call argument is not structured: %#v", block.Statements[1])
	}
	if _, ok := block.Terminator.(MIRBranch).Cond.(MIRBinary); !ok {
		t.Fatalf("branch condition is not structured: %#v", block.Terminator)
	}
	ret := MIRReturn{Value: result}
	if _, ok := ret.Value.(MIRBinary); !ok {
		t.Fatalf("return value is not structured: %#v", ret)
	}

	semanticDump := strings.Join([]string{dumpMIRValue(result), dumpMIRValue(condition), dumpMIRValue(lowerMIRValue("float64(left)", "Float")), dumpMIRValue(lowerMIRValue("__octClone(items)", "Int[]")), dumpMIRValue(lowerMIRValue("__octRange{Start: left, End: right, Step: step}", "Range")), dumpMIRValue(lowerMIRValue("__octIntArrayToFloat(items)", "Float[]"))}, "\n")
	for _, goShape := range []string{"float64(", "__octClone(", "__octRange{", "__octIntArrayToFloat("} {
		if strings.Contains(semanticDump, goShape) {
			t.Fatalf("semantic MIR dump retained Go shape %q:\n%s", goShape, semanticDump)
		}
	}
}
