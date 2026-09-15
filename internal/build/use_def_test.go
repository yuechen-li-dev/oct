package build

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractUseDefInfoFromStructuredMIR(t *testing.T) {
	fn := MIRFunction{
		Package: "Example",
		Name:    "Straight",
		Params:  []MIRField{{Name: "input", Type: "Int"}},
		Blocks: []MIRBlock{{
			Label: "entry",
			Statements: []MIRStmt{
				MIRAssign{Target: "x", Value: MIRLiteral{Type: "Int", Value: "1"}},
				MIRAssign{Target: "y", Value: MIRBinary{Op: "+", Left: MIRLocal{Name: "x", Type: "Int"}, Right: MIRConvert{TargetType: "Int", Value: MIRUnary{Op: "-", Value: MIRLocal{Name: "input", Type: "Int"}, Type: "Int"}}, Type: "Int"}},
				MIRAssign{Target: "x", Value: MIRBinary{Op: "*", Left: MIRLocal{Name: "y", Type: "Int"}, Right: MIRLiteral{Type: "Int", Value: "3"}, Type: "Int"}},
			},
			Terminator: MIRReturn{Value: MIRLocal{Name: "x", Type: "Int"}},
		}},
	}
	info, err := ExtractUseDefInfo(fn)
	if err != nil {
		t.Fatal(err)
	}
	gotDefs := make([]string, len(info.Definitions))
	for i, def := range info.Definitions {
		gotDefs[i] = string(def.Kind) + ":" + def.Local
	}
	wantDefs := []string{"parameter:input", "statement:x", "statement:y", "statement:x"}
	if !reflect.DeepEqual(gotDefs, wantDefs) {
		t.Fatalf("definitions = %#v, want %#v", gotDefs, wantDefs)
	}
	gotUses := make([]string, len(info.Uses))
	for i, use := range info.Uses {
		gotUses[i] = use.Site.Operand + ":" + use.Site.Local
	}
	wantUses := []string{"value.left:x", "value.right.value.value:input", "value.left:y", "return:x"}
	if !reflect.DeepEqual(gotUses, wantUses) {
		t.Fatalf("uses = %#v, want %#v", gotUses, wantUses)
	}
	if len(info.CompoundDefinitions) != 0 {
		t.Fatalf("compound definitions = %#v, want none", info.CompoundDefinitions)
	}
}

func TestExtractUseDefInfoClassifiesCompoundWrites(t *testing.T) {
	fn := MIRFunction{Package: "Example", Name: "Compound", Blocks: []MIRBlock{{
		Label: "entry",
		Statements: []MIRStmt{
			MIRIndexAssign{Target: "xs", Indices: []MIRValue{MIRLocal{Name: "i", Type: "Int"}}, Value: MIRLocal{Name: "value", Type: "Int"}},
			MIRRowAssign{Target: "matrix", Index: MIRLocal{Name: "row", Type: "Int"}, Value: MIRLocal{Name: "values", Type: "Int[]"}},
		},
		Terminator: MIRReturn{Value: MIRLiteral{Type: "Int", Value: "0"}},
	}}}
	info, err := ExtractUseDefInfo(fn)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Definitions) != 0 {
		t.Fatalf("whole-local definitions = %#v, want none", info.Definitions)
	}
	if got := []string{info.CompoundDefinitions[0].Kind, info.CompoundDefinitions[1].Kind}; !reflect.DeepEqual(got, []string{"index", "row"}) {
		t.Fatalf("compound kinds = %#v", got)
	}
	gotUses := make([]string, len(info.Uses))
	for i, use := range info.Uses {
		gotUses[i] = use.Site.Local
	}
	if want := []string{"xs", "i", "value", "matrix", "row", "values"}; !reflect.DeepEqual(gotUses, want) {
		t.Fatalf("uses = %#v, want %#v", gotUses, want)
	}
}

func TestExtractUseDefInfoRecognizesEveryWholeLocalResultStatement(t *testing.T) {
	fn := MIRFunction{Package: "Example", Name: "Results", Blocks: []MIRBlock{{
		Label: "entry",
		Statements: []MIRStmt{
			MIRCall{Target: "call", Callee: "F"},
			MIRGenericOctxiliaryCall{Target: "wrapper"},
			MIRDestructureCall{Targets: []string{"first", "second"}, Callee: "Pair"},
			MIRConstructRecord{Target: "record", TypeName: "R"},
			MIRConstructArray{Target: "array", ElemType: "Int"},
			MIRBatchMap{Target: "batch", Input: MIRLocal{Name: "array"}, Worker: "Work"},
		},
		Terminator: MIRReturn{Value: MIRLocal{Name: "call"}},
	}}}
	info, err := ExtractUseDefInfo(fn)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := localsOfDefinitions(info.Definitions), []string{"call", "wrapper", "first", "second", "record", "array", "batch"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("definition locals = %#v, want %#v", got, want)
	}
}

func TestExtractUseDefInfoWalksAllStructuredValueForms(t *testing.T) {
	local := func(name string) MIRValue { return MIRLocal{Name: name, Type: "Int"} }
	value := MIRIntrinsicValue{Kind: "all", Args: []MIRValue{
		MIRClone{Value: local("clone")},
		MIRIndex{Target: local("indexed"), Index: local("index")},
		MIRFieldAccess{Target: local("record"), Field: "X"},
		MIRRangeValue{Start: local("start"), End: local("end"), Step: local("step"), HasStart: true, HasEnd: true, HasStep: true},
		MIRArrayConvert{Value: local("converted")},
		MIRLength{Value: local("length")},
		MIRMatrixColumnCount{Value: local("matrix")},
		MIRResultValue{Value: local("ok")},
		MIRResultValue{Error: local("error"), IsError: true},
		MIREnumValue{EnumType: "E", Variant: "Some", Payload: local("payload")},
		MIREnumPayload{Value: local("subject")},
		MIRFunctionRef{Name: "F"},
		MIRLiteral{Type: "Int", Value: "1"},
	}}
	fn := MIRFunction{Package: "Example", Name: "Values", Blocks: []MIRBlock{{
		Label: "entry", Statements: []MIRStmt{MIRAssign{Target: "out", Value: value}}, Terminator: MIRReturn{Value: local("out")},
	}}}
	info, err := ExtractUseDefInfo(fn)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(info.Uses)-1)
	for _, use := range info.Uses[:len(info.Uses)-1] {
		got = append(got, use.Site.Local)
	}
	want := []string{"clone", "indexed", "index", "record", "start", "end", "step", "converted", "length", "matrix", "ok", "error", "payload", "subject"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nested uses = %#v, want %#v", got, want)
	}
}

func TestExtractUseDefInfoRejectsOpaqueOrMalformedMIR(t *testing.T) {
	tests := []struct {
		name string
		fn   MIRFunction
		want string
	}{
		{
			name: "opaque backend value",
			fn: MIRFunction{Package: "P", Name: "F", Blocks: []MIRBlock{{Label: "entry", Statements: []MIRStmt{
				MIRAssign{Target: "x", Value: MIRBackendValue{Backend: "go", Reason: "legacy"}},
			}, Terminator: MIRReturn{Value: MIRLocal{Name: "x"}}}}},
			want: "local uses are not structurally available",
		},
		{
			name: "empty target",
			fn: MIRFunction{Package: "P", Name: "F", Blocks: []MIRBlock{{Label: "entry", Statements: []MIRStmt{
				MIRAssign{Value: MIRLiteral{Type: "Int", Value: "1"}},
			}, Terminator: MIRReturn{Value: MIRLiteral{Type: "Int", Value: "0"}}}}},
			want: "empty definition target",
		},
		{
			name: "nil nested value",
			fn:   MIRFunction{Package: "P", Name: "F", Blocks: []MIRBlock{{Label: "entry", Terminator: MIRReturn{Value: MIRUnary{Op: "-"}}}}},
			want: "nil MIR value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractUseDefInfo(tt.fn)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestExtractUseDefInfoAllowsVoidReturnWithoutAUse(t *testing.T) {
	fn := MIRFunction{Package: "P", Name: "Void", Blocks: []MIRBlock{{Label: "entry", Terminator: MIRReturn{}}}}
	info, err := ExtractUseDefInfo(fn)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Uses) != 0 {
		t.Fatalf("void return uses = %#v, want none", info.Uses)
	}
}

func localsOfDefinitions(definitions []DefinitionID) []string {
	locals := make([]string, len(definitions))
	for i, def := range definitions {
		locals[i] = def.Local
	}
	return locals
}
