package lex

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuechen-li-dev/oct/internal/source"
)

type TokenKind string

const (
	EOF TokenKind = "EOF"

	Identifier    TokenKind = "Identifier"
	IntLiteral    TokenKind = "IntLiteral"
	FloatLiteral  TokenKind = "FloatLiteral"
	StringLiteral TokenKind = "StringLiteral"

	KeywordFn         TokenKind = "KeywordFn"
	KeywordLet        TokenKind = "KeywordLet"
	KeywordVar        TokenKind = "KeywordVar"
	KeywordReturn     TokenKind = "KeywordReturn"
	KeywordFor        TokenKind = "KeywordFor"
	KeywordIn         TokenKind = "KeywordIn"
	KeywordAs         TokenKind = "KeywordAs"
	KeywordStep       TokenKind = "KeywordStep"
	KeywordTrue       TokenKind = "KeywordTrue"
	KeywordFalse      TokenKind = "KeywordFalse"
	KeywordMatch      TokenKind = "KeywordMatch"
	KeywordIf         TokenKind = "KeywordIf"
	KeywordElse       TokenKind = "KeywordElse"
	KeywordWhile      TokenKind = "KeywordWhile"
	KeywordFlow       TokenKind = "KeywordFlow"
	KeywordState      TokenKind = "KeywordState"
	KeywordGoto       TokenKind = "KeywordGoto"
	KeywordSuspend    TokenKind = "KeywordSuspend"
	KeywordRemember   TokenKind = "KeywordRemember"
	KeywordResume     TokenKind = "KeywordResume"
	KeywordWhen       TokenKind = "KeywordWhen"
	KeywordSwitch     TokenKind = "KeywordSwitch"
	KeywordBatch      TokenKind = "KeywordBatch"
	KeywordCase       TokenKind = "KeywordCase"
	KeywordAnd        TokenKind = "KeywordAnd"
	KeywordOr         TokenKind = "KeywordOr"
	KeywordNot        TokenKind = "KeywordNot"
	KeywordRecord     TokenKind = "KeywordRecord"
	KeywordEnum       TokenKind = "KeywordEnum"
	KeywordPackage    TokenKind = "KeywordPackage"
	KeywordImport     TokenKind = "KeywordImport"
	KeywordWith       TokenKind = "KeywordWith"
	KeywordPrometheus TokenKind = "KeywordPrometheus"

	LeftParen    TokenKind = "LeftParen"
	RightParen   TokenKind = "RightParen"
	LeftBrace    TokenKind = "LeftBrace"
	RightBrace   TokenKind = "RightBrace"
	LeftBracket  TokenKind = "LeftBracket"
	RightBracket TokenKind = "RightBracket"
	LeftAngle    TokenKind = "LeftAngle"
	RightAngle   TokenKind = "RightAngle"
	Comma        TokenKind = "Comma"
	Dot          TokenKind = "Dot"
	Colon        TokenKind = "Colon"
	Assign       TokenKind = "Assign"
	Arrow        TokenKind = "Arrow"
	DotDot       TokenKind = "DotDot"
	Question     TokenKind = "Question"
	Bang         TokenKind = "Bang"
	BangEqual    TokenKind = "BangEqual"
	Caret        TokenKind = "Caret"
	Plus         TokenKind = "Plus"
	Minus        TokenKind = "Minus"
	Star         TokenKind = "Star"
	Slash        TokenKind = "Slash"
	Percent      TokenKind = "Percent"
	At           TokenKind = "At"
	EqualEqual   TokenKind = "EqualEqual"
	LeftEqual    TokenKind = "LeftEqual"
	RightEqual   TokenKind = "RightEqual"
)

type Token struct {
	Kind   TokenKind
	Lexeme string
	Line   int
	Column int
}

type Result struct {
	Source source.File
	Tokens []Token
}

func Analyze(file source.File) (Result, error) {
	lexer := lexer{source: file.Text}
	tokens, err := lexer.lexAll()
	if err != nil {
		return Result{}, fmt.Errorf("lex %s: %w", file.Path, err)
	}

	return Result{Source: file, Tokens: tokens}, nil
}

type lexer struct {
	source string
	offset int
	line   int
	column int
}

func (l *lexer) lexAll() ([]Token, error) {
	l.line = 1
	l.column = 1

	var tokens []Token
	for {
		l.skipWhitespaceAndComments()
		if l.atEnd() {
			tokens = append(tokens, Token{Kind: EOF, Line: l.line, Column: l.column})
			return tokens, nil
		}

		token, err := l.nextToken()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
}

func (l *lexer) skipWhitespaceAndComments() {
	for {
		for {
			r, _ := l.peekRune()
			if !isWhitespace(r) {
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

func (l *lexer) nextToken() (Token, error) {
	line, column := l.line, l.column
	r, _ := l.peekRune()

	if isIdentifierStart(r) {
		lexeme := l.scanIdentifier()
		return Token{Kind: lookupKeyword(lexeme), Lexeme: lexeme, Line: line, Column: column}, nil
	}

	if unicode.IsDigit(r) {
		kind, lexeme, err := l.scanNumber()
		if err != nil {
			return Token{}, err
		}
		return Token{Kind: kind, Lexeme: lexeme, Line: line, Column: column}, nil
	}

	switch r {
	case '"':
		lexeme, err := l.scanString()
		if err != nil {
			return Token{}, err
		}
		return Token{Kind: StringLiteral, Lexeme: lexeme, Line: line, Column: column}, nil
	case '(':
		l.advanceRune()
		return Token{Kind: LeftParen, Lexeme: "(", Line: line, Column: column}, nil
	case ')':
		l.advanceRune()
		return Token{Kind: RightParen, Lexeme: ")", Line: line, Column: column}, nil
	case '{':
		l.advanceRune()
		return Token{Kind: LeftBrace, Lexeme: "{", Line: line, Column: column}, nil
	case '}':
		l.advanceRune()
		return Token{Kind: RightBrace, Lexeme: "}", Line: line, Column: column}, nil
	case '[':
		l.advanceRune()
		return Token{Kind: LeftBracket, Lexeme: "[", Line: line, Column: column}, nil
	case ']':
		l.advanceRune()
		return Token{Kind: RightBracket, Lexeme: "]", Line: line, Column: column}, nil
	case '<':
		l.advanceRune()
		if l.matchString("=") {
			return Token{Kind: LeftEqual, Lexeme: "<=", Line: line, Column: column}, nil
		}
		return Token{Kind: LeftAngle, Lexeme: "<", Line: line, Column: column}, nil
	case '>':
		l.advanceRune()
		if l.matchString("=") {
			return Token{Kind: RightEqual, Lexeme: ">=", Line: line, Column: column}, nil
		}
		return Token{Kind: RightAngle, Lexeme: ">", Line: line, Column: column}, nil
	case ',':
		l.advanceRune()
		return Token{Kind: Comma, Lexeme: ",", Line: line, Column: column}, nil
	case ':':
		l.advanceRune()
		return Token{Kind: Colon, Lexeme: ":", Line: line, Column: column}, nil
	case '.':
		l.advanceRune()
		if l.matchString(".") {
			return Token{Kind: DotDot, Lexeme: "..", Line: line, Column: column}, nil
		}
		return Token{Kind: Dot, Lexeme: ".", Line: line, Column: column}, nil
	case '=':
		l.advanceRune()
		if l.matchString(">") {
			return Token{Kind: Arrow, Lexeme: "=>", Line: line, Column: column}, nil
		}
		if l.matchString("=") {
			return Token{Kind: EqualEqual, Lexeme: "==", Line: line, Column: column}, nil
		}
		return Token{Kind: Assign, Lexeme: "=", Line: line, Column: column}, nil
	case '?':
		l.advanceRune()
		return Token{Kind: Question, Lexeme: "?", Line: line, Column: column}, nil
	case '!':
		l.advanceRune()
		if l.matchString("=") {
			return Token{Kind: BangEqual, Lexeme: "!=", Line: line, Column: column}, nil
		}
		return Token{Kind: Bang, Lexeme: "!", Line: line, Column: column}, nil
	case '^':
		l.advanceRune()
		return Token{Kind: Caret, Lexeme: "^", Line: line, Column: column}, nil
	case '+':
		l.advanceRune()
		return Token{Kind: Plus, Lexeme: "+", Line: line, Column: column}, nil
	case '*':
		l.advanceRune()
		return Token{Kind: Star, Lexeme: "*", Line: line, Column: column}, nil
	case '/':
		l.advanceRune()
		return Token{Kind: Slash, Lexeme: "/", Line: line, Column: column}, nil
	case '%':
		l.advanceRune()
		return Token{Kind: Percent, Lexeme: "%", Line: line, Column: column}, nil
	case '-':
		l.advanceRune()
		if l.matchString(">") {
			return Token{Kind: Arrow, Lexeme: "->", Line: line, Column: column}, nil
		}
		return Token{Kind: Minus, Lexeme: "-", Line: line, Column: column}, nil
	case '@':
		l.advanceRune()
		return Token{Kind: At, Lexeme: "@", Line: line, Column: column}, nil
	case ';':
		return Token{}, fmt.Errorf("invalid token at %d:%d: ';' (Oct does not use semicolons to separate statements; use one statement per line)", line, column)
	case '&':
		if l.matchString("&&") {
			return Token{}, fmt.Errorf("invalid token at %d:%d: '&&' is not an Oct operator; use 'and'", line, column)
		}
		return Token{}, fmt.Errorf("invalid token at %d:%d: %q", line, column, string(r))
	case '|':
		if l.matchString("||") {
			return Token{}, fmt.Errorf("invalid token at %d:%d: '||' is not an Oct operator; use 'or'", line, column)
		}
		return Token{}, fmt.Errorf("invalid token at %d:%d: %q", line, column, string(r))
	default:
		return Token{}, fmt.Errorf("invalid token at %d:%d: %q", line, column, string(r))
	}
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

func (l *lexer) scanNumber() (TokenKind, string, error) {
	start := l.offset
	for !l.atEnd() {
		r, _ := l.peekRune()
		if !unicode.IsDigit(r) {
			break
		}
		l.advanceRune()
	}

	kind := IntLiteral
	if l.matchString(".") {
		r, ok := l.peekRune()
		if ok && unicode.IsDigit(r) {
			kind = FloatLiteral
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

	r, ok := l.peekRune()
	if ok && (r == 'e' || r == 'E') {
		kind = FloatLiteral
		l.advanceRune()
		sign, signOK := l.peekRune()
		if signOK && (sign == '+' || sign == '-') {
			l.advanceRune()
		}
		expStart := l.offset
		for !l.atEnd() {
			expDigit, _ := l.peekRune()
			if !unicode.IsDigit(expDigit) {
				break
			}
			l.advanceRune()
		}
		if l.offset == expStart {
			return "", "", fmt.Errorf("invalid float literal at %d:%d: %q", l.line, l.column, l.source[start:l.offset])
		}
	}

	return kind, l.source[start:l.offset], nil
}

func (l *lexer) scanString() (string, error) {
	line, column := l.line, l.column
	l.advanceRune()

	var builder strings.Builder
	for !l.atEnd() {
		r, _ := l.peekRune()
		if r == '"' {
			l.advanceRune()
			return builder.String(), nil
		}
		if r == '\n' {
			return "", fmt.Errorf("unterminated string literal at %d:%d", line, column)
		}
		if r == '\\' {
			l.advanceRune()
			if l.atEnd() {
				return "", fmt.Errorf("unterminated string literal at %d:%d", line, column)
			}
			escaped, _ := l.peekRune()
			switch escaped {
			case 'n':
				builder.WriteRune('\n')
			case '"':
				builder.WriteRune('"')
			case '\\':
				builder.WriteRune('\\')
			default:
				return "", fmt.Errorf("unsupported string escape \\%c at %d:%d", escaped, l.line, l.column)
			}
			l.advanceRune()
			continue
		}
		builder.WriteRune(r)
		l.advanceRune()
	}

	return "", fmt.Errorf("unterminated string literal at %d:%d", line, column)
}

func (l *lexer) atEnd() bool {
	return l.offset >= len(l.source)
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
	for range expected {
		l.advanceRune()
	}
	return true
}

func lookupKeyword(lexeme string) TokenKind {
	switch lexeme {
	case "fn":
		return KeywordFn
	case "let":
		return KeywordLet
	case "var":
		return KeywordVar
	case "return":
		return KeywordReturn
	case "for":
		return KeywordFor
	case "in":
		return KeywordIn
	case "as":
		return KeywordAs
	case "step":
		return KeywordStep
	case "true":
		return KeywordTrue
	case "false":
		return KeywordFalse
	case "match":
		return KeywordMatch
	case "if":
		return KeywordIf
	case "else":
		return KeywordElse
	case "while":
		return KeywordWhile
	case "flow":
		return KeywordFlow
	case "state":
		return KeywordState
	case "goto":
		return KeywordGoto
	case "suspend":
		return KeywordSuspend
	case "remember":
		return KeywordRemember
	case "resume":
		return KeywordResume
	case "when":
		return KeywordWhen
	case "switch":
		return KeywordSwitch
	case "batch":
		return KeywordBatch
	case "case":
		return KeywordCase
	case "and":
		return KeywordAnd
	case "or":
		return KeywordOr
	case "not":
		return KeywordNot
	case "record":
		return KeywordRecord
	case "enum":
		return KeywordEnum
	case "package":
		return KeywordPackage
	case "import":
		return KeywordImport
	case "with":
		return KeywordWith
	case "PROMETHEUS":
		return KeywordPrometheus
	default:
		return Identifier
	}
}

func isWhitespace(r rune) bool {
	return unicode.IsSpace(r)
}

func isIdentifierStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentifierPart(r rune) bool {
	return isIdentifierStart(r) || unicode.IsDigit(r)
}
