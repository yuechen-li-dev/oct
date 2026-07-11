package test

import (
	"fmt"
	"sort"

	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
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
type Suite struct {
	Source string
	Cases  []CanonicalCase
	Groups []CompilationGroup
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
