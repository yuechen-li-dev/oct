package parse

import (
	"fmt"
	"strconv"
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
	case token.KeywordStream:
		return p.parseStream()
	case token.KeywordEnum:
		return p.parseEnum()
	case token.KeywordShader:
		return p.parseShader()
	case token.KeywordFn:
		return p.parseFunction("")
	case token.KeywordInterface, token.KeywordFlow, token.KeywordCompile:
		kind := p.current().Lexeme
		p.skipUnsupportedTopLevel()
		return ast.UnsupportedDecl{Kind: kind}, nil
	default:
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
	ref, err := p.parseTypeRef()
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

func (p *parser) parseEnum() (ast.EnumDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected enum name")
	if err != nil {
		return ast.EnumDecl{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after enum name"); err != nil {
		return ast.EnumDecl{}, err
	}
	var variants []string
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.EnumDecl{}, p.errorAtCurrent("expected '}' to close enum")
		}
		variant, err := p.expect(token.Identifier, "expected enum variant")
		if err != nil {
			return ast.EnumDecl{}, err
		}
		variants = append(variants, variant.Lexeme)
		if _, err := p.expect(token.Semicolon, "expected ';' after enum variant"); err != nil {
			return ast.EnumDecl{}, err
		}
	}
	p.advance()
	return ast.EnumDecl{Name: name.Lexeme, Variants: variants}, nil
}

func (p *parser) parseShader() (ast.ShaderDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected shader name")
	if err != nil {
		return ast.ShaderDecl{}, err
	}
	if _, err := p.expect(token.LeftBrace, "expected '{' after shader name"); err != nil {
		return ast.ShaderDecl{}, err
	}
	shader := ast.ShaderDecl{Name: name.Lexeme}
	for p.current().Kind != token.RightBrace {
		if p.current().Kind == token.EOF {
			return ast.ShaderDecl{}, p.errorAtCurrent("expected '}' to close shader")
		}
		switch p.current().Kind {
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

func (p *parser) parseWorkgroup() (ast.WorkgroupDecl, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected workgroup name")
	if err != nil {
		return ast.WorkgroupDecl{}, err
	}
	if _, err := p.expect(token.Colon, "expected ':' after workgroup name"); err != nil {
		return ast.WorkgroupDecl{}, err
	}
	ref, err := p.parseTypeRef()
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
		ref, err := p.parseTypeRef()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Semicolon, "expected ';' after resource"); err != nil {
			return nil, err
		}
		resources = append(resources, ast.ResourceDecl{Name: name.Lexeme, Access: access, Type: ref})
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
		x, err := p.parsePositiveInt()
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		if _, err := p.expect(token.Comma, "expected ',' in numthreads"); err != nil {
			return ast.FunctionDecl{}, err
		}
		y, err := p.parsePositiveInt()
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		if _, err := p.expect(token.Comma, "expected ',' in numthreads"); err != nil {
			return ast.FunctionDecl{}, err
		}
		z, err := p.parsePositiveInt()
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
	ret, err := p.parseTypeRef()
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
			ref, err := p.parseTypeRef()
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
	switch p.current().Kind {
	case token.KeywordLet:
		return p.parseLet()
	case token.KeywordReturn:
		return p.parseReturn()
	case token.KeywordIf:
		return p.parseIf()
	case token.KeywordFor:
		return p.parseFor()
	default:
		left, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.match(token.Assign) {
			value, err := p.parseExpression()
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

func (p *parser) parseLet() (ast.LetStmt, error) {
	p.advance()
	name, err := p.expect(token.Identifier, "expected local name")
	if err != nil {
		return ast.LetStmt{}, err
	}
	if _, err := p.expect(token.Colon, "expected ':' after local name"); err != nil {
		return ast.LetStmt{}, err
	}
	ref, err := p.parseTypeRef()
	if err != nil {
		return ast.LetStmt{}, err
	}
	var value ast.Expr
	if p.match(token.Assign) {
		value, err = p.parseExpression()
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
	value, err := p.parseExpression()
	if err != nil {
		return ast.ReturnStmt{}, err
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after return"); err != nil {
		return ast.ReturnStmt{}, err
	}
	return ast.ReturnStmt{Value: value}, nil
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

func (p *parser) parseFor() (ast.ForStmt, error) {
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
	return ast.ForStmt{Name: name.Lexeme, Start: start, End: end, Step: step, Body: body}, nil
}

func (p *parser) parseExpression() (ast.Expr, error) {
	return p.parseWith()
}

func (p *parser) parseWith() (ast.Expr, error) {
	base, err := p.parseBinary(0)
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

func (p *parser) parseBinary(minPrec int) (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		prec, ok := binaryPrecedence(p.current().Kind)
		if !ok || prec < minPrec {
			break
		}
		op := p.current().Lexeme
		p.advance()
		right, err := p.parseBinary(prec + 1)
		if err != nil {
			return nil, err
		}
		left = ast.BinaryExpr{Left: left, Operator: op, Right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (ast.Expr, error) {
	if p.current().Kind == token.Minus {
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
			expr = ast.FieldAccessExpr{Target: expr, Field: field.Lexeme}
		case token.LeftBracket:
			p.advance()
			index, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.RightBracket, "expected ']' after index"); err != nil {
				return nil, err
			}
			expr = ast.IndexExpr{Target: expr, Index: index}
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
	case token.KeywordWhen:
		return p.parseWhenUtility()
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
	default:
		return nil, p.errorAtCurrent("expected expression")
	}
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

func (p *parser) parseTypeRef() (ast.TypeRef, error) {
	name, err := p.expect(token.Identifier, "expected type name")
	if err != nil {
		return ast.TypeRef{}, err
	}
	ref := ast.TypeRef{Name: name.Lexeme}
	if ref.Name == "array" && p.match(token.LeftAngle) {
		elem, err := p.parseTypeRef()
		if err != nil {
			return ast.TypeRef{}, err
		}
		ref.Args = append(ref.Args, elem)
		if p.match(token.Comma) {
			sizeToken, err := p.expect(token.IntLiteral, "expected array size")
			if err != nil {
				return ast.TypeRef{}, err
			}
			size, err := strconv.Atoi(strings.TrimRight(sizeToken.Lexeme, "uU"))
			if err != nil || size <= 0 {
				return ast.TypeRef{}, p.errorAtToken(sizeToken, "expected positive array size")
			}
			ref.ArraySize = size
			ref.HasArraySize = true
		}
		if _, err := p.expect(token.RightAngle, "expected '>' after array type"); err != nil {
			return ast.TypeRef{}, err
		}
	}
	return ref, nil
}

func (p *parser) parseField() (ast.Field, error) {
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
	ref, err := p.parseTypeRef()
	if err != nil {
		return ast.Field{}, err
	}
	if _, err := p.expect(token.Semicolon, "expected ';' after field"); err != nil {
		return ast.Field{}, err
	}
	return ast.Field{Name: name.Lexeme, Access: access, Type: ref}, nil
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

func (p *parser) parsePositiveInt() (int, error) {
	t, err := p.expect(token.IntLiteral, "expected positive integer literal")
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(strings.TrimRight(t.Lexeme, "uU"))
	if err != nil || value <= 0 {
		return 0, p.errorAtToken(t, "expected positive integer literal")
	}
	return value, nil
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

func binaryPrecedence(kind token.Kind) (int, bool) {
	switch kind {
	case token.EqualEqual, token.BangEqual, token.LeftAngle, token.LeftEqual, token.RightAngle, token.RightEqual:
		return 1, true
	case token.Plus, token.Minus:
		return 2, true
	case token.Star, token.Slash:
		return 3, true
	default:
		return 0, false
	}
}
