package layoutcontract

import "testing"

func TestInvariantAndMetadataHaveDistinctRepresentations(t *testing.T) {
	subject := DataSubjectRef{Kind: CompiledDataRoot, Identity: "Rows"}
	contract := Contract{
		Subject:    subject,
		Invariants: Invariants{ExactExtent: &ExactExtent{Value: 4}},
		Metadata:   Metadata{ColumnProjections: []ColumnProjectionEligibility{{Field: FieldRef{Subject: subject, Ordinal: 1, Name: "Status"}}}},
	}
	if contract.Invariants.ExactExtent.Value != 4 {
		t.Fatal("exact extent invariant was lost")
	}
	if got := contract.Metadata.ColumnProjections[0].Field.Name; got != "Status" {
		t.Fatalf("projection metadata field = %q", got)
	}
}

func TestEnrichRejectsCrossSubjectFact(t *testing.T) {
	subject := DataSubjectRef{Kind: CompiledDataRoot, Identity: "Catalog"}
	other := DataSubjectRef{Kind: CompiledDataRoot, Identity: "Other"}
	contract := Contract{Subject: subject}
	Enrich(&contract, []StaticFact{{
		Subject: other, Kind: Unique,
		Fields:     []FieldRef{{Subject: other, Ordinal: 0, Name: "ID"}},
		Provenance: StaticFactProvenance{Phase: ArtifactEvaluation, Source: StaticAssertProof},
	}})
	if len(contract.Invariants.UniqueFields) != 0 {
		t.Fatal("cross-subject fact was promoted")
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
