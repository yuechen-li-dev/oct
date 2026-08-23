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
	MIRRecordTable   SubjectKind = "mir-record-table"
	MIRTableRow      SubjectKind = "mir-table-row"
	CompiledDataRoot SubjectKind = "compiled-data-root"
	StaticArray      SubjectKind = "static-array"
)

// DataSubjectRef names an existing compiler/type identity; it is not a copy of
// that subject's type graph.
type DataSubjectRef struct {
	Kind     SubjectKind
	Identity string
}

type ProofPhase string

const (
	MIRLowering         ProofPhase = "mir-lowering"
	PublicationMaterial ProofPhase = "publication-materialization"
)

type Provenance struct {
	Phase  ProofPhase
	Source string
}

// Invariant facts may be relied on for correctness.
type Invariants struct {
	NominalIdentity *NominalIdentity
	ExactExtent     *ExactExtent
	LogicalOrder    *LogicalOrder
	Immutable       *Immutable
}

type NominalIdentity struct{ Provenance Provenance }
type ExactExtent struct {
	Value      int
	Provenance Provenance
}
type LogicalOrder struct{ Provenance Provenance }
type Immutable struct{ Provenance Provenance }

// Metadata facts are optional opportunities. Backends may ignore them and
// must never treat them as proofs of uniqueness, bounds, or identity.
type Metadata struct {
	ColumnProjections []ColumnProjectionEligibility
}

type ColumnProjectionEligibility struct {
	Field      string
	Provenance Provenance
}

type Contract struct {
	Subject    DataSubjectRef
	Invariants Invariants
	Metadata   Metadata
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
		facts = append(facts, "metadata=column-projection:"+projection.Field)
	}
	return fmt.Sprintf("subject=%s:%s %s", contract.Subject.Kind, contract.Subject.Identity, strings.Join(facts, " "))
}
