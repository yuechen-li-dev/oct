package ocfmt

import (
	"math"
	"slices"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/judgment"
)

// This file holds the call-layout judgment for automatic line wrapping.
// Wrapping is not enabled: the formatter never moves a token to another
// line, and nothing in the package calls expandReadableLine. The code is kept
// as the worked design for when wrapping is turned on. It tokenizes with its
// own rough scanner, which predates the lexer-based layout in layout.go and
// must be replaced by the real tokens before it is used.

type callLayoutContext struct {
	callee                                                                  string
	renderedWidth, maxWidth, nestingDepth, argCount                         int
	hasNestedArray, hasNestedRecord, hasNestedCall, hasCommentRisk, isHeavy bool
	mode                                                                    Mode
}

func expandReadableLine(code string, baseIndent int, comment string) ([]string, judgment.Result, error) {
	if code == "" {
		return nil, judgment.Result{}, nil
	}
	tokens := scanTokens(code)
	ctx, ok := detectCallContext(tokens, code, baseIndent, comment)
	if !ok {
		if !shouldExpand(tokens, code) {
			return nil, judgment.Result{}, nil
		}
		lines := formatTokensReadable(tokens, baseIndent)
		if len(lines) <= 1 {
			return nil, judgment.Result{}, nil
		}
		return lines, judgment.Result{}, nil
	}
	j := buildLayoutJudgment(ctx)
	res, err := j.Decide()
	if err != nil {
		return nil, judgment.Result{}, err
	}
	if res.Winner == "leaveUnchanged" {
		return nil, res, nil
	}
	if res.Winner == "inline" {
		return nil, res, nil
	}
	lines := formatTokensReadable(tokens, baseIndent)
	if len(lines) <= 1 {
		return nil, res, nil
	}
	return lines, res, nil
}

func detectCallContext(tokens []string, code string, baseIndent int, comment string) (callLayoutContext, bool) {
	idx := -1
	for i := 1; i < len(tokens); i++ {
		if tokens[i] == "(" && isCallToken(tokens[i-1]) {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return callLayoutContext{}, false
	}
	callee := tokens[idx-1]
	if idx >= 3 && tokens[idx-2] == "." && isCallToken(tokens[idx-3]) {
		callee = tokens[idx-3] + "." + tokens[idx-1]
	}
	if !isTargetCallee(callee) {
		return callLayoutContext{}, false
	}
	nesting := 0
	maxNest := 0
	args := 1
	hasArr := false
	hasRec := false
	hasCall := false
	for i := idx; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok {
		case "(", "[", "{":
			nesting++
			if nesting > maxNest {
				maxNest = nesting
			}
			if tok == "[" {
				hasArr = true
			}
			if tok == "{" {
				hasRec = true
			}
			if tok == "(" && i > idx {
				hasCall = true
			}
		case ")", "]", "}":
			nesting--
		case ",":
			if nesting == 1 {
				args++
			}
		}
	}
	if strings.Contains(code[idx:], "()") {
		args = 0
	}
	width := baseIndent*4 + len(code)
	return callLayoutContext{callee: callee, renderedWidth: width, maxWidth: 100, nestingDepth: maxNest, argCount: args, hasNestedArray: hasArr, hasNestedRecord: hasRec, hasNestedCall: hasCall, hasCommentRisk: comment != "", isHeavy: isHeavyCallee(callee), mode: ModeReadable}, true
}
func isTargetCallee(c string) bool {
	targets := []string{"Markdown.Report", "Markdown.Section", "Markdown.Subsection", "Markdown.Callout", "Markdown.Table", "Markdown.KeyValueTable", "Artifact.WriteMarkdown", "Artifact.WriteOctagon", "Artifact.WriteCsv", "Artifact.WriteJson", "Json.Object", "String.Join", "Markdown.H1"}
	return slices.Contains(targets, c)
}
func isCallToken(tok string) bool {
	if tok == "" {
		return false
	}
	c := tok[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}
func isHeavyCallee(c string) bool { return c != "Markdown.H1" }
func buildLayoutJudgment(ctx callLayoutContext) judgment.Judgment {
	inlineEligible := ctx.renderedWidth <= ctx.maxWidth && !(ctx.nestingDepth > 3 && (ctx.hasNestedArray || ctx.hasNestedRecord || ctx.hasNestedCall))
	multilineEligible := !ctx.hasCommentRisk
	cands := []judgment.Candidate{{Name: "inline", Eligible: inlineEligible, Priority: inlinePriority(ctx), Reason: "width or nesting unsafe"}, {Name: "multiline", Eligible: multilineEligible, Priority: multilinePriority(ctx), Reason: "comment risk"}, {Name: "leaveUnchanged", Eligible: true, Priority: 0}}
	considerations := []judgment.Consideration{
		{Name: "widthFit", Weight: 0.5, Score: func(c judgment.Candidate) float64 {
			if c.Name == "inline" {
				return 1 - math.Max(0, float64(ctx.renderedWidth-ctx.maxWidth))/float64(ctx.maxWidth)
			}
			if c.Name == "multiline" {
				if ctx.renderedWidth > ctx.maxWidth {
					return 1
				}
				return float64(ctx.renderedWidth) / float64(ctx.maxWidth) * 0.4
			}
			return 0.1
		}},
		{Name: "nestingReadability", Weight: 0.8, Score: func(c judgment.Candidate) float64 {
			nested := ctx.hasNestedArray || ctx.hasNestedRecord || ctx.hasNestedCall || ctx.nestingDepth >= 3
			if c.Name == "multiline" && nested {
				return 1
			}
			if c.Name == "inline" && nested {
				return -0.8
			}
			return 0
		}},
		{Name: "heavyCalleePreference", Weight: 0.7, Score: func(c judgment.Candidate) float64 {
			if !ctx.isHeavy {
				return 0
			}
			if c.Name == "multiline" && (ctx.argCount >= 2 || ctx.renderedWidth > 60 || ctx.nestingDepth >= 2) {
				return 1
			}
			if c.Name == "inline" && ctx.renderedWidth < 40 && ctx.argCount <= 1 {
				return 0.3
			}
			return 0
		}},
		{Name: "commentSafety", Weight: 1.0, Score: func(c judgment.Candidate) float64 {
			if !ctx.hasCommentRisk {
				return 0
			}
			if c.Name == "leaveUnchanged" {
				return 1
			}
			if c.Name == "inline" || c.Name == "multiline" {
				return -1
			}
			return 0
		}},
		{Name: "diffStability", Weight: 0.2, Score: func(c judgment.Candidate) float64 {
			low := ctx.argCount <= 1 && ctx.nestingDepth <= 2 && ctx.renderedWidth < 70
			if c.Name == "leaveUnchanged" && low {
				return 0.6
			}
			if c.Name == "inline" && low {
				return 0.4
			}
			return 0
		}},
	}
	return judgment.Judgment{Name: "ocfmt.callLayout:" + ctx.callee, Candidates: cands, Considerations: considerations}
}
func inlinePriority(ctx callLayoutContext) int {
	if ctx.isHeavy {
		return 1
	}
	return 3
}
func multilinePriority(ctx callLayoutContext) int {
	if ctx.isHeavy {
		return 3
	}
	return 1
}

func shouldExpand(tokens []string, code string) bool {
	if strings.HasPrefix(code, "fn ") || strings.HasPrefix(code, "[") {
		return false
	}
	if !strings.Contains(code, "=") && !strings.HasPrefix(code, "return Markdown.") {
		return false
	}
	if len(code) > 110 {
		return true
	}
	if slices.Contains(tokens, "+") && len(code) > 80 {
		return true
	}
	nested := 0
	for _, tok := range tokens {
		if tok == "(" || tok == "[" || tok == "{" {
			nested++
		}
	}
	if nested >= 3 {
		return true
	}
	heavy := []string{"Markdown", "Report", "Section", "Callout", "Table", "KeyValueTable", "Artifact", "Json", "Write"}
	for _, h := range heavy {
		if strings.Contains(code, h) {
			return true
		}
	}
	return false
}
func formatTokensReadable(tokens []string, baseIndent int) []string {
	lines := []string{strings.Repeat("    ", baseIndent)}
	level := 0
	last := ""
	for _, tok := range tokens {
		switch tok {
		case "(", "[", "{":
			lines[len(lines)-1] += tok
			level++
			lines = append(lines, strings.Repeat("    ", baseIndent+level))
		case ")", "]", "}":
			level--
			if strings.TrimSpace(lines[len(lines)-1]) == "" {
				lines = lines[:len(lines)-1]
			}
			lines = append(lines, strings.Repeat("    ", baseIndent+level)+tok)
		case ",":
			lines[len(lines)-1] += tok
			lines = append(lines, strings.Repeat("    ", baseIndent+level))
		case "+":
			lines[len(lines)-1] += " +"
			lines = append(lines, strings.Repeat("    ", baseIndent+level+1))
		default:
			cur := lines[len(lines)-1]
			if strings.TrimSpace(cur) != "" && needsSpace(last, tok) {
				lines[len(lines)-1] += " "
			}
			lines[len(lines)-1] += tok
		}
		last = tok
	}
	filtered := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		filtered = append(filtered, l)
	}
	return filtered
}
func needsSpaceCompact(prev, curr string) bool {
	if isWord(prev) && isWord(curr) {
		return true
	}
	if (prev == "]" || prev == ")" || prev == "}") && isWord(curr) {
		return true
	}
	return false
}
func isWord(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '"'
}
func scanTokens(code string) []string {
	var tokens []string
	for i := 0; i < len(code); {
		c := code[i]
		if c == ' ' || c == '\t' {
			i++
			continue
		}
		if c == '"' {
			j := i + 1
			escaped := false
			for j < len(code) {
				if escaped {
					escaped = false
					j++
					continue
				}
				if code[j] == '\\' {
					escaped = true
					j++
					continue
				}
				if code[j] == '"' {
					j++
					break
				}
				j++
			}
			tokens = append(tokens, code[i:j])
			i = j
			continue
		}
		if i+1 < len(code) {
			two := code[i : i+2]
			switch two {
			case "->", "=>", "==", "!=", "<=", ">=", "..":
				tokens = append(tokens, two)
				i += 2
				continue
			}
		}
		switch c {
		case '(', ')', '{', '}', '[', ']', ',', ':', '.', '+', '-', '*', '/', '=', '<', '>', '?', '!', '^', '@':
			tokens = append(tokens, string(c))
			i++
			continue
		}
		j := i
		for j < len(code) {
			ch := code[j]
			if ch == ' ' || ch == '\t' || strings.ContainsRune("(){}[],:.+-*/=<>?!^@", rune(ch)) {
				break
			}
			j++
		}
		tokens = append(tokens, code[i:j])
		i = j
	}
	return tokens
}
func needsSpace(prev, curr string) bool {
	if prev == "" || curr == "" {
		return false
	}
	if curr == "," || curr == ")" || curr == "]" || curr == "}" || curr == ":" {
		return false
	}
	if curr == "." {
		return prev == ":"
	}
	if prev == "(" || prev == "[" || prev == "{" || prev == "." {
		return false
	}
	if curr == "(" {
		return prev == "if" || prev == "switch" || prev == "match" || prev == "while" || prev == "for" || prev == "state" || prev == "when"
	}
	if curr == "[" {
		return false
	}
	if isOperator(prev) || isOperator(curr) {
		if curr == "!" || curr == "?" || prev == "!" || prev == "?" {
			return false
		}
		return true
	}
	return true
}
func isOperator(tok string) bool {
	switch tok {
	case "+", "-", "*", "/", "%", "=", "==", "!=", "<=", ">=", "<", ">", "->", "=>", "..", "^", "and", "or", "not":
		return true
	default:
		return false
	}
}
