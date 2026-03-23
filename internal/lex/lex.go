package lex

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"oct/internal/source"
)

type TokenKind string

const (
	EOF TokenKind = "EOF"

	Identifier   TokenKind = "Identifier"
	IntLiteral   TokenKind = "IntLiteral"
	FloatLiteral TokenKind = "FloatLiteral"

	KeywordFn     TokenKind = "KeywordFn"
	KeywordLet    TokenKind = "KeywordLet"
	KeywordReturn TokenKind = "KeywordReturn"
	KeywordTrue   TokenKind = "KeywordTrue"
	KeywordFalse  TokenKind = "KeywordFalse"

	LeftParen  TokenKind = "LeftParen"
	RightParen TokenKind = "RightParen"
	LeftBrace  TokenKind = "LeftBrace"
	RightBrace TokenKind = "RightBrace"
	Comma      TokenKind = "Comma"
	Colon      TokenKind = "Colon"
	Assign     TokenKind = "Assign"
	Arrow      TokenKind = "Arrow"
	Plus       TokenKind = "Plus"
	Minus      TokenKind = "Minus"
	Star       TokenKind = "Star"
	Slash      TokenKind = "Slash"
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
		kind, lexeme := l.scanNumber()
		return Token{Kind: kind, Lexeme: lexeme, Line: line, Column: column}, nil
	}

	switch r {
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
	case ',':
		l.advanceRune()
		return Token{Kind: Comma, Lexeme: ",", Line: line, Column: column}, nil
	case ':':
		l.advanceRune()
		return Token{Kind: Colon, Lexeme: ":", Line: line, Column: column}, nil
	case '=':
		l.advanceRune()
		return Token{Kind: Assign, Lexeme: "=", Line: line, Column: column}, nil
	case '+':
		l.advanceRune()
		return Token{Kind: Plus, Lexeme: "+", Line: line, Column: column}, nil
	case '*':
		l.advanceRune()
		return Token{Kind: Star, Lexeme: "*", Line: line, Column: column}, nil
	case '/':
		l.advanceRune()
		return Token{Kind: Slash, Lexeme: "/", Line: line, Column: column}, nil
	case '-':
		l.advanceRune()
		if l.matchString(">") {
			return Token{Kind: Arrow, Lexeme: "->", Line: line, Column: column}, nil
		}
		return Token{Kind: Minus, Lexeme: "-", Line: line, Column: column}, nil
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

func (l *lexer) scanNumber() (TokenKind, string) {
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

	return kind, l.source[start:l.offset]
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
	case "return":
		return KeywordReturn
	case "true":
		return KeywordTrue
	case "false":
		return KeywordFalse
	default:
		return Identifier
	}
}

func isIdentifierStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentifierPart(r rune) bool {
	return isIdentifierStart(r) || unicode.IsDigit(r)
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
