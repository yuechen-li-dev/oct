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
		case i+1 < len(text) && text[i:i+2] == "!=":
			tokens = append(tokens, Token{Lexeme: "!=", Span: start})
			i += 2
			column += 2
		case i+1 < len(text) && text[i:i+2] == "==":
			tokens = append(tokens, Token{Lexeme: "==", Span: start})
			i += 2
			column += 2
		case i+1 < len(text) && text[i:i+2] == "<=":
			tokens = append(tokens, Token{Lexeme: "<=", Span: start})
			i += 2
			column += 2
		case i+1 < len(text) && text[i:i+2] == ">=":
			tokens = append(tokens, Token{Lexeme: ">=", Span: start})
			i += 2
			column += 2
		case i+1 < len(text) && text[i:i+2] == "=>":
			tokens = append(tokens, Token{Lexeme: "=>", Span: start})
			i += 2
			column += 2
		case strings.ContainsRune("(){};,.*+-=<>!", rune(c)):
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
		switch p.peekLexeme() {
		case "comptime":
			isFunction, err := p.looksLikeComptimeFunction()
			if err != nil {
				return module, err
			}
			if isFunction {
				fn, err := p.parseFunctionDecl("", true)
				if err != nil {
					return module, err
				}
				module.ComptimeFns = append(module.ComptimeFns, fn)
				continue
			}
			decl, err := p.parseComptimeDecl()
			if err != nil {
				return module, err
			}
			module.ComptimeDecls = append(module.ComptimeDecls, decl)
		case "static_assert":
			assertion, err := p.parseStaticAssert()
			if err != nil {
				return module, err
			}
			module.StaticAsserts = append(module.StaticAsserts, assertion)
		case "template":
			templateDecl, err := p.parseTemplateDecl()
			if err != nil {
				return module, err
			}
			module.Templates = append(module.Templates, templateDecl)
		case "immovable":
			structDecl, err := p.parseStructDecl(true)
			if err != nil {
				return module, err
			}
			module.Structs = append(module.Structs, structDecl)
		case "struct":
			structDecl, err := p.parseStructDecl(false)
			if err != nil {
				return module, err
			}
			module.Structs = append(module.Structs, structDecl)
		case "enum":
			enumDecl, err := p.parseEnumDecl()
			if err != nil {
				return module, err
			}
			module.Enums = append(module.Enums, enumDecl)
		case "concept":
			conceptDecl, err := p.parseConceptDecl()
			if err != nil {
				return module, err
			}
			module.Concepts = append(module.Concepts, conceptDecl)
		case "requires":
			assertion, err := p.parseConceptAssertion()
			if err != nil {
				return module, err
			}
			module.Assertions = append(module.Assertions, assertion)
		default:
			fn, err := p.parseFunctionDecl("", false)
			if err != nil {
				return module, err
			}
			module.Functions = append(module.Functions, fn)
		}
	}
	return module, nil
}

func (p *evt1Parser) parseTemplateDecl() (EVT1TemplateDecl, error) {
	start, err := p.expect("template")
	if err != nil {
		return EVT1TemplateDecl{}, err
	}
	if _, err := p.expect("<"); err != nil {
		return EVT1TemplateDecl{}, err
	}
	if _, err := p.expect("typename"); err != nil {
		return EVT1TemplateDecl{}, evt1Diagnostic("CV4165", "template declarations require exactly `template <typename T>`", p.currentSpan())
	}
	paramTok, err := p.expectIdentifier("CV4165", "expected one template type parameter")
	if err != nil {
		return EVT1TemplateDecl{}, err
	}
	if _, err := p.expect(">"); err != nil {
		return EVT1TemplateDecl{}, err
	}
	reqTok, err := p.expect("requires")
	if err != nil {
		return EVT1TemplateDecl{}, evt1Diagnostic("CV4166", "template declarations require exactly one named concept constraint", p.currentSpan())
	}
	ref, err := p.parseConceptUse(paramTok.Lexeme)
	if err != nil {
		return EVT1TemplateDecl{}, err
	}
	fn, err := p.parseFunctionDecl(paramTok.Lexeme, false)
	if err != nil {
		return EVT1TemplateDecl{}, err
	}
	if fn.Body == nil {
		return EVT1TemplateDecl{}, evt1Diagnostic("CV4167", "template declarations require a function body", fn.Span)
	}
	return EVT1TemplateDecl{
		Name:          fn.Name,
		TypeParam:     paramTok.Lexeme,
		TypeParamSpan: paramTok.Span,
		Constraint: EVT1TemplateConstraint{
			ConceptName: ref.Name,
			TypeArg:     ref.TypeArgs[0],
			Span:        reqTok.Span,
		},
		ReturnType: fn.ReturnType,
		Params:     fn.Params,
		Body:       fn.Body,
		Span:       start.Span,
	}, nil
}

func (p *evt1Parser) looksLikeComptimeFunction() (bool, error) {
	if p.peekLexeme() != "comptime" {
		return false, nil
	}
	save := p.pos
	defer func() { p.pos = save }()
	p.next()
	if _, err := p.parseType(""); err != nil {
		return false, err
	}
	if !isIdentifier(p.peekLexeme()) {
		return false, evt1Diagnostic("CV4183", "expected comptime declaration name", p.currentSpan())
	}
	p.next()
	return p.peekLexeme() == "(", nil
}

func (p *evt1Parser) parseComptimeDecl() (EVT1ComptimeDecl, error) {
	start, err := p.expect("comptime")
	if err != nil {
		return EVT1ComptimeDecl{}, err
	}
	t, err := p.parseType("")
	if err != nil {
		return EVT1ComptimeDecl{}, err
	}
	nameTok, err := p.expectIdentifier("CV4183", "expected comptime declaration name")
	if err != nil {
		return EVT1ComptimeDecl{}, err
	}
	if _, err := p.expect("="); err != nil {
		return EVT1ComptimeDecl{}, err
	}
	value, err := p.parseExpr()
	if err != nil {
		return EVT1ComptimeDecl{}, err
	}
	if _, err := p.expect(";"); err != nil {
		return EVT1ComptimeDecl{}, err
	}
	return EVT1ComptimeDecl{Type: t, Name: nameTok.Lexeme, Value: value, Span: start.Span}, nil
}

func (p *evt1Parser) parseStaticAssert() (EVT1StaticAssert, error) {
	start, err := p.expect("static_assert")
	if err != nil {
		return EVT1StaticAssert{}, err
	}
	if _, err := p.expect("("); err != nil {
		return EVT1StaticAssert{}, err
	}
	condition, err := p.parseExpr()
	if err != nil {
		return EVT1StaticAssert{}, err
	}
	assertion := EVT1StaticAssert{Condition: condition, Span: start.Span}
	if p.peekLexeme() == "," {
		p.next()
		message, err := p.parseExpr()
		if err != nil {
			return EVT1StaticAssert{}, err
		}
		assertion.Message = message
	}
	if _, err := p.expect(")"); err != nil {
		return EVT1StaticAssert{}, err
	}
	if _, err := p.expect(";"); err != nil {
		return EVT1StaticAssert{}, err
	}
	return assertion, nil
}

func (p *evt1Parser) parseStructDecl(immovable bool) (EVT1StructDecl, error) {
	start := p.currentSpan()
	if immovable {
		p.next()
	}
	if _, err := p.expect("struct"); err != nil {
		return EVT1StructDecl{}, err
	}
	nameTok, err := p.expectIdentifier("CV4120", "expected struct name")
	if err != nil {
		return EVT1StructDecl{}, err
	}
	if _, err := p.expect("{"); err != nil {
		return EVT1StructDecl{}, err
	}
	decl := EVT1StructDecl{Name: nameTok.Lexeme, Immovable: immovable, Span: start}
	for !p.done() && p.peekLexeme() != "}" {
		fieldType, err := p.parseType("")
		if err != nil {
			return EVT1StructDecl{}, err
		}
		fieldName, err := p.expectIdentifier("CV4121", "expected field name")
		if err != nil {
			return EVT1StructDecl{}, err
		}
		if _, err := p.expect(";"); err != nil {
			return EVT1StructDecl{}, err
		}
		decl.Fields = append(decl.Fields, EVT1Field{Type: fieldType, Name: fieldName.Lexeme, Span: fieldName.Span})
	}
	if _, err := p.expect("}"); err != nil {
		return EVT1StructDecl{}, err
	}
	if _, err := p.expect(";"); err != nil {
		return EVT1StructDecl{}, err
	}
	return decl, nil
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
					fieldType, err := p.parseType("")
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

func (p *evt1Parser) parseConceptDecl() (EVT1ConceptDecl, error) {
	start := p.next().Span
	nameTok, err := p.expectIdentifier("CV4140", "expected concept name")
	if err != nil {
		return EVT1ConceptDecl{}, err
	}
	if _, err := p.expect("<"); err != nil {
		return EVT1ConceptDecl{}, err
	}
	paramTok, err := p.expectIdentifier("CV4141", "expected one concept type parameter")
	if err != nil {
		return EVT1ConceptDecl{}, err
	}
	if _, err := p.expect(">"); err != nil {
		return EVT1ConceptDecl{}, err
	}
	if _, err := p.expect("{"); err != nil {
		return EVT1ConceptDecl{}, err
	}
	decl := EVT1ConceptDecl{Name: nameTok.Lexeme, TypeParam: paramTok.Lexeme, Span: start}
	for !p.done() && p.peekLexeme() != "}" {
		req, err := p.parseConceptRequirement(paramTok.Lexeme)
		if err != nil {
			return EVT1ConceptDecl{}, err
		}
		decl.Requirements = append(decl.Requirements, req)
	}
	if _, err := p.expect("}"); err != nil {
		return EVT1ConceptDecl{}, err
	}
	return decl, nil
}

func (p *evt1Parser) parseConceptRequirement(typeParam string) (EVT1ConceptRequirement, error) {
	start, err := p.expect("requires")
	if err != nil {
		return nil, err
	}
	if p.peekLexeme() == "" {
		return nil, evt1Diagnostic("CV4142", "expected concept requirement", start.Span)
	}
	if p.isConceptApplicationAhead(typeParam) {
		ref, err := p.parseConceptUse(typeParam)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(";"); err != nil {
			return nil, err
		}
		return &EVT1PrerequisiteRequirement{ConceptName: ref.Name, TypeArg: ref.TypeArgs[0], Span: start.Span}, nil
	}
	retType, err := p.parseType(typeParam)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expectIdentifier("CV4143", "expected required operation name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	req := &EVT1OperationRequirement{ReturnType: retType, Name: nameTok.Lexeme, Span: start.Span}
	if p.peekLexeme() != ")" {
		for {
			paramType, err := p.parseType(typeParam)
			if err != nil {
				return nil, err
			}
			paramName, err := p.expectIdentifier("CV4144", "expected required parameter name")
			if err != nil {
				return nil, err
			}
			req.Params = append(req.Params, EVT1Param{Type: paramType, Name: paramName.Lexeme, Span: paramName.Span})
			if p.peekLexeme() != "," {
				break
			}
			p.next()
		}
	}
	if _, err := p.expect(")"); err != nil {
		return nil, err
	}
	if _, err := p.expect(";"); err != nil {
		return nil, err
	}
	return req, nil
}

func (p *evt1Parser) parseConceptAssertion() (EVT1ConceptAssertion, error) {
	start, err := p.expect("requires")
	if err != nil {
		return EVT1ConceptAssertion{}, err
	}
	ref, err := p.parseConceptUse("")
	if err != nil {
		return EVT1ConceptAssertion{}, err
	}
	if _, err := p.expect(";"); err != nil {
		return EVT1ConceptAssertion{}, err
	}
	return EVT1ConceptAssertion{ConceptName: ref.Name, ConcreteType: ref.TypeArgs[0], Span: start.Span}, nil
}

func (p *evt1Parser) parseFunctionDecl(conceptParam string, comptime bool) (EVT1FunctionDecl, error) {
	if comptime {
		if _, err := p.expect("comptime"); err != nil {
			return EVT1FunctionDecl{}, err
		}
	}
	retType, err := p.parseType(conceptParam)
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
	fn := EVT1FunctionDecl{Comptime: comptime, Name: nameTok.Lexeme, ReturnType: retType, Span: nameTok.Span}
	if p.peekLexeme() != ")" {
		for {
			paramType, err := p.parseType(conceptParam)
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

func (p *evt1Parser) parseType(conceptParam string) (EVT1Type, error) {
	start := p.currentSpan()
	t := EVT1Type{Span: start}
	for {
		switch p.peekLexeme() {
		case "unsafe":
			t.Unsafe = true
			p.next()
		case "imported":
			t.Imported = true
			p.next()
		case "owned":
			t.Ownership = "owned"
			p.next()
		case "borrow":
			t.Ownership = "borrow"
			p.next()
		case "const":
			t.Const = true
			p.next()
		default:
			goto done
		}
	}
done:
	nameTok, err := p.expectIdentifier("CV4008", "expected type name")
	if err != nil {
		return EVT1Type{}, err
	}
	if builtin, ok := evt1BuiltinType(nameTok.Lexeme, nameTok.Span); ok {
		t.Name = builtin.Name
		t.Kind = builtin.Kind
	} else if conceptParam != "" && nameTok.Lexeme == conceptParam {
		t.Name = nameTok.Lexeme
		t.Kind = EVT1TypeConceptParam
	} else {
		t.Name = nameTok.Lexeme
		t.Kind = EVT1TypeStruct
	}
	if p.peekLexeme() == "<" {
		p.next()
		for {
			arg, err := p.parseType(conceptParam)
			if err != nil {
				return EVT1Type{}, err
			}
			t.TypeArgs = append(t.TypeArgs, arg)
			if p.peekLexeme() != "," {
				break
			}
			p.next()
		}
		if _, err := p.expect(">"); err != nil {
			return EVT1Type{}, err
		}
		t.Kind = EVT1TypeApplied
	}
	if p.peekLexeme() == "*" {
		p.next()
		pointee := t
		t = EVT1Type{
			Name:      pointee.Name + "*",
			Kind:      EVT1TypePointer,
			Ownership: pointee.Ownership,
			Const:     pointee.Const,
			Imported:  pointee.Imported,
			Unsafe:    pointee.Unsafe,
			PointerTo: &pointee,
			Span:      nameTok.Span,
		}
	}
	return t, nil
}

func (p *evt1Parser) parseConceptUse(conceptParam string) (EVT1Type, error) {
	nameTok, err := p.expectIdentifier("CV4145", "expected concept name")
	if err != nil {
		return EVT1Type{}, err
	}
	if _, err := p.expect("<"); err != nil {
		return EVT1Type{}, err
	}
	arg, err := p.parseType(conceptParam)
	if err != nil {
		return EVT1Type{}, err
	}
	if _, err := p.expect(">"); err != nil {
		return EVT1Type{}, err
	}
	return EVT1Type{Name: nameTok.Lexeme, Kind: EVT1TypeApplied, TypeArgs: []EVT1Type{arg}, Span: nameTok.Span}, nil
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
	case "comptime":
		return p.parseLocalComptimeDecl()
	case "static_assert":
		assertion, err := p.parseStaticAssert()
		if err != nil {
			return nil, err
		}
		return &EVT1StaticAssertStmt{Condition: assertion.Condition, Message: assertion.Message, Span: assertion.Span}, nil
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
	case "while":
		return p.parseWhileStmt()
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
		if p.peekLexeme() == "=" {
			p.next()
			rhs, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(";"); err != nil {
				return nil, err
			}
			return &EVT1AssignStmt{Target: value, Value: rhs, Span: value.exprSpan()}, nil
		}
		if _, err := p.expect(";"); err != nil {
			return nil, err
		}
		return &EVT1ExprStmt{Value: value, Span: value.exprSpan()}, nil
	}
}

func (p *evt1Parser) parseLocalComptimeDecl() (EVT1Statement, error) {
	decl, err := p.parseComptimeDecl()
	if err != nil {
		return nil, err
	}
	return &EVT1VarDecl{Comptime: true, Type: decl.Type, Name: decl.Name, Value: decl.Value, Span: decl.Span}, nil
}

func (p *evt1Parser) looksLikeVarDecl() bool {
	if p.done() {
		return false
	}
	save := p.pos
	defer func() { p.pos = save }()
	if _, err := p.parseType(""); err != nil {
		return false
	}
	if !isIdentifier(p.peekLexeme()) {
		return false
	}
	p.next()
	return p.peekLexeme() == "="
}

func (p *evt1Parser) parseVarDecl() (EVT1Statement, error) {
	t, err := p.parseType("")
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
	return p.parseIfExpr()
}

func (p *evt1Parser) parseIfExpr() (EVT1Expr, error) {
	if p.peekLexeme() != "if" {
		return p.parseLogicalOr()
	}
	start := p.next().Span
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	condition, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(")"); err != nil {
		return nil, err
	}
	thenExpr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect("else"); err != nil {
		return nil, evt1Diagnostic("CV4184", "if expressions require an else branch", p.currentSpan())
	}
	elseExpr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if evt1IsDirectIfExpr(elseExpr) {
		return nil, evt1Diagnostic("CV4185", "else-if ladders are not supported; use match for multi-branch selection", elseExpr.exprSpan())
	}
	return &EVT1IfExpr{Condition: condition, Then: thenExpr, Else: elseExpr, Span: start}, nil
}

func evt1IsDirectIfExpr(expr EVT1Expr) bool {
	switch e := expr.(type) {
	case *EVT1IfExpr:
		return true
	case *EVT1ParenExpr:
		return evt1IsDirectIfExpr(e.Value)
	default:
		return false
	}
}

func (p *evt1Parser) parseLogicalOr() (EVT1Expr, error) {
	left, err := p.parseLogicalAnd()
	if err != nil {
		return nil, err
	}
	for p.peekLexeme() == "or" {
		op := p.next()
		right, err := p.parseLogicalAnd()
		if err != nil {
			return nil, err
		}
		left = &EVT1BinaryExpr{Op: op.Lexeme, Left: left, Right: right, Span: op.Span}
	}
	return left, nil
}

func (p *evt1Parser) parseLogicalAnd() (EVT1Expr, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}
	for p.peekLexeme() == "and" {
		op := p.next()
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		left = &EVT1BinaryExpr{Op: op.Lexeme, Left: left, Right: right, Span: op.Span}
	}
	return left, nil
}

func (p *evt1Parser) parseEquality() (EVT1Expr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.peekLexeme() == "==" || p.peekLexeme() == "!=" {
		op := p.next()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &EVT1BinaryExpr{Op: op.Lexeme, Left: left, Right: right, Span: op.Span}
	}
	return left, nil
}

func (p *evt1Parser) parseComparison() (EVT1Expr, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	for p.peekLexeme() == "<" || p.peekLexeme() == ">" || p.peekLexeme() == "<=" || p.peekLexeme() == ">=" {
		op := p.next()
		right, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		left = &EVT1BinaryExpr{Op: op.Lexeme, Left: left, Right: right, Span: op.Span}
	}
	return left, nil
}

func (p *evt1Parser) parseAdditive() (EVT1Expr, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}
	for p.peekLexeme() == "+" || p.peekLexeme() == "-" {
		op := p.next()
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = &EVT1BinaryExpr{Op: op.Lexeme, Left: left, Right: right, Span: op.Span}
	}
	return left, nil
}

func (p *evt1Parser) parseMultiplicative() (EVT1Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peekLexeme() == "*" {
		op := p.next()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &EVT1BinaryExpr{Op: op.Lexeme, Left: left, Right: right, Span: op.Span}
	}
	return left, nil
}

func (p *evt1Parser) parseUnary() (EVT1Expr, error) {
	if p.peekLexeme() == "-" || p.peekLexeme() == "not" {
		op := p.next()
		value, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &EVT1UnaryExpr{Op: op.Lexeme, Value: value, Span: op.Span}, nil
	}
	return p.parsePrimary()
}

func (p *evt1Parser) parsePrimary() (EVT1Expr, error) {
	switch {
	case p.done():
		return nil, evt1Diagnostic("CV4013", "unexpected end of expression", p.currentSpan())
	case p.peekLexeme() == "(":
		start := p.next().Span
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(")"); err != nil {
			return nil, err
		}
		return &EVT1ParenExpr{Value: expr, Span: start}, nil
	case p.peekLexeme() == "if":
		return p.parseIfExpr()
	case p.peekLexeme() == "match":
		return p.parseMatchExpr()
	case p.peekLexeme() == "true" || p.peekLexeme() == "false":
		tok := p.next()
		return &EVT1BoolLiteral{Value: tok.Lexeme == "true", Span: tok.Span}, nil
	case strings.HasPrefix(p.peekLexeme(), "\""):
		tok := p.next()
		value, err := strconv.Unquote(tok.Lexeme)
		if err != nil {
			return nil, evt1Diagnostic("CV4000", "invalid string literal", tok.Span)
		}
		return &EVT1StringLiteral{Value: value, Span: tok.Span}, nil
	case isNumber(p.peekLexeme()):
		tok := p.next()
		value, _ := strconv.Atoi(tok.Lexeme)
		return &EVT1IntLiteral{Value: value, Span: tok.Span}, nil
	default:
		return p.parseNameLikeExpr()
	}
}

func (p *evt1Parser) parseWhileStmt() (EVT1Statement, error) {
	start, err := p.expect("while")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	condition, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(")"); err != nil {
		return nil, err
	}
	stmt := &EVT1WhileStmt{Condition: condition, Span: start.Span}
	if p.peekLexeme() == "bounded" {
		p.next()
		if _, err := p.expect("("); err != nil {
			return nil, err
		}
		bound, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(")"); err != nil {
			return nil, err
		}
		stmt.Bound = bound
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	stmt.Body = body
	return stmt, nil
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
	if p.peekLexeme() == "{" {
		p.next()
		args := []EVT1Expr{}
		if p.peekLexeme() != "}" {
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
		if _, err := p.expect("}"); err != nil {
			return nil, err
		}
		expr = &EVT1StructConstructExpr{StructName: nameTok.Lexeme, Args: args, Span: nameTok.Span}
	}
	for {
		switch p.peekLexeme() {
		case "<":
			nameExpr, ok := expr.(*EVT1NameExpr)
			if !ok || !p.looksLikeTemplateInvocation() {
				return expr, nil
			}
			p.next()
			typeArg, err := p.parseType("")
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(">"); err != nil {
				return nil, err
			}
			if _, err := p.expect("("); err != nil {
				return nil, err
			}
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
			expr = &EVT1TemplateCallExpr{Callee: nameExpr.Name, TypeArg: typeArg, Args: args, Span: nameTok.Span}
		case "(":
			nameExpr, ok := expr.(*EVT1NameExpr)
			if !ok {
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
			expr = &EVT1CallExpr{Callee: nameExpr.Name, Args: args, Span: nameTok.Span}
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

func (p *evt1Parser) looksLikeTemplateInvocation() bool {
	if p.peekLexeme() != "<" {
		return false
	}
	save := p.pos
	defer func() { p.pos = save }()
	p.next()
	if _, err := p.parseType(""); err != nil {
		return false
	}
	if p.peekLexeme() != ">" {
		return false
	}
	p.next()
	return p.peekLexeme() == "("
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

func (p *evt1Parser) isConceptApplicationAhead(conceptParam string) bool {
	if !isIdentifier(p.peekLexeme()) || p.peekLexemeN(1) != "<" {
		return false
	}
	save := p.pos
	_, err := p.parseConceptUse(conceptParam)
	p.pos = save
	return err == nil
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
	return p.peekLexemeN(0)
}

func (p *evt1Parser) peekLexemeN(offset int) string {
	if p.pos+offset >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.pos+offset].Lexeme
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
