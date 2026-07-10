package parse

import (
	"fmt"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/token"
)

func BuildModule(result lex.Result) (ast.Module, error) {
	p := parser{tokens: result.Tokens}
	module, err := p.parseModule()
	if err != nil {
		return ast.Module{}, fmt.Errorf("parse %s: %w", result.Source.Path, err)
	}
	module.Source = result.Source
	return module, nil
}

type parser struct {
	tokens   []token.Token
	position int
}

func (p *parser) parseModule() (ast.Module, error) {
	var module ast.Module
	if p.match(token.KeywordNamespace) {
		path, err := p.parsePath()
		if err != nil {
			return ast.Module{}, err
		}
		module.Namespace = path
		if _, err := p.expect(token.Semicolon, "expected ';' after namespace"); err != nil {
			return ast.Module{}, err
		}
	}
	for p.match(token.KeywordUse) {
		path, err := p.parsePath()
		if err != nil {
			return ast.Module{}, err
		}
		module.Uses = append(module.Uses, path)
		if _, err := p.expect(token.Semicolon, "expected ';' after use"); err != nil {
			return ast.Module{}, err
		}
	}
	for p.current().Kind != token.EOF {
		decl, err := p.parseDecl()
		if err != nil {
			return ast.Module{}, err
		}
		module.Decls = append(module.Decls, decl)
	}
	return module, nil
}

func (p *parser) parseDecl() (ast.Decl, error) {
	switch p.current().Kind {
	case token.KeywordType:
		return p.parseTypeAlias()
	case token.KeywordRecord:
		return p.parseRecord()
	case token.KeywordBoard:
		return p.parseBoard()
	case token.KeywordStream:
		return p.parseStream()
	case token.KeywordConcept:
		return p.parseConcept()
	case token.KeywordConfig:
		return p.parseConfig()
	case token.KeywordEnum:
		return p.parseEnum()
	case token.KeywordTemplate:
		return p.parseTemplateShader()
	case token.KeywordShader:
		return p.parseShader(nil)
	case token.KeywordCompile:
		return p.parseCompileDecl()
	case token.KeywordFn:
		return p.parseFunction("")
	case token.KeywordInterface, token.KeywordFlow:
		kind := p.current().Lexeme
		p.skipUnsupportedTopLevel()
		return ast.UnsupportedDecl{Kind: kind}, nil
	default:
		if p.current().Kind == token.Identifier && p.current().Lexeme == "state" {
			return nil, p.errorAtCurrent("state blocks are only valid inside flow blocks in SDSL-V M22")
		}
		return nil, p.errorAtCurrent("expected SDSL-V top-level declaration")
	}
}

func (p *parser) parseTypeAlias() (ast.TypeAliasDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected type alias name")
	if err != nil {
		return ast.TypeAliasDecl{}, err
	}
	if _, err := p.expect(token.Assign, "expected '=' in type alias"); err != nil {
		return ast.TypeAliasDecl{}, err
	}
	ref, err := p.parseTypeRef(false)
	if err != nil {
		return ast.TypeAliasDecl{}, err
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after type alias"); err != nil {
		return ast.TypeAliasDecl{}, err
	}
	return ast.TypeAliasDecl{Name: name.Lexeme, Type: ref}, nil
}

func (p *parser) parseRecord() (ast.RecordDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected record name")
	if err != nil {
		return ast.RecordDecl{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after record name"); err != nil {
		return ast.RecordDecl{}, err
	}
	var fields []ast.Field
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.RecordDecl{}, p.errorAtCurrent("expected '}' to close record")
		}
		field, err := p.parseField()
		if err != nil {
			return ast.RecordDecl{}, err
		}
		fields = append(fields, field)
	}
	p.advance()
	return ast.RecordDecl{Name: name.Lexeme, Fields: fields}, nil
}

func (p *parser) parseBoard() (ast.BoardDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected board name")
	if err != nil {
		return ast.BoardDecl{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after board name"); err != nil {
		return ast.BoardDecl{}, err
	}
	var fields []ast.Field
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.BoardDecl{}, p.errorAtCurrent("expected '}' to close board")
		}
		field, err := p.parseField()
		if err != nil {
			return ast.BoardDecl{}, err
		}
		fields = append(fields, field)
	}
	p.advance()
	return ast.BoardDecl{Name: name.Lexeme, Fields: fields}, nil
}

func (p *parser) parseStream() (ast.StreamDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected stream name")
	if err != nil {
		return ast.StreamDecl{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after stream name"); err != nil {
		return ast.StreamDecl{}, err
	}
	var fields []ast.Field
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.StreamDecl{}, p.errorAtCurrent("expected '}' to close stream")
		}
		field, err := p.parseField()
		if err != nil {
			return ast.StreamDecl{}, err
		}
		fields = append(fields, field)
	}
	p.advance()
	return ast.StreamDecl{Name: name.Lexeme, Fields: fields}, nil
}

func (p *parser) parseConcept() (ast.ConceptDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected concept name")
	if err != nil {
		return ast.ConceptDecl{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after concept name"); err != nil {
		return ast.ConceptDecl{}, err
	}
	var members []ast.ConceptMember
	var requirements []ast.RequireStmt
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.ConceptDecl{}, p.errorAtCurrent("expected '}' to close concept")
		}
		if p.current().Kind == token.KeywordRequire {
			requirement, err := p.parseRequireStmt()
			if err != nil {
				return ast.ConceptDecl{}, err
			}
			requirements = append(requirements, requirement)
			continue
		}
		member, err := p.parseConceptMember()
		if err != nil {
			return ast.ConceptDecl{}, err
		}
		members = append(members, member)
	}
	p.advance()
	return ast.ConceptDecl{Name: name.Lexeme, Members: members, Requirements: requirements}, nil
}

func (p *parser) parseConfig() (ast.ConfigDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected config name")
	if err != nil {
		return ast.ConfigDecl{}, err
	}
	if _, err := p.expect(token.Colon, "expected ':' after config name"); err != nil {
		return ast.ConfigDecl{}, err
	}
	concept, err := p.expect(token.Identifier, "expected concept name after ':'")
	if err != nil {
		return ast.ConfigDecl{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after config concept"); err != nil {
		return ast.ConfigDecl{}, err
	}
	var fields []ast.ConfigField
	var requirements []ast.RequireStmt
	style := ast.ConfigAssignmentStyle("")
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.ConfigDecl{}, p.errorAtCurrent("expected '}' to close config")
		}
		if p.current().Kind == token.KeywordRequire {
			requirement, err := p.parseRequireStmt()
			if err != nil {
				return ast.ConfigDecl{}, err
			}
			requirements = append(requirements, requirement)
			continue
		}
		fieldPath, err := p.parseDottedName("expected config field name")
		if err != nil {
			return ast.ConfigDecl{}, err
		}
		assignmentStyle := ast.ConfigAssignmentLegacy
		if p.current().Kind == token.Colon {
			p.advance()
			if strings.Contains(fieldPath, ".") {
				return ast.ConfigDecl{}, p.errorAtCurrent("dotted config field paths require '=>' assignments in SDSL-V M11")
			}
		} else if p.current().Kind == token.Arrow && p.current().Lexeme == "=>" {
			p.advance()
			assignmentStyle = ast.ConfigAssignmentFatArrow
		} else {
			return ast.ConfigDecl{}, p.errorAtCurrent("expected ':' or '=>' after config field name")
		}
		if style == "" {
			style = assignmentStyle
		} else if style != assignmentStyle {
			return ast.ConfigDecl{}, p.errorAtCurrent("config declarations must not mix legacy ':' assignments with fat-arrow '=>' assignments")
		}
		value, err := p.parseExpression()
		if err != nil {
			return ast.ConfigDecl{}, err
		}
		if _, err := p.expect(token.Semicolon, "expected ';' after config field"); err != nil {
			return ast.ConfigDecl{}, err
		}
		fields = append(fields, ast.ConfigField{Path: fieldPath, Value: value, Style: assignmentStyle})
	}
	p.advance()
	return ast.ConfigDecl{Name: name.Lexeme, ConceptName: concept.Lexeme, Fields: fields, Requirements: requirements}, nil
}

func (p *parser) parseEnum() (ast.EnumDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected enum name")
	if err != nil {
		return ast.EnumDecl{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after enum name"); err != nil {
		return ast.EnumDecl{}, err
	}
	var variants []ast.EnumVariant
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.EnumDecl{}, p.errorAtCurrent("expected '}' to close enum")
		}
		variant, err := p.expect(token.Identifier, "expected enum variant")
		if err != nil {
			return ast.EnumDecl{}, err
		}
		entry := ast.EnumVariant{Name: variant.Lexeme}
		if p.match(token.LeftBrace) {
			entry.Payload = true
			for p.current().Kind != token.RightBrace {
				fieldName, err := p.expect(token.Identifier, "expected payload field name")
				if err != nil {
					return ast.EnumDecl{}, err
				}
				if _, err := p.expect(token.Colon, "expected ':' after payload field name"); err != nil {
					return ast.EnumDecl{}, err
				}
				ref, err := p.parseTypeRef(false)
				if err != nil {
					return ast.EnumDecl{}, err
				}
				if _, err := p.expect(token.Semicolon, "expected ';' after payload field"); err != nil {
					return ast.EnumDecl{}, err
				}
				entry.Fields = append(entry.Fields, ast.Field{Name: fieldName.Lexeme, Type: ref})
			}
			p.advance()
		} else {
			if _, err := p.expect(token.Semicolon, "expected ';' after enum variant"); err != nil {
				return ast.EnumDecl{}, err
			}
		}
		variants = append(variants, entry)
	}
	p.advance()
	return ast.EnumDecl{Name: name.Lexeme, Variants: variants}, nil
}

func (p *parser) parseTemplateShader() (ast.ShaderDecl, error) {
	param, err := p.parseTemplateParam()
	if err != nil {
		return ast.ShaderDecl{}, err
	}
	if p.current().Kind != token.KeywordShader {
		return ast.ShaderDecl{}, p.errorAtCurrent("expected shader after template parameter")
	}
	return p.parseShader(&param)
}

func (p *parser) parseTemplateParam() (ast.TemplateParam, error) {
	p.advance()
	if _, err := p.expect(token.LeftAngle, "expected '<' after template"); err != nil {
		return ast.TemplateParam{}, err
	}
	name, err := p.expect(token.Identifier, "expected template parameter name")
	if err != nil {
		return ast.TemplateParam{}, err
	}
	if _, err := p.expect(token.Colon, "expected ':' after template parameter name"); err != nil {
		return ast.TemplateParam{}, err
	}
	concept, err := p.expect(token.Identifier, "expected concept name in template parameter")
	if err != nil {
		return ast.TemplateParam{}, err
	}
	if p.match(token.Comma) {
		return ast.TemplateParam{}, p.errorAtCurrent("templates support exactly one concept config parameter in SDSL-V M5")
	}
	if _, err := p.expect(token.RightAngle, "expected '>' after template parameter"); err != nil {
		return ast.TemplateParam{}, err
	}
	return ast.TemplateParam{Name: name.Lexeme, ConceptName: concept.Lexeme}, nil
}

func (p *parser) parseShader(templateParam *ast.TemplateParam) (ast.ShaderDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected shader name")
	if err != nil {
		return ast.ShaderDecl{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after shader name"); err != nil {
		return ast.ShaderDecl{}, err
	}
	shader := ast.ShaderDecl{Name: name.Lexeme, Template: templateParam}
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.ShaderDecl{}, p.errorAtCurrent("expected '}' to close shader")
		}
		switch p.current().Kind {
		case token.KeywordStatic:
			staticAssert, err := p.parseStaticAssertStmt()
			if err != nil {
				return ast.ShaderDecl{}, err
			}
			shader.StaticAsserts = append(shader.StaticAsserts, staticAssert)
		case token.KeywordResources:
			resources, err := p.parseResources()
			if err != nil {
				return ast.ShaderDecl{}, err
			}
			if len(resources) == 1 && resources[0].Name != "" && resources[0].Access == "" && resources[0].Type.Name == "" {
				shader.ResourceBundleName = resources[0].Name
			} else {
				shader.Resources = append(shader.Resources, resources...)
			}
		case token.KeywordWorkgroup:
			workgroup, err := p.parseWorkgroup()
			if err != nil {
				return ast.ShaderDecl{}, err
			}
			shader.Workgroups = append(shader.Workgroups, workgroup)
		case token.KeywordStage:
			method, err := p.parseStageFunction()
			if err != nil {
				return ast.ShaderDecl{}, err
			}
			shader.Methods = append(shader.Methods, method)
		case token.KeywordFn:
			method, err := p.parseFunction("")
			if err != nil {
				return ast.ShaderDecl{}, err
			}
			shader.Methods = append(shader.Methods, method)
		default:
			return ast.ShaderDecl{}, p.errorAtCurrent("expected resources block, workgroup declaration, or shader function")
		}
	}
	p.advance()
	return shader, nil
}

func (p *parser) parseCompileDecl() (ast.CompileDecl, error) {
	p.advance()
	shaderName, err := p.expect(token.Identifier, "expected template shader name after compile")
	if err != nil {
		return ast.CompileDecl{}, err
	}
	if _, err := p.expect(token.LeftAngle, "expected '<' after compile target"); err != nil {
		return ast.CompileDecl{}, err
	}
	configName, err := p.expect(token.Identifier, "expected config name in compile declaration")
	if err != nil {
		return ast.CompileDecl{}, err
	}
	if p.match(token.Comma) {
		return ast.CompileDecl{}, p.errorAtCurrent("templates support exactly one concept config parameter in SDSL-V M5")
	}
	if _, err := p.expect(token.RightAngle, "expected '>' after compile config"); err != nil {
		return ast.CompileDecl{}, err
	}
	asTok, err := p.expect(token.Identifier, "expected 'as' in compile declaration")
	if err != nil {
		return ast.CompileDecl{}, err
	}
	if asTok.Lexeme != "as" {
		return ast.CompileDecl{}, p.errorAtToken(asTok, "expected 'as' in compile declaration")
	}
	alias, err := p.expect(token.Identifier, "expected alias name in compile declaration")
	if err != nil {
		return ast.CompileDecl{}, err
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after compile declaration"); err != nil {
		return ast.CompileDecl{}, err
	}
	return ast.CompileDecl{ShaderName: shaderName.Lexeme, ConfigName: configName.Lexeme, AliasName: alias.Lexeme}, nil
}

func (p *parser) parseWorkgroup() (ast.WorkgroupDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected workgroup name")
	if err != nil {
		return ast.WorkgroupDecl{}, err
	}
	if _, err := p.expect(token.Colon, "expected ':' after workgroup name"); err != nil {
		return ast.WorkgroupDecl{}, err
	}
	ref, err := p.parseTypeRef(false)
	if err != nil {
		return ast.WorkgroupDecl{}, err
	}
	if p.match(token.Assign) {
		return ast.WorkgroupDecl{}, p.errorAtCurrent("workgroup declarations must not have initializers")
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after workgroup declaration"); err != nil {
		return ast.WorkgroupDecl{}, err
	}
	return ast.WorkgroupDecl{Name: name.Lexeme, Type: ref}, nil
}

func (p *parser) parseResources() ([]ast.ResourceDecl, error) {
	p.advance()
	if p.current().Kind == token.Identifier {
		name, err := p.expect(token.Identifier, "expected resource bundle name")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Semicolon, "expected ';' after resource bundle name"); err != nil {
			return nil, err
		}
		return []ast.ResourceDecl{{Name: name.Lexeme}}, nil
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after resources"); err != nil {
		return nil, err
	}
	var resources []ast.ResourceDecl
	for p.current().Kind != token.RightBrace {
		attributes, err := p.parseAttributes(ast.AttributePlacementField)
		if err != nil {
			return nil, err
		}
		name, err := p.expect(token.Identifier, "expected resource name")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Colon, "expected ':' after resource name"); err != nil {
			return nil, err
		}
		accessToken := p.current()
		access := ""
		if p.match(token.KeywordReadonly) {
			access = "readonly"
		} else if p.match(token.KeywordReadwrite) {
			access = "readwrite"
		} else {
			return nil, p.errorAtToken(accessToken, "expected readonly or readwrite resource access")
		}
		ref, err := p.parseTypeRef(false)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Semicolon, "expected ';' after resource"); err != nil {
			return nil, err
		}
		resources = append(resources, ast.ResourceDecl{Name: name.Lexeme, Access: access, Type: ref, Attributes: attributes})
	}
	p.advance()
	return resources, nil
}

func (p *parser) parseStageFunction() (ast.FunctionDecl, error) {
	p.advance()
	stageToken := p.current()
	stage := ""
	switch stageToken.Kind {
	case token.KeywordCompute:
		stage = "compute"
	case token.KeywordVertex:
		stage = "vertex"
	case token.KeywordPixel:
		stage = "pixel"
	case token.Identifier:
		stage = stageToken.Lexeme
	default:
		return ast.FunctionDecl{}, p.errorAtCurrent("expected stage name")
	}
	p.advance()
	var threads *ast.NumThreads
	if p.match(token.LeftBracket) {
		attr, err := p.expect(token.Identifier, "expected stage attribute")
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		if attr.Lexeme != "numthreads" {
			return ast.FunctionDecl{}, p.errorAtToken(attr, "expected numthreads attribute for compute stage")
		}
		if _, err := p.expect(token.LeftParen, "expected '(' after numthreads"); err != nil {
			return ast.FunctionDecl{}, err
		}
		x, err := p.parseExpression()
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		if _, err := p.expect(token.Comma, "expected ',' in numthreads"); err != nil {
			return ast.FunctionDecl{}, err
		}
		y, err := p.parseExpression()
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		if _, err := p.expect(token.Comma, "expected ',' in numthreads"); err != nil {
			return ast.FunctionDecl{}, err
		}
		z, err := p.parseExpression()
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		if _, err := p.expect(token.RightParen, "expected ')' after numthreads"); err != nil {
			return ast.FunctionDecl{}, err
		}
		if _, err := p.expect(token.RightBracket, "expected ']' after numthreads"); err != nil {
			return ast.FunctionDecl{}, err
		}
		threads = &ast.NumThreads{X: x, Y: y, Z: z}
	}
	fn, err := p.parseFunction(stage)
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	fn.NumThreads = threads
	return fn, nil
}

func (p *parser) parseFunction(stage string) (ast.FunctionDecl, error) {
	if _, err := p.expect(token.KeywordFn, "expected fn"); err != nil {
		return ast.FunctionDecl{}, err
	}
	name, err := p.expect(token.Identifier, "expected function name")
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	params, err := p.parseParameters()
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	if _, err := p.expect(token.Arrow, "expected '->' before return type"); err != nil {
		return ast.FunctionDecl{}, err
	}
	ret, err := p.parseTypeRef(false)
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	return ast.FunctionDecl{Name: name.Lexeme, Stage: stage, Parameters: params, ReturnType: ret, Body: body}, nil
}

func (p *parser) parseParameters() ([]ast.Parameter, error) {
	if _, err := p.expect(token.LeftParen, "expected '(' before parameters"); err != nil {
		return nil, err
	}
	var params []ast.Parameter
	if p.current().Kind != token.RightParen {
		for {
			name, err := p.expect(token.Identifier, "expected parameter name")
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.Colon, "expected ':' after parameter name"); err != nil {
				return nil, err
			}
			ref, err := p.parseTypeRef(false)
			if err != nil {
				return nil, err
			}
			params = append(params, ast.Parameter{Name: name.Lexeme, Type: ref})
			if !p.match(token.Comma) {
				break
			}
		}
	}
	if _, err := p.expect(token.RightParen, "expected ')' after parameters"); err != nil {
		return nil, err
	}
	return params, nil
}

func (p *parser) parseBlock() (ast.Block, error) {
	if _, err := p.expect(token.LeftBrace, "expected '{' to start block"); err != nil {
		return ast.Block{}, err
	}
	var stmts []ast.Stmt
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.Block{}, p.errorAtCurrent("expected '}' to close block")
		}
		stmt, err := p.parseStmt()
		if err != nil {
			return ast.Block{}, err
		}
		stmts = append(stmts, stmt)
	}
	p.advance()
	return ast.Block{Statements: stmts}, nil
}

func (p *parser) parseStmt() (ast.Stmt, error) {
	if p.current().Kind == token.LeftBracket {
		attributes, err := p.parseAttributes(ast.AttributePlacementStmt)
		if err != nil {
			return nil, err
		}
		if p.current().Kind != token.KeywordFor {
			return nil, p.errorAtCurrent("statement attributes currently apply only to for loops; reduction attributes are only supported on direct let initializers, assignment RHS, or return values")
		}
		return p.parseFor(attributes)
	}
	switch p.current().Kind {
	case token.KeywordComptime:
		return p.parseComptimeStmt()
	case token.KeywordStatic:
		return p.parseStaticAssertStmt()
	case token.KeywordLet:
		return p.parseLet()
	case token.KeywordReturn:
		return p.parseReturn()
	case token.KeywordWrite:
		return p.parseGuardedWrite()
	case token.KeywordIf:
		return p.parseIf()
	case token.KeywordWhen:
		if p.peek(1).Kind == token.LeftBrace {
			return p.parseGuardWhen()
		}
		if p.peek(1).Kind == token.Identifier && p.peek(1).Lexeme == "policy" {
			return nil, p.errorAtCurrent("when policy requires persistent policy state; SDSL-V M19 does not support it yet")
		}
		left, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Semicolon, "expected ';' after expression statement"); err != nil {
			return nil, err
		}
		return ast.ExprStmt{Value: left}, nil
	case token.KeywordFlow:
		return p.parseFlowStmt()
	case token.KeywordFor:
		return p.parseFor(nil)
	default:
		if p.current().Kind == token.Identifier {
			switch p.current().Lexeme {
			case "goto":
				return nil, p.errorAtCurrent("SDSL-V M22 does not support goto in shader flow; mutable/transition flow is planned for M23+")
			case "remember", "resume", "suspend":
				return nil, p.errorAtCurrent("SDSL-V M22 does not support remember/resume/suspend; persistent flow state is deferred")
			case "state":
				return nil, p.errorAtCurrent("state blocks are only valid inside flow blocks in SDSL-V M22")
			}
		}
		left, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.match(token.Assign) {
			value, err := p.parseReductionValueOrExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.Semicolon, "expected ';' after assignment"); err != nil {
				return nil, err
			}
			return ast.AssignStmt{Target: left, Value: value}, nil
		}
		if _, err := p.expect(token.Semicolon, "expected ';' after expression statement"); err != nil {
			return nil, err
		}
		return ast.ExprStmt{Value: left}, nil
	}
}

func (p *parser) parseFlowStmt() (ast.FlowStmt, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected flow name")
	if err != nil {
		return ast.FlowStmt{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after flow name"); err != nil {
		return ast.FlowStmt{}, err
	}
	states := make([]ast.StateBlock, 0, 2)
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.FlowStmt{}, p.errorAtCurrent("expected '}' to close flow")
		}
		state, err := p.parseStateBlock()
		if err != nil {
			return ast.FlowStmt{}, err
		}
		states = append(states, state)
	}
	p.advance()
	return ast.FlowStmt{Name: name.Lexeme, States: states}, nil
}

func (p *parser) parseStateBlock() (ast.StateBlock, error) {
	if p.current().Kind != token.Identifier || p.current().Lexeme != "state" {
		return ast.StateBlock{}, p.errorAtCurrent("expected state declaration inside flow")
	}
	p.advance()
	name, err := p.expect(token.Identifier, "expected state name")
	if err != nil {
		return ast.StateBlock{}, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return ast.StateBlock{}, err
	}
	return ast.StateBlock{Name: name.Lexeme, Body: body}, nil
}

func (p *parser) parseComptimeStmt() (ast.Stmt, error) {
	p.advance()
	switch p.current().Kind {
	case token.KeywordLet:
		return p.parseComptimeLet()
	case token.KeywordIf:
		return p.parseComptimeIf()
	case token.KeywordMatch:
		return p.parseComptimeMatch()
	case token.KeywordWhen:
		return p.parseComptimeWhenUtility()
	case token.KeywordFor:
		return p.parseComptimeFor()
	default:
		return nil, p.errorAtCurrent("expected let, if, match, when, or for after comptime")
	}
}

func (p *parser) parseComptimeLet() (ast.ComptimeLetStmt, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected comptime local name")
	if err != nil {
		return ast.ComptimeLetStmt{}, err
	}
	if _, err := p.expect(token.Colon, "expected ':' after comptime local name"); err != nil {
		return ast.ComptimeLetStmt{}, err
	}
	ref, err := p.parseTypeRef(false)
	if err != nil {
		return ast.ComptimeLetStmt{}, err
	}
	if _, err := p.expect(token.Assign, "comptime let must be initialized"); err != nil {
		return ast.ComptimeLetStmt{}, err
	}
	value, err := p.parseExpression()
	if err != nil {
		return ast.ComptimeLetStmt{}, err
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after comptime let"); err != nil {
		return ast.ComptimeLetStmt{}, err
	}
	return ast.ComptimeLetStmt{Name: name.Lexeme, Type: ref, Value: value}, nil
}

func (p *parser) parseComptimeIf() (ast.ComptimeIfStmt, error) {
	p.advance()
	condition, err := p.parseExpression()
	if err != nil {
		return ast.ComptimeIfStmt{}, err
	}
	thenBody, err := p.parseBlock()
	if err != nil {
		return ast.ComptimeIfStmt{}, err
	}
	var elseBody *ast.Block
	if p.match(token.KeywordElse) {
		body, err := p.parseBlock()
		if err != nil {
			return ast.ComptimeIfStmt{}, err
		}
		elseBody = &body
	}
	return ast.ComptimeIfStmt{Condition: condition, ThenBody: thenBody, ElseBody: elseBody}, nil
}

func (p *parser) parseComptimeMatch() (ast.ComptimeMatchStmt, error) {
	p.advance()
	subject, err := p.parseExpression()
	if err != nil {
		return ast.ComptimeMatchStmt{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after comptime match subject"); err != nil {
		return ast.ComptimeMatchStmt{}, err
	}
	var arms []ast.ComptimeMatchArm
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.ComptimeMatchStmt{}, p.errorAtCurrent("expected '}' to close comptime match")
		}
		arm := ast.ComptimeMatchArm{}
		if p.match(token.KeywordElse) {
			arm.IsElse = true
		} else {
			pattern, err := p.parseExpression()
			if err != nil {
				return ast.ComptimeMatchStmt{}, err
			}
			arm.Pattern = pattern
		}
		if _, err := p.expect(token.Arrow, "expected '=>' after comptime match arm pattern"); err != nil {
			return ast.ComptimeMatchStmt{}, err
		}
		body, err := p.parseBlock()
		if err != nil {
			return ast.ComptimeMatchStmt{}, err
		}
		arm.Body = body
		arms = append(arms, arm)
	}
	p.advance()
	return ast.ComptimeMatchStmt{Subject: subject, Arms: arms}, nil
}

func (p *parser) parseComptimeWhenUtility() (ast.ComptimeWhenUtilityStmt, error) {
	p.advance()
	if _, err := p.expect(token.KeywordUtility, "expected 'utility' after comptime when"); err != nil {
		return ast.ComptimeWhenUtilityStmt{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after comptime when utility"); err != nil {
		return ast.ComptimeWhenUtilityStmt{}, err
	}
	var cases []ast.ComptimeWhenUtilityCase
	var elseBody *ast.Block
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.ComptimeWhenUtilityStmt{}, p.errorAtCurrent("expected '}' to close comptime when utility")
		}
		switch p.current().Kind {
		case token.KeywordCase:
			p.advance()
			label, err := p.parseDottedName("expected comptime when utility case label")
			if err != nil {
				return ast.ComptimeWhenUtilityStmt{}, err
			}
			var condition ast.Expr
			if p.match(token.KeywordWhen) {
				condition, err = p.parseExpression()
				if err != nil {
					return ast.ComptimeWhenUtilityStmt{}, err
				}
			}
			if _, err := p.expect(token.KeywordScore, "expected score in comptime when utility case"); err != nil {
				return ast.ComptimeWhenUtilityStmt{}, err
			}
			score, err := p.parseExpression()
			if err != nil {
				return ast.ComptimeWhenUtilityStmt{}, err
			}
			body, err := p.parseBlock()
			if err != nil {
				return ast.ComptimeWhenUtilityStmt{}, err
			}
			cases = append(cases, ast.ComptimeWhenUtilityCase{Label: label, Condition: condition, Score: score, Body: body})
		case token.KeywordElse:
			p.advance()
			if elseBody != nil {
				return ast.ComptimeWhenUtilityStmt{}, p.errorAtCurrent("duplicate comptime when utility else block")
			}
			body, err := p.parseBlock()
			if err != nil {
				return ast.ComptimeWhenUtilityStmt{}, err
			}
			elseBody = &body
		default:
			return ast.ComptimeWhenUtilityStmt{}, p.errorAtCurrent("expected case or else in comptime when utility")
		}
	}
	p.advance()
	return ast.ComptimeWhenUtilityStmt{Cases: cases, ElseBody: elseBody}, nil
}

func (p *parser) parseComptimeFor() (ast.ComptimeForStmt, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected comptime for loop variable")
	if err != nil {
		return ast.ComptimeForStmt{}, err
	}
	if _, err := p.expect(token.KeywordIn, "expected 'in' after comptime for loop variable"); err != nil {
		return ast.ComptimeForStmt{}, err
	}
	start, err := p.parseExpression()
	if err != nil {
		return ast.ComptimeForStmt{}, err
	}
	if _, err := p.expect(token.DotDot, "expected '..' in comptime for range"); err != nil {
		return ast.ComptimeForStmt{}, err
	}
	end, err := p.parseExpression()
	if err != nil {
		return ast.ComptimeForStmt{}, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return ast.ComptimeForStmt{}, err
	}
	return ast.ComptimeForStmt{Name: name.Lexeme, Start: start, End: end, Body: body}, nil
}

func (p *parser) parseLet() (ast.LetStmt, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected local name")
	if err != nil {
		return ast.LetStmt{}, err
	}
	if _, err := p.expect(token.Colon, "expected ':' after local name"); err != nil {
		return ast.LetStmt{}, err
	}
	ref, err := p.parseTypeRef(false)
	if err != nil {
		return ast.LetStmt{}, err
	}
	var value ast.Expr
	if p.match(token.Assign) {
		value, err = p.parseReductionValueOrExpression()
		if err != nil {
			return ast.LetStmt{}, err
		}
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after let"); err != nil {
		return ast.LetStmt{}, err
	}
	return ast.LetStmt{Name: name.Lexeme, Type: ref, Value: value}, nil
}

func (p *parser) parseReturn() (ast.ReturnStmt, error) {
	p.advance()
	if p.match(token.Semicolon) {
		return ast.ReturnStmt{}, nil
	}
	value, err := p.parseReductionValueOrExpression()
	if err != nil {
		return ast.ReturnStmt{}, err
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after return"); err != nil {
		return ast.ReturnStmt{}, err
	}
	return ast.ReturnStmt{Value: value}, nil
}

func (p *parser) parseGuardedWrite() (ast.GuardedWriteStmt, error) {
	p.advance()
	target, err := p.parseExpression()
	if err != nil {
		return ast.GuardedWriteStmt{}, err
	}
	if _, err := p.expect(token.Assign, "expected '=' after guarded write target"); err != nil {
		return ast.GuardedWriteStmt{}, err
	}
	value, err := p.parseReductionValueOrExpression()
	if err != nil {
		return ast.GuardedWriteStmt{}, err
	}
	if _, err := p.expect(token.KeywordWhen, "expected 'when' after guarded write value"); err != nil {
		return ast.GuardedWriteStmt{}, err
	}
	condition, err := p.parseExpression()
	if err != nil {
		return ast.GuardedWriteStmt{}, err
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after guarded write"); err != nil {
		return ast.GuardedWriteStmt{}, err
	}
	return ast.GuardedWriteStmt{Target: target, Value: value, Condition: condition}, nil
}

func (p *parser) parseIf() (ast.IfStmt, error) {
	p.advance()
	condition, err := p.parseExpression()
	if err != nil {
		return ast.IfStmt{}, err
	}
	thenBody, err := p.parseBlock()
	if err != nil {
		return ast.IfStmt{}, err
	}
	var elseBody *ast.Block
	if p.match(token.KeywordElse) {
		body, err := p.parseBlock()
		if err != nil {
			return ast.IfStmt{}, err
		}
		elseBody = &body
	}
	return ast.IfStmt{Condition: condition, ThenBody: thenBody, ElseBody: elseBody}, nil
}

func (p *parser) parseGuardWhen() (ast.GuardWhenStmt, error) {
	p.advance()
	if _, err := p.expect(token.LeftBrace, "expected '{' after guard when"); err != nil {
		return ast.GuardWhenStmt{}, err
	}
	var cases []ast.GuardWhenCase
	var elseBody *ast.Block
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.GuardWhenStmt{}, p.errorAtCurrent("expected '}' to close guard when")
		}
		switch p.current().Kind {
		case token.KeywordCase:
			if elseBody != nil {
				return ast.GuardWhenStmt{}, p.errorAtCurrent("guard when case cannot appear after else")
			}
			p.advance()
			condition, err := p.parseExpression()
			if err != nil {
				return ast.GuardWhenStmt{}, err
			}
			if _, err := p.expect(token.Arrow, "expected '->' after guard when case condition"); err != nil {
				return ast.GuardWhenStmt{}, err
			}
			body, err := p.parseWhenArmBody()
			if err != nil {
				return ast.GuardWhenStmt{}, err
			}
			cases = append(cases, ast.GuardWhenCase{Condition: condition, Body: body})
		case token.KeywordElse:
			if elseBody != nil {
				return ast.GuardWhenStmt{}, p.errorAtCurrent("duplicate guard when else arm")
			}
			p.advance()
			if _, err := p.expect(token.Arrow, "expected '->' after guard when else"); err != nil {
				return ast.GuardWhenStmt{}, err
			}
			body, err := p.parseWhenArmBody()
			if err != nil {
				return ast.GuardWhenStmt{}, err
			}
			elseBody = &body
		default:
			return ast.GuardWhenStmt{}, p.errorAtCurrent("expected case or else in guard when")
		}
	}
	p.advance()
	if len(cases) == 0 {
		return ast.GuardWhenStmt{}, p.errorAtCurrent("guard when requires at least one case")
	}
	return ast.GuardWhenStmt{Cases: cases, ElseBody: elseBody}, nil
}

func (p *parser) parseWhenArmBody() (ast.Block, error) {
	if p.current().Kind == token.LeftBrace {
		return p.parseBlock()
	}
	stmt, err := p.parseStmt()
	if err != nil {
		return ast.Block{}, err
	}
	return ast.Block{Statements: []ast.Stmt{stmt}}, nil
}

func (p *parser) parseFor(attributes []ast.Attribute) (ast.ForStmt, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected for loop variable")
	if err != nil {
		return ast.ForStmt{}, err
	}
	if _, err := p.expect(token.KeywordIn, "expected 'in' after for loop variable"); err != nil {
		return ast.ForStmt{}, err
	}
	start, err := p.parseExpression()
	if err != nil {
		return ast.ForStmt{}, err
	}
	if _, err := p.expect(token.DotDot, "expected '..' in for range"); err != nil {
		return ast.ForStmt{}, err
	}
	end, err := p.parseExpression()
	if err != nil {
		return ast.ForStmt{}, err
	}
	var step ast.Expr = ast.IntegerLiteral{Value: "1"}
	if p.match(token.KeywordStep) {
		step, err = p.parseExpression()
		if err != nil {
			return ast.ForStmt{}, err
		}
	}
	body, err := p.parseBlock()
	if err != nil {
		return ast.ForStmt{}, err
	}
	return ast.ForStmt{Attributes: attributes, Name: name.Lexeme, Start: start, End: end, Step: step, Body: body}, nil
}

func (p *parser) parseExpression() (ast.Expr, error) {
	return p.parseWith()
}

func (p *parser) parseWith() (ast.Expr, error) {
	return p.parseWithInternal(false)
}

func (p *parser) parseExpressionUntilRightAngle() (ast.Expr, error) {
	return p.parseWithInternal(true)
}

func (p *parser) parseExpressionUntilCommaOrRightAngle() (ast.Expr, error) {
	return p.parseWithInternal(true)
}

func (p *parser) parseWithInternal(stopAtRightAngle bool) (ast.Expr, error) {
	base, err := p.parseBinary(0, stopAtRightAngle)
	if err != nil {
		return nil, err
	}
	if !p.match(token.KeywordWith) {
		return base, nil
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after with"); err != nil {
		return nil, err
	}
	var updates []ast.FieldUpdate
	for p.current().Kind != token.RightBrace {
		name, err := p.expect(token.Identifier, "expected field name in with update")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Colon, "expected ':' after with field name"); err != nil {
			return nil, err
		}
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		updates = append(updates, ast.FieldUpdate{Name: name.Lexeme, Value: value})
		if !p.match(token.Comma) {
			break
		}
		if p.current().Kind == token.RightBrace {
			break
		}
	}
	if _, err := p.expect(token.RightBrace, "expected '}' after with update"); err != nil {
		return nil, err
	}
	return ast.WithExpr{Base: base, Updates: updates}, nil
}

func (p *parser) parseBinary(minPrec int, stopAtRightAngle bool) (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		prec, ok := binaryPrecedence(p.current().Kind, stopAtRightAngle)
		if !ok || prec < minPrec {
			break
		}
		op := p.current().Lexeme
		p.advance()
		right, err := p.parseBinary(prec+1, stopAtRightAngle)
		if err != nil {
			return nil, err
		}
		left = ast.BinaryExpr{Left: left, Operator: op, Right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (ast.Expr, error) {
	if p.current().Kind == token.Bang {
		return nil, p.errorAtCurrent("use `not` instead of `!` for logical negation")
	}
	if p.current().Kind == token.Minus || p.current().Kind == token.KeywordNot {
		op := p.current().Lexeme
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return ast.UnaryExpr{Operator: op, Operand: operand}, nil
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() (ast.Expr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.current().Kind {
		case token.Dot:
			p.advance()
			field, err := p.expect(token.Identifier, "expected field name after '.'")
			if err != nil {
				return nil, err
			}
			if enumName, ok := identifierName(expr); ok && p.payloadInitializerStarts() {
				fields, err := p.parseFieldInitializers()
				if err != nil {
					return nil, err
				}
				expr = ast.EnumConstructExpr{EnumName: enumName, VariantName: field.Lexeme, Fields: fields}
				continue
			}
			expr = ast.FieldAccessExpr{Target: expr, Field: field.Lexeme}
		case token.LeftBracket:
			p.advance()
			index, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			var index2 ast.Expr
			hasSecond := false
			if p.match(token.Comma) {
				hasSecond = true
				index2, err = p.parseExpression()
				if err != nil {
					return nil, err
				}
				if p.match(token.Comma) {
					return nil, p.errorAtCurrent("SDSL-V M12 supports at most two indices")
				}
			}
			if _, err := p.expect(token.RightBracket, "expected ']' after index"); err != nil {
				return nil, err
			}
			expr = ast.IndexExpr{Target: expr, Index: index, Index2: index2, HasSecond: hasSecond}
		case token.LeftParen:
			p.advance()
			var args []ast.Expr
			if p.current().Kind != token.RightParen {
				for {
					arg, err := p.parseExpression()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if !p.match(token.Comma) {
						break
					}
				}
			}
			if _, err := p.expect(token.RightParen, "expected ')' after call arguments"); err != nil {
				return nil, err
			}
			expr = ast.CallExpr{Callee: expr, Arguments: args}
		case token.LeftBrace:
			typeName, ok := identifierName(expr)
			if !ok || !p.payloadInitializerStarts() {
				return expr, nil
			}
			fields, err := p.parseFieldInitializers()
			if err != nil {
				return nil, err
			}
			expr = ast.BoardLiteralExpr{TypeName: typeName, Fields: fields}
		default:
			return expr, nil
		}
	}
}

func (p *parser) parsePrimary() (ast.Expr, error) {
	t := p.current()
	switch t.Kind {
	case token.IntLiteral:
		p.advance()
		return ast.IntegerLiteral{Value: t.Lexeme}, nil
	case token.FloatLiteral:
		p.advance()
		return ast.FloatLiteral{Value: t.Lexeme}, nil
	case token.StringLiteral:
		p.advance()
		return ast.StringLiteral{Value: t.Lexeme}, nil
	case token.KeywordTrue:
		p.advance()
		return ast.BoolLiteral{Value: true}, nil
	case token.KeywordFalse:
		p.advance()
		return ast.BoolLiteral{Value: false}, nil
	case token.Identifier:
		p.advance()
		return ast.IdentifierExpr{Name: t.Lexeme}, nil
	case token.KeywordSum, token.KeywordProduct, token.KeywordMax, token.KeywordMin:
		if p.peek(1).Kind == token.Identifier && p.peek(2).Kind == token.KeywordIn {
			return p.parseReductionExpr(nil)
		}
		p.advance()
		return ast.IdentifierExpr{Name: t.Lexeme}, nil
	case token.KeywordWhen:
		if p.peek(1).Kind == token.LeftBrace {
			return nil, p.errorAtCurrent("runtime guard when is a statement-only form in SDSL-V M19")
		}
		if p.peek(1).Kind == token.Identifier && p.peek(1).Lexeme == "policy" {
			return nil, p.errorAtCurrent("when policy requires persistent policy state; SDSL-V M19 does not support it yet")
		}
		return p.parseWhenUtility()
	case token.KeywordRead:
		return p.parseGuardedRead()
	case token.KeywordMatch:
		return p.parseMatchExpr()
	case token.LeftParen:
		p.advance()
		inner, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.RightParen, "expected ')' after expression"); err != nil {
			return nil, err
		}
		return ast.ParenExpr{Inner: inner}, nil
	case token.LeftBracket:
		return nil, p.errorAtCurrent("reduction attributes are only supported before direct reduction expressions in let initializers, assignment RHS, or return values")
	default:
		return nil, p.errorAtCurrent("expected expression")
	}
}

func (p *parser) parseGuardedRead() (ast.GuardedReadExpr, error) {
	p.advance()
	target, err := p.parsePostfix()
	if err != nil {
		return ast.GuardedReadExpr{}, err
	}
	if _, err := p.expect(token.KeywordWhen, "expected 'when' after guarded read target"); err != nil {
		return ast.GuardedReadExpr{}, err
	}
	condition, err := p.parseExpression()
	if err != nil {
		return ast.GuardedReadExpr{}, err
	}
	if _, err := p.expect(token.KeywordElse, "expected 'else' after guarded read condition"); err != nil {
		return ast.GuardedReadExpr{}, err
	}
	fallback, err := p.parseExpression()
	if err != nil {
		return ast.GuardedReadExpr{}, err
	}
	return ast.GuardedReadExpr{Target: target, Condition: condition, Fallback: fallback}, nil
}

func (p *parser) parseReductionValueOrExpression() (ast.Expr, error) {
	if p.current().Kind == token.LeftBracket {
		attributes, err := p.parseAttributes(ast.AttributePlacementExpr)
		if err != nil {
			return nil, err
		}
		if !isReductionToken(p.current().Kind) {
			return nil, p.errorAtCurrent("reduction attributes must be followed by sum, product, max, or min")
		}
		return p.parseReductionExpr(attributes)
	}
	return p.parseExpression()
}

func (p *parser) parseReductionExpr(attributes []ast.Attribute) (ast.ReductionExpr, error) {
	op, err := reductionOpFromToken(p.current())
	if err != nil {
		return ast.ReductionExpr{}, err
	}
	p.advance()
	name, err := p.expect(token.Identifier, "expected reduction index variable")
	if err != nil {
		return ast.ReductionExpr{}, err
	}
	if _, err := p.expect(token.KeywordIn, "expected 'in' after reduction index variable"); err != nil {
		return ast.ReductionExpr{}, err
	}
	start, err := p.parseExpression()
	if err != nil {
		return ast.ReductionExpr{}, err
	}
	if _, err := p.expect(token.DotDot, "expected '..' in reduction range"); err != nil {
		return ast.ReductionExpr{}, err
	}
	end, err := p.parseExpression()
	if err != nil {
		return ast.ReductionExpr{}, err
	}
	var step ast.Expr = ast.IntegerLiteral{Value: "1"}
	if p.match(token.KeywordStep) {
		step, err = p.parseExpression()
		if err != nil {
			return ast.ReductionExpr{}, err
		}
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' before reduction body"); err != nil {
		return ast.ReductionExpr{}, err
	}
	body, err := p.parseExpression()
	if err != nil {
		return ast.ReductionExpr{}, err
	}
	if _, err := p.expect(token.RightBrace, "expected '}' after reduction body"); err != nil {
		return ast.ReductionExpr{}, err
	}
	return ast.ReductionExpr{Attributes: append([]ast.Attribute(nil), attributes...), Op: op, Name: name.Lexeme, Start: start, End: end, Step: step, Body: body}, nil
}

func (p *parser) parseMatchExpr() (ast.MatchExpr, error) {
	p.advance()
	subject, err := p.parseExpression()
	if err != nil {
		return ast.MatchExpr{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after match subject"); err != nil {
		return ast.MatchExpr{}, err
	}
	var arms []ast.MatchArm
	for p.current().Kind != token.RightBrace {
		enumName, err := p.expect(token.Identifier, "expected enum name in match arm")
		if err != nil {
			return ast.MatchExpr{}, err
		}
		if _, err := p.expect(token.Dot, "expected '.' after enum name in match arm"); err != nil {
			return ast.MatchExpr{}, err
		}
		variantName, err := p.expect(token.Identifier, "expected variant name in match arm")
		if err != nil {
			return ast.MatchExpr{}, err
		}
		arm := ast.MatchArm{EnumName: enumName.Lexeme, VariantName: variantName.Lexeme}
		if p.match(token.LeftParen) {
			binding, err := p.expect(token.Identifier, "expected payload binding name")
			if err != nil {
				return ast.MatchExpr{}, err
			}
			arm.BindingName = binding.Lexeme
			if _, err := p.expect(token.RightParen, "expected ')' after payload binding"); err != nil {
				return ast.MatchExpr{}, err
			}
		}
		if _, err := p.expect(token.Arrow, "expected '=>' after match arm pattern"); err != nil {
			return ast.MatchExpr{}, err
		}
		value, err := p.parseExpression()
		if err != nil {
			return ast.MatchExpr{}, err
		}
		arm.Value = value
		arms = append(arms, arm)
	}
	p.advance()
	return ast.MatchExpr{Subject: subject, Arms: arms}, nil
}

func (p *parser) parseFieldInitializers() ([]ast.FieldInit, error) {
	if _, err := p.expect(token.LeftBrace, "expected '{' before payload fields"); err != nil {
		return nil, err
	}
	var fields []ast.FieldInit
	for p.current().Kind != token.RightBrace {
		name, err := p.expect(token.Identifier, "expected payload field name")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Colon, "expected ':' after payload field name"); err != nil {
			return nil, err
		}
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		fields = append(fields, ast.FieldInit{Name: name.Lexeme, Value: value})
		if p.match(token.Semicolon) {
			continue
		}
		if !p.match(token.Comma) {
			break
		}
		if p.current().Kind == token.RightBrace {
			break
		}
	}
	if _, err := p.expect(token.RightBrace, "expected '}' after payload fields"); err != nil {
		return nil, err
	}
	return fields, nil
}

func (p *parser) parseWhenUtility() (ast.WhenUtilityExpr, error) {
	p.advance()
	if _, err := p.expect(token.KeywordUtility, "expected 'utility' after when"); err != nil {
		return ast.WhenUtilityExpr{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after when utility"); err != nil {
		return ast.WhenUtilityExpr{}, err
	}
	var cases []ast.UtilityCase
	var elseValue ast.Expr
	for p.current().Kind != token.RightBrace {
		switch p.current().Kind {
		case token.KeywordCase:
			p.advance()
			value, err := p.parseExpression()
			if err != nil {
				return ast.WhenUtilityExpr{}, err
			}
			if _, err := p.expect(token.KeywordWhen, "expected when in utility case"); err != nil {
				return ast.WhenUtilityExpr{}, err
			}
			condition, err := p.parseExpression()
			if err != nil {
				return ast.WhenUtilityExpr{}, err
			}
			if _, err := p.expect(token.KeywordScore, "expected score in utility case"); err != nil {
				return ast.WhenUtilityExpr{}, err
			}
			score, err := p.parseExpression()
			if err != nil {
				return ast.WhenUtilityExpr{}, err
			}
			cases = append(cases, ast.UtilityCase{Value: value, Condition: condition, Score: score})
		case token.KeywordElse:
			p.advance()
			value, err := p.parseExpression()
			if err != nil {
				return ast.WhenUtilityExpr{}, err
			}
			elseValue = value
		default:
			return ast.WhenUtilityExpr{}, p.errorAtCurrent("expected case or else in when utility")
		}
	}
	p.advance()
	return ast.WhenUtilityExpr{Cases: cases, Else: elseValue}, nil
}

func (p *parser) parseTypeRef(allowZeroBang bool) (ast.TypeRef, error) {
	name, err := p.expect(token.Identifier, "expected type name")
	if err != nil {
		return ast.TypeRef{}, err
	}
	ref := ast.TypeRef{Name: name.Lexeme}
	if p.match(token.Bang) {
		if !allowZeroBang {
			return ast.TypeRef{}, p.errorAtCurrent("u32! is only valid for concept/config fields in SDSL-V M11")
		}
		if ref.Name != "u32" {
			return ast.TypeRef{}, p.errorAtCurrent("only u32! is supported as a zero-permitted concept/config field type in SDSL-V M11")
		}
		ref.ZeroAllowed = true
	}
	if (ref.Name == "array" || ref.Name == "matrix_view" || ref.Name == "tile" || ref.Name == "reg_tile") && p.match(token.LeftAngle) {
		elem, err := p.parseTypeRef(false)
		if err != nil {
			return ast.TypeRef{}, err
		}
		ref.Args = append(ref.Args, elem)
		if ref.Name == "tile" || ref.Name == "reg_tile" {
			if _, err := p.expect(token.Comma, "expected ',' after tile element type"); err != nil {
				return ast.TypeRef{}, err
			}
			rows, err := p.parseExpressionUntilCommaOrRightAngle()
			if err != nil {
				return ast.TypeRef{}, err
			}
			if _, err := p.expect(token.Comma, "expected ',' after tile row count"); err != nil {
				return ast.TypeRef{}, err
			}
			cols, err := p.parseExpressionUntilRightAngle()
			if err != nil {
				return ast.TypeRef{}, err
			}
			ref.TileRows = rows
			ref.TileCols = cols
			ref.HasTileShape = true
		} else if p.match(token.Comma) {
			sizeExpr, err := p.parseExpressionUntilRightAngle()
			if err != nil {
				return ast.TypeRef{}, err
			}
			ref.ArraySize = sizeExpr
			ref.HasArraySize = true
		}
		if _, err := p.expect(token.RightAngle, "expected '>' after type arguments"); err != nil {
			return ast.TypeRef{}, err
		}
	}
	return ref, nil
}

func (p *parser) parseField() (ast.Field, error) {
	attributes, err := p.parseAttributes(ast.AttributePlacementField)
	if err != nil {
		return ast.Field{}, err
	}
	name, err := p.expect(token.Identifier, "expected field name")
	if err != nil {
		return ast.Field{}, err
	}
	if _, err := p.expect(token.Colon, "expected ':' after field name"); err != nil {
		return ast.Field{}, err
	}
	access := ""
	if p.match(token.KeywordReadonly) {
		access = "readonly"
	} else if p.match(token.KeywordReadwrite) {
		access = "readwrite"
	}
	ref, err := p.parseTypeRef(false)
	if err != nil {
		return ast.Field{}, err
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after field"); err != nil {
		return ast.Field{}, err
	}
	return ast.Field{Name: name.Lexeme, Access: access, Type: ref, Attributes: attributes}, nil
}

func (p *parser) parseConceptMember() (ast.ConceptMember, error) {
	name, err := p.expect(token.Identifier, "expected concept field or group name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Colon, "expected ':' after concept field or group name"); err != nil {
		return nil, err
	}
	if p.match(token.LeftBrace) {
		var members []ast.ConceptMember
		for p.current().Kind != token.RightBrace {
			if p.current().Kind == token.EOF {
				return nil, p.errorAtCurrent("expected '}' to close concept group")
			}
			member, err := p.parseConceptMember()
			if err != nil {
				return nil, err
			}
			members = append(members, member)
		}
		p.advance()
		if _, err := p.expect(token.Semicolon, "expected ';' after concept group"); err != nil {
			return nil, err
		}
		return ast.ConceptGroup{Name: name.Lexeme, Members: members}, nil
	}
	ref, err := p.parseTypeRef(true)
	if err != nil {
		return nil, err
	}
	var defaultValue ast.Expr
	if p.match(token.Assign) {
		defaultValue, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after concept field"); err != nil {
		return nil, err
	}
	return ast.ConceptField{Name: name.Lexeme, Type: ref, DefaultValue: defaultValue}, nil
}

func (p *parser) parseDottedName(message string) (string, error) {
	name, err := p.expect(token.Identifier, message)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(name.Lexeme)
	for p.match(token.Dot) {
		part, err := p.expect(token.Identifier, "expected identifier after '.'")
		if err != nil {
			return "", err
		}
		b.WriteByte('.')
		b.WriteString(part.Lexeme)
	}
	return b.String(), nil
}

func (p *parser) parseRequireStmt() (ast.RequireStmt, error) {
	if _, err := p.expect(token.KeywordRequire, "expected require"); err != nil {
		return ast.RequireStmt{}, err
	}
	start := p.position
	expr, err := p.parseExpression()
	if err != nil {
		return ast.RequireStmt{}, err
	}
	text := p.exprText(start, p.position)
	if _, err := p.expect(token.Semicolon, "expected ';' after require"); err != nil {
		return ast.RequireStmt{}, err
	}
	return ast.RequireStmt{Expr: expr, Text: text}, nil
}

func (p *parser) parseStaticAssertStmt() (ast.StaticAssertStmt, error) {
	if _, err := p.expect(token.KeywordStatic, "expected static"); err != nil {
		return ast.StaticAssertStmt{}, err
	}
	if _, err := p.expect(token.KeywordAssert, "expected assert after static"); err != nil {
		return ast.StaticAssertStmt{}, err
	}
	start := p.position
	expr, err := p.parseExpression()
	if err != nil {
		return ast.StaticAssertStmt{}, err
	}
	text := p.exprText(start, p.position)
	if _, err := p.expect(token.Semicolon, "expected ';' after static assert"); err != nil {
		return ast.StaticAssertStmt{}, err
	}
	return ast.StaticAssertStmt{Expr: expr, Text: text}, nil
}

func (p *parser) parseAttributes(placement ast.AttributePlacement) ([]ast.Attribute, error) {
	var attributes []ast.Attribute
	for p.current().Kind == token.LeftBracket {
		left := p.current()
		p.advance()
		name, err := p.expect(token.Identifier, "expected attribute name")
		if err != nil {
			return nil, err
		}
		attr := ast.Attribute{Name: name.Lexeme, Placement: placement, Line: left.Line, Column: left.Column}
		if p.match(token.LeftParen) {
			if p.current().Kind != token.RightParen {
				for {
					arg, err := p.parseExpression()
					if err != nil {
						return nil, err
					}
					attr.Arguments = append(attr.Arguments, arg)
					if !p.match(token.Comma) {
						break
					}
				}
			}
			if _, err := p.expect(token.RightParen, "expected ')' after attribute arguments"); err != nil {
				return nil, err
			}
		}
		if _, err := p.expect(token.RightBracket, "expected ']' after attribute"); err != nil {
			return nil, err
		}
		attributes = append(attributes, attr)
	}
	return attributes, nil
}

func (p *parser) exprText(start, end int) string {
	parts := make([]string, 0, end-start)
	for i := start; i < end && i < len(p.tokens); i++ {
		if p.tokens[i].Kind == token.EOF {
			break
		}
		parts = append(parts, p.tokens[i].Lexeme)
	}
	return strings.Join(parts, " ")
}

func (p *parser) parsePath() (string, error) {
	first, err := p.expect(token.Identifier, "expected path segment")
	if err != nil {
		return "", err
	}
	segments := []string{first.Lexeme}
	for p.match(token.Dot) {
		next, err := p.expect(token.Identifier, "expected path segment after '.'")
		if err != nil {
			return "", err
		}
		segments = append(segments, next.Lexeme)
	}
	return strings.Join(segments, "."), nil
}

func (p *parser) skipUnsupportedTopLevel() {
	if p.current().Kind == token.EOF {
		return
	}
	p.advance()
	depth := 0
	for p.current().Kind != token.EOF {
		switch p.current().Kind {
		case token.LeftBrace:
			depth++
		case token.RightBrace:
			if depth == 0 {
				p.advance()
				return
			}
			depth--
			if depth == 0 {
				p.advance()
				return
			}
		case token.Semicolon:
			if depth == 0 {
				p.advance()
				return
			}
		}
		p.advance()
	}
}

func (p *parser) match(kind token.Kind) bool {
	if p.current().Kind != kind {
		return false
	}
	p.advance()
	return true
}

func (p *parser) expect(kind token.Kind, message string) (token.Token, error) {
	t := p.current()
	if t.Kind != kind {
		return token.Token{}, p.errorAtCurrent(message)
	}
	p.advance()
	return t, nil
}

func (p *parser) current() token.Token { return p.peek(0) }

func (p *parser) peek(offset int) token.Token {
	pos := p.position + offset
	if pos >= len(p.tokens) {
		return token.Token{Kind: token.EOF}
	}
	return p.tokens[pos]
}

func (p *parser) advance() {
	if p.position < len(p.tokens) {
		p.position++
	}
}

func (p *parser) errorAtCurrent(message string) error {
	return p.errorAtToken(p.current(), message)
}

func (p *parser) errorAtToken(t token.Token, message string) error {
	if t.Kind == token.EOF {
		return fmt.Errorf("%s at end of file", message)
	}
	return fmt.Errorf("%s at %d:%d near %q", message, t.Line, t.Column, t.Lexeme)
}

func binaryPrecedence(kind token.Kind, stopAtRightAngle bool) (int, bool) {
	switch kind {
	case token.KeywordOr, token.OrOr:
		return 1, true
	case token.KeywordAnd, token.AndAnd:
		return 2, true
	case token.EqualEqual, token.BangEqual, token.LeftAngle, token.LeftEqual, token.RightAngle, token.RightEqual:
		if stopAtRightAngle && kind == token.RightAngle {
			return 0, false
		}
		return 3, true
	case token.Plus, token.Minus:
		return 4, true
	case token.Star, token.Slash, token.Percent:
		return 5, true
	default:
		return 0, false
	}
}

func identifierName(expr ast.Expr) (string, bool) {
	id, ok := expr.(ast.IdentifierExpr)
	if !ok {
		return "", false
	}
	return id.Name, true
}

func reductionOpFromToken(tok token.Token) (ast.ReductionOp, error) {
	switch tok.Kind {
	case token.KeywordSum:
		return ast.ReductionSum, nil
	case token.KeywordProduct:
		return ast.ReductionProduct, nil
	case token.KeywordMax:
		return ast.ReductionMax, nil
	case token.KeywordMin:
		return ast.ReductionMin, nil
	default:
		return "", fmt.Errorf("expected reduction operator at %d:%d near %q", tok.Line, tok.Column, tok.Lexeme)
	}
}

func isReductionToken(kind token.Kind) bool {
	switch kind {
	case token.KeywordSum, token.KeywordProduct, token.KeywordMax, token.KeywordMin:
		return true
	default:
		return false
	}
}

func (p *parser) payloadInitializerStarts() bool {
	return p.current().Kind == token.LeftBrace && p.peek(1).Kind == token.Identifier && p.peek(2).Kind == token.Colon
}
