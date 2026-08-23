package layoutcontract

import "testing"

func TestInvariantAndMetadataHaveDistinctRepresentations(t *testing.T) {
	contract := Contract{
		Subject:    DataSubjectRef{Kind: CompiledDataRoot, Identity: "Rows"},
		Invariants: Invariants{ExactExtent: &ExactExtent{Value: 4}},
		Metadata:   Metadata{ColumnProjections: []ColumnProjectionEligibility{{Field: "Status"}}},
	}
	if contract.Invariants.ExactExtent.Value != 4 {
		t.Fatal("exact extent invariant was lost")
	}
	if got := contract.Metadata.ColumnProjections[0].Field; got != "Status" {
		t.Fatalf("projection metadata field = %q", got)
	}
}

func TestFormatIsStable(t *testing.T) {
	c := Contract{Subject: DataSubjectRef{Kind: StaticArray, Identity: "PublishedIDs"}, Invariants: Invariants{
		ExactExtent: &ExactExtent{Value: 4}, LogicalOrder: &LogicalOrder{}, Immutable: &Immutable{},
	}}
	want := "subject=static-array:PublishedIDs extent=exact:4 order=logical publication=immutable"
	if got := Format(c); got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}
