// Package judgment provides a small deterministic utility-scoring engine.
//
// A Judgment evaluates a fixed set of candidates against weighted
// considerations, filters ineligible candidates, and deterministically picks a
// winner. The resulting trace is intentionally inspectable for auditing and
// debugging (for example formatter layout choices in ocfmt readable mode).
//
// This package is Go-layer infrastructure only. It does not introduce Oct
// syntax. It is intended to be a reusable mechanism that can later align with
// conceptual lowering for future "when utility" semantics.
package judgment

import (
	"errors"
	"fmt"
	"math"
)

// Epsilon is the tolerance used when comparing floating-point totals and
// determining ties.
const Epsilon = 1e-9

// Candidate represents a selectable option.
type Candidate struct {
	Name     string
	Priority int
	Eligible bool
	Reason   string
}

// Consideration represents one weighted scoring component.
//
// Raw scores are expected to use a caller-chosen convention (commonly [-1,1]
// or [0,1]). Final contribution is computed as Weight * Raw.
type Consideration struct {
	Name   string
	Weight float64
	Score  func(candidate Candidate) float64
}

// Contribution captures a single consideration's per-candidate scoring result.
type Contribution struct {
	Name         string
	Weight       float64
	Raw          float64
	Contribution float64
}

// CandidateTrace captures how a candidate was evaluated.
type CandidateTrace struct {
	Index         int
	Name          string
	Priority      int
	Eligible      bool
	IneligibleWhy string
	Contributions []Contribution
	TotalScore    float64
}

// Result captures the decision outcome and full trace.
type Result struct {
	Winner      string
	WinnerIndex int
	Traces      []CandidateTrace
	TieBreak    string
}

// Judgment is an immutable input bundle for one decision.
type Judgment struct {
	Name           string
	Candidates     []Candidate
	Considerations []Consideration
}

// Decide evaluates all candidates and returns a deterministic winner.
func (j Judgment) Decide() (Result, error) {
	if len(j.Candidates) == 0 {
		return Result{}, errors.New("judgment has no candidates")
	}

	result := Result{WinnerIndex: -1, Traces: make([]CandidateTrace, 0, len(j.Candidates))}

	for idx, c := range j.Candidates {
		trace := CandidateTrace{
			Index:         idx,
			Name:          c.Name,
			Priority:      c.Priority,
			Eligible:      c.Eligible,
			IneligibleWhy: c.Reason,
		}

		if c.Eligible {
			trace.Contributions = make([]Contribution, 0, len(j.Considerations))
			for _, cons := range j.Considerations {
				raw := cons.Score(c)
				contrib := cons.Weight * raw
				trace.Contributions = append(trace.Contributions, Contribution{
					Name:         cons.Name,
					Weight:       cons.Weight,
					Raw:          raw,
					Contribution: contrib,
				})
				trace.TotalScore += contrib
			}
		}

		result.Traces = append(result.Traces, trace)
	}

	for i := range result.Traces {
		trace := result.Traces[i]
		if !trace.Eligible {
			continue
		}

		if result.WinnerIndex < 0 {
			result.WinnerIndex = i
			result.Winner = trace.Name
			continue
		}

		win := result.Traces[result.WinnerIndex]
		delta := trace.TotalScore - win.TotalScore
		if delta > Epsilon {
			result.WinnerIndex = i
			result.Winner = trace.Name
			result.TieBreak = "higher score"
			continue
		}
		if math.Abs(delta) <= Epsilon {
			if trace.Priority > win.Priority {
				result.WinnerIndex = i
				result.Winner = trace.Name
				result.TieBreak = "priority"
				continue
			}
			// If same score and same priority, declaration order wins, so keep
			// current winner (lower index).
			if trace.Priority == win.Priority {
				result.TieBreak = "declaration order"
			}
		}
	}

	if result.WinnerIndex < 0 {
		return Result{}, fmt.Errorf("judgment %q has no eligible candidates", j.Name)
	}

	if result.TieBreak == "" {
		result.TieBreak = "none"
	}

	return result, nil
}
