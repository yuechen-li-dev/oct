package ocfmt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/dimension"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/source"
)

// The formatter never moves a token to another line and never changes a
// token. It decides two things: how far each line is indented, and whether
// two neighbouring tokens on a line are separated by a space. Both decisions
// are made from the lexer's tokens and the parser's markup extents, so the
// formatter reads the source the way the compiler does.
//
// Indentation follows bracket nesting. A line sits one level inside the line
// that holds its innermost open '(', '[' or '{'; a line that starts with a
// closing bracket sits level with the line that opened it.
//
// Oct-XML is the exception to both decisions. Text inside an element can be
// significant down to its relative indentation (a raw body), and the
// formatter cannot tell which bodies are raw. It therefore copies an element
// verbatim and moves its lines as one block.

const indentUnit = "    "

// tokenRole is what the formatter has worked out about a token beyond its
// lexical kind.
type tokenRole uint8

const (
	roleNone tokenRole = iota
	// roleGenericOpen and roleGenericClose are the angle brackets of a type
	// argument list, such as Float<m> or Keyed<Job, String>.
	roleGenericOpen
	roleGenericClose
	// rolePrefix is a '-', '+' or '!' that applies to the operand after it.
	rolePrefix
	// rolePostfix is a '!' or '?' that applies to the expression before it.
	rolePostfix
)

// openBracket is an unclosed '(', '[' or '{' and the indent level of the line
// it is on.
type openBracket struct {
	kind  lex.TokenKind
	level int
}

// placement says where a line starts. An ordinary line is indented by level.
// A line inside a multi-line markup element is copied from the byte offset
// from, after prefix, with nothing in it changed.
type placement struct {
	level    int
	inMarkup bool
	prefix   string
	from     int
}

type sourceLine struct {
	start, end  int // byte offsets; end excludes the newline
	first, last int // token index range [first, last) of tokens starting on the line
}

type layout struct {
	src     string
	compact bool
	arrow   string      // the spelling every arrow is written with; "" keeps each one
	toks    []lex.Token // without the trailing EOF
	lines   []sourceLine
	spans   []ast.MarkupSpan // outermost markup elements, in source order
	nested  []ast.MarkupSpan // every markup element the parser reported

	role      []tokenRole
	inGeneric []bool // strictly inside a type argument list
	unitTight []bool // part of a literal's unit suffix; joined to the token before
	inMarkup  []bool // inside a markup element
	rowStart  []bool // a '[' that starts a row of a matrix literal

	glue map[string]bool // memo for gluesSafely
}

// formatLayout formats src, which has already been lexed and parsed
// successfully and uses "\n" line endings.
func formatLayout(src string, tokens []lex.Token, spans []ast.MarkupSpan, resolved settings) (string, error) {
	l := &layout{src: src, compact: resolved.compact, arrow: resolved.arrow}
	for _, tok := range tokens {
		if tok.Kind != lex.EOF {
			l.toks = append(l.toks, tok)
		}
	}
	l.spans = outermostSpans(spans)
	l.nested = spans
	l.splitLines()
	l.markMarkupTokens()
	l.classify()
	l.markMatrixRows()

	places := l.indentation()
	out := make([]string, len(l.lines))
	for i := range l.lines {
		out[i] = l.renderLine(i, places[i])
	}
	result := strings.Join(out, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	if err := l.verify(result); err != nil {
		return "", err
	}
	return result, nil
}

// outermostSpans drops every span that lies inside another and sorts the rest.
func outermostSpans(spans []ast.MarkupSpan) []ast.MarkupSpan {
	sorted := append([]ast.MarkupSpan(nil), spans...)
	sort.Slice(sorted, func(a, b int) bool {
		if sorted[a].Offset != sorted[b].Offset {
			return sorted[a].Offset < sorted[b].Offset
		}
		return sorted[a].EndOffset > sorted[b].EndOffset
	})
	var out []ast.MarkupSpan
	for _, span := range sorted {
		if len(out) > 0 && span.EndOffset <= out[len(out)-1].EndOffset {
			continue
		}
		out = append(out, span)
	}
	return out
}

func (l *layout) splitLines() {
	start := 0
	tok := 0
	for {
		end := strings.IndexByte(l.src[start:], '\n')
		last := end < 0
		if last {
			end = len(l.src)
		} else {
			end += start
		}
		line := sourceLine{start: start, end: end, first: tok}
		for tok < len(l.toks) && l.toks[tok].Offset < end {
			tok++
		}
		line.last = tok
		l.lines = append(l.lines, line)
		if last {
			break
		}
		start = end + 1
	}
}

func (l *layout) markMarkupTokens() {
	l.inMarkup = make([]bool, len(l.toks))
	span := 0
	for i, tok := range l.toks {
		for span < len(l.spans) && l.spans[span].EndOffset <= tok.Offset {
			span++
		}
		if span < len(l.spans) && tok.Offset >= l.spans[span].Offset {
			l.inMarkup[i] = true
		}
	}
}

// spanAt returns the outermost markup element that covers offset.
func (l *layout) spanAt(offset int) (ast.MarkupSpan, bool) {
	i := sort.Search(len(l.spans), func(i int) bool { return l.spans[i].EndOffset > offset })
	if i < len(l.spans) && l.spans[i].Offset <= offset {
		return l.spans[i], true
	}
	return ast.MarkupSpan{}, false
}

// classify assigns token roles. It runs once over the whole file because a
// role can depend on the token before it, which may be on an earlier line.
func (l *layout) classify() {
	n := len(l.toks)
	l.role = make([]tokenRole, n)
	l.inGeneric = make([]bool, n)
	l.unitTight = make([]bool, n)

	for i := 0; i < n; i++ {
		if l.inMarkup[i] {
			continue
		}
		tok := l.toks[i]
		switch tok.Kind {
		case lex.LeftAngle:
			if l.role[i] == roleNone {
				if closeAt, ok := l.genericClose(i); ok {
					l.role[i] = roleGenericOpen
					l.role[closeAt] = roleGenericClose
					for k := i + 1; k < closeAt; k++ {
						l.inGeneric[k] = true
					}
				}
			}
		case lex.IntLiteral, lex.FloatLiteral:
			l.markUnitSuffix(i)
		}
	}
	// Prefix and postfix operators depend on generic brackets, so they are
	// decided after those are known.
	for i := 0; i < n; i++ {
		if l.inMarkup[i] {
			continue
		}
		switch l.toks[i].Kind {
		case lex.Minus, lex.Plus:
			if l.prefixPosition(i) {
				l.role[i] = rolePrefix
			}
		case lex.Bang:
			switch {
			case l.prefixPosition(i):
				l.role[i] = rolePrefix
			case l.separatesErrorType(i):
				// "T ! Error" in a signature: the '!' is a separator.
			default:
				l.role[i] = rolePostfix
			}
		case lex.Question:
			l.role[i] = rolePostfix
		}
	}
}

// markMatrixRows finds each '[' that starts a row of a matrix literal. Such
// a '[' is an operand, never an index, and that matters for the row that
// follows a count, as in "[0.0, 0.0] ... n [1.0, 2.0]": the parser reads the
// '[' as the next row whatever the spacing, and written against the count it
// would look like an index. The first row, after the literal's own '[', and a
// row after another row's ']' are spaced by earlier rules.
//
// A matrix literal is recognised as the parser recognises it: the name
// "matrix", then '[', then '[' or ']'.
func (l *layout) markMatrixRows() {
	l.rowStart = make([]bool, len(l.toks))
	// One entry per open bracket: whether it is the '[' of a matrix literal.
	var open []bool
	for i, tok := range l.toks {
		if l.inMarkup[i] {
			continue
		}
		switch tok.Kind {
		case lex.LeftBracket:
			if len(open) > 0 && open[len(open)-1] {
				l.rowStart[i] = true
			}
			literal := i > 0 && i+1 < len(l.toks) && l.toks[i-1].Kind == lex.Identifier && l.toks[i-1].Lexeme == "matrix" &&
				(l.toks[i+1].Kind == lex.LeftBracket || l.toks[i+1].Kind == lex.RightBracket)
			open = append(open, literal)
		case lex.LeftParen, lex.LeftBrace:
			open = append(open, false)
		case lex.RightBracket, lex.RightParen, lex.RightBrace:
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
		}
	}
}

// separatesErrorType reports whether the '!' at i is the separator of a
// fallible type, "Int ! Error", and not an unwrap, "Parse(raw)!". An unwrap is
// never followed by a name on its line; a separator always is.
func (l *layout) separatesErrorType(i int) bool {
	if i+1 >= len(l.toks) || l.toks[i+1].Kind != lex.Identifier || l.toks[i+1].Line != l.toks[i].Line {
		return false
	}
	if l.toks[i+1].Lexeme == "Error" {
		return true
	}
	switch l.toks[i-1].Kind {
	case lex.Identifier, lex.RightBracket:
		return true
	case lex.RightAngle:
		return l.role[i-1] == roleGenericClose
	}
	return false
}

// prefixPosition reports whether the token at i stands where an expression
// may begin, which is what makes a '-' a sign and a '.' a selector.
func (l *layout) prefixPosition(i int) bool {
	if i == 0 {
		return true
	}
	if l.inMarkup[i-1] {
		return false
	}
	if l.isName(i - 1) {
		return false
	}
	prev := l.toks[i-1]
	switch prev.Kind {
	case lex.IntLiteral, lex.FloatLiteral, lex.StringLiteral,
		lex.KeywordTrue, lex.KeywordFalse,
		lex.RightParen, lex.RightBracket, lex.RightBrace:
		return false
	case lex.RightAngle:
		return l.role[i-1] != roleGenericClose
	case lex.Bang, lex.Question:
		return l.role[i-1] != rolePostfix
	}
	return true
}

// isName reports whether the token at i is used as a name. The parser accepts
// flow, state, step and descend as names, so "state.Count" and "let step = 2"
// are ordinary. Used as keywords they follow a complete expression or start a
// declaration; as names they stand where an expression begins or start a
// statement.
func (l *layout) isName(i int) bool {
	switch l.toks[i].Kind {
	case lex.Identifier:
		return true
	case lex.KeywordFlow, lex.KeywordState, lex.KeywordStep, lex.KeywordDescend:
		if i+1 < len(l.toks) && l.toks[i+1].Kind == lex.Identifier && l.toks[i+1].Line == l.toks[i].Line {
			return false // "state Idle {", "flow Run(", "step stride"
		}
		return i == 0 || l.toks[i-1].Line != l.toks[i].Line || l.prefixPosition(i)
	}
	return false
}

// genericClose decides whether the '<' at open starts a type argument list
// and, if so, returns the index of its '>'. Oct writes "A < B" and
// "Float<m>" with the same tokens, so this is a judgement from shape: the
// name before it, what lies between the brackets, and what follows. A wrong
// answer costs spacing only; it cannot change the program.
func (l *layout) genericClose(open int) (int, bool) {
	if open == 0 || l.inMarkup[open-1] {
		return 0, false
	}
	name := l.toks[open-1]
	if name.Kind != lex.Identifier {
		return 0, false
	}
	typeName := startsUpper(name.Lexeme)
	depth := 0
	for i := open; i < len(l.toks); i++ {
		tok := l.toks[i]
		if tok.Line != l.toks[open].Line || l.inMarkup[i] {
			return 0, false
		}
		switch tok.Kind {
		case lex.LeftAngle:
			depth++
		case lex.RightAngle:
			depth--
			if depth == 0 {
				if i == open+1 {
					return 0, false
				}
				if !typeName {
					// A lower-case name is a template function, which is
					// always applied or declared with '(' and names types.
					first := l.toks[open+1]
					applied := i+1 < len(l.toks) && l.toks[i+1].Kind == lex.LeftParen
					if !applied || first.Kind != lex.Identifier || !startsUpper(first.Lexeme) {
						return 0, false
					}
				}
				if i+1 < len(l.toks) && l.toks[i+1].Line == tok.Line {
					switch l.toks[i+1].Kind {
					case lex.IntLiteral, lex.FloatLiteral, lex.StringLiteral, lex.Minus, lex.KeywordTrue, lex.KeywordFalse, lex.KeywordNot:
						return 0, false
					}
				}
				return i, true
			}
		case lex.Identifier, lex.Dot, lex.Comma, lex.IntLiteral,
			lex.Star, lex.Slash, lex.Caret, lex.Minus, lex.Plus,
			lex.LeftBracket, lex.RightBracket, lex.LeftParen, lex.RightParen,
			lex.Arrow, lex.KeywordFn, lex.Bang:
		default:
			return 0, false
		}
	}
	return 0, false
}

// markUnitSuffix marks the tokens of a literal's unit suffix, such as the
// "kg*m^-3" of 7.2kg*m^-3, so that they are written without spaces. It
// follows the parser's rule for where a suffix starts and how far it runs.
func (l *layout) markUnitSuffix(number int) {
	i := number + 1
	if i >= len(l.toks) || l.inMarkup[i] || l.toks[i].Kind != lex.Identifier {
		return
	}
	unit := l.toks[i]
	adjacent := l.toks[number].EndOffset == unit.Offset
	next := lex.EOF
	if i+1 < len(l.toks) {
		next = l.toks[i+1].Kind
	}
	continues := next == lex.Star || next == lex.Slash || next == lex.Caret
	if adjacent && (unit.Lexeme == "deg" || (unit.Lexeme == "C" && !continues)) {
		return // an angle or Celsius literal: the suffix is this one name
	}
	if !adjacent {
		if unit.Line != l.toks[number].Line {
			return
		}
		if _, ok := dimension.FromBaseName(unit.Lexeme); !ok {
			return
		}
	}
	i = l.markUnitFactor(i)
	for i+1 < len(l.toks) && (l.toks[i].Kind == lex.Star || l.toks[i].Kind == lex.Slash) && l.toks[i+1].Kind == lex.Identifier && !l.inMarkup[i+1] {
		l.unitTight[i] = true
		l.unitTight[i+1] = true
		i = l.markUnitFactor(i + 1)
	}
}

// markUnitFactor marks the exponent of the unit name at i, if it has one, and
// returns the index after the factor.
func (l *layout) markUnitFactor(i int) int {
	i++
	if i >= len(l.toks) || l.toks[i].Kind != lex.Caret {
		return i
	}
	l.unitTight[i] = true
	i++
	if i < len(l.toks) && (l.toks[i].Kind == lex.Minus || l.toks[i].Kind == lex.Plus) {
		l.unitTight[i] = true
		i++
	}
	if i < len(l.toks) && l.toks[i].Kind == lex.IntLiteral {
		l.unitTight[i] = true
		i++
	}
	return i
}

func startsUpper(s string) bool {
	return s != "" && s[0] >= 'A' && s[0] <= 'Z'
}

// indentation returns the placement of every line.
func (l *layout) indentation() []placement {
	places := make([]placement, len(l.lines))
	// One entry per open bracket: the indent level of the line it is on.
	var open []openBracket
	for i := 0; i < len(l.lines); i++ {
		line := l.lines[i]
		if places[i].inMarkup {
			// The line continues a markup element from an earlier line, whose
			// first line placed the whole block. Only code after the element
			// can open or close a bracket.
			span, _ := l.spanAt(line.start)
			l.applyBrackets(line, span.EndOffset, 0, &open, places[i].level)
			continue
		}

		level := 0
		inBlock := true
		if len(open) > 0 {
			level = open[len(open)-1].level + 1
			inBlock = open[len(open)-1].kind == lex.LeftBrace
		}
		closers := 0
		for t := line.first; t < line.last && !l.inMarkup[t] && isCloser(l.toks[t].Kind); t++ {
			closers++
		}
		if closers > 0 {
			keep := len(open) - closers
			if keep < 0 {
				keep = 0
			}
			level = 0
			if keep < len(open) {
				level = open[keep].level
			}
			open = open[:keep]
		}
		// Inside '(' or '[' every line already sits one level in, so a
		// continued expression there is not indented again.
		if closers == 0 && inBlock && l.continuesStatement(i) {
			level++
		}
		places[i].level = level
		l.applyBrackets(line, line.start, closers, &open, level)

		if span, ok := l.spanStartingOn(line); ok && span.EndOffset > line.end {
			l.placeMarkupBlock(i, span, level, places)
		}
	}
	return places
}

// continuesStatement reports whether line i carries on an expression from the
// line before without a bracket to show it: the earlier line ends with a
// binary operator, or this one starts with one. Such a line is indented one
// level further.
func (l *layout) continuesStatement(i int) bool {
	line := l.lines[i]
	if line.first == line.last || l.inMarkup[line.first] {
		return false
	}
	first := line.first
	if first == 0 || l.inMarkup[first-1] {
		return false
	}
	if l.joinsOperands(first-1) && !isOpener(l.toks[first-1].Kind) {
		return true
	}
	if l.toks[first].Kind == lex.Dot {
		return !l.prefixPosition(first)
	}
	return l.joinsOperands(first)
}

// joinsOperands reports whether the token at i is a binary operator.
func (l *layout) joinsOperands(i int) bool {
	switch l.toks[i].Kind {
	case lex.Plus, lex.Minus:
		return l.role[i] != rolePrefix
	case lex.LeftAngle, lex.RightAngle:
		return l.role[i] == roleNone
	case lex.Star, lex.Slash, lex.Percent, lex.Caret, lex.At, lex.Ampersand, lex.Pipe,
		lex.EqualEqual, lex.BangEqual, lex.LeftEqual, lex.RightEqual,
		lex.KeywordAnd, lex.KeywordOr, lex.Assign:
		return !l.unitTight[i] && !l.inGeneric[i]
	}
	return false
}

// applyBrackets updates the open-bracket stack for the tokens of line at or
// after offset from, skipping the first skip tokens and all markup.
func (l *layout) applyBrackets(line sourceLine, from int, skip int, open *[]openBracket, level int) {
	for t := line.first + skip; t < line.last; t++ {
		if l.inMarkup[t] || l.toks[t].Offset < from {
			continue
		}
		switch {
		case isOpener(l.toks[t].Kind):
			*open = append(*open, openBracket{kind: l.toks[t].Kind, level: level})
		case isCloser(l.toks[t].Kind):
			if len(*open) > 0 {
				*open = (*open)[:len(*open)-1]
			}
		}
	}
}

// codeStart is the offset of the first non-blank byte of line, or its end.
func (l *layout) codeStart(line sourceLine) int {
	for i := line.start; i < line.end; i++ {
		if l.src[i] != ' ' && l.src[i] != '\t' && l.src[i] != '\r' {
			return i
		}
	}
	return line.end
}

// spanStartingOn returns the last markup element that starts on line; it is
// the only one that can run past the end of the line.
func (l *layout) spanStartingOn(line sourceLine) (ast.MarkupSpan, bool) {
	i := sort.Search(len(l.spans), func(i int) bool { return l.spans[i].Offset >= line.end })
	if i > 0 && l.spans[i-1].Offset >= line.start {
		return l.spans[i-1], true
	}
	return ast.MarkupSpan{}, false
}

// placeMarkupBlock places the lines of a multi-line markup element that
// starts on line first.
//
// A raw body's value is its lines with their common leading white space
// removed, so the formatter may replace exactly that common part and nothing
// else. The lines after the first therefore move as one block: the white
// space they all share is replaced by one level more than the first line's
// indentation, and each line keeps whatever follows it, byte for byte. A line
// of white space only is written empty. A last line that starts with the
// element's own closing tag sits level with the first line.
//
// When an element's body starts on the same line as its opening tag, that
// line has no leading white space, the body has no common part to replace,
// and moving its other lines would change its value. Such a block is pinned:
// every line after the first is copied exactly.
func (l *layout) placeMarkupBlock(first int, span ast.MarkupSpan, level int, places []placement) {
	last := first
	for last+1 < len(l.lines) && l.lines[last+1].start < span.EndOffset {
		last++
	}
	for i := first + 1; i <= last; i++ {
		places[i] = placement{level: level, inMarkup: true, from: l.lines[i].start}
	}
	if l.pinned(span) {
		return
	}
	closesBlock := l.codeStart(l.lines[last]) == span.TerminatorOffset
	blockEnd := last
	if closesBlock {
		blockEnd = last - 1
		places[last].prefix = strings.Repeat(indentUnit, level)
		places[last].from = l.codeStart(l.lines[last])
	}
	shared := -1
	for i := first + 1; i <= blockEnd; i++ {
		line := l.lines[i]
		if l.codeStart(line) == line.end {
			continue
		}
		if width := l.codeStart(line) - line.start; shared < 0 || width < shared {
			shared = width
		}
	}
	for i := first + 1; i <= blockEnd; i++ {
		line := l.lines[i]
		if l.codeStart(line) == line.end {
			places[i].from = line.end
			continue
		}
		places[i].prefix = strings.Repeat(indentUnit, level+1)
		places[i].from = line.start + shared
	}
}

// pinned reports whether some element inside span, or span itself, has body
// text on the line of its opening tag and continues on later lines.
func (l *layout) pinned(span ast.MarkupSpan) bool {
	for _, element := range l.nested {
		if element.Offset < span.Offset || element.EndOffset > span.EndOffset {
			continue
		}
		lineEnd := strings.IndexByte(l.src[element.BodyOffset:], '\n')
		if lineEnd < 0 || element.BodyOffset+lineEnd >= element.TerminatorOffset {
			continue // the body ends on the line it starts on
		}
		if strings.TrimSpace(l.src[element.BodyOffset:element.BodyOffset+lineEnd]) != "" {
			return true
		}
	}
	return false
}

func isOpener(kind lex.TokenKind) bool {
	return kind == lex.LeftParen || kind == lex.LeftBracket || kind == lex.LeftBrace
}

func isCloser(kind lex.TokenKind) bool {
	return kind == lex.RightParen || kind == lex.RightBracket || kind == lex.RightBrace
}

// piece is one unit of a rendered line: a token, a ';', or a markup element
// copied verbatim.
type piece struct {
	text  string
	token int // index into toks, or -1
	kind  pieceKind
}

type pieceKind uint8

const (
	pieceToken pieceKind = iota
	pieceSemicolon
	pieceMarkup
)

func (l *layout) renderLine(index int, place placement) string {
	line := l.lines[index]
	codeStart := l.codeStart(line)

	var pieces []piece
	var b strings.Builder
	cursor := codeStart
	if place.inMarkup {
		// The line continues a markup element: copy it up to the element's
		// end exactly as written, from where its own text begins.
		span, _ := l.spanAt(line.start)
		end := min(span.EndOffset, line.end)
		if place.from >= end {
			return ""
		}
		b.WriteString(place.prefix)
		pieces = append(pieces, piece{text: l.src[place.from:end], token: -1, kind: pieceMarkup})
		cursor = end
	} else {
		if codeStart == line.end {
			return ""
		}
		b.WriteString(strings.Repeat(indentUnit, place.level))
	}

	comment := ""
	// gap handles the text between two pieces: white space, ';' separators
	// (which the lexer skips) and a trailing comment.
	gap := func(to int) {
		text := l.src[cursor:to]
		if at := strings.Index(text, "//"); at >= 0 {
			comment = strings.TrimRight(l.src[cursor+at:line.end], " \t\r")
			text = text[:at]
		}
		for range strings.Count(text, ";") {
			pieces = append(pieces, piece{text: ";", token: -1, kind: pieceSemicolon})
		}
	}
	for t := line.first; t < line.last && comment == ""; t++ {
		tok := l.toks[t]
		if tok.Offset < cursor {
			continue
		}
		gap(tok.Offset)
		if comment != "" {
			break
		}
		if l.inMarkup[t] {
			span, _ := l.spanAt(tok.Offset)
			end := min(span.EndOffset, line.end)
			pieces = append(pieces, piece{text: l.src[tok.Offset:end], token: -1, kind: pieceMarkup})
			cursor = end
			continue
		}
		text := l.src[tok.Offset:tok.EndOffset]
		if tok.Kind == lex.Arrow && l.arrow != "" {
			text = l.arrow
		}
		pieces = append(pieces, piece{text: text, token: t, kind: pieceToken})
		cursor = tok.EndOffset
	}
	if comment == "" {
		gap(line.end)
	}

	for i, p := range pieces {
		if i > 0 && l.spaceBetween(pieces[i-1], p) {
			b.WriteByte(' ')
		}
		b.WriteString(p.text)
	}
	if comment != "" {
		if len(pieces) > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(comment)
	}
	// White space at the end of a line is dropped, except where it may be
	// part of a raw body: on a line that ends inside a markup element, unless
	// that line only opens the element.
	if n := len(pieces); n > 0 && pieces[n-1].kind == pieceMarkup && cursor == line.end && comment == "" {
		if span, ok := l.spanAt(line.end - 1); ok && span.EndOffset > line.end && (place.inMarkup || l.pinned(span)) {
			return b.String()
		}
	}
	return strings.TrimRight(b.String(), " \t\r")
}

// spaceBetween decides whether a space separates two neighbouring pieces.
func (l *layout) spaceBetween(prev, cur piece) bool {
	if kept, ok := l.authorSpacing(prev, cur); ok {
		return kept
	}
	space := l.readableSpace(prev, cur)
	if l.compact {
		// Compact output keeps a space only between two words, and between a
		// closing bracket and a word.
		space = space && wordLike(l, cur) && (wordLike(l, prev) || prev.kind == pieceToken && isCloser(l.toks[prev.token].Kind))
	}
	if space || prev.kind == pieceMarkup || cur.kind == pieceMarkup {
		return space
	}
	// Two pieces are only written together if they still read as the tokens
	// they were.
	return !l.gluesSafely(prev.text, cur.text)
}

// authorSpacing covers the two junctions that are written as the author
// spaced them, in every mode.
//
// A name that touches a number is that number's unit, and the parser reads it
// so; the formatter must not join or split them.
//
// "a[i][j]" indexes twice and "[1, 2] [3, 4]" lists two matrix rows. The
// tokens are the same and only the author knows which is meant.
func (l *layout) authorSpacing(prev, cur piece) (space bool, ok bool) {
	if prev.kind != pieceToken || cur.kind != pieceToken {
		return false, false
	}
	pt, ct := l.toks[prev.token], l.toks[cur.token]
	number := pt.Kind == lex.IntLiteral || pt.Kind == lex.FloatLiteral
	if (number && ct.Kind == lex.Identifier) || (pt.Kind == lex.RightBracket && ct.Kind == lex.LeftBracket) {
		return pt.EndOffset != ct.Offset, true
	}
	return false, false
}

func wordLike(l *layout, p piece) bool {
	if p.kind == pieceMarkup {
		return true
	}
	if p.kind != pieceToken {
		return false
	}
	switch kind := l.toks[p.token].Kind; kind {
	case lex.Identifier, lex.IntLiteral, lex.FloatLiteral, lex.StringLiteral:
		return true
	default:
		return strings.HasPrefix(string(kind), "Keyword")
	}
}

func (l *layout) readableSpace(prev, cur piece) bool {
	if cur.kind == pieceSemicolon {
		return false
	}
	if prev.kind == pieceSemicolon {
		return true
	}
	if prev.kind == pieceMarkup || cur.kind == pieceMarkup {
		return l.markupSpace(prev, cur)
	}
	p, c := prev.token, cur.token
	pt, ct := l.toks[p], l.toks[c]

	if l.unitTight[c] {
		return false
	}
	if l.role[c] == roleGenericOpen || l.role[p] == roleGenericOpen || l.role[c] == roleGenericClose {
		return false
	}
	if l.inGeneric[p] && l.inGeneric[c] && (isUnitOperator(pt.Kind) || isUnitOperator(ct.Kind)) {
		return false
	}
	if l.role[c] == rolePostfix {
		return false
	}

	switch pt.Kind {
	case lex.LeftParen, lex.LeftBracket, lex.Dot:
		return false
	case lex.DotDot:
		// A range is written tight, "0..n", but an open-ended one does not
		// swallow the brace after it: "for i in 0.. {".
		return ct.Kind == lex.LeftBrace || ct.Kind == lex.RightBrace
	}
	if l.role[p] == rolePrefix {
		return false
	}

	switch ct.Kind {
	case lex.Comma, lex.RightParen, lex.RightBracket, lex.Colon:
		return false
	case lex.DotDot:
		// "..n" has no left operand and is spaced like any other operand.
		return l.prefixPosition(c)
	case lex.Dot:
		// ".Field" after an operator or separator is a selector; anywhere
		// else the dot joins a name to what it belongs to.
		return l.prefixPosition(c)
	case lex.RightBrace:
		return pt.Kind != lex.LeftBrace
	case lex.LeftParen:
		return !callsOrGroups(l, p)
	case lex.LeftBracket:
		return l.rowStart[c] || !callsOrGroups(l, p)
	}
	return true
}

// callsOrGroups reports whether an opening '(' or '[' attaches to the token
// at p: a call, an index, an array type, or an immediately nested bracket.
func callsOrGroups(l *layout, p int) bool {
	if l.isName(p) {
		return true
	}
	switch l.toks[p].Kind {
	case lex.RightParen, lex.RightBracket, lex.KeywordFn:
		return true
	case lex.RightAngle:
		return l.role[p] == roleGenericClose
	case lex.Bang, lex.Question:
		return l.role[p] == rolePostfix
	}
	return false
}

func isUnitOperator(kind lex.TokenKind) bool {
	return kind == lex.Star || kind == lex.Slash || kind == lex.Caret || kind == lex.Minus || kind == lex.Plus
}

// markupSpace spaces a verbatim markup element like an operand.
func (l *layout) markupSpace(prev, cur piece) bool {
	if cur.kind == pieceMarkup {
		if prev.kind != pieceToken {
			return true
		}
		switch l.toks[prev.token].Kind {
		case lex.LeftParen, lex.LeftBracket:
			return false
		}
		return true
	}
	switch l.toks[cur.token].Kind {
	case lex.Comma, lex.RightParen, lex.RightBracket, lex.Dot:
		return false
	}
	return true
}

// gluesSafely reports whether writing a directly before b leaves both as the
// tokens they were. "!" before "==" would not: the lexer would read "!=".
func (l *layout) gluesSafely(a, b string) bool {
	key := a + "\x00" + b
	if safe, known := l.glue[key]; known {
		return safe
	}
	safe := gluesSafely(a, b)
	if l.glue == nil {
		l.glue = map[string]bool{}
	}
	l.glue[key] = safe
	return safe
}

func gluesSafely(a, b string) bool {
	separate, err := lex.Analyze(source.File{Text: a + " " + b})
	if err != nil {
		return false
	}
	joined, err := lex.Analyze(source.File{Text: a + b})
	if err != nil || len(joined.Tokens) != len(separate.Tokens) {
		return false
	}
	for i := range joined.Tokens {
		if joined.Tokens[i].Kind != separate.Tokens[i].Kind || joined.Tokens[i].Lexeme != separate.Tokens[i].Lexeme {
			return false
		}
	}
	return true
}

// verify checks the one promise the formatter makes about meaning: the
// output has the same tokens as the input, on the same lines, and every
// number keeps or lacks its touching unit name exactly as before. An arrow may
// change its spelling only when a spelling was asked for, and then only to
// that one.
func (l *layout) verify(out string) error {
	lexed, err := lex.Analyze(source.File{Text: out})
	if err != nil {
		return fmt.Errorf("internal error: formatted output does not lex: %w", err)
	}
	got := lexed.Tokens
	if len(got) > 0 && got[len(got)-1].Kind == lex.EOF {
		got = got[:len(got)-1]
	}
	if len(got) != len(l.toks) {
		return fmt.Errorf("internal error: formatting would change the token count from %d to %d", len(l.toks), len(got))
	}
	for i, want := range l.toks {
		have := got[i]
		lexeme := want.Lexeme
		if want.Kind == lex.Arrow && l.arrow != "" {
			lexeme = l.arrow
		}
		same := have.Kind == want.Kind && have.Lexeme == lexeme && have.Line == want.Line
		if same && i > 0 && want.Kind == lex.Identifier && (l.toks[i-1].Kind == lex.IntLiteral || l.toks[i-1].Kind == lex.FloatLiteral) {
			same = (l.toks[i-1].EndOffset == want.Offset) == (got[i-1].EndOffset == have.Offset)
		}
		if !same {
			return fmt.Errorf("internal error: formatting would change the program at line %d near %q", want.Line, want.Lexeme)
		}
	}
	return nil
}
