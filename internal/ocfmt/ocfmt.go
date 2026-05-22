package ocfmt

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/judgment"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

type Mode string

const (
	ModeDefault      Mode = ""
	ModeReadable     Mode = "readable"
	ModeCompact      Mode = "compact"
	ModeEnLLM        Mode = "en-llm"
	ModeEnLLMCompact Mode = "en-llm-compact"
)

type Options struct {
	Mode  Mode
	Check bool
}

type DecisionDiagnostics struct{ Traces []judgment.Result }

var octExtensions = map[string]struct{}{
	".oct":     {},
	".octest":  {},
	".octfail": {},
}

func FormatPath(path string) error { return FormatPathWithOptions(path, Options{}) }
func FormatPathWithOptions(path string, options Options) error { /* unchanged body omitted */
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return filepath.WalkDir(path, func(current string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !isOctFile(current) {
				return nil
			}
			return formatFile(current, options)
		})
	}
	if !isOctFile(path) {
		return fmt.Errorf("unsupported file extension: %s", path)
	}
	return formatFile(path, options)
}
func FormatSource(src string) (string, error) { return FormatSourceWithOptions(src, Options{}) }
func FormatSourceWithOptions(src string, options Options) (string, error) {
	return formatSourceWithPath("<format>.oct", src, options)
}
func formatSourceWithPath(path string, src string, options Options) (string, error) {
	mode, err := resolveMode(options.Mode)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(path), ".octfail") {
		return formatOctFailSource(src, mode)
	}
	out, _, err := formatRegularSource(path, src, mode, false)
	return out, err
}
func formatSourceWithDiagnostics(src string, options Options) (string, DecisionDiagnostics, error) {
	mode, err := resolveMode(options.Mode)
	if err != nil {
		return "", DecisionDiagnostics{}, err
	}
	if mode == ModeEnLLMCompact {
		out, _, err := formatRegularSource("<format>.oct", src, mode, false)
		return out, DecisionDiagnostics{}, err
	}
	out, diags, err := formatRegularSource("<format>.oct", src, mode, true)
	return out, diags, err
}
func resolveMode(mode Mode) (Mode, error) {
	switch mode {
	case ModeDefault, ModeReadable:
		if mode == ModeDefault {
			return ModeEnLLM, nil
		}
		return ModeEnLLM, nil
	case ModeCompact:
		return ModeEnLLMCompact, nil
	case ModeEnLLM, ModeEnLLMCompact:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid --mode %q; expected en-llm|en-llm-compact", mode)
	}
}
func formatRegularSource(path string, src string, mode Mode, withDiag bool) (string, DecisionDiagnostics, error) {
	lexed, err := lex.Analyze(source.File{Path: path, Text: src})
	if err != nil {
		return "", DecisionDiagnostics{}, err
	}
	if _, err := parse.BuildFile(lexed); err != nil {
		return "", DecisionDiagnostics{}, err
	}
	if mode == ModeEnLLMCompact {
		return normalizeCompact(src), DecisionDiagnostics{}, nil
	}
	return normalizeReadable(src, withDiag)
}

var octFailHeaderPattern = regexp.MustCompile(`^expect error:\s*"(.*)"\s*$`)

func formatOctFailSource(src string, mode Mode) (string, error) {
	lines := strings.Split(src, "\n")
	headerIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		headerIndex = i
		break
	}
	if headerIndex == -1 {
		return "", fmt.Errorf("missing expectation header")
	}
	header := strings.TrimSpace(lines[headerIndex])
	if !octFailHeaderPattern.MatchString(header) {
		return "", fmt.Errorf("malformed expectation header")
	}
	formattedSource, _, err := formatRegularSource("<format>.octfail", strings.Join(lines[headerIndex+1:], "\n"), mode, false)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i := 0; i < headerIndex; i++ {
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	b.WriteString(header)
	b.WriteByte('\n')
	b.WriteString(formattedSource)
	return b.String(), nil
}
func isOctFile(path string) bool {
	_, ok := octExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}
func formatFile(path string, options Options) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	formatted, err := formatSourceWithPath(path, string(bytes), options)
	if err != nil {
		return fmt.Errorf("format %s: %w", path, err)
	}
	if options.Check {
		if string(bytes) != formatted {
			return fmt.Errorf("%s is not formatted", path)
		}
		return nil
	}
	return os.WriteFile(path, []byte(formatted), 0o644)
}

func normalizeReadable(src string, withDiag bool) (string, DecisionDiagnostics, error) {
	return normalize(src, false, withDiag)
}
func normalizeCompact(src string) string { out, _, _ := normalize(src, true, false); return out }

func normalize(src string, compact bool, withDiag bool) (string, DecisionDiagnostics, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	indent := 0
	out := make([]string, 0, len(lines))
	diags := DecisionDiagnostics{}
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			out = append(out, "")
			continue
		}
		if strings.HasPrefix(trimmed, "}") && indent > 0 {
			indent--
		}
		if strings.HasPrefix(trimmed, "//") {
			out = append(out, strings.Repeat("    ", indent)+trimmed)
			continue
		}
		code, comment := splitCodeAndComment(trimmed)
		normCode := normalizeCode(code, compact)
		if !compact {
			line := strings.Repeat("    ", indent) + normCode
			if comment != "" {
				if normCode != "" {
					line += " "
				}
				line += comment
			}
			out = append(out, line)
		} else {
			line := strings.Repeat("    ", indent) + normCode
			if comment != "" {
				if normCode != "" {
					line += " "
				}
				line += comment
			}
			out = append(out, line)
		}
		if strings.HasSuffix(normCode, "{") {
			indent++
		}
	}
	result := strings.Join(out, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result, diags, nil
}

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

// unchanged helpers below
func splitCodeAndComment(line string) (string, string) {
	inString := false
	escaped := false
	for i := 0; i < len(line)-1; i++ {
		c := line[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == '/' && line[i+1] == '/' {
			return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i:])
		}
	}
	return strings.TrimSpace(line), ""
}
func normalizeCode(code string, compact bool) string {
	tokens := scanTokens(code)
	if len(tokens) == 0 {
		return ""
	}
	var b strings.Builder
	for i, tok := range tokens {
		if tok == "=>" {
			tok = "->"
		}
		prev := ""
		if i > 0 {
			prev = tokens[i-1]
			if prev == "=>" {
				prev = "->"
			}
		}
		if !compact && i > 0 && needsSpace(prev, tok) {
			b.WriteByte(' ')
		}
		if compact && i > 0 && needsSpaceCompact(prev, tok) {
			b.WriteByte(' ')
		}
		b.WriteString(tok)
	}
	return b.String()
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
	if curr == "," || curr == ")" || curr == "]" || curr == "}" || curr == "." || curr == ":" {
		return false
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
