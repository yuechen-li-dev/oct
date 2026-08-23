package interpret

import (
	"fmt"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/layoutcontract"
)

func TestStaticProofRequiresCompleteTypedCoverage(t *testing.T) {
	state := newStaticProofState()
	subject := state.newSubject("Rows")
	field := layoutcontract.FieldRef{Subject: subject, Ordinal: 0, Name: "ID"}
	previous := state.begin("fixture:1:1")
	state.observe("==", staticIntField(1, field, 0, 3), staticIntField(2, field, 1, 3), Value{Kind: ValueBool, Bool: false})
	state.observe("==", staticIntField(1, field, 0, 3), staticIntField(3, field, 2, 3), Value{Kind: ValueBool, Bool: false})
	state.finish(previous, true)
	if facts := state.facts.ForSubject(subject); len(facts) != 0 {
		t.Fatalf("incomplete all-pairs coverage emitted facts: %#v", facts)
	}
}

func TestStaticProofDoesNotCrossSubjectsWithSameFieldName(t *testing.T) {
	state := newStaticProofState()
	leftSubject := state.newSubject("Left")
	rightSubject := state.newSubject("Right")
	leftField := layoutcontract.FieldRef{Subject: leftSubject, Ordinal: 0, Name: "ID"}
	rightField := layoutcontract.FieldRef{Subject: rightSubject, Ordinal: 0, Name: "ID"}
	previous := state.begin("fixture:1:1")
	state.observe("==", staticIntField(1, leftField, 0, 2), staticIntField(2, rightField, 1, 2), Value{Kind: ValueBool, Bool: false})
	state.finish(previous, true)
	if len(state.facts.Facts) != 0 {
		t.Fatalf("cross-subject comparison emitted facts: %#v", state.facts.Facts)
	}
}

func BenchmarkStaticUniqueAndSortedProofCoverage(b *testing.B) {
	for _, rows := range []int{6, 1_000} {
		b.Run(fmt.Sprintf("rows=%d", rows), func(b *testing.B) {
			for iteration := 0; iteration < b.N; iteration++ {
				state := newStaticProofState()
				subject := state.newSubject("Rows")
				field := layoutcontract.FieldRef{Subject: subject, Ordinal: 0, Name: "ID"}
				previous := state.begin("benchmark:1:1")
				for left := 0; left < rows; left++ {
					for right := left + 1; right < rows; right++ {
						state.observe("==", staticIntField(int64(left), field, left, rows), staticIntField(int64(right), field, right, rows), Value{Kind: ValueBool, Bool: false})
					}
				}
				for right := 1; right < rows; right++ {
					state.observe(">", staticIntField(int64(right-1), field, right-1, rows), staticIntField(int64(right), field, right, rows), Value{Kind: ValueBool, Bool: false})
				}
				state.finish(previous, true)
				if len(state.facts.Facts) != 2 {
					b.Fatal("proof facts missing")
				}
			}
		})
	}
}

func staticIntField(value int64, field layoutcontract.FieldRef, row, extent int) Value {
	origin := &staticFieldOrigin{Field: field, Row: row, Extent: extent}
	return Value{Kind: ValueInt, Int: value, StaticField: origin}
}
