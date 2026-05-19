package judgment

import (
	"reflect"
	"testing"
)

func TestHighestScoreWins(t *testing.T) {
	j := Judgment{
		Name: "highest",
		Candidates: []Candidate{{Name: "A", Eligible: true}, {Name: "B", Eligible: true}},
		Considerations: []Consideration{{Name: "fit", Weight: 1.0, Score: func(c Candidate) float64 {
			if c.Name == "A" {
				return 0.3
			}
			return 0.7
		}}},
	}
	result, err := j.Decide()
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if result.Winner != "B" {
		t.Fatalf("winner=%q want B", result.Winner)
	}
}

func TestIneligibleCannotWin(t *testing.T) {
	j := Judgment{
		Name: "ineligible",
		Candidates: []Candidate{
			{Name: "A", Eligible: false, Reason: "comment preservation risk"},
			{Name: "B", Eligible: true},
		},
		Considerations: []Consideration{{Name: "score", Weight: 1.0, Score: func(c Candidate) float64 {
			if c.Name == "A" {
				return 10
			}
			return 1
		}}},
	}
	result, err := j.Decide()
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if result.Winner != "B" {
		t.Fatalf("winner=%q want B", result.Winner)
	}
	if got := result.Traces[0].IneligibleWhy; got != "comment preservation risk" {
		t.Fatalf("ineligible reason=%q", got)
	}
}

func TestNoEligibleCandidatesErrors(t *testing.T) {
	j := Judgment{Name: "none", Candidates: []Candidate{{Name: "A", Eligible: false}}}
	_, err := j.Decide()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTiebreakByPriority(t *testing.T) {
	j := Judgment{
		Name: "priority",
		Candidates: []Candidate{
			{Name: "A", Eligible: true, Priority: 1},
			{Name: "B", Eligible: true, Priority: 5},
		},
		Considerations: []Consideration{{Name: "equal", Weight: 1, Score: func(c Candidate) float64 { return 1 }}},
	}
	result, err := j.Decide()
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if result.Winner != "B" || result.TieBreak != "priority" {
		t.Fatalf("winner=%q tie=%q", result.Winner, result.TieBreak)
	}
}

func TestTiebreakByDeclarationOrder(t *testing.T) {
	j := Judgment{
		Name: "order",
		Candidates: []Candidate{{Name: "A", Eligible: true}, {Name: "B", Eligible: true}},
		Considerations: []Consideration{{Name: "equal", Weight: 1, Score: func(c Candidate) float64 { return 0.2 }}},
	}
	result, err := j.Decide()
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if result.Winner != "A" || result.TieBreak != "declaration order" {
		t.Fatalf("winner=%q tie=%q", result.Winner, result.TieBreak)
	}
}

func TestTraceContributions(t *testing.T) {
	j := Judgment{
		Name: "trace",
		Candidates: []Candidate{{Name: "A", Eligible: true}},
		Considerations: []Consideration{
			{Name: "c1", Weight: 2, Score: func(c Candidate) float64 { return 0.5 }},
			{Name: "c2", Weight: -1, Score: func(c Candidate) float64 { return 0.25 }},
		},
	}
	result, err := j.Decide()
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	trace := result.Traces[0]
	if len(trace.Contributions) != 2 {
		t.Fatalf("contrib count=%d", len(trace.Contributions))
	}
	if trace.Contributions[0].Name != "c1" || trace.Contributions[0].Contribution != 1.0 {
		t.Fatalf("unexpected first contribution: %+v", trace.Contributions[0])
	}
	if trace.Contributions[1].Name != "c2" || trace.Contributions[1].Contribution != -0.25 {
		t.Fatalf("unexpected second contribution: %+v", trace.Contributions[1])
	}
	if trace.TotalScore != 0.75 {
		t.Fatalf("total=%v", trace.TotalScore)
	}
}

func TestDeterministicRepeatability(t *testing.T) {
	j := Judgment{
		Name: "repeat",
		Candidates: []Candidate{{Name: "A", Eligible: true}, {Name: "B", Eligible: true, Priority: 1}},
		Considerations: []Consideration{{Name: "s", Weight: 1, Score: func(c Candidate) float64 {
			if c.Name == "A" {
				return 0.6
			}
			return 0.6
		}}},
	}
	first, err := j.Decide()
	if err != nil {
		t.Fatalf("first Decide error: %v", err)
	}
	for i := 0; i < 10; i++ {
		next, err := j.Decide()
		if err != nil {
			t.Fatalf("run %d error: %v", i, err)
		}
		if first.Winner != next.Winner || !reflect.DeepEqual(first.Traces, next.Traces) {
			t.Fatalf("nondeterministic result on run %d", i)
		}
	}
}

func TestNegativePenaltyConsideration(t *testing.T) {
	j := Judgment{
		Name: "penalty",
		Candidates: []Candidate{{Name: "inline", Eligible: true}, {Name: "multiline", Eligible: true}},
		Considerations: []Consideration{
			{Name: "width", Weight: 1, Score: func(c Candidate) float64 {
				if c.Name == "inline" {
					return 0.8
				}
				return 0.4
			}},
			{Name: "commentRisk", Weight: 1, Score: func(c Candidate) float64 {
				if c.Name == "inline" {
					return -1
				}
				return 0
			}},
		},
	}
	result, err := j.Decide()
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if result.Winner != "multiline" {
		t.Fatalf("winner=%q want multiline", result.Winner)
	}
}

func TestFormatterShapedExample(t *testing.T) {
	t.Run("multiline wins under width and callee pressure", func(t *testing.T) {
		j := Judgment{
			Name: "fmt-pressure",
			Candidates: []Candidate{{Name: "inline", Eligible: true}, {Name: "multiline", Eligible: true}, {Name: "leaveUnchanged", Eligible: true}},
			Considerations: []Consideration{
				{Name: "lineWidth", Weight: 1.2, Score: func(c Candidate) float64 {
					switch c.Name {
					case "inline":
						return -1.0 // renderedWidth=160 exceeds maxWidth=100
					case "multiline":
						return 0.8
					default:
						return 0.0
					}
				}},
				{Name: "nestingDepth", Weight: 0.5, Score: func(c Candidate) float64 {
					if c.Name == "multiline" {
						return 0.6 // nestingDepth=4 favors expansion
					}
					return 0
				}},
				{Name: "calleePreference", Weight: 0.6, Score: func(c Candidate) float64 {
					if c.Name == "multiline" {
						return 0.7 // callee=Markdown.Report
					}
					return 0
				}},
			},
		}
		result, err := j.Decide()
		if err != nil {
			t.Fatalf("Decide error: %v", err)
		}
		if result.Winner != "multiline" {
			t.Fatalf("winner=%q want multiline", result.Winner)
		}
	})

	t.Run("leaveUnchanged wins when rewrite is risky", func(t *testing.T) {
		j := Judgment{
			Name: "fmt-risk",
			Candidates: []Candidate{
				{Name: "inline", Eligible: false, Reason: "comment risk prevents safe rewrite"},
				{Name: "multiline", Eligible: false, Reason: "comment risk prevents safe rewrite"},
				{Name: "leaveUnchanged", Eligible: true, Priority: 10},
			},
			Considerations: []Consideration{{Name: "stability", Weight: 1, Score: func(c Candidate) float64 {
				if c.Name == "leaveUnchanged" {
					return 1
				}
				return 0
			}}},
		}
		result, err := j.Decide()
		if err != nil {
			t.Fatalf("Decide error: %v", err)
		}
		if result.Winner != "leaveUnchanged" {
			t.Fatalf("winner=%q want leaveUnchanged", result.Winner)
		}
	})
}
