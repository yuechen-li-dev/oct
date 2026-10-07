package jsoninfer

import (
	"fmt"
	"strconv"
	"strings"
)

// Loads reports whether the declarations load the document: nothing in it
// was refused.
func (r Result) Loads() bool { return len(r.Refusals) == 0 }

// Text is the output of `oct json infer`: the declarations, then the call
// that loads the document, or the values that have no declaration. Every
// line is Oct source or a comment, so the whole of it can be pasted.
func (r Result) Text() string {
	var out strings.Builder
	for _, d := range r.declarations {
		keyword := "concept"
		if d.table {
			keyword = "record table"
		}
		fmt.Fprintf(&out, "%s %s {\n", keyword, d.name)
		for _, f := range d.fields {
			switch {
			case f.expr == "":
				fmt.Fprintf(&out, "    // %s: no declaration; see below\n", f.name)
			case f.note != "":
				fmt.Fprintf(&out, "    %s: %s // %s\n", f.name, f.expr, f.note)
			default:
				fmt.Fprintf(&out, "    %s: %s\n", f.name, f.expr)
			}
		}
		out.WriteString("}\n\n")
	}
	if r.Loads() {
		if r.note != "" {
			fmt.Fprintf(&out, "// %s\n", r.note)
		}
		fmt.Fprintf(&out, "// Json.Load<%s>(%s)?\n", r.Type, strconv.Quote(r.source))
		return out.String()
	}
	fmt.Fprintf(&out, "// No declaration, so these do not load %s:\n", r.source)
	for _, refusal := range r.Refusals {
		fmt.Fprintf(&out, "//   %s (line %d, column %d): %s\n", refusal.Path, refusal.Line, refusal.Column, refusal.Reason)
	}
	return out.String()
}

// Explain is the trace of every choice made, as comments: each candidate
// with what each consideration gave it, or why it could not be chosen.
func (r Result) Explain() string {
	var out strings.Builder
	out.WriteString("//\n// Choices:\n")
	if len(r.Decisions) == 0 {
		out.WriteString("//   none: nothing in this document reads two ways\n")
	}
	for _, decision := range r.Decisions {
		fmt.Fprintf(&out, "//   %s: %s\n", decision.Path, decision.Trace.Winner)
		for _, candidate := range decision.Trace.Traces {
			if !candidate.Eligible {
				fmt.Fprintf(&out, "//     %s: cannot be: %s\n", candidate.Name, candidate.IneligibleWhy)
				continue
			}
			parts := make([]string, 0, len(candidate.Contributions))
			for _, c := range candidate.Contributions {
				if c.Raw == 0 {
					continue
				}
				parts = append(parts, fmt.Sprintf("%s %.2f x %.2f", c.Name, c.Weight, c.Raw))
			}
			if len(parts) == 0 {
				parts = []string{"nothing counts for it"}
			}
			fmt.Fprintf(&out, "//     %s: %.2f = %s\n", candidate.Name, candidate.TotalScore, strings.Join(parts, " + "))
		}
	}
	return out.String()
}
