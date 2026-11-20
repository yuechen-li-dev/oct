package lsp

import (
	"sort"
	"unicode"

	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/source"
)

// Semantic token legend. Order is the wire contract with the client and must
// match tools/vscode-oct/package.json `semanticTokenTypes` for custom types.
var semanticTokenTypes = []string{
	"keyword",    // 0
	"type",       // 1
	"function",   // 2
	"variable",   // 3
	"parameter",  // 4
	"property",   // 5
	"number",     // 6
	"string",     // 7
	"operator",   // 8
	"namespace",  // 9
	"enumMember", // 10
	"unit",       // 11  Oct-specific: SI unit annotations (`3.0 m`, Float<m/s>)
}

var semanticTokenModifiers = []string{
	"declaration",    // bit 0
	"defaultLibrary", // bit 1
}

const (
	tokKeyword = iota
	tokType
	tokFunction
	tokVariable
	tokParameter
	tokProperty
	tokNumber
	tokString
	tokOperator
	tokNamespace
	tokEnumMember
	tokUnit
)

const (
	modDeclaration = 1 << 0
)

// contextualKeywords are identifiers the parser treats as keywords by lexeme.
// They are not lexer keyword kinds, so a token-kind-only highlighter (and the
// old TextMate grammar) renders them as plain identifiers.
var contextualKeywords = map[string]bool{
	"template": true, "query": true, "profile": true, "yields": true,
	"table": true, "board": true, "accepts": true, "comptime": true,
	"go": true, "test": true, "benchmark": true,
}

type classified struct {
	start, end int // byte offsets
	kind       int
	mods       int
}

// classifyTokens assigns semantic kinds using local token context only. It
// never guesses across declarations: an identifier whose role is not evident
// from its neighbours is left unclassified, so the TextMate grammar shows it.
func classifyTokens(tokens []lex.Token) []classified {
	out := make([]classified, 0, len(tokens))
	prev := func(i int) lex.Token {
		if i > 0 {
			return tokens[i-1]
		}
		return lex.Token{Kind: lex.EOF}
	}
	next := func(i int) lex.Token {
		if i+1 < len(tokens) {
			return tokens[i+1]
		}
		return lex.Token{Kind: lex.EOF}
	}
	// angleDepth tracks `<...>` type-argument context after a type name so
	// `Float<m/s>` marks m and s as units.
	unitAngle := 0
	enclosing := enclosingOpeners(tokens)
	for i, t := range tokens {
		if t.Kind == lex.EOF {
			break
		}
		c := classified{start: t.Offset, end: t.EndOffset, kind: -1}
		switch {
		case isKeywordKind(t.Kind):
			c.kind = tokKeyword
		case t.Kind == lex.IntLiteral || t.Kind == lex.FloatLiteral:
			c.kind = tokNumber
		case t.Kind == lex.StringLiteral:
			c.kind = tokString
		case t.Kind == lex.Identifier:
			c.kind, c.mods = classifyIdentifier(tokens, i, prev(i), next(i), unitAngle > 0, enclosing[i])
		default:
			if isOperatorKind(t.Kind) {
				c.kind = tokOperator
			}
		}
		// Track unit type-argument context: Float<...> / Int<...>.
		if t.Kind == lex.LeftAngle && i > 0 && isNumericTypeName(prev(i).Lexeme) {
			unitAngle++
		} else if t.Kind == lex.RightAngle && unitAngle > 0 {
			unitAngle--
		}
		if c.kind >= 0 && c.end > c.start {
			out = append(out, c)
		}
	}
	return out
}

func classifyIdentifier(tokens []lex.Token, i int, prev, next lex.Token, inUnitAngle bool, opener lex.TokenKind) (int, int) {
	name := tokens[i].Lexeme
	if contextualKeywords[name] && next.Kind != lex.Colon && next.Kind != lex.Assign {
		return tokKeyword, 0
	}
	if name == "true" || name == "false" {
		return tokKeyword, 0
	}
	switch prev.Kind {
	case lex.KeywordFn:
		return tokFunction, modDeclaration
	case lex.KeywordRecord, lex.KeywordEnum, lex.KeywordConcept:
		return tokType, modDeclaration
	case lex.KeywordFlow:
		return tokType, modDeclaration
	case lex.KeywordPackage, lex.KeywordImport:
		return tokNamespace, 0
	case lex.KeywordLet, lex.KeywordVar:
		return tokVariable, modDeclaration
	}
	if inUnitAngle {
		return tokUnit, 0
	}
	// A unit annotation directly follows a numeric literal: `3.0 m`, `20.0C`.
	if (prev.Kind == lex.FloatLiteral || prev.Kind == lex.IntLiteral) && sameLine(prev, tokens[i]) {
		return tokUnit, 0
	}
	// ...and continues through a compound unit: `9.81 m/s^2`, `1 kg*m`.
	if (prev.Kind == lex.Slash || prev.Kind == lex.Star || prev.Kind == lex.Caret) && i >= 2 {
		if k := classifyUnitChainStart(tokens, i); k {
			return tokUnit, 0
		}
	}
	// Type positions: `name: Type`, `-> Type`, `as Type`.
	if prev.Kind == lex.Colon || prev.Kind == lex.Arrow || prev.Kind == lex.KeywordAs {
		if startsUpper(name) {
			return tokType, 0
		}
	}
	// Calls: `Name(` and generic calls `Name<`.
	if next.Kind == lex.LeftParen {
		return tokFunction, 0
	}
	// Selector: `Package.Member` / `value.Field`.
	if prev.Kind == lex.Dot {
		if next.Kind == lex.LeftParen {
			return tokFunction, 0
		}
		return tokProperty, 0
	}
	if next.Kind == lex.Dot && startsUpper(name) {
		return tokNamespace, 0
	}
	// `name: T` is a parameter inside a signature's parens and a field inside
	// braces (record declarations and record literals).
	if next.Kind == lex.Colon && (prev.Kind == lex.LeftParen || prev.Kind == lex.Comma) && opener == lex.LeftParen {
		return tokParameter, modDeclaration
	}
	if next.Kind == lex.Colon && opener == lex.LeftBrace {
		return tokProperty, 0
	}
	return -1, 0
}

// classifyUnitChainStart walks back over `ident (op ident)*` to see whether
// the chain is anchored on a numeric literal.
func classifyUnitChainStart(tokens []lex.Token, i int) bool {
	j := i
	for j >= 2 {
		op := tokens[j-1].Kind
		if op != lex.Slash && op != lex.Star && op != lex.Caret {
			return false
		}
		left := tokens[j-2]
		if left.Kind == lex.IntLiteral && op == lex.Caret {
			// exponent: `s^2` -> step over the exponent's base
			j -= 2
			continue
		}
		if left.Kind != lex.Identifier && left.Kind != lex.IntLiteral {
			return false
		}
		if j-3 >= 0 && (tokens[j-3].Kind == lex.FloatLiteral || tokens[j-3].Kind == lex.IntLiteral) && left.Kind == lex.Identifier && sameLine(tokens[j-3], left) {
			return true
		}
		j -= 2
	}
	return false
}

func sameLine(a, b lex.Token) bool { return a.Line == b.Line }

func startsUpper(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

func isNumericTypeName(s string) bool { return s == "Float" || s == "Int" }

func isKeywordKind(k lex.TokenKind) bool {
	switch k {
	case lex.KeywordFn, lex.KeywordLet, lex.KeywordVar, lex.KeywordReturn, lex.KeywordFor,
		lex.KeywordIn, lex.KeywordAs, lex.KeywordStep, lex.KeywordDescend, lex.KeywordTrue,
		lex.KeywordFalse, lex.KeywordMatch, lex.KeywordIf, lex.KeywordElse, lex.KeywordWhile,
		lex.KeywordFlow, lex.KeywordState, lex.KeywordGoto, lex.KeywordSuspend, lex.KeywordYield,
		lex.KeywordRemember, lex.KeywordResume, lex.KeywordWhen, lex.KeywordSwitch, lex.KeywordBatch,
		lex.KeywordCase, lex.KeywordAnd, lex.KeywordOr, lex.KeywordNot, lex.KeywordRecord,
		lex.KeywordConcept, lex.KeywordEnum, lex.KeywordPackage, lex.KeywordImport, lex.KeywordWith,
		lex.KeywordPrometheus, lex.KeywordAsync, lex.KeywordAwait:
		return true
	}
	return false
}

func isOperatorKind(k lex.TokenKind) bool {
	switch k {
	case lex.Arrow, lex.DotDot, lex.Ellipsis, lex.Question, lex.Bang, lex.BangEqual, lex.Caret,
		lex.Plus, lex.Minus, lex.Star, lex.Slash, lex.Percent, lex.EqualEqual, lex.LeftEqual,
		lex.RightEqual, lex.Assign:
		return true
	}
	return false
}

// semanticTokens lexes text and encodes LSP relative semantic token data.
// A lex failure yields no tokens; the client's TextMate grammar remains.
func semanticTokens(text string) []uint32 {
	result, err := lex.Analyze(source.File{Path: "buffer.oct", Text: text})
	if err != nil {
		return []uint32{}
	}
	toks := classifyTokens(result.Tokens)
	sort.SliceStable(toks, func(a, b int) bool { return toks[a].start < toks[b].start })
	ix := newLineIndex(text)
	data := make([]uint32, 0, len(toks)*5)
	prevLine, prevChar := 0, 0
	for _, t := range toks {
		start := ix.position(t.start)
		end := ix.position(t.end)
		if end.Line != start.Line {
			// Multi-line tokens (raw strings) are emitted only for their first
			// line; LSP clients without multilineTokenSupport reject them.
			end = Position{Line: start.Line, Character: utf16Len(text[ix.starts[start.Line]:ix.lineEnd(start.Line)])}
		}
		length := end.Character - start.Character
		if length <= 0 {
			continue
		}
		deltaLine := start.Line - prevLine
		deltaChar := start.Character
		if deltaLine == 0 {
			deltaChar = start.Character - prevChar
		}
		data = append(data, uint32(deltaLine), uint32(deltaChar), uint32(length), uint32(t.kind), uint32(t.mods))
		prevLine, prevChar = start.Line, start.Character
	}
	return data
}

// enclosingOpeners records, for each token, the innermost unclosed bracket
// kind ('(' / '{' / '[') or EOF at top level.
func enclosingOpeners(tokens []lex.Token) []lex.TokenKind {
	out := make([]lex.TokenKind, len(tokens))
	stack := []lex.TokenKind{}
	for i, t := range tokens {
		if len(stack) > 0 {
			out[i] = stack[len(stack)-1]
		} else {
			out[i] = lex.EOF
		}
		switch t.Kind {
		case lex.LeftParen, lex.LeftBrace, lex.LeftBracket:
			stack = append(stack, t.Kind)
		case lex.RightParen, lex.RightBrace, lex.RightBracket:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return out
}

// tokenTypeName renders a legend index; used by tests.
func tokenTypeName(kind int) string {
	if kind >= 0 && kind < len(semanticTokenTypes) {
		return semanticTokenTypes[kind]
	}
	return "unknown"
}
