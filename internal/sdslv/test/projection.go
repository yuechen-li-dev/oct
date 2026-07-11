package test

import (
	"fmt"
	"sort"

	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
)

// CanonicalCase is the compiler-to-runtime projection. It deliberately keeps
// the validated declaration available to the compiler; Manifest is not input
// to grouping or bootstrap code generation.
type CanonicalCase struct{ Test validate.ValidatedTestCase }
type CompilationGroup struct {
	ID            string
	WorkgroupSize [3]uint32
	Cases         []CanonicalCase
}

// BuildTestProgram is the compiler projection used by backends. It is not a
// manifest projection and carries no host/CLI presentation data.
func BuildTestProgram(suite Suite) vdmir.TestProgram {
	p := vdmir.TestProgram{Module: suite.MIR, ABI: vdmir.TestResultContract{ABIVersion: ResultABIVersion, LinearIndex: vdmir.TestInvocationLinearIndex{UsesXYZ: true}}}
	for _, group := range suite.Groups {
		g := vdmir.TestCompilationGroup{ID: group.ID, WorkgroupSize: group.WorkgroupSize}
		for i, c := range group.Cases {
			e := vdmir.TestEntry{Selector: uint32(i), FunctionName: c.Test.Decl.Function.Name, DispatchGroups: c.Test.Decl.Launch.DispatchGroups}
			if c.Test.Row != nil {
				row := &vdmir.TestTheoryRow{Index: uint32(c.Test.Row.Index)}
				for _, value := range c.Test.Row.Values {
					row.Values = append(row.Values, testLiteral(value))
				}
				e.TheoryRow = row
			}
			g.Entries = append(g.Entries, e)
		}
		p.Groups = append(p.Groups, g)
	}
	return p
}

func testLiteral(value validate.ConstValue) vdmir.LiteralExpr {
	t := vdmir.Type{Kind: vdmir.TypeI32, Name: "i32"}
	kind := vdmir.LiteralInteger
	switch value.Kind {
	case "bool":
		t = vdmir.Type{Kind: vdmir.TypeBool, Name: "bool"}
		kind = vdmir.LiteralBool
	case "float":
		t = vdmir.Type{Kind: vdmir.TypeF32, Name: "f32"}
		kind = vdmir.LiteralFloat
	case "integer":
		if len(value.Text) > 0 && (value.Text[len(value.Text)-1] == 'u' || value.Text[len(value.Text)-1] == 'U') {
			t = vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}
		}
	}
	return vdmir.LiteralExpr{ExprType: t, Kind: kind, Value: value.Text}
}

type Suite struct {
	Source string
	Cases  []CanonicalCase
	Groups []CompilationGroup
	// MIR is the normal SDSL-V lowering result.  The test backend only owns the
	// compiler-generated dispatcher, assertion state, and result epilogue.
	MIR vdmir.Module
}

// GroupValidatedCases is the sole deterministic grouping projection. It uses
// only canonical case/declaration fields and never host manifest DTOs.
func GroupValidatedCases(cases []CanonicalCase) []CompilationGroup {
	bySize := map[[3]uint32][]CanonicalCase{}
	for _, c := range cases {
		key := c.Test.Decl.Launch.WorkgroupSize
		bySize[key] = append(bySize[key], c)
	}
	keys := make([][3]uint32, 0, len(bySize))
	for k := range bySize {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	groups := make([]CompilationGroup, 0, len(keys))
	for i, k := range keys {
		members := bySize[k]
		sort.SliceStable(members, func(a, b int) bool { return members[a].Test.StableID < members[b].Test.StableID })
		groups = append(groups, CompilationGroup{ID: fmt.Sprintf("group-%d", i), WorkgroupSize: k, Cases: members})
	}
	return groups
}
