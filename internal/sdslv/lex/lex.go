package lex

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuechen-li-dev/oct/internal/sdslv/token"
	"github.com/yuechen-li-dev/oct/internal/source"
)

type Result struct {
	Source source.File
	Tokens []token.Token
}

func Analyze(file source.File) (Result, error) {
	l := lexer{source: file.Text, line: 1, column: 1}
	tokens, err := l.lexAll()
	if err != nil {
		return Result{}, fmt.Errorf("lex %s: %w", file.Path, err)
	}
	return Result{Source: file, Tokens: tokens}, nil
}

type lexer struct {
	source        string
	offset        int
	line          int
	column        int
	foreignHeader bool
}

func (l *lexer) lexAll() ([]token.Token, error) {
	var tokens []token.Token
	for {
		l.skipWhitespaceAndComments()
		if l.atEnd() {
			pos := l.position()
			tokens = append(tokens, token.Token{Kind: token.EOF, Line: l.line, Column: l.column, Span: source.Span{Start: pos, End: pos}})
			return tokens, nil
		}
		start := l.position()
		t, err := l.nextToken()
		if err != nil {
			return nil, err
		}
		// All token constructors share this finalization path, including raw
		// foreign blocks and punctuation.
		t.Span = source.Span{Start: start, End: l.position()}
		tokens = append(tokens, t)
	}
}

func (l *lexer) skipWhitespaceAndComments() {
	for {
		for {
			r, _ := l.peekRune()
			if !unicode.IsSpace(r) {
				break
			}
			l.advanceRune()
		}
		if l.matchString("//") {
			for !l.atEnd() {
				r, _ := l.peekRune()
				if r == '\n' {
					break
				}
				l.advanceRune()
			}
			continue
		}
		return
	}
}

func (l *lexer) nextToken() (token.Token, error) {
	line, column := l.line, l.column
	start := l.position()
	finish := func(kind token.Kind, lexeme string) token.Token {
		return token.Token{Kind: kind, Lexeme: lexeme, Line: line, Column: column, Span: source.Span{Start: start, End: l.position()}}
	}
	r, _ := l.peekRune()
	if isIdentifierStart(r) {
		lexeme := l.scanIdentifier()
		kind := token.LookupKeyword(lexeme)
		if kind == token.KeywordHLSL {
			l.foreignHeader = true
		}
		return finish(kind, lexeme), nil
	}
	if unicode.IsDigit(r) {
		kind, lexeme, err := l.scanNumber()
		if err != nil {
			return token.Token{}, err
		}
		return finish(kind, lexeme), nil
	}
	switch r {
	case '"':
		lexeme, err := l.scanString()
		return finish(token.StringLiteral, lexeme), err
	case '(':
		l.advanceRune()
		return token.Token{Kind: token.LeftParen, Lexeme: "(", Line: line, Column: column}, nil
	case ')':
		l.advanceRune()
		return token.Token{Kind: token.RightParen, Lexeme: ")", Line: line, Column: column}, nil
	case '{':
		if l.foreignHeader {
			l.advanceRune()
			raw, err := l.scanForeignBlock(line, column)
			l.foreignHeader = false
			return token.Token{Kind: token.RawForeignSource, Lexeme: raw, Line: line, Column: column}, err
		}
		l.advanceRune()
		return token.Token{Kind: token.LeftBrace, Lexeme: "{", Line: line, Column: column}, nil
	case '}':
		l.advanceRune()
		return token.Token{Kind: token.RightBrace, Lexeme: "}", Line: line, Column: column}, nil
	case '[':
		l.advanceRune()
		return token.Token{Kind: token.LeftBracket, Lexeme: "[", Line: line, Column: column}, nil
	case ']':
		l.advanceRune()
		return token.Token{Kind: token.RightBracket, Lexeme: "]", Line: line, Column: column}, nil
	case '<':
		l.advanceRune()
		if l.matchString("=") {
			return token.Token{Kind: token.LeftEqual, Lexeme: "<=", Line: line, Column: column}, nil
		}
		return token.Token{Kind: token.LeftAngle, Lexeme: "<", Line: line, Column: column}, nil
	case '>':
		l.advanceRune()
		if l.matchString("=") {
			return token.Token{Kind: token.RightEqual, Lexeme: ">=", Line: line, Column: column}, nil
		}
		return token.Token{Kind: token.RightAngle, Lexeme: ">", Line: line, Column: column}, nil
	case ',':
		l.advanceRune()
		return token.Token{Kind: token.Comma, Lexeme: ",", Line: line, Column: column}, nil
	case '.':
		l.advanceRune()
		if l.matchString(".") {
			return token.Token{Kind: token.DotDot, Lexeme: "..", Line: line, Column: column}, nil
		}
		return token.Token{Kind: token.Dot, Lexeme: ".", Line: line, Column: column}, nil
	case ':':
		l.advanceRune()
		return token.Token{Kind: token.Colon, Lexeme: ":", Line: line, Column: column}, nil
	case ';':
		l.advanceRune()
		return token.Token{Kind: token.Semicolon, Lexeme: ";", Line: line, Column: column}, nil
	case '=':
		l.advanceRune()
		if l.matchString(">") {
			return token.Token{Kind: token.Arrow, Lexeme: "=>", Line: line, Column: column}, nil
		}
		if l.matchString("=") {
			return token.Token{Kind: token.EqualEqual, Lexeme: "==", Line: line, Column: column}, nil
		}
		return token.Token{Kind: token.Assign, Lexeme: "=", Line: line, Column: column}, nil
	case '+':
		l.advanceRune()
		return token.Token{Kind: token.Plus, Lexeme: "+", Line: line, Column: column}, nil
	case '-':
		l.advanceRune()
		if l.matchString(">") {
			return token.Token{Kind: token.Arrow, Lexeme: "->", Line: line, Column: column}, nil
		}
		return token.Token{Kind: token.Minus, Lexeme: "-", Line: line, Column: column}, nil
	case '*':
		l.advanceRune()
		return token.Token{Kind: token.Star, Lexeme: "*", Line: line, Column: column}, nil
	case '/':
		l.advanceRune()
		return token.Token{Kind: token.Slash, Lexeme: "/", Line: line, Column: column}, nil
	case '%':
		l.advanceRune()
		return token.Token{Kind: token.Percent, Lexeme: "%", Line: line, Column: column}, nil
	case '&':
		l.advanceRune()
		if l.matchString("&") {
			return token.Token{}, fmt.Errorf("use `and` instead of `&&` at %d:%d", line, column)
		}
	case '|':
		l.advanceRune()
		if l.matchString("|") {
			return token.Token{}, fmt.Errorf("use `or` instead of `||` at %d:%d", line, column)
		}
	case '!':
		l.advanceRune()
		if l.matchString("=") {
			return token.Token{Kind: token.BangEqual, Lexeme: "!=", Line: line, Column: column}, nil
		}
		return token.Token{Kind: token.Bang, Lexeme: "!", Line: line, Column: column}, nil
	}
	return token.Token{}, fmt.Errorf("invalid token at %d:%d: %q", line, column, string(r))
}

// scanForeignBlock owns raw target text. It recognizes HLSL strings and both comment
// styles so braces in them never affect the SDSL-V boundary.
func (l *lexer) scanForeignBlock(line, column int) (string, error) {
	start, depth := l.offset, 1
	for !l.atEnd() {
		if l.matchString("//") {
			for !l.atEnd() {
				r, _ := l.peekRune()
				if r == '\n' {
					break
				}
				l.advanceRune()
			}
			continue
		}
		if l.matchString("/*") {
			for !l.atEnd() && !l.matchString("*/") {
				l.advanceRune()
			}
			continue
		}
		r, _ := l.peekRune()
		if r == '"' {
			l.advanceRune()
			for !l.atEnd() {
				q, _ := l.peekRune()
				l.advanceRune()
				if q == '\\' && !l.atEnd() {
					l.advanceRune()
					continue
				}
				if q == '"' {
					break
				}
			}
			continue
		}
		if r == '{' {
			depth++
			l.advanceRune()
			continue
		}
		if r == '}' {
			depth--
			if depth == 0 {
				raw := l.source[start:l.offset]
				l.advanceRune()
				return raw, nil
			}
			l.advanceRune()
			continue
		}
		l.advanceRune()
	}
	return "", fmt.Errorf("unterminated inline HLSL block at %d:%d", line, column)
}

func (l *lexer) scanIdentifier() string {
	start := l.offset
	for !l.atEnd() {
		r, _ := l.peekRune()
		if !isIdentifierPart(r) {
			break
		}
		l.advanceRune()
	}
	return l.source[start:l.offset]
}

func (l *lexer) scanNumber() (token.Kind, string, error) {
	start := l.offset
	for !l.atEnd() {
		r, _ := l.peekRune()
		if !unicode.IsDigit(r) {
			break
		}
		l.advanceRune()
	}
	kind := token.IntLiteral
	if l.matchString(".") {
		r, ok := l.peekRune()
		if ok && unicode.IsDigit(r) {
			kind = token.FloatLiteral
			for !l.atEnd() {
				r, _ := l.peekRune()
				if !unicode.IsDigit(r) {
					break
				}
				l.advanceRune()
			}
		} else {
			l.offset--
			l.column--
		}
	}
	if r, ok := l.peekRune(); ok && (r == 'u' || r == 'U') {
		l.advanceRune()
	}
	return kind, l.source[start:l.offset], nil
}

func (l *lexer) scanString() (string, error) {
	line, column := l.line, l.column
	l.advanceRune()
	var b strings.Builder
	for !l.atEnd() {
		r, _ := l.peekRune()
		if r == '"' {
			l.advanceRune()
			return b.String(), nil
		}
		if r == '\n' {
			return "", fmt.Errorf("unterminated string literal at %d:%d", line, column)
		}
		b.WriteRune(r)
		l.advanceRune()
	}
	return "", fmt.Errorf("unterminated string literal at %d:%d", line, column)
}

func (l *lexer) atEnd() bool { return l.offset >= len(l.source) }

func (l *lexer) position() source.Position {
	return source.Position{Offset: uint32(l.offset), Line: uint32(l.line), Column: uint32(l.column)}
}

func (l *lexer) peekRune() (rune, bool) {
	if l.atEnd() {
		return 0, false
	}
	r, _ := utf8.DecodeRuneInString(l.source[l.offset:])
	return r, true
}

func (l *lexer) advanceRune() {
	r, size := utf8.DecodeRuneInString(l.source[l.offset:])
	l.offset += size
	if r == '\n' {
		l.line++
		l.column = 1
		return
	}
	l.column++
}

func (l *lexer) matchString(expected string) bool {
	if len(l.source[l.offset:]) < len(expected) || l.source[l.offset:l.offset+len(expected)] != expected {
		return false
	}
	for _, r := range expected {
		_ = r
		l.advanceRune()
	}
	return true
}

func isIdentifierStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isIdentifierPart(r rune) bool  { return isIdentifierStart(r) || unicode.IsDigit(r) }
