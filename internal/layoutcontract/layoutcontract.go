// Package layoutcontract carries compiler-internal semantic facts about typed
// data subjects. It deliberately describes meaning and optimizer eligibility,
// not a Go or machine representation.
package layoutcontract

import (
	"fmt"
	"strings"
)

type SubjectKind string

const (
	NominalRecord    SubjectKind = "nominal-record"
	MIRRecordTable   SubjectKind = "mir-record-table"
	MIRTableRow      SubjectKind = "mir-table-row"
	StaticEvalValue  SubjectKind = "static-evaluation-value"
	CompiledDataRoot SubjectKind = "compiled-data-root"
	StaticArray      SubjectKind = "static-array"
)

// DataSubjectRef names an existing compiler/type identity; it is not a copy of
// that subject's type graph.
type DataSubjectRef struct {
	Kind     SubjectKind
	Identity string
}

// FieldRef identifies a field within one exact data subject. Name is retained
// for diagnostics and generated identifiers; Ordinal is the typed schema
// position that prevents same-named fields on different subjects from being
// treated as interchangeable.
type FieldRef struct {
	Subject DataSubjectRef
	Ordinal int
	Name    string
}

type ProofPhase string

const (
	MIRLowering         ProofPhase = "mir-lowering"
	ArtifactEvaluation  ProofPhase = "artifact-static-evaluation"
	PublicationMaterial ProofPhase = "publication-materialization"
)

type Provenance struct {
	Phase  ProofPhase
	Source string
}

type StaticFactKind string

const (
	Unique          StaticFactKind = "unique"
	SortedAscending StaticFactKind = "sorted-ascending"
)

type StaticFactSource string

const (
	StaticAssertProof StaticFactSource = "static-assert"
)

// StaticFactProvenance records why a fact is eligible to become a semantic
// invariant. Runtime checks and profile hints intentionally have no source
// variant in this bounded M2 carrier.
type StaticFactProvenance struct {
	Phase    ProofPhase
	Source   StaticFactSource
	Identity string
}

type StaticFact struct {
	Subject    DataSubjectRef
	Kind       StaticFactKind
	Fields     []FieldRef
	Provenance StaticFactProvenance
}

type StaticFactSet struct {
	Facts []StaticFact
}

func (s *StaticFactSet) Add(fact StaticFact) {
	for _, existing := range s.Facts {
		if sameStaticFact(existing, fact) {
			return
		}
	}
	s.Facts = append(s.Facts, fact)
}

func (s StaticFactSet) ForSubject(subject DataSubjectRef) []StaticFact {
	facts := make([]StaticFact, 0)
	for _, fact := range s.Facts {
		if fact.Subject == subject {
			facts = append(facts, fact)
		}
	}
	return facts
}

func sameStaticFact(left, right StaticFact) bool {
	if left.Subject != right.Subject || left.Kind != right.Kind || left.Provenance != right.Provenance || len(left.Fields) != len(right.Fields) {
		return false
	}
	for index := range left.Fields {
		if left.Fields[index] != right.Fields[index] {
			return false
		}
	}
	return true
}

// Invariant facts may be relied on for correctness.
type Invariants struct {
	NominalIdentity *NominalIdentity
	ExactExtent     *ExactExtent
	LogicalOrder    *LogicalOrder
	Immutable       *Immutable
	UniqueFields    []FieldInvariant
	SortedFields    []FieldInvariant
}

type NominalIdentity struct{ Provenance Provenance }
type ExactExtent struct {
	Value      int
	Provenance Provenance
}
type LogicalOrder struct{ Provenance Provenance }
type Immutable struct{ Provenance Provenance }

type FieldInvariant struct {
	Field      FieldRef
	Provenance StaticFactProvenance
}

// Metadata facts are optional opportunities. Backends may ignore them and
// must never treat them as proofs of uniqueness, bounds, or identity.
type Metadata struct {
	ColumnProjections []ColumnProjectionEligibility
	SearchIndexes     []SearchIndexEligibility
}

type ColumnProjectionEligibility struct {
	Field      FieldRef
	Provenance Provenance
}

// SearchIndexEligibility describes a zero-initialization lookup derived from
// an existing static projection. It does not confer primary-key or entity
// identity semantics on the field.
type SearchIndexEligibility struct {
	Field       FieldRef
	DerivedFrom DataSubjectRef
	ProofBasis  []StaticFactKind
}

type Contract struct {
	Subject    DataSubjectRef
	Invariants Invariants
	Metadata   Metadata
}

// Enrich promotes only the bounded semantic fact vocabulary into invariants.
// Callers must first validate and rebase facts to the contract's exact subject.
func Enrich(contract *Contract, facts []StaticFact) {
	for _, fact := range facts {
		if fact.Subject != contract.Subject || len(fact.Fields) != 1 || fact.Fields[0].Subject != contract.Subject {
			continue
		}
		invariant := FieldInvariant{Field: fact.Fields[0], Provenance: fact.Provenance}
		switch fact.Kind {
		case Unique:
			contract.Invariants.UniqueFields = appendInvariant(contract.Invariants.UniqueFields, invariant)
		case SortedAscending:
			contract.Invariants.SortedFields = appendInvariant(contract.Invariants.SortedFields, invariant)
		}
	}
}

func appendInvariant(existing []FieldInvariant, candidate FieldInvariant) []FieldInvariant {
	for _, item := range existing {
		if item.Field == candidate.Field {
			return existing
		}
	}
	return append(existing, candidate)
}

// Format is a deterministic, test/debug-only inspection surface.
func Format(contract Contract) string {
	var facts []string
	if contract.Invariants.NominalIdentity != nil {
		facts = append(facts, "identity=nominal")
	}
	if extent := contract.Invariants.ExactExtent; extent != nil {
		facts = append(facts, fmt.Sprintf("extent=exact:%d", extent.Value))
	}
	if contract.Invariants.LogicalOrder != nil {
		facts = append(facts, "order=logical")
	}
	if contract.Invariants.Immutable != nil {
		facts = append(facts, "publication=immutable")
	}
	for _, projection := range contract.Metadata.ColumnProjections {
		facts = append(facts, "metadata=column-projection:"+projection.Field.Name)
	}
	for _, unique := range contract.Invariants.UniqueFields {
		facts = append(facts, formatFieldFact("unique", unique))
	}
	for _, sorted := range contract.Invariants.SortedFields {
		facts = append(facts, formatFieldFact("sorted-ascending", sorted))
	}
	for _, index := range contract.Metadata.SearchIndexes {
		facts = append(facts, "metadata=binary-search:"+index.Field.Name)
	}
	return fmt.Sprintf("subject=%s:%s %s", contract.Subject.Kind, contract.Subject.Identity, strings.Join(facts, " "))
}

func formatFieldFact(kind string, invariant FieldInvariant) string {
	return fmt.Sprintf("fact=%s:%s[%d] provenance=%s/%s:%s", kind, invariant.Field.Name, invariant.Field.Ordinal, invariant.Provenance.Phase, invariant.Provenance.Source, invariant.Provenance.Identity)
}
