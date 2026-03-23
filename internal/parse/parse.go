package parse

import (
	"fmt"

	"oct/internal/ast"
	"oct/internal/lex"
	"oct/internal/source"
)

func BuildFile(result lex.Result) (ast.File, error) {
	parser := parser{
		sourcePath: result.Source.Path,
		tokens:     result.Tokens,
	}

	file, err := parser.parseFile(result.Source)
	if err != nil {
		return ast.File{}, fmt.Errorf("parse %s: %w", result.Source.Path, err)
	}
	return file, nil
}

type parser struct {
	sourcePath string
	tokens     []lex.Token
	position   int
}

func (p *parser) parseFile(src source.File) (ast.File, error) {
	file := ast.File{Source: src}
	for p.current().Kind != lex.EOF {
		function, err := p.parseFunctionDecl()
		if err != nil {
			return ast.File{}, err
		}
		file.Functions = append(file.Functions, function)
	}
	return file, nil
}

func (p *parser) parseFunctionDecl() (ast.FunctionDecl, error) {
	if _, err := p.expect(lex.KeywordFn, "expected 'fn' at top level"); err != nil {
		return ast.FunctionDecl{}, err
	}

	name, err := p.expect(lex.Identifier, "expected function name")
	if err != nil {
		return ast.FunctionDecl{}, err
	}

	if _, err := p.expect(lex.LeftParen, "expected '(' after function name"); err != nil {
		return ast.FunctionDecl{}, err
	}

	parameters, err := p.parseParameters()
	if err != nil {
		return ast.FunctionDecl{}, err
	}

	if _, err := p.expect(lex.RightParen, "expected ')' after parameter list"); err != nil {
		return ast.FunctionDecl{}, err
	}
	if _, err := p.expect(lex.Arrow, "expected '->' before return type"); err != nil {
		return ast.FunctionDecl{}, err
	}

	returnType, err := p.parseTypeRef()
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return ast.FunctionDecl{}, err
	}

	return ast.FunctionDecl{Name: name.Lexeme, Parameters: parameters, ReturnType: returnType, Body: body}, nil
}

func (p *parser) parseParameters() ([]ast.Parameter, error) {
	if p.current().Kind == lex.RightParen {
		return nil, nil
	}

	var parameters []ast.Parameter
	for {
		name, err := p.expect(lex.Identifier, "expected parameter name")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lex.Colon, "expected ':' after parameter name"); err != nil {
			return nil, err
		}
		typeRef, err := p.parseTypeRef()
		if err != nil {
			return nil, err
		}
		parameters = append(parameters, ast.Parameter{Name: name.Lexeme, Type: typeRef})

		if !p.match(lex.Comma) {
			break
		}
	}

	return parameters, nil
}

func (p *parser) parseTypeRef() (ast.TypeRef, error) {
	token, err := p.expect(lex.Identifier, "expected type name")
	if err != nil {
		return ast.TypeRef{}, err
	}
	return ast.TypeRef{Name: token.Lexeme}, nil
}

func (p *parser) parseBlock() (ast.Block, error) {
	if _, err := p.expect(lex.LeftBrace, "expected '{' to start block"); err != nil {
		return ast.Block{}, err
	}

	var statements []ast.Stmt
	for p.current().Kind != lex.RightBrace {
		if p.current().Kind == lex.EOF {
			return ast.Block{}, p.errorAtCurrent("expected '}' to close block")
		}
		statement, err := p.parseStatement()
		if err != nil {
			return ast.Block{}, err
		}
		statements = append(statements, statement)
	}
	p.advance()

	return ast.Block{Statements: statements}, nil
}

func (p *parser) parseStatement() (ast.Stmt, error) {
	switch p.current().Kind {
	case lex.KeywordLet:
		return p.parseLetStmt()
	case lex.KeywordReturn:
		return p.parseReturnStmt()
	default:
		return nil, p.errorAtCurrent("expected statement")
	}
}

func (p *parser) parseLetStmt() (ast.Stmt, error) {
	p.advance()
	name, err := p.expect(lex.Identifier, "expected identifier after 'let'")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lex.Assign, "expected '=' after binding name"); err != nil {
		return nil, err
	}
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return ast.LetStmt{Name: name.Lexeme, Value: value}, nil
}

func (p *parser) parseReturnStmt() (ast.Stmt, error) {
	p.advance()
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return ast.ReturnStmt{Value: value}, nil
}

func (p *parser) parseExpression() (ast.Expr, error) {
	return p.parseBinaryExpr(0)
}

func (p *parser) parseBinaryExpr(minPrecedence int) (ast.Expr, error) {
	left, err := p.parsePrimaryExpr()
	if err != nil {
		return nil, err
	}

	for {
		opToken := p.current()
		precedence, ok := binaryPrecedence(opToken.Kind)
		if !ok || precedence < minPrecedence {
			break
		}

		p.advance()
		right, err := p.parseBinaryExpr(precedence + 1)
		if err != nil {
			return nil, err
		}

		left = ast.BinaryExpr{Left: left, Operator: opToken.Lexeme, Right: right}
	}

	return left, nil
}

func (p *parser) parsePrimaryExpr() (ast.Expr, error) {
	token := p.current()
	switch token.Kind {
	case lex.IntLiteral:
		p.advance()
		return ast.IntegerLiteral{Value: token.Lexeme}, nil
	case lex.FloatLiteral:
		p.advance()
		return ast.FloatLiteral{Value: token.Lexeme}, nil
	case lex.KeywordTrue:
		p.advance()
		return ast.BoolLiteral{Value: true}, nil
	case lex.KeywordFalse:
		p.advance()
		return ast.BoolLiteral{Value: false}, nil
	case lex.Identifier:
		p.advance()
		return ast.IdentifierExpr{Name: token.Lexeme}, nil
	case lex.LeftParen:
		p.advance()
		inner, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lex.RightParen, "expected ')' after expression"); err != nil {
			return nil, err
		}
		return ast.ParenExpr{Inner: inner}, nil
	default:
		return nil, p.errorAtCurrent("expected expression")
	}
}

func (p *parser) match(kind lex.TokenKind) bool {
	if p.current().Kind != kind {
		return false
	}
	p.advance()
	return true
}

func (p *parser) expect(kind lex.TokenKind, message string) (lex.Token, error) {
	token := p.current()
	if token.Kind != kind {
		return lex.Token{}, p.errorAtCurrent(message)
	}
	p.advance()
	return token, nil
}

func (p *parser) current() lex.Token {
	if p.position >= len(p.tokens) {
		return lex.Token{Kind: lex.EOF}
	}
	return p.tokens[p.position]
}

func (p *parser) advance() {
	if p.position < len(p.tokens) {
		p.position++
	}
}

func (p *parser) errorAtCurrent(message string) error {
	token := p.current()
	if token.Kind == lex.EOF {
		return fmt.Errorf("%s at end of file", message)
	}
	return fmt.Errorf("%s at %d:%d near %q", message, token.Line, token.Column, token.Lexeme)
}

func binaryPrecedence(kind lex.TokenKind) (int, bool) {
	switch kind {
	case lex.Plus, lex.Minus:
		return 1, true
	case lex.Star, lex.Slash:
		return 2, true
	default:
		return 0, false
	}
}
