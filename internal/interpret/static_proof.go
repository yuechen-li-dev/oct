package interpret

import (
	"fmt"
	"sort"

	"github.com/yuechen-li-dev/oct/internal/layoutcontract"
)

// staticProofState is artifact-evaluation-only evidence state. It observes
// typed field comparisons while a StaticAssert.True condition executes and
// retains only complete, recognized coverage proofs.
type staticProofState struct {
	nextSubject int
	active      *staticProofTrace
	facts       layoutcontract.StaticFactSet
}

type staticFieldOrigin struct {
	Field  layoutcontract.FieldRef
	Row    int
	Extent int
}

type staticProofTrace struct {
	identity string
	coverage map[staticCoverageKey]map[staticRowPair]struct{}
}

type staticCoverageKey struct {
	subject layoutcontract.DataSubjectRef
	ordinal int
	field   string
	kind    layoutcontract.StaticFactKind
	extent  int
}

type staticRowPair struct{ left, right int }

func newStaticProofState() *staticProofState {
	return &staticProofState{}
}

func (s *staticProofState) newSubject(typeName string) layoutcontract.DataSubjectRef {
	if s == nil {
		return layoutcontract.DataSubjectRef{}
	}
	s.nextSubject++
	return layoutcontract.DataSubjectRef{Kind: layoutcontract.StaticEvalValue, Identity: fmt.Sprintf("%s#%d", typeName, s.nextSubject)}
}

func (s *staticProofState) begin(identity string) *staticProofTrace {
	if s == nil {
		return nil
	}
	previous := s.active
	s.active = &staticProofTrace{identity: identity, coverage: make(map[staticCoverageKey]map[staticRowPair]struct{})}
	return previous
}

func (s *staticProofState) finish(previous *staticProofTrace, proved bool) {
	if s == nil || s.active == nil {
		return
	}
	trace := s.active
	s.active = previous
	if !proved {
		return
	}
	keys := make([]staticCoverageKey, 0, len(trace.coverage))
	for key := range trace.coverage {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].subject.Identity != keys[j].subject.Identity {
			return keys[i].subject.Identity < keys[j].subject.Identity
		}
		if keys[i].ordinal != keys[j].ordinal {
			return keys[i].ordinal < keys[j].ordinal
		}
		return keys[i].kind < keys[j].kind
	})
	for _, key := range keys {
		if !completeStaticCoverage(key, trace.coverage[key]) {
			continue
		}
		field := layoutcontract.FieldRef{Subject: key.subject, Ordinal: key.ordinal, Name: key.field}
		s.facts.Add(layoutcontract.StaticFact{
			Subject: key.subject, Kind: key.kind, Fields: []layoutcontract.FieldRef{field},
			Provenance: layoutcontract.StaticFactProvenance{
				Phase: layoutcontract.ArtifactEvaluation, Source: layoutcontract.StaticAssertProof,
				Identity: trace.identity + "/field-comparison-coverage",
			},
		})
	}
}

func completeStaticCoverage(key staticCoverageKey, pairs map[staticRowPair]struct{}) bool {
	if key.extent < 2 {
		return false
	}
	switch key.kind {
	case layoutcontract.Unique:
		if len(pairs) != key.extent*(key.extent-1)/2 {
			return false
		}
		for left := 0; left < key.extent; left++ {
			for right := left + 1; right < key.extent; right++ {
				if _, ok := pairs[staticRowPair{left: left, right: right}]; !ok {
					return false
				}
			}
		}
		return true
	case layoutcontract.SortedAscending:
		if len(pairs) != key.extent-1 {
			return false
		}
		for right := 1; right < key.extent; right++ {
			if _, ok := pairs[staticRowPair{left: right - 1, right: right}]; !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (s *staticProofState) observe(operator string, left, right, result Value) {
	if s == nil || s.active == nil || result.Kind != ValueBool || left.Kind != ValueInt || right.Kind != ValueInt || left.StaticField == nil || right.StaticField == nil {
		return
	}
	l, r := *left.StaticField, *right.StaticField
	if l.Field != r.Field || l.Extent != r.Extent || l.Row == r.Row {
		return
	}
	kind := layoutcontract.StaticFactKind("")
	pair := staticRowPair{left: l.Row, right: r.Row}
	switch {
	case (operator == "==" && !result.Bool) || (operator == "!=" && result.Bool):
		kind = layoutcontract.Unique
		if pair.left > pair.right {
			pair.left, pair.right = pair.right, pair.left
		}
	case operator == ">" && !result.Bool && pair.left+1 == pair.right:
		kind = layoutcontract.SortedAscending
	default:
		return
	}
	key := staticCoverageKey{subject: l.Field.Subject, ordinal: l.Field.Ordinal, field: l.Field.Name, kind: kind, extent: l.Extent}
	if s.active.coverage[key] == nil {
		s.active.coverage[key] = make(map[staticRowPair]struct{})
	}
	s.active.coverage[key][pair] = struct{}{}
}
