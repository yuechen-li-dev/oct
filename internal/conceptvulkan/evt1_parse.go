package conceptvulkan

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type evt1Parser struct {
	path   string
	tokens []Token
	pos    int
}

func ParseEVT1(path, text string) (EVT1Module, error) {
	tokens, err := lexEVT1(text)
	if err != nil {
		return EVT1Module{}, err
	}
	p := &evt1Parser{path: filepath.ToSlash(path), tokens: tokens}
	module, err := p.parseModule()
	if err != nil {
		return EVT1Module{}, err
	}
	if err := validateEVT1Module(module); err != nil {
		return EVT1Module{}, err
	}
	return module, nil
}

func lexEVT1(text string) ([]Token, error) {
	var tokens []Token
	line, column := 1, 1
	for i := 0; i < len(text); {
		c := text[i]
		if c == '\n' {
			line++
			column = 1
			i++
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' {
			column++
			i++
			continue
		}
		if c == '/' && i+1 < len(text) && text[i+1] == '/' {
			for i < len(text) && text[i] != '\n' {
				i++
				column++
			}
			continue
		}
		start := Span{Line: line, Column: column}
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_':
			j := i + 1
			for j < len(text) {
				d := text[j]
				if (d >= 'a' && d <= 'z') || (d >= 'A' && d <= 'Z') || (d >= '0' && d <= '9') || d == '_' {
					j++
					continue
				}
				break
			}
			tokens = append(tokens, Token{Lexeme: text[i:j], Span: start})
			column += j - i
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(text) && text[j] >= '0' && text[j] <= '9' {
				j++
			}
			tokens = append(tokens, Token{Lexeme: text[i:j], Span: start})
			column += j - i
			i = j
		case c == '"' && i+1 < len(text):
			j := i + 1
			for j < len(text) && text[j] != '"' {
				if text[j] == '\n' {
					return nil, evt1Diagnostic("CV4000", "unterminated string literal", start)
				}
				j++
			}
			if j >= len(text) {
				return nil, evt1Diagnostic("CV4000", "unterminated string literal", start)
			}
			j++
			tokens = append(tokens, Token{Lexeme: text[i:j], Span: start})
			column += j - i
			i = j
		case i+1 < len(text) && text[i:i+2] == "::":
			tokens = append(tokens, Token{Lexeme: "::", Span: start})
			i += 2
			column += 2
		case i+1 < len(text) && text[i:i+2] == "=>":
			tokens = append(tokens, Token{Lexeme: "=>", Span: start})
			i += 2
			column += 2
		case strings.ContainsRune("(){};,.*+.", rune(c)):
			tokens = append(tokens, Token{Lexeme: string(c), Span: start})
			i++
			column++
		default:
			return nil, evt1Diagnostic("CV4000", fmt.Sprintf("invalid token %q", c), start)
		}
	}
	return tokens, nil
}

func (p *evt1Parser) parseModule() (EVT1Module, error) {
	module := EVT1Module{Path: p.path}
	if err := p.expectKeyword("profile"); err != nil {
		return module, err
	}
	if _, err := p.expect("Vulkan"); err != nil {
		return module, evt1Diagnostic("CV4001", "expected `profile Vulkan;`", p.currentSpan())
	}
	if _, err := p.expect(";"); err != nil {
		return module, err
	}
	for p.peekLexeme() == "import" {
		p.next()
		var parts []string
		for {
			tok, err := p.expectIdentifier("CV4002", "expected import path")
			if err != nil {
				return module, err
			}
			parts = append(parts, tok.Lexeme)
			if p.peekLexeme() != "." {
				break
			}
			p.next()
		}
		if _, err := p.expect(";"); err != nil {
			return module, err
		}
		module.Imports = append(module.Imports, strings.Join(parts, "."))
	}
	for !p.done() {
		if p.peekLexeme() == "enum" {
			enumDecl, err := p.parseEnumDecl()
			if err != nil {
				return module, err
			}
			module.Enums = append(module.Enums, enumDecl)
			continue
		}
		fn, err := p.parseFunctionDecl()
		if err != nil {
			return module, err
		}
		module.Functions = append(module.Functions, fn)
	}
	return module, nil
}

func (p *evt1Parser) parseEnumDecl() (EVT1EnumDecl, error) {
	start := p.next().Span
	nameTok, err := p.expectIdentifier("CV4003", "expected enum name")
	if err != nil {
		return EVT1EnumDecl{}, err
	}
	if _, err := p.expect("{"); err != nil {
		return EVT1EnumDecl{}, err
	}
	enumDecl := EVT1EnumDecl{Name: nameTok.Lexeme, Span: start}
	for !p.done() && p.peekLexeme() != "}" {
		variantTok, err := p.expectIdentifier("CV4004", "expected enum variant name")
		if err != nil {
			return EVT1EnumDecl{}, err
		}
		var payload []EVT1Field
		if p.peekLexeme() == "(" {
			p.next()
			if p.peekLexeme() != ")" {
				for {
					fieldType, err := p.parseType()
					if err != nil {
						return EVT1EnumDecl{}, err
					}
					fieldName, err := p.expectIdentifier("CV4005", "expected payload name")
					if err != nil {
						return EVT1EnumDecl{}, err
					}
					payload = append(payload, EVT1Field{Type: fieldType, Name: fieldName.Lexeme, Span: fieldName.Span})
					if p.peekLexeme() != "," {
						break
					}
					p.next()
				}
			}
			if _, err := p.expect(")"); err != nil {
				return EVT1EnumDecl{}, err
			}
		}
		enumDecl.Variants = append(enumDecl.Variants, EVT1VariantDecl{Name: variantTok.Lexeme, Payload: payload, Tag: len(enumDecl.Variants), Span: variantTok.Span})
		if p.peekLexeme() == "," {
			p.next()
		}
	}
	if _, err := p.expect("}"); err != nil {
		return EVT1EnumDecl{}, err
	}
	return enumDecl, nil
}

func (p *evt1Parser) parseFunctionDecl() (EVT1FunctionDecl, error) {
	retType, err := p.parseType()
	if err != nil {
		return EVT1FunctionDecl{}, err
	}
	nameTok, err := p.expectIdentifier("CV4006", "expected function name")
	if err != nil {
		return EVT1FunctionDecl{}, err
	}
	if _, err := p.expect("("); err != nil {
		return EVT1FunctionDecl{}, err
	}
	fn := EVT1FunctionDecl{Name: nameTok.Lexeme, ReturnType: retType, Span: nameTok.Span}
	if p.peekLexeme() != ")" {
		for {
			paramType, err := p.parseType()
			if err != nil {
				return EVT1FunctionDecl{}, err
			}
			paramName, err := p.expectIdentifier("CV4007", "expected parameter name")
			if err != nil {
				return EVT1FunctionDecl{}, err
			}
			fn.Params = append(fn.Params, EVT1Param{Type: paramType, Name: paramName.Lexeme, Span: paramName.Span})
			if p.peekLexeme() != "," {
				break
			}
			p.next()
		}
	}
	if _, err := p.expect(")"); err != nil {
		return EVT1FunctionDecl{}, err
	}
	if p.peekLexeme() == ";" {
		p.next()
		return fn, nil
	}
	block, err := p.parseBlock()
	if err != nil {
		return EVT1FunctionDecl{}, err
	}
	fn.Body = &block
	return fn, nil
}

func (p *evt1Parser) parseType() (EVT1Type, error) {
	tok := p.current()
	qualifier := ""
	if tok.Lexeme == "owned" || tok.Lexeme == "borrow" || tok.Lexeme == "unsafe" || tok.Lexeme == "imported" {
		qualifier = tok.Lexeme
		p.next()
		tok = p.current()
	}
	nameTok, err := p.expectIdentifier("CV4008", "expected type name")
	if err != nil {
		return EVT1Type{}, err
	}
	t, ok := evt1BuiltinType(nameTok.Lexeme, nameTok.Span)
	if !ok {
		t = EVT1Type{Name: nameTok.Lexeme, Kind: EVT1TypeEnum, Span: nameTok.Span}
	}
	t.Qualifier = qualifier
	if p.peekLexeme() == "*" {
		p.next()
		pointee := t
		t = EVT1Type{Name: pointee.Name + "*", Kind: EVT1TypePointer, PointerTo: &pointee, Span: nameTok.Span}
	}
	return t, nil
}

func (p *evt1Parser) parseBlock() (EVT1Block, error) {
	open, err := p.expect("{")
	if err != nil {
		return EVT1Block{}, err
	}
	block := EVT1Block{Span: open.Span}
	for !p.done() && p.peekLexeme() != "}" {
		stmt, err := p.parseStatement()
		if err != nil {
			return EVT1Block{}, err
		}
		block.Statements = append(block.Statements, stmt)
	}
	if _, err := p.expect("}"); err != nil {
		return EVT1Block{}, err
	}
	return block, nil
}

func (p *evt1Parser) parseStatement() (EVT1Statement, error) {
	switch p.peekLexeme() {
	case "return":
		start := p.next().Span
		if p.peekLexeme() == ";" {
			p.next()
			return &EVT1ReturnStmt{Span: start}, nil
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(";"); err != nil {
			return nil, err
		}
		return &EVT1ReturnStmt{Value: value, Span: start}, nil
	case "match":
		return p.parseMatchStmt()
	case "{":
		block, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return &block, nil
	default:
		if p.looksLikeVarDecl() {
			return p.parseVarDecl()
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(";"); err != nil {
			return nil, err
		}
		return &EVT1ExprStmt{Value: value, Span: value.exprSpan()}, nil
	}
}

func (p *evt1Parser) looksLikeVarDecl() bool {
	if p.done() {
		return false
	}
	lex := p.peekLexeme()
	switch lex {
	case "int", "void", "PipelineLayout", "Pipeline", "VulkanError", "owned", "borrow", "unsafe", "imported":
		return true
	default:
	}
	if p.pos+1 < len(p.tokens) {
		next := p.tokens[p.pos+1].Lexeme
		return isIdentifier(lex) && isIdentifier(next)
	}
	return false
}

func (p *evt1Parser) parseVarDecl() (EVT1Statement, error) {
	t, err := p.parseType()
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expectIdentifier("CV4009", "expected local name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect("="); err != nil {
		return nil, err
	}
	value, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(";"); err != nil {
		return nil, err
	}
	return &EVT1VarDecl{Type: t, Name: nameTok.Lexeme, Value: value, Span: nameTok.Span}, nil
}

func (p *evt1Parser) parseMatchStmt() (EVT1Statement, error) {
	start := p.next().Span
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	subject, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(")"); err != nil {
		return nil, err
	}
	if _, err := p.expect("{"); err != nil {
		return nil, err
	}
	stmt := &EVT1MatchStmt{Subject: subject, Span: start}
	for !p.done() && p.peekLexeme() != "}" {
		pattern, err := p.parsePattern()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect("=>"); err != nil {
			return nil, err
		}
		if p.peekLexeme() != "{" {
			return nil, evt1Diagnostic("CV4118", "statement-form match arms require braced blocks", p.currentSpan())
		}
		block, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		stmt.Arms = append(stmt.Arms, EVT1StatementArm{Pattern: pattern, Block: block, Span: pattern.Span})
		if p.peekLexeme() == "," {
			p.next()
		}
	}
	if _, err := p.expect("}"); err != nil {
		return nil, err
	}
	return stmt, nil
}

func (p *evt1Parser) parsePattern() (EVT1Pattern, error) {
	enumTok, err := p.expectIdentifier("CV4010", "expected enum name in match arm")
	if err != nil {
		return EVT1Pattern{}, err
	}
	if _, err := p.expect("::"); err != nil {
		return EVT1Pattern{}, err
	}
	variantTok, err := p.expectIdentifier("CV4011", "expected variant name in match arm")
	if err != nil {
		return EVT1Pattern{}, err
	}
	pattern := EVT1Pattern{EnumName: enumTok.Lexeme, VariantName: variantTok.Lexeme, Span: enumTok.Span}
	if p.peekLexeme() == "(" {
		p.next()
		if p.peekLexeme() != ")" {
			for {
				binding, err := p.expectIdentifier("CV4012", "expected payload binding name")
				if err != nil {
					return EVT1Pattern{}, err
				}
				pattern.Bindings = append(pattern.Bindings, binding.Lexeme)
				if p.peekLexeme() != "," {
					break
				}
				p.next()
			}
		}
		if _, err := p.expect(")"); err != nil {
			return EVT1Pattern{}, err
		}
	}
	return pattern, nil
}

func (p *evt1Parser) parseExpr() (EVT1Expr, error) {
	return p.parseAdditive()
}

func (p *evt1Parser) parseAdditive() (EVT1Expr, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.peekLexeme() == "+" {
		op := p.next()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		left = &EVT1BinaryExpr{Op: op.Lexeme, Left: left, Right: right, Span: op.Span}
	}
	return left, nil
}

func (p *evt1Parser) parsePrimary() (EVT1Expr, error) {
	switch {
	case p.done():
		return nil, evt1Diagnostic("CV4013", "unexpected end of expression", p.currentSpan())
	case p.peekLexeme() == "(":
		p.next()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(")"); err != nil {
			return nil, err
		}
		return expr, nil
	case p.peekLexeme() == "match":
		return p.parseMatchExpr()
	case isNumber(p.peekLexeme()):
		tok := p.next()
		value, _ := strconv.Atoi(tok.Lexeme)
		return &EVT1IntLiteral{Value: value, Span: tok.Span}, nil
	default:
		return p.parseNameLikeExpr()
	}
}

func (p *evt1Parser) parseMatchExpr() (EVT1Expr, error) {
	start := p.next().Span
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	subject, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(")"); err != nil {
		return nil, err
	}
	if _, err := p.expect("{"); err != nil {
		return nil, err
	}
	expr := &EVT1MatchExpr{Subject: subject, Span: start}
	for !p.done() && p.peekLexeme() != "}" {
		pattern, err := p.parsePattern()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect("=>"); err != nil {
			return nil, err
		}
		if p.peekLexeme() == "{" {
			return nil, evt1Diagnostic("CV4117", "expression-form match arms require a single expression, not a statement block", p.currentSpan())
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		expr.Arms = append(expr.Arms, EVT1ExprArm{Pattern: pattern, Value: value, Span: pattern.Span})
		if p.peekLexeme() == "," {
			p.next()
		} else if p.peekLexeme() != "}" {
			return nil, evt1Diagnostic("CV4014", "expression-form match arms must be comma-separated", p.currentSpan())
		}
	}
	if _, err := p.expect("}"); err != nil {
		return nil, err
	}
	return expr, nil
}

func (p *evt1Parser) parseNameLikeExpr() (EVT1Expr, error) {
	nameTok, err := p.expectIdentifier("CV4015", "expected expression")
	if err != nil {
		return nil, err
	}
	var expr EVT1Expr = &EVT1NameExpr{Name: nameTok.Lexeme, Span: nameTok.Span}
	if p.peekLexeme() == "::" {
		p.next()
		variantTok, err := p.expectIdentifier("CV4016", "expected variant name after ::")
		if err != nil {
			return nil, err
		}
		args := []EVT1Expr{}
		if p.peekLexeme() == "(" {
			p.next()
			if p.peekLexeme() != ")" {
				for {
					arg, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.peekLexeme() != "," {
						break
					}
					p.next()
				}
			}
			if _, err := p.expect(")"); err != nil {
				return nil, err
			}
		}
		expr = &EVT1ConstructExpr{EnumName: nameTok.Lexeme, VariantName: variantTok.Lexeme, Args: args, Span: nameTok.Span}
	}
	for {
		switch p.peekLexeme() {
		case "(":
			if _, ok := expr.(*EVT1NameExpr); !ok {
				return nil, evt1Diagnostic("CV4017", "only simple function calls are supported in EVT1 expressions", p.currentSpan())
			}
			p.next()
			var args []EVT1Expr
			if p.peekLexeme() != ")" {
				for {
					arg, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.peekLexeme() != "," {
						break
					}
					p.next()
				}
			}
			if _, err := p.expect(")"); err != nil {
				return nil, err
			}
			expr = &EVT1CallExpr{Callee: expr.(*EVT1NameExpr).Name, Args: args, Span: nameTok.Span}
		case ".":
			p.next()
			fieldTok, err := p.expectIdentifier("CV4018", "expected field name after .")
			if err != nil {
				return nil, err
			}
			expr = &EVT1FieldExpr{Receiver: expr, Field: fieldTok.Lexeme, Span: fieldTok.Span}
		default:
			return expr, nil
		}
	}
}

func (p *evt1Parser) expectKeyword(keyword string) error {
	if p.peekLexeme() != keyword {
		return evt1Diagnostic("CV4019", fmt.Sprintf("expected %s", keyword), p.currentSpan())
	}
	p.next()
	return nil
}

func (p *evt1Parser) expectIdentifier(code, message string) (Token, error) {
	tok := p.current()
	if !isIdentifier(tok.Lexeme) {
		return Token{}, evt1Diagnostic(code, message, tok.Span)
	}
	p.next()
	return tok, nil
}

func (p *evt1Parser) expect(lexeme string) (Token, error) {
	tok := p.current()
	if tok.Lexeme != lexeme {
		return Token{}, evt1Diagnostic("CV4020", fmt.Sprintf("expected %q", lexeme), tok.Span)
	}
	p.next()
	return tok, nil
}

func (p *evt1Parser) done() bool {
	return p.pos >= len(p.tokens)
}

func (p *evt1Parser) current() Token {
	if p.done() {
		if len(p.tokens) == 0 {
			return Token{Span: Span{Line: 1, Column: 1}}
		}
		last := p.tokens[len(p.tokens)-1]
		return Token{Span: Span{Line: last.Span.Line, Column: last.Span.Column + len(last.Lexeme)}}
	}
	return p.tokens[p.pos]
}

func (p *evt1Parser) currentSpan() Span {
	return p.current().Span
}

func (p *evt1Parser) peekLexeme() string {
	if p.done() {
		return ""
	}
	return p.tokens[p.pos].Lexeme
}

func (p *evt1Parser) next() Token {
	tok := p.current()
	p.pos++
	return tok
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
		return false
	}
	for i := 1; i < len(s); i++ {
		c = s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
