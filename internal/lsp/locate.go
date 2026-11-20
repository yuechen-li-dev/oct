package lsp

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/judgment"
	"github.com/yuechen-li-dev/oct/internal/lex"
)

// Type-check errors name an enclosing declaration but carry no source
// position. Choosing which token inside that declaration to highlight is a
// bounded, multi-signal decision (AGENTS.md Judgment Rule): the message may
// name a binding, an undefined name, an unknown type and a callee at once,
// and no single signal should always dominate. The choice is therefore made by
// internal/judgment, and its trace travels with the diagnostic.

// locatorCandidate is one possible highlight target.
type locatorCandidate struct {
	name        string
	token       lex.Token
	found       bool
	why         string  // ineligibility reason when !found
	specificity float64 // how directly the token is the subject of the error
	anchor      float64 // how early the message names this evidence
	unique      float64 // 1 when the named token occurs once in the body
}

// LocatorTrace is attached to diagnostics as `data.locator` so the choice can
// be inspected from the client (e.g. "Developer: Inspect" in VS Code).
type LocatorTrace struct {
	Winner     string             `json:"winner"`
	TieBreak   string             `json:"tieBreak,omitempty"`
	Candidates []LocatorCandidate `json:"candidates"`
}

type LocatorCandidate struct {
	Name     string  `json:"name"`
	Eligible bool    `json:"eligible"`
	Why      string  `json:"why,omitempty"`
	Score    float64 `json:"score"`
}

const (
	weightSpecificity = 0.5
	weightAnchor      = 0.35
	weightUnique      = 0.15
)

// chooseTarget picks the token to highlight within a declaration body.
// rest is the message after the `function NAME: ` container prefix.
func chooseTarget(tokens []lex.Token, bodyStart, bodyEnd int, nameTok lex.Token, rest string) (lex.Token, LocatorTrace) {
	body := tokens[bodyStart:bodyEnd]

	occurrences := func(name string, want func(i int) bool) (lex.Token, int) {
		var first lex.Token
		count := 0
		for i, t := range body {
			if t.Kind == lex.Identifier && t.Lexeme == name && (want == nil || want(i)) {
				if count == 0 {
					first = t
				}
				count++
			}
		}
		return first, count
	}
	// anchorAt scores evidence found at byte index pos of the message: the
	// leading clause is the error's subject; later mentions are context.
	anchorAt := func(pos int) float64 {
		if pos < 0 || len(rest) == 0 {
			return 0
		}
		return 1 - float64(pos)/float64(len(rest))
	}
	build := func(name string, evidence string, pos int, specificity float64, tok lex.Token, count int, noun string) locatorCandidate {
		c := locatorCandidate{name: name, specificity: specificity, anchor: anchorAt(pos)}
		switch {
		case evidence == "":
			c.why = "message does not name a " + noun
		case count == 0:
			c.why = fmt.Sprintf("%s %q not found in declaration body", noun, evidence)
		default:
			c.found, c.token = true, tok
			if count == 1 {
				c.unique = 1
			} else {
				c.unique = 0.5
			}
		}
		return c
	}

	var cands []locatorCandidate

	// 1. A binding: `let z: cannot add m and s`.
	{
		var ev string
		pos := -1
		if m := reLetVar.FindStringSubmatchIndex(rest); m != nil {
			ev, pos = rest[m[2]:m[3]], m[0]
		}
		tok, n := occurrences(ev, func(i int) bool {
			return i > 0 && (body[i-1].Kind == lex.KeywordLet || body[i-1].Kind == lex.KeywordVar)
		})
		cands = append(cands, build("binding", ev, pos, 1.0, tok, n, "binding"))
	}
	// 2. An undefined name: `undefined variable: undefinedThing`.
	{
		var ev string
		pos := -1
		if m := reUndef.FindStringSubmatchIndex(rest); m != nil {
			ev, pos = lastSegment(rest[m[2]:m[3]]), m[0]
		}
		tok, n := occurrences(ev, nil)
		cands = append(cands, build("undefinedName", ev, pos, 1.0, tok, n, "undefined name"))
	}
	// 3. An unknown type: `unknown type 'Meters'`.
	{
		var ev string
		pos := -1
		if m := reTypeName.FindStringSubmatchIndex(rest); m != nil {
			ev, pos = lastSegment(rest[m[2]:m[3]]), m[0]
		}
		tok, n := occurrences(ev, nil)
		cands = append(cands, build("unknownType", ev, pos, 0.9, tok, n, "type"))
	}
	// 4. A quoted callee with a call site: `function 'Sqrt' expects ...`.
	{
		var ev string
		pos := -1
		var tok lex.Token
		n := 0
		for _, m := range reQuoted.FindAllStringSubmatchIndex(rest, -1) {
			name := lastSegment(rest[m[2]:m[3]])
			t, count := occurrences(name, func(i int) bool { return i+1 < len(body) && body[i+1].Kind == lex.LeftParen })
			if count > 0 {
				ev, pos, tok, n = name, m[0], t, count
				break
			}
			if ev == "" {
				ev, pos = name, m[0]
			}
		}
		cands = append(cands, build("callee", ev, pos, 0.7, tok, n, "call site"))
	}
	// 5. The declaration itself: always available, least specific.
	cands = append(cands, locatorCandidate{name: "declaration", token: nameTok, found: true, specificity: 0, anchor: 0, unique: 1})

	j := judgment.Judgment{Name: "lsp.diagnostic.target"}
	for i, c := range cands {
		j.Candidates = append(j.Candidates, judgment.Candidate{
			Name:     c.name,
			Priority: len(cands) - i, // declaration order as the final tie-break
			Eligible: c.found,
			Reason:   c.why,
		})
	}
	byName := map[string]locatorCandidate{}
	for _, c := range cands {
		byName[c.name] = c
	}
	j.Considerations = []judgment.Consideration{
		{Name: "specificity", Weight: weightSpecificity, Score: func(c judgment.Candidate) float64 { return byName[c.Name].specificity }},
		{Name: "messageAnchor", Weight: weightAnchor, Score: func(c judgment.Candidate) float64 { return byName[c.Name].anchor }},
		{Name: "uniqueness", Weight: weightUnique, Score: func(c judgment.Candidate) float64 { return byName[c.Name].unique }},
	}
	result, err := j.Decide()
	trace := LocatorTrace{Winner: "declaration"}
	if err != nil {
		return nameTok, trace
	}
	trace.Winner, trace.TieBreak = result.Winner, result.TieBreak
	for _, t := range result.Traces {
		trace.Candidates = append(trace.Candidates, LocatorCandidate{Name: t.Name, Eligible: t.Eligible, Why: t.IneligibleWhy, Score: t.TotalScore})
	}
	return byName[result.Winner].token, trace
}
