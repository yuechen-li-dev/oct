package parse

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/dimension"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/source"
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
	sourcePath            string
	tokens                []lex.Token
	position              int
	nextUtilityWhenSiteID int
	docByLine             map[int]ast.DocComment
}

func (p *parser) parseFile(src source.File) (ast.File, error) {
	p.docByLine = scanDocComments(src.Text)
	file := ast.File{Source: src, IsTest: strings.HasSuffix(src.Path, ".octest")}
	if _, err := p.expect(lex.KeywordPackage, "missing package declaration"); err != nil {
		return ast.File{}, err
	}
	packageName, err := p.expect(lex.Identifier, "expected package name")
	if err != nil {
		return ast.File{}, err
	}
	file.Package = packageName.Lexeme

	seenImports := make(map[string]struct{})
	for p.current().Kind == lex.KeywordImport {
		p.advance()
		importName, err := p.expect(lex.Identifier, "expected import package name")
		if err != nil {
			return ast.File{}, err
		}
		if importName.Lexeme == file.Package {
			return ast.File{}, p.errorAtToken(importName, fmt.Sprintf("package '%s' cannot import itself", file.Package))
		}
		if _, exists := seenImports[importName.Lexeme]; exists {
			return ast.File{}, p.errorAtToken(importName, fmt.Sprintf("duplicate import '%s'", importName.Lexeme))
		}
		seenImports[importName.Lexeme] = struct{}{}
		file.Imports = append(file.Imports, importName.Lexeme)
	}

	pendingFact := false
	pendingTheory := false
	pendingArtifact := false
	pendingBenchmark := false
	pendingInlineData := make([]ast.InlineDataRow, 0)
	pendingSuites := make([]string, 0)
	var pendingCycleTime ast.Expr
	for p.current().Kind != lex.EOF {
		if p.current().Kind == lex.LeftBracket {
			if !file.IsTest {
				attrName := "attribute"
				if p.position+1 < len(p.tokens) && p.tokens[p.position+1].Kind == lex.Identifier {
					attrName = "[" + p.tokens[p.position+1].Lexeme + "]"
				}
				return ast.File{}, p.errorAtCurrent(fmt.Sprintf("%s is only valid in .octest files", attrName))
			}
			attribute, err := p.parseTestAttribute()
			if err != nil {
				return ast.File{}, err
			}
			switch attribute.kind {
			case "Fact":
				if pendingFact {
					return ast.File{}, p.errorAtCurrent("duplicate [Fact] attribute on function")
				}
				if pendingTheory {
					return ast.File{}, p.errorAtCurrent("[Fact] and [Theory] cannot both apply to the same function")
				}
				if pendingArtifact {
					return ast.File{}, p.errorAtCurrent("[Artifact] cannot be combined with [Fact]")
				}
				if pendingBenchmark {
					return ast.File{}, p.errorAtCurrent("[Benchmark] cannot be combined with [Fact]")
				}
				pendingFact = true
			case "Theory":
				if pendingTheory {
					return ast.File{}, p.errorAtCurrent("duplicate [Theory] attribute on function")
				}
				if pendingFact {
					return ast.File{}, p.errorAtCurrent("[Fact] and [Theory] cannot both apply to the same function")
				}
				if pendingArtifact {
					return ast.File{}, p.errorAtCurrent("[Artifact] cannot be combined with [Theory]")
				}
				if pendingBenchmark {
					return ast.File{}, p.errorAtCurrent("[Benchmark] cannot be combined with [Theory]")
				}
				pendingTheory = true
			case "Artifact":
				if pendingArtifact {
					return ast.File{}, p.errorAtCurrent("duplicate [Artifact] attribute on function")
				}
				if pendingFact {
					return ast.File{}, p.errorAtCurrent("[Artifact] cannot be combined with [Fact]")
				}
				if pendingTheory {
					return ast.File{}, p.errorAtCurrent("[Artifact] cannot be combined with [Theory]")
				}
				if pendingBenchmark {
					return ast.File{}, p.errorAtCurrent("[Benchmark] cannot be combined with [Artifact]")
				}
				pendingArtifact = true
			case "Benchmark":
				if pendingBenchmark {
					return ast.File{}, p.errorAtCurrent("duplicate [Benchmark] attribute on function")
				}
				if pendingFact {
					return ast.File{}, p.errorAtCurrent("[Benchmark] cannot be combined with [Fact]")
				}
				if pendingTheory {
					return ast.File{}, p.errorAtCurrent("[Benchmark] cannot be combined with [Theory]")
				}
				if pendingArtifact {
					return ast.File{}, p.errorAtCurrent("[Benchmark] cannot be combined with [Artifact]")
				}
				pendingBenchmark = true
			case "InlineData":
				pendingInlineData = append(pendingInlineData, ast.InlineDataRow{Values: attribute.values})
			case "Suite":
				pendingSuites = append(pendingSuites, attribute.suiteName)
			case "CycleTime":
				if pendingCycleTime != nil {
					return ast.File{}, p.errorAtCurrent("duplicate [CycleTime] attribute on function")
				}
				pendingCycleTime = attribute.value
			}
			continue
		}
		switch p.current().Kind {
		case lex.KeywordRecord:
			if pendingFact || pendingTheory || pendingArtifact || pendingBenchmark || len(pendingInlineData) > 0 || len(pendingSuites) > 0 || pendingCycleTime != nil {
				return ast.File{}, p.errorAtCurrent("test attributes must apply to a function declaration")
			}
			record, err := p.parseRecordDecl()
			if err != nil {
				return ast.File{}, err
			}
			file.Records = append(file.Records, record)
		case lex.KeywordEnum:
			if pendingFact || pendingTheory || pendingArtifact || pendingBenchmark || len(pendingInlineData) > 0 || len(pendingSuites) > 0 || pendingCycleTime != nil {
				return ast.File{}, p.errorAtCurrent("test attributes must apply to a function declaration")
			}
			enumDecl, err := p.parseEnumDecl()
			if err != nil {
				return ast.File{}, err
			}
			file.Enums = append(file.Enums, enumDecl)
		case lex.KeywordFn:
			function, err := p.parseFunctionDecl()
			if err != nil {
				return ast.File{}, err
			}
			function.IsTestFile = file.IsTest
			if pendingFact {
				if len(function.Parameters) != 0 {
					return ast.File{}, p.errorAtCurrent("[Fact] function must not declare parameters")
				}
				if function.ReturnType.Name != "Void" || function.ReturnType.IsArray || function.ReturnType.VectorOf != nil || function.ReturnType.MatrixOf != nil || function.ReturnType.HasUnit || function.ReturnType.Package != "" {
					return ast.File{}, p.errorAtCurrent("[Fact] function must return Void")
				}
				if len(pendingInlineData) > 0 {
					return ast.File{}, p.errorAtCurrent("[InlineData] cannot be used with [Fact]")
				}
				if pendingCycleTime != nil {
					return ast.File{}, p.errorAtCurrent("[CycleTime] is only valid on [Theory] functions")
				}
				function.IsFact = true
				function.Suites = append(function.Suites, pendingSuites...)
				pendingFact = false
				pendingSuites = pendingSuites[:0]
			}
			if pendingArtifact {
				if len(function.Parameters) != 0 {
					return ast.File{}, p.errorAtCurrent("[Artifact] function must not declare parameters")
				}
				if function.ReturnType.Name != "Void" || function.ReturnType.IsArray || function.ReturnType.VectorOf != nil || function.ReturnType.MatrixOf != nil || function.ReturnType.HasUnit || function.ReturnType.Package != "" {
					return ast.File{}, p.errorAtCurrent("[Artifact] function must return Void")
				}
				if len(pendingInlineData) > 0 {
					return ast.File{}, p.errorAtCurrent("[InlineData] cannot be used with [Artifact]")
				}
				if pendingCycleTime != nil {
					return ast.File{}, p.errorAtCurrent("[CycleTime] is only valid on [Theory] functions")
				}
				function.IsArtifact = true
				function.Suites = append(function.Suites, pendingSuites...)
				pendingArtifact = false
				pendingSuites = pendingSuites[:0]
			}
			if pendingBenchmark {
				if len(function.Parameters) != 0 {
					return ast.File{}, p.errorAtCurrent("[Benchmark] function must not declare parameters")
				}
				if function.ReturnType.Name != "Void" || function.ReturnType.IsArray || function.ReturnType.VectorOf != nil || function.ReturnType.MatrixOf != nil || function.ReturnType.HasUnit || function.ReturnType.Package != "" {
					return ast.File{}, p.errorAtCurrent("[Benchmark] function must return Void")
				}
				if len(pendingInlineData) > 0 {
					return ast.File{}, p.errorAtCurrent("[InlineData] cannot be used with [Benchmark]")
				}
				if pendingCycleTime != nil {
					return ast.File{}, p.errorAtCurrent("[CycleTime] is only valid on [Theory] functions")
				}
				function.IsBenchmark = true
				function.Suites = append(function.Suites, pendingSuites...)
				pendingBenchmark = false
				pendingSuites = pendingSuites[:0]
			}
			if pendingTheory {
				if len(function.Parameters) == 0 {
					return ast.File{}, p.errorAtCurrent("[Theory] function must declare at least one parameter")
				}
				if function.ReturnType.Name != "Void" || function.ReturnType.IsArray || function.ReturnType.VectorOf != nil || function.ReturnType.MatrixOf != nil || function.ReturnType.HasUnit || function.ReturnType.Package != "" {
					return ast.File{}, p.errorAtCurrent("[Theory] function must return Void")
				}
				if len(pendingInlineData) == 0 {
					return ast.File{}, p.errorAtCurrent("[Theory] function must declare at least one [InlineData] row")
				}
				function.IsTheory = true
				function.InlineData = append(function.InlineData, pendingInlineData...)
				function.Suites = append(function.Suites, pendingSuites...)
				function.CycleTime = pendingCycleTime
				pendingTheory = false
				pendingInlineData = pendingInlineData[:0]
				pendingSuites = pendingSuites[:0]
				pendingCycleTime = nil
			} else if len(pendingInlineData) > 0 {
				return ast.File{}, p.errorAtCurrent("[InlineData] must apply to a [Theory] function")
			} else if len(pendingSuites) > 0 {
				return ast.File{}, p.errorAtCurrent("[Suite] must apply to a [Fact], [Theory], [Artifact], or [Benchmark] function")
			} else if pendingCycleTime != nil {
				return ast.File{}, p.errorAtCurrent("[CycleTime] must apply to a [Theory] function")
			}
			file.Functions = append(file.Functions, function)
		case lex.KeywordFlow:
			if pendingFact || pendingTheory || pendingArtifact || pendingBenchmark || len(pendingInlineData) > 0 || len(pendingSuites) > 0 || pendingCycleTime != nil {
				return ast.File{}, p.errorAtCurrent("test attributes must apply to a function declaration")
			}
			flow, err := p.parseFlowDecl()
			if err != nil {
				return ast.File{}, err
			}
			file.Flows = append(file.Flows, flow)
		default:
			return ast.File{}, p.errorAtCurrent("expected 'record', 'enum', 'fn', or 'flow' at top level")
		}
	}
	if pendingFact || pendingTheory || pendingArtifact || pendingBenchmark || len(pendingInlineData) > 0 || len(pendingSuites) > 0 || pendingCycleTime != nil {
		return ast.File{}, p.errorAtCurrent("test attributes must apply to a function declaration")
	}
	return file, nil
}

func (p *parser) docCommentAtLine(line int) *ast.DocComment {
	doc, ok := p.docByLine[line]
	if !ok {
		return nil
	}
	docCopy := doc
	return &docCopy
}

func scanDocComments(src string) map[int]ast.DocComment {
	lines := strings.Split(src, "\n")
	attachments := make(map[int]ast.DocComment)
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "///") {
			i++
			continue
		}

		block := make([]string, 0, 4)
		for i < len(lines) {
			lineTrimmed := strings.TrimSpace(lines[i])
			if !strings.HasPrefix(lineTrimmed, "///") {
				break
			}
			block = append(block, strings.TrimSpace(strings.TrimPrefix(lineTrimmed, "///")))
			i++
		}

		if i >= len(lines) {
			break
		}
		next := strings.TrimSpace(lines[i])
		if next == "" || strings.HasPrefix(next, "//") {
			continue
		}
		attachments[i+1] = ast.DocComment{
			Lines:      block,
			Structured: extractDocSections(block),
		}
	}
	return attachments
}

func extractDocSections(lines []string) []ast.DocSection {
	sections := make([]ast.DocSection, 0, len(lines))
	for _, line := range lines {
		section, ok := parseDocSection(line)
		if !ok {
			continue
		}
		sections = append(sections, section)
	}
	return sections
}

func parseDocSection(line string) (ast.DocSection, bool) {
	keywords := []string{"Param ", "Returns:", "Units:", "Remarks:", "Example:"}
	for _, keyword := range keywords {
		if !strings.HasPrefix(line, keyword) {
			continue
		}
		switch keyword {
		case "Param ":
			remainder := strings.TrimSpace(strings.TrimPrefix(line, "Param "))
			colonIdx := strings.Index(remainder, ":")
			if colonIdx < 0 {
				return ast.DocSection{Keyword: "Param", Target: remainder}, true
			}
			return ast.DocSection{
				Keyword: "Param",
				Target:  strings.TrimSpace(remainder[:colonIdx]),
				Text:    strings.TrimSpace(remainder[colonIdx+1:]),
			}, true
		default:
			return ast.DocSection{
				Keyword: strings.TrimSuffix(keyword, ":"),
				Text:    strings.TrimSpace(strings.TrimPrefix(line, keyword)),
			}, true
		}
	}
	return ast.DocSection{}, false
}

type testAttribute struct {
	kind      string
	values    []ast.Expr
	value     ast.Expr
	suiteName string
}

func (p *parser) parseTestAttribute() (testAttribute, error) {
	if _, err := p.expect(lex.LeftBracket, "expected '['"); err != nil {
		return testAttribute{}, err
	}
	name, err := p.expect(lex.Identifier, "expected attribute name")
	if err != nil {
		return testAttribute{}, err
	}
	switch name.Lexeme {
	case "Fact", "Theory", "Artifact", "Benchmark":
		if _, err := p.expect(lex.RightBracket, "expected ']' after attribute"); err != nil {
			return testAttribute{}, err
		}
		return testAttribute{kind: name.Lexeme}, nil
	case "InlineData":
		if _, err := p.expect(lex.LeftParen, "expected '(' after InlineData"); err != nil {
			return testAttribute{}, err
		}
		values := make([]ast.Expr, 0)
		if p.current().Kind != lex.RightParen {
			for {
				value, err := p.parseExpression()
				if err != nil {
					return testAttribute{}, err
				}
				if !isInlineDataValueExpr(value) {
					return testAttribute{}, p.errorAtCurrent("[InlineData] supports only scalar literals and enum values in M24b")
				}
				values = append(values, value)
				if p.current().Kind != lex.Comma {
					break
				}
				p.advance()
			}
		}
		if _, err := p.expect(lex.RightParen, "expected ')' after InlineData arguments"); err != nil {
			return testAttribute{}, err
		}
		if _, err := p.expect(lex.RightBracket, "expected ']' after attribute"); err != nil {
			return testAttribute{}, err
		}
		return testAttribute{kind: "InlineData", values: values}, nil
	case "CycleTime":
		if _, err := p.expect(lex.LeftParen, "expected '(' after CycleTime"); err != nil {
			return testAttribute{}, err
		}
		value, err := p.parseExpression()
		if err != nil {
			return testAttribute{}, err
		}
		if _, err := p.expect(lex.RightParen, "expected ')' after CycleTime argument"); err != nil {
			return testAttribute{}, err
		}
		if _, err := p.expect(lex.RightBracket, "expected ']' after attribute"); err != nil {
			return testAttribute{}, err
		}
		return testAttribute{kind: "CycleTime", value: value}, nil
	case "Suite":
		if _, err := p.expect(lex.LeftParen, "expected '(' after Suite"); err != nil {
			return testAttribute{}, err
		}
		value, err := p.parseExpression()
		if err != nil {
			return testAttribute{}, err
		}
		stringLiteral, ok := value.(ast.StringLiteralExpr)
		if !ok || strings.TrimSpace(stringLiteral.Value) == "" {
			return testAttribute{}, p.errorAtCurrent("[Suite] requires a non-empty string literal")
		}
		if _, err := p.expect(lex.RightParen, "expected ')' after Suite argument"); err != nil {
			return testAttribute{}, err
		}
		if _, err := p.expect(lex.RightBracket, "expected ']' after attribute"); err != nil {
			return testAttribute{}, err
		}
		return testAttribute{kind: "Suite", suiteName: strings.TrimSpace(stringLiteral.Value)}, nil
	default:
		return testAttribute{}, p.errorAtToken(name, fmt.Sprintf("unsupported attribute [%s]", name.Lexeme))
	}
}

func isInlineDataValueExpr(expr ast.Expr) bool {
	switch expr.(type) {
	case ast.IntegerLiteral, ast.FloatLiteral, ast.BoolLiteral, ast.StringLiteralExpr, ast.FieldAccessExpr:
		return true
	default:
		return false
	}
}

func (p *parser) parseRecordDecl() (ast.RecordDecl, error) {
	recordToken, err := p.expect(lex.KeywordRecord, "expected 'record'")
	if err != nil {
		return ast.RecordDecl{}, err
	}
	name, err := p.expect(lex.Identifier, "expected record name")
	if err != nil {
		return ast.RecordDecl{}, err
	}
	if _, err := p.expect(lex.LeftBrace, "expected '{' after record name"); err != nil {
		return ast.RecordDecl{}, err
	}

	var fields []ast.RecordField
	for p.current().Kind != lex.RightBrace {
		if p.current().Kind == lex.EOF {
			return ast.RecordDecl{}, p.errorAtCurrent("expected '}' to close record declaration")
		}
		fieldName, err := p.expectIdentifierLike("expected record field name")
		if err != nil {
			return ast.RecordDecl{}, err
		}
		if _, err := p.expect(lex.Colon, "expected ':' after record field name"); err != nil {
			return ast.RecordDecl{}, err
		}
		fieldType, err := p.parseTypeRef()
		if err != nil {
			return ast.RecordDecl{}, err
		}
		fields = append(fields, ast.RecordField{
			Name: fieldName.Lexeme,
			Type: fieldType,
			Doc:  p.docCommentAtLine(fieldName.Line),
		})
	}
	p.advance()

	return ast.RecordDecl{
		Name:   name.Lexeme,
		Fields: fields,
		Doc:    p.docCommentAtLine(recordToken.Line),
	}, nil
}

func (p *parser) parseEnumDecl() (ast.EnumDecl, error) {
	enumToken, err := p.expect(lex.KeywordEnum, "expected 'enum'")
	if err != nil {
		return ast.EnumDecl{}, err
	}
	name, err := p.expect(lex.Identifier, "expected enum name")
	if err != nil {
		return ast.EnumDecl{}, err
	}
	if _, err := p.expect(lex.LeftBrace, "expected '{' after enum name"); err != nil {
		return ast.EnumDecl{}, err
	}

	var variants []ast.EnumVariantDecl
	for p.current().Kind != lex.RightBrace {
		if p.current().Kind == lex.EOF {
			return ast.EnumDecl{}, p.errorAtCurrent("expected '}' to close enum declaration")
		}
		variant, err := p.expect(lex.Identifier, "expected enum variant name")
		if err != nil {
			return ast.EnumDecl{}, err
		}
		var payload *ast.TypeRef
		if p.match(lex.LeftParen) {
			payloadType, err := p.parseTypeRef()
			if err != nil {
				return ast.EnumDecl{}, err
			}
			if _, err := p.expect(lex.RightParen, "expected ')' after enum variant payload type"); err != nil {
				return ast.EnumDecl{}, err
			}
			payload = &payloadType
		}
		variants = append(variants, ast.EnumVariantDecl{Name: variant.Lexeme, Payload: payload})
	}
	p.advance()

	return ast.EnumDecl{
		Name:     name.Lexeme,
		Variants: variants,
		Doc:      p.docCommentAtLine(enumToken.Line),
	}, nil
}

func (p *parser) parseFunctionDecl() (ast.FunctionDecl, error) {
	fnToken, err := p.expect(lex.KeywordFn, "expected 'fn' at top level")
	if err != nil {
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
	if _, err := p.expect(lex.Arrow, "expected arrow before return type"); err != nil {
		return ast.FunctionDecl{}, err
	}

	returnType, err := p.parseTypeRef()
	if err != nil {
		return ast.FunctionDecl{}, err
	}

	function := ast.FunctionDecl{
		Name:       name.Lexeme,
		Doc:        p.docCommentAtLine(fnToken.Line),
		SourcePath: p.sourcePath,
		Parameters: parameters,
		ReturnType: returnType,
	}
	if p.match(lex.Bang) {
		errorType, err := p.parseTypeRef()
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		function.IsFallible = true
		function.ErrorType = errorType
	}

	body, err := p.parseBlock()
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	function.Body = body

	return function, nil
}

func (p *parser) parseFlowDecl() (ast.FlowDecl, error) {
	if _, err := p.expect(lex.KeywordFlow, "expected 'flow' at top level"); err != nil {
		return ast.FlowDecl{}, err
	}
	name, err := p.expect(lex.Identifier, "expected flow name")
	if err != nil {
		return ast.FlowDecl{}, err
	}
	if _, err := p.expect(lex.LeftParen, "expected '(' after flow name"); err != nil {
		return ast.FlowDecl{}, err
	}
	parameters, err := p.parseParameters()
	if err != nil {
		return ast.FlowDecl{}, err
	}
	if _, err := p.expect(lex.RightParen, "expected ')' after parameter list"); err != nil {
		return ast.FlowDecl{}, err
	}
	if _, err := p.expect(lex.Arrow, "expected arrow before return type"); err != nil {
		return ast.FlowDecl{}, err
	}
	returnType, err := p.parseTypeRef()
	if err != nil {
		return ast.FlowDecl{}, err
	}
	if _, err := p.expect(lex.LeftBrace, "expected '{' after flow signature"); err != nil {
		return ast.FlowDecl{}, err
	}
	boardFields := make([]ast.BoardField, 0)
	if p.current().Kind == lex.Identifier && p.current().Lexeme == "board" {
		p.advance()
		if _, err := p.expect(lex.LeftBrace, "expected '{' after 'board'"); err != nil {
			return ast.FlowDecl{}, err
		}
		for p.current().Kind != lex.RightBrace {
			if p.current().Kind == lex.EOF {
				return ast.FlowDecl{}, p.errorAtCurrent("expected '}' to close board declaration")
			}
			fieldName, err := p.expect(lex.Identifier, "expected board field name")
			if err != nil {
				return ast.FlowDecl{}, err
			}
			if _, err := p.expect(lex.Colon, "expected ':' after board field name"); err != nil {
				return ast.FlowDecl{}, err
			}
			fieldType, err := p.parseTypeRef()
			if err != nil {
				return ast.FlowDecl{}, err
			}
			boardFields = append(boardFields, ast.BoardField{Name: fieldName.Lexeme, Type: fieldType})
		}
		p.advance()
	}
	states := make([]ast.StateDecl, 0)
	for p.current().Kind != lex.RightBrace {
		if p.current().Kind == lex.EOF {
			return ast.FlowDecl{}, p.errorAtCurrent("expected '}' to close flow declaration")
		}
		stateDecl, err := p.parseStateDecl()
		if err != nil {
			return ast.FlowDecl{}, err
		}
		states = append(states, stateDecl)
	}
	p.advance()
	flow := ast.FlowDecl{Name: name.Lexeme, Parameters: parameters, ReturnType: returnType, Board: boardFields, States: states}
	if len(states) > 0 {
		flow.EntryState = states[0].Name
	}
	return flow, nil
}

func (p *parser) parseStateDecl() (ast.StateDecl, error) {
	if _, err := p.expect(lex.KeywordState, "expected 'state' declaration inside flow"); err != nil {
		return ast.StateDecl{}, err
	}
	name, err := p.expect(lex.Identifier, "expected state name")
	if err != nil {
		return ast.StateDecl{}, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return ast.StateDecl{}, err
	}
	return ast.StateDecl{Name: name.Lexeme, Body: body}, nil
}

func (p *parser) parseParameters() ([]ast.Parameter, error) {
	if p.current().Kind == lex.RightParen {
		return nil, nil
	}

	var parameters []ast.Parameter
	for {
		name, err := p.expectIdentifierLike("expected parameter name")
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
	if p.match(lex.LeftParen) {
		return ast.TypeRef{}, p.errorAtCurrent("tuple types are not supported; use records for heterogeneous values")
	}

	if p.current().Kind == lex.KeywordFn {
		p.advance()
		if _, err := p.expect(lex.LeftParen, "expected '(' after 'fn' in function type"); err != nil {
			return ast.TypeRef{}, err
		}
		parameters := make([]ast.TypeRef, 0)
		if p.current().Kind != lex.RightParen {
			for {
				parameterType, err := p.parseTypeRef()
				if err != nil {
					return ast.TypeRef{}, err
				}
				parameters = append(parameters, parameterType)
				if !p.match(lex.Comma) {
					break
				}
			}
		}
		if _, err := p.expect(lex.RightParen, "expected ')' after function type parameter list"); err != nil {
			return ast.TypeRef{}, err
		}
		if _, err := p.expect(lex.Arrow, "expected arrow in function type"); err != nil {
			return ast.TypeRef{}, err
		}
		returnType, err := p.parseTypeRef()
		if err != nil {
			return ast.TypeRef{}, err
		}
		functionType := ast.FunctionTypeRef{Parameters: parameters, ReturnType: returnType}
		if p.match(lex.Bang) {
			errorType, err := p.parseTypeRef()
			if err != nil {
				return ast.TypeRef{}, err
			}
			functionType.IsFallible = true
			functionType.ErrorType = &errorType
		}
		return ast.TypeRef{Function: &functionType}, nil
	}

	token, err := p.expect(lex.Identifier, "expected type name")
	if err != nil {
		return ast.TypeRef{}, err
	}
	if token.Lexeme == "Vector" || token.Lexeme == "Matrix" {
		container := token.Lexeme
		if _, err := p.expect(lex.LeftAngle, fmt.Sprintf("expected '<' after %s", container)); err != nil {
			return ast.TypeRef{}, err
		}
		elementType, err := p.parseTypeRef()
		if err != nil {
			return ast.TypeRef{}, err
		}
		if _, err := p.expect(lex.RightAngle, fmt.Sprintf("expected '>' after %s element type", container)); err != nil {
			return ast.TypeRef{}, err
		}
		if container == "Vector" {
			return ast.TypeRef{VectorOf: &elementType}, nil
		}
		return ast.TypeRef{MatrixOf: &elementType}, nil
	}

	typeRef := ast.TypeRef{Name: token.Lexeme}
	if p.match(lex.Dot) {
		typeName, err := p.expect(lex.Identifier, "expected type name after '.'")
		if err != nil {
			return ast.TypeRef{}, err
		}
		typeRef.Package = token.Lexeme
		typeRef.Name = typeName.Lexeme
	}
	if p.match(lex.LeftAngle) {
		dim, err := p.parseDimensionSpec()
		if err != nil {
			return ast.TypeRef{}, err
		}
		if _, err := p.expect(lex.RightAngle, "expected '>' after dimension qualifier"); err != nil {
			return ast.TypeRef{}, err
		}
		typeRef.Dimension = dim
		typeRef.HasUnit = true
	}
	for p.match(lex.LeftBracket) {
		if _, err := p.expect(lex.RightBracket, "expected ']' after '[' in array type"); err != nil {
			return ast.TypeRef{}, err
		}
		typeRef.ArrayDepth++
	}
	typeRef.IsArray = typeRef.ArrayDepth > 0

	return typeRef, nil
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
	case lex.KeywordVar:
		return p.parseVarStmt()
	case lex.KeywordReturn:
		return p.parseReturnStmt()
	case lex.KeywordFor:
		return p.parseForStmt()
	case lex.KeywordMatch:
		return p.parseMatchStmt()
	case lex.KeywordIf:
		return p.parseIfStmt()
	case lex.KeywordWhile:
		return p.parseWhileStmt()
	case lex.KeywordPrometheus:
		return p.parsePrometheusStmt()
	case lex.KeywordGoto:
		return p.parseGotoStmt()
	case lex.KeywordSuspend:
		return p.parseSuspendStmt()
	case lex.KeywordRemember:
		return p.parseRememberStmt()
	case lex.KeywordResume:
		return p.parseResumeStmt()
	case lex.KeywordWhen:
		return p.parseWhenStmt()
	default:
		if p.isIdentifierLike(p.current().Kind) {
			if stmt, handled, err := p.tryParseIdentifierLeadingAssignment(); err != nil {
				return nil, err
			} else if handled {
				return stmt, nil
			}
		}
		if p.isExpressionStart(p.current().Kind) {
			return p.parseExprStmt()
		}
		return nil, p.errorAtCurrent("expected statement")
	}
}

func (p *parser) parsePrometheusStmt() (ast.Stmt, error) {
	p.advance()
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return ast.PrometheusStmt{Body: body}, nil
}

func (p *parser) parseWhenStmt() (ast.Stmt, error) {
	p.advance()
	if _, err := p.expect(lex.LeftBrace, "expected '{' after 'when'"); err != nil {
		return nil, err
	}

	cases := make([]ast.WhenCase, 0)
	var elseAction ast.WhenAction
	hasElse := false
	for p.current().Kind != lex.RightBrace {
		if p.current().Kind == lex.EOF {
			return nil, p.errorAtCurrent("expected '}' to close when")
		}
		switch p.current().Kind {
		case lex.KeywordCase:
			if hasElse {
				return nil, p.errorAtCurrent("case arms must come before else arm")
			}
			p.advance()
			condition, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lex.Arrow, "expected arrow after case condition"); err != nil {
				return nil, err
			}
			action, err := p.parseWhenAction()
			if err != nil {
				return nil, err
			}
			cases = append(cases, ast.WhenCase{Condition: condition, Action: action})
		case lex.KeywordElse:
			if hasElse {
				return nil, p.errorAtCurrent("when can only have one else arm")
			}
			p.advance()
			if _, err := p.expect(lex.Arrow, "expected arrow after else"); err != nil {
				return nil, err
			}
			action, err := p.parseWhenAction()
			if err != nil {
				return nil, err
			}
			elseAction = action
			hasElse = true
		default:
			return nil, p.errorAtCurrent("expected 'case' or 'else' in when")
		}
	}
	p.advance()
	return ast.WhenStmt{Cases: cases, Else: elseAction}, nil
}

func (p *parser) parseWhenAction() (ast.WhenAction, error) {
	if p.current().Kind == lex.LeftBrace {
		block, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return ast.WhenBlockAction{Statements: block.Statements}, nil
	}
	switch p.current().Kind {
	case lex.KeywordGoto:
		p.advance()
		target, err := p.expect(lex.Identifier, "expected state name after 'goto'")
		if err != nil {
			return nil, err
		}
		return ast.WhenGotoAction{Target: target.Lexeme}, nil
	case lex.KeywordSuspend:
		p.advance()
		return ast.WhenSuspendAction{}, nil
	case lex.KeywordReturn:
		p.advance()
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return ast.WhenReturnAction{Value: value}, nil
	default:
		return nil, p.errorAtCurrent("expected 'goto', 'suspend', or 'return' in when branch")
	}
}

func (p *parser) parseGotoStmt() (ast.Stmt, error) {
	p.advance()
	target, err := p.expect(lex.Identifier, "expected state name after 'goto'")
	if err != nil {
		return nil, err
	}
	return ast.GotoStmt{Target: target.Lexeme}, nil
}

func (p *parser) parseSuspendStmt() (ast.Stmt, error) {
	p.advance()
	return ast.SuspendStmt{}, nil
}

func (p *parser) parseRememberStmt() (ast.Stmt, error) {
	p.advance()
	return ast.RememberStmt{}, nil
}

func (p *parser) parseResumeStmt() (ast.Stmt, error) {
	p.advance()
	return ast.ResumeStmt{}, nil
}

func (p *parser) parseLetStmt() (ast.Stmt, error) {
	p.advance()
	name, err := p.expectIdentifierLike("expected identifier after 'let'")
	if err != nil {
		return nil, err
	}
	var typeHint *ast.TypeRef
	if p.match(lex.Colon) {
		parsedType, typeErr := p.parseTypeRef()
		if typeErr != nil {
			return nil, typeErr
		}
		typeHint = &parsedType
	}
	if _, err := p.expect(lex.Assign, "expected '=' after binding name"); err != nil {
		return nil, err
	}
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return ast.LetStmt{Name: name.Lexeme, TypeHint: typeHint, Value: value}, nil
}

func (p *parser) parseVarStmt() (ast.Stmt, error) {
	p.advance()
	name, err := p.expectIdentifierLike("expected identifier after 'var'")
	if err != nil {
		return nil, err
	}
	var typeHint *ast.TypeRef
	if p.match(lex.Colon) {
		parsedType, typeErr := p.parseTypeRef()
		if typeErr != nil {
			return nil, typeErr
		}
		typeHint = &parsedType
	}
	if _, err := p.expect(lex.Assign, "expected '=' after binding name"); err != nil {
		return nil, err
	}
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return ast.VarStmt{Name: name.Lexeme, TypeHint: typeHint, Value: value}, nil
}

func (p *parser) parseAssignStmt() (ast.Stmt, error) {
	name, err := p.expectIdentifierLike("expected assignment target")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lex.Assign, "expected '=' after assignment target"); err != nil {
		return nil, err
	}
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return ast.AssignStmt{Name: name.Lexeme, Value: value}, nil
}

func (p *parser) tryParseIdentifierLeadingAssignment() (ast.Stmt, bool, error) {
	savedPosition := p.position
	name, err := p.expectIdentifierLike("expected assignment target")
	if err != nil {
		return nil, false, err
	}

	if p.match(lex.Assign) {
		value, err := p.parseExpression()
		if err != nil {
			return nil, false, err
		}
		if p.match(lex.Comma) {
			return nil, false, p.errorAtCurrent("destructuring assignment requires exactly one right-hand expression")
		}
		return ast.AssignStmt{Name: name.Lexeme, Value: value}, true, nil
	}

	if p.match(lex.Comma) {
		return nil, false, p.errorAtCurrent("destructuring assignment is not supported; use a record return value")
	}

	if p.match(lex.Dot) {
		field, err := p.expectIdentifierLike("expected field name after '.'")
		if err != nil {
			return nil, false, err
		}
		if !p.match(lex.Assign) {
			p.position = savedPosition
			return nil, false, nil
		}
		value, err := p.parseExpression()
		if err != nil {
			return nil, false, err
		}
		return ast.FieldAssignStmt{Target: name.Lexeme, Field: field.Lexeme, Value: value}, true, nil
	}

	if !p.match(lex.LeftBracket) {
		p.position = savedPosition
		return nil, false, nil
	}

	indices := make([]ast.Expr, 0, 2)
	index, err := p.parseExpression()
	if err != nil {
		return nil, false, err
	}
	indices = append(indices, index)
	for p.match(lex.Comma) {
		nextIndex, nextErr := p.parseExpression()
		if nextErr != nil {
			return nil, false, nextErr
		}
		indices = append(indices, nextIndex)
	}
	if _, err := p.expect(lex.RightBracket, "expected ']' after index assignment target"); err != nil {
		return nil, false, err
	}
	if p.current().Kind == lex.LeftBracket {
		return nil, false, p.errorAtCurrent("nested index assignment targets are not supported")
	}
	if !p.match(lex.Assign) {
		p.position = savedPosition
		return nil, false, nil
	}

	value, err := p.parseExpression()
	if err != nil {
		return nil, false, err
	}
	return ast.IndexAssignStmt{Target: name.Lexeme, Indices: indices, Value: value}, true, nil
}

func (p *parser) parseReturnStmt() (ast.Stmt, error) {
	p.advance()
	if !p.isExpressionStart(p.current().Kind) {
		return ast.ReturnStmt{}, nil
	}
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return ast.ReturnStmt{Value: value}, nil
}

func (p *parser) parseExprStmt() (ast.Stmt, error) {
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return ast.ExprStmt{Value: value}, nil
}

func (p *parser) parseForStmt() (ast.Stmt, error) {
	p.advance()
	name, err := p.expectIdentifierLike("expected loop variable after 'for'")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lex.KeywordIn, "expected 'in' after loop variable"); err != nil {
		return nil, err
	}
	rangeExpr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return ast.ForStmt{Name: name.Lexeme, Range: rangeExpr, Body: body}, nil
}

func (p *parser) parseMatchStmt() (ast.Stmt, error) {
	p.advance()
	subject, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lex.LeftBrace, "expected '{' to start match"); err != nil {
		return nil, err
	}

	okName, okBody, err := p.parseMatchArm("ok")
	if err != nil {
		return nil, err
	}
	errName, errBody, err := p.parseMatchArm("err")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lex.RightBrace, "expected '}' to close match"); err != nil {
		return nil, err
	}

	return ast.MatchStmt{Subject: subject, OkName: okName, OkBody: okBody, ErrName: errName, ErrBody: errBody}, nil
}

func (p *parser) parseIfStmt() (ast.Stmt, error) {
	p.advance()
	condition, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	thenBody, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	var elseBody *ast.Block
	if p.match(lex.KeywordElse) {
		block, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		elseBody = &block
	}

	return ast.IfStmt{Condition: condition, ThenBody: thenBody, ElseBody: elseBody}, nil
}

func (p *parser) parseWhileStmt() (ast.Stmt, error) {
	p.advance()
	condition, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return ast.WhileStmt{Condition: condition, Body: body}, nil
}

func (p *parser) isExpressionStart(kind lex.TokenKind) bool {
	switch kind {
	case lex.IntLiteral, lex.FloatLiteral, lex.KeywordTrue, lex.KeywordFalse, lex.StringLiteral, lex.Identifier, lex.KeywordFlow, lex.KeywordState, lex.KeywordStep, lex.LeftParen, lex.LeftBracket, lex.KeywordSwitch, lex.KeywordIf, lex.KeywordBatch, lex.KeywordWhen, lex.KeywordMatch, lex.KeywordNot, lex.Minus:
		return true
	default:
		return false
	}
}

func (p *parser) parseMatchArm(expectedName string) (string, ast.Block, error) {
	name, err := p.expect(lex.Identifier, fmt.Sprintf("expected '%s' arm", expectedName))
	if err != nil {
		return "", ast.Block{}, err
	}
	if name.Lexeme != expectedName {
		return "", ast.Block{}, p.errorAtToken(name, fmt.Sprintf("expected '%s' arm", expectedName))
	}
	if _, err := p.expect(lex.LeftParen, fmt.Sprintf("expected '(' after '%s'", expectedName)); err != nil {
		return "", ast.Block{}, err
	}
	binding, err := p.expectIdentifierLike(fmt.Sprintf("expected identifier in %s arm", expectedName))
	if err != nil {
		return "", ast.Block{}, err
	}
	if _, err := p.expect(lex.RightParen, fmt.Sprintf("expected ')' after %s binding", expectedName)); err != nil {
		return "", ast.Block{}, err
	}
	if _, err := p.expect(lex.Arrow, fmt.Sprintf("expected arrow after %s arm", expectedName)); err != nil {
		return "", ast.Block{}, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return "", ast.Block{}, err
	}
	return binding.Lexeme, body, nil
}

func (p *parser) parseExpression() (ast.Expr, error) {
	return p.parseRangeExpr()
}

func (p *parser) parseRangeExpr() (ast.Expr, error) {
	start, err := p.parseBinaryExpr(precedenceOr)
	if err != nil {
		return nil, err
	}
	if !p.match(lex.DotDot) {
		return start, nil
	}

	end, err := p.parseBinaryExpr(precedenceOr)
	if err != nil {
		return nil, err
	}

	var step ast.Expr
	if p.match(lex.KeywordStep) {
		step, err = p.parseBinaryExpr(precedenceOr)
		if err != nil {
			return nil, err
		}
	}

	return ast.RangeExpr{Start: start, End: end, Step: step}, nil
}

func (p *parser) parseBinaryExpr(minPrecedence int) (ast.Expr, error) {
	left, err := p.parsePrefixExpr()
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

func (p *parser) parsePrefixExpr() (ast.Expr, error) {
	if p.current().Kind == lex.Bang {
		return nil, p.errorAtCurrent("'!' is not boolean not in Oct; use 'not' (postfix 'expr!' remains valid unwrap syntax)")
	}
	if p.match(lex.Minus) {
		operand, err := p.parsePrefixExpr()
		if err != nil {
			return nil, err
		}
		return ast.UnaryExpr{Operator: "-", Operand: operand}, nil
	}
	if p.match(lex.KeywordNot) {
		operand, err := p.parseBinaryExpr(precedenceComparison)
		if err != nil {
			return nil, err
		}
		return ast.UnaryExpr{Operator: "not", Operand: operand}, nil
	}
	return p.parsePostfixExpr()
}

func (p *parser) parsePostfixExpr() (ast.Expr, error) {
	expr, err := p.parsePrimaryExpr()
	if err != nil {
		return nil, err
	}

	for {
		switch {
		case p.current().Kind == lex.LeftParen:
			arguments, err := p.parseCallArguments()
			if err != nil {
				return nil, err
			}
			expr = ast.CallExpr{Callee: expr, Arguments: arguments}
		case p.current().Kind == lex.LeftAngle && p.looksLikeTypeArgumentList():
			typeArguments, err := p.parseTypeArguments()
			if err != nil {
				return nil, err
			}
			arguments, err := p.parseCallArguments()
			if err != nil {
				return nil, err
			}
			expr = ast.CallExpr{Callee: expr, TypeArguments: typeArguments, Arguments: arguments}
		case p.current().Kind == lex.LeftBracket && p.looksLikeLegacyTypeArgumentList():
			return nil, p.errorAtCurrent("type arguments must use '<...>'; legacy '[...]' syntax is no longer supported")
		case p.current().Kind == lex.LeftBrace && p.looksLikeRecordLiteral() && p.isRecordLiteralTypeExpr(expr):
			typeName, err := p.flattenTypeExpr(expr)
			if err != nil {
				return nil, err
			}
			expr, err = p.parseRecordLiteralExpr(typeName)
			if err != nil {
				return nil, err
			}
		case p.match(lex.KeywordWith):
			fields, err := p.parseRecordLiteralFields("record update", true)
			if err != nil {
				return nil, err
			}
			expr = ast.RecordUpdateExpr{Source: expr, Fields: fields}
		case p.match(lex.Dot):
			field, err := p.expectIdentifierLike("expected field name after '.'")
			if err != nil {
				return nil, err
			}
			expr = ast.FieldAccessExpr{Target: expr, Field: field.Lexeme}
		case p.match(lex.LeftBracket):
			var indices []ast.Expr
			for {
				index, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				indices = append(indices, index)
				if !p.match(lex.Comma) {
					break
				}
			}
			if _, err := p.expect(lex.RightBracket, "expected ']' after index expression"); err != nil {
				return nil, err
			}
			expr = ast.IndexExpr{Target: expr, Indices: indices}
		case p.match(lex.Question):
			expr = ast.PropagateExpr{Inner: expr}
		case p.match(lex.Bang):
			expr = ast.UnwrapExpr{Inner: expr}
		default:
			return expr, nil
		}
	}
}

func (p *parser) looksLikeTypeArgumentList() bool {
	start := p.position
	p.advance()
	if p.current().Kind == lex.RightAngle {
		p.position = start
		return false
	}
	if _, err := p.parseTypeRef(); err != nil {
		p.position = start
		return false
	}
	if p.current().Kind != lex.RightAngle {
		p.position = start
		return false
	}
	p.advance()
	isTypeArgCall := p.current().Kind == lex.LeftParen
	p.position = start
	return isTypeArgCall
}

func (p *parser) looksLikeLegacyTypeArgumentList() bool {
	start := p.position
	p.advance()
	if p.current().Kind == lex.RightBracket {
		p.position = start
		return false
	}
	if _, err := p.parseTypeRef(); err != nil {
		p.position = start
		return false
	}
	if p.current().Kind != lex.RightBracket {
		p.position = start
		return false
	}
	p.advance()
	isTypeArgCall := p.current().Kind == lex.LeftParen
	p.position = start
	return isTypeArgCall
}

func (p *parser) parseTypeArguments() ([]ast.TypeRef, error) {
	if _, err := p.expect(lex.LeftAngle, "expected '<' before type arguments"); err != nil {
		return nil, err
	}
	typeArgument, err := p.parseTypeRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lex.RightAngle, "expected '>' after type arguments"); err != nil {
		return nil, err
	}
	return []ast.TypeRef{typeArgument}, nil
}

func (p *parser) isRecordLiteralTypeExpr(expr ast.Expr) bool {
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		return true
	case ast.FieldAccessExpr:
		_, ok := node.Target.(ast.IdentifierExpr)
		return ok
	default:
		return false
	}
}

func (p *parser) flattenTypeExpr(expr ast.Expr) (string, error) {
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		return node.Name, nil
	case ast.FieldAccessExpr:
		left, ok := node.Target.(ast.IdentifierExpr)
		if !ok {
			return "", p.errorAtCurrent("qualified symbol form is not supported here")
		}
		return left.Name + "." + node.Field, nil
	default:
		return "", p.errorAtCurrent("qualified symbol form is not supported here")
	}
}

func (p *parser) parseCallArguments() ([]ast.Expr, error) {
	if _, err := p.expect(lex.LeftParen, "expected '(' after function name"); err != nil {
		return nil, err
	}
	if p.current().Kind == lex.RightParen {
		p.advance()
		return nil, nil
	}

	var arguments []ast.Expr
	for {
		argument, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, argument)
		if !p.match(lex.Comma) {
			break
		}
	}
	if _, err := p.expect(lex.RightParen, "expected ')' after argument list"); err != nil {
		return nil, err
	}
	return arguments, nil
}

func (p *parser) parsePrimaryExpr() (ast.Expr, error) {
	token := p.current()
	switch token.Kind {
	case lex.KeywordSwitch:
		return p.parseSwitchExpr()
	case lex.KeywordIf:
		return p.parseIfExpr()
	case lex.KeywordBatch:
		return p.parseBatchExpr()
	case lex.KeywordWhen:
		return p.parseUtilityWhenExpr()
	case lex.KeywordMatch:
		return p.parseMatchExpr()
	case lex.IntLiteral:
		p.advance()
		if p.current().Kind == lex.Identifier && p.current().Lexeme == "C" && tokensAdjacent(token, p.current()) && !literalSuffixContinuesAsDimensionSpec(p.peek(1)) {
			p.advance()
			intValue, err := strconv.ParseInt(token.Lexeme, 10, 64)
			if err != nil {
				return nil, p.errorAtToken(token, "invalid integer literal")
			}
			kelvin := float64(intValue) + 273.15
			kelvinDim, _ := dimension.FromBaseName("K")
			return ast.FloatLiteral{Value: strconv.FormatFloat(kelvin, 'g', -1, 64), Dimension: kelvinDim, HasUnit: true}, nil
		}
		if p.current().Kind == lex.Identifier && p.current().Lexeme == "deg" && tokensAdjacent(token, p.current()) {
			p.advance()
			intValue, err := strconv.ParseInt(token.Lexeme, 10, 64)
			if err != nil {
				return nil, p.errorAtToken(token, "invalid integer literal")
			}
			radians := float64(intValue) * math.Pi / 180.0
			return ast.FloatLiteral{Value: strconv.FormatFloat(radians, 'g', -1, 64), Dimension: dimension.Zero(), HasUnit: false}, nil
		}
		dim, hasUnit, err := p.parseLiteralUnitSuffix(token)
		if err != nil {
			return nil, err
		}
		return ast.IntegerLiteral{Value: token.Lexeme, Dimension: dim, HasUnit: hasUnit}, nil
	case lex.FloatLiteral:
		p.advance()
		if p.current().Kind == lex.Identifier && p.current().Lexeme == "C" && tokensAdjacent(token, p.current()) && !literalSuffixContinuesAsDimensionSpec(p.peek(1)) {
			p.advance()
			floatValue, err := strconv.ParseFloat(token.Lexeme, 64)
			if err != nil {
				return nil, p.errorAtToken(token, "invalid float literal")
			}
			kelvin := floatValue + 273.15
			kelvinDim, _ := dimension.FromBaseName("K")
			return ast.FloatLiteral{Value: strconv.FormatFloat(kelvin, 'g', -1, 64), Dimension: kelvinDim, HasUnit: true}, nil
		}
		if p.current().Kind == lex.Identifier && p.current().Lexeme == "deg" && tokensAdjacent(token, p.current()) {
			p.advance()
			floatValue, err := strconv.ParseFloat(token.Lexeme, 64)
			if err != nil {
				return nil, p.errorAtToken(token, "invalid float literal")
			}
			radians := floatValue * math.Pi / 180.0
			return ast.FloatLiteral{Value: strconv.FormatFloat(radians, 'g', -1, 64), Dimension: dimension.Zero(), HasUnit: false}, nil
		}
		dim, hasUnit, err := p.parseLiteralUnitSuffix(token)
		if err != nil {
			return nil, err
		}
		return ast.FloatLiteral{Value: token.Lexeme, Dimension: dim, HasUnit: hasUnit}, nil
	case lex.StringLiteral:
		p.advance()
		return ast.StringLiteralExpr{Value: token.Lexeme}, nil
	case lex.KeywordTrue:
		p.advance()
		return ast.BoolLiteral{Value: true}, nil
	case lex.KeywordFalse:
		p.advance()
		return ast.BoolLiteral{Value: false}, nil
	case lex.Identifier, lex.KeywordFlow, lex.KeywordState, lex.KeywordStep:
		p.advance()
		if token.Lexeme == "vector" && p.current().Kind == lex.LeftBracket {
			return p.parseVectorLiteralExpr()
		}
		if token.Lexeme == "matrix" && p.current().Kind == lex.LeftBracket {
			return p.parseMatrixLiteralExpr()
		}
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
	case lex.LeftBracket:
		return p.parseArrayLiteralExpr()
	default:
		return nil, p.errorAtCurrent("expected expression")
	}
}

func (p *parser) parseBatchExpr() (ast.Expr, error) {
	p.advance()
	input, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lex.KeywordAs, "expected 'as' after batch input expression"); err != nil {
		return nil, err
	}
	itemName, err := p.expectIdentifierLike("expected item binding name after 'as'")
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return ast.BatchExpr{Input: input, ItemName: itemName.Lexeme, Body: body}, nil
}

func (p *parser) parseUtilityWhenExpr() (ast.Expr, error) {
	p.advance()
	modeToken, err := p.expect(lex.Identifier, "expected 'policy' or 'utility' after 'when'")
	if err != nil {
		return nil, err
	}
	var (
		policy          ast.UtilityWhenPolicy
		controllerBound bool
	)
	switch modeToken.Lexeme {
	case "policy":
		controllerBound = true
		policy, err = p.parseUtilityWhenPolicy(true)
		if err != nil {
			return nil, err
		}
	case "utility":
		controllerBound = false
		policy, err = p.parseStandaloneUtilityWhenPolicy()
		if err != nil {
			return nil, err
		}
	default:
		return nil, p.errorAtToken(modeToken, "expected 'policy' or 'utility' after 'when'")
	}
	if _, err := p.expect(lex.LeftBrace, "expected '{' to start utility when cases"); err != nil {
		return nil, err
	}

	cases := make([]ast.UtilityWhenCase, 0)
	var elseValue ast.Expr
	hasElse := false
	for p.current().Kind != lex.RightBrace {
		if p.current().Kind == lex.EOF {
			return nil, p.errorAtCurrent("expected '}' to close utility when")
		}
		switch p.current().Kind {
		case lex.KeywordCase:
			if hasElse {
				return nil, p.errorAtCurrent("case arms must come before else arm")
			}
			p.advance()
			value, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lex.KeywordWhen, "expected 'when' after utility case value"); err != nil {
				return nil, err
			}
			condition, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			scoreToken, err := p.expect(lex.Identifier, "expected 'score' after utility case condition")
			if err != nil {
				return nil, err
			}
			if scoreToken.Lexeme != "score" {
				return nil, p.errorAtToken(scoreToken, "expected 'score' after utility case condition")
			}
			score, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			cases = append(cases, ast.UtilityWhenCase{Value: value, Condition: condition, Score: score})
		case lex.KeywordElse:
			if hasElse {
				return nil, p.errorAtCurrent("utility when can only have one else arm")
			}
			p.advance()
			value, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			elseValue = value
			hasElse = true
		default:
			return nil, p.errorAtCurrent("expected 'case' or 'else' in utility when")
		}
	}
	p.advance()
	if !hasElse {
		return nil, p.errorAtCurrent("utility when requires else arm")
	}
	siteID := p.nextUtilityWhenSiteID
	p.nextUtilityWhenSiteID++
	return ast.UtilityWhenExpr{
		SiteID:          siteID,
		Policy:          policy,
		Cases:           cases,
		Else:            elseValue,
		ControllerBound: controllerBound,
	}, nil
}

func (p *parser) parseStandaloneUtilityWhenPolicy() (ast.UtilityWhenPolicy, error) {
	if p.current().Kind != lex.LeftBrace {
		return ast.UtilityWhenPolicy{
			Hysteresis: ast.IntegerLiteral{Value: "0"},
			MinCommit:  ast.IntegerLiteral{Value: "0"},
		}, nil
	}
	if p.position+1 >= len(p.tokens) || p.tokens[p.position+1].Kind != lex.Identifier {
		return ast.UtilityWhenPolicy{
			Hysteresis: ast.IntegerLiteral{Value: "0"},
			MinCommit:  ast.IntegerLiteral{Value: "0"},
		}, nil
	}
	return p.parseUtilityWhenPolicy(false)
}

func (p *parser) parseUtilityWhenPolicy(requireAllFields bool) (ast.UtilityWhenPolicy, error) {
	if _, err := p.expect(lex.LeftBrace, "expected '{' to start utility when policy"); err != nil {
		return ast.UtilityWhenPolicy{}, err
	}
	var (
		hysteresis ast.Expr
		minCommit  ast.Expr
	)
	for p.current().Kind != lex.RightBrace {
		if p.current().Kind == lex.EOF {
			return ast.UtilityWhenPolicy{}, p.errorAtCurrent("expected '}' to close utility when policy")
		}
		field, err := p.expect(lex.Identifier, "expected utility policy field name")
		if err != nil {
			return ast.UtilityWhenPolicy{}, err
		}
		if _, err := p.expect(lex.Colon, "expected ':' after utility policy field name"); err != nil {
			return ast.UtilityWhenPolicy{}, err
		}
		value, err := p.parseExpression()
		if err != nil {
			return ast.UtilityWhenPolicy{}, err
		}
		switch field.Lexeme {
		case "hysteresis":
			if hysteresis != nil {
				return ast.UtilityWhenPolicy{}, p.errorAtToken(field, "duplicate utility policy field 'hysteresis'")
			}
			hysteresis = value
		case "min_commit":
			if minCommit != nil {
				return ast.UtilityWhenPolicy{}, p.errorAtToken(field, "duplicate utility policy field 'min_commit'")
			}
			minCommit = value
		default:
			return ast.UtilityWhenPolicy{}, p.errorAtToken(field, fmt.Sprintf("unsupported utility policy field '%s'", field.Lexeme))
		}
	}
	p.advance()
	if hysteresis == nil && requireAllFields {
		return ast.UtilityWhenPolicy{}, p.errorAtCurrent("utility policy requires 'hysteresis'")
	}
	if minCommit == nil && requireAllFields {
		return ast.UtilityWhenPolicy{}, p.errorAtCurrent("utility policy requires 'min_commit'")
	}
	if hysteresis == nil {
		hysteresis = ast.IntegerLiteral{Value: "0"}
	}
	if minCommit == nil {
		minCommit = ast.IntegerLiteral{Value: "0"}
	}
	return ast.UtilityWhenPolicy{Hysteresis: hysteresis, MinCommit: minCommit}, nil
}

func (p *parser) parseIfExpr() (ast.Expr, error) {
	p.advance()
	condition, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	thenExpr, err := p.parseIfExprBranch("then")
	if err != nil {
		return nil, err
	}
	if !p.match(lex.KeywordElse) {
		return nil, p.errorAtCurrent("if expression requires else branch")
	}
	elseExpr, err := p.parseIfExprBranch("else")
	if err != nil {
		return nil, err
	}
	return ast.IfExpr{Condition: condition, ThenExpr: thenExpr, ElseExpr: elseExpr}, nil
}

func (p *parser) parseIfExprBranch(name string) (ast.Expr, error) {
	if name == "else" && p.current().Kind == lex.KeywordIf {
		return nil, p.errorAtCurrent("Oct does not support `else if`; use a switch expression for multi-way branching, or put a nested `if` inside the `else { ... }` block.")
	}
	if _, err := p.expect(lex.LeftBrace, fmt.Sprintf("expected '{' to start if expression %s branch", name)); err != nil {
		return nil, err
	}
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lex.RightBrace, "expected '}' to close if expression branch"); err != nil {
		return nil, err
	}
	return value, nil
}

func (p *parser) looksLikeRecordLiteral() bool {
	if p.peek(0).Kind != lex.LeftBrace {
		return false
	}
	return p.isIdentifierLike(p.peek(1).Kind) && p.peek(2).Kind == lex.Colon
}

func (p *parser) parseRecordLiteralExpr(typeName string) (ast.Expr, error) {
	fields, err := p.parseRecordLiteralFields("record literal", false)
	if err != nil {
		return nil, err
	}
	return ast.RecordLiteralExpr{TypeName: typeName, Fields: fields}, nil
}

func (p *parser) parseRecordLiteralFields(context string, requireAtLeastOne bool) ([]ast.RecordLiteralField, error) {
	if _, err := p.expect(lex.LeftBrace, "expected '{' to start "+context); err != nil {
		return nil, err
	}

	var fields []ast.RecordLiteralField
	for p.current().Kind != lex.RightBrace {
		if p.current().Kind == lex.EOF {
			return nil, p.errorAtCurrent("expected '}' to close " + context)
		}
		name, err := p.expectIdentifierLike("expected " + context + " field name")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lex.Colon, "expected ':' after "+context+" field name"); err != nil {
			return nil, err
		}
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		fields = append(fields, ast.RecordLiteralField{Name: name.Lexeme, Value: value})
	}
	p.advance()
	if requireAtLeastOne && len(fields) == 0 {
		return nil, p.errorAtCurrent(context + " requires at least one field")
	}
	return fields, nil
}

func (p *parser) parseSwitchExpr() (ast.Expr, error) {
	p.advance()
	var (
		subject           ast.Expr
		err               error
		isConditionSwitch bool
	)
	if p.current().Kind == lex.LeftBrace {
		isConditionSwitch = true
	} else {
		subject, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(lex.LeftBrace, "expected '{' to start switch"); err != nil {
		return nil, err
	}

	var cases []ast.SwitchCase
	var elseValue ast.Expr
	hasElse := false
	for p.current().Kind != lex.RightBrace {
		if p.current().Kind == lex.EOF {
			return nil, p.errorAtCurrent("expected '}' to close switch")
		}
		switch p.current().Kind {
		case lex.KeywordCase:
			if hasElse {
				return nil, p.errorAtCurrent("case arms must come before else arm")
			}
			p.advance()
			var match ast.Expr
			if isConditionSwitch {
				match, err = p.parseExpression()
				if err != nil {
					return nil, err
				}
			} else {
				match, err = p.parseSwitchCaseLabel()
				if err != nil {
					return nil, err
				}
			}
			if _, err := p.expect(lex.Arrow, "expected arrow after case label"); err != nil {
				return nil, err
			}
			value, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			cases = append(cases, ast.SwitchCase{Match: match, Value: value})
		case lex.KeywordElse:
			if hasElse {
				return nil, p.errorAtCurrent("switch can only have one else arm")
			}
			p.advance()
			if _, err := p.expect(lex.Arrow, "expected arrow after else"); err != nil {
				return nil, err
			}
			value, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			elseValue = value
			hasElse = true
		default:
			return nil, p.errorAtCurrent("expected 'case' or 'else' in switch")
		}
	}
	p.advance()

	return ast.SwitchExpr{Subject: subject, Cases: cases, Else: elseValue}, nil
}

func (p *parser) parseSwitchCaseLabel() (ast.Expr, error) {
	token := p.current()
	switch token.Kind {
	case lex.IntLiteral, lex.FloatLiteral, lex.StringLiteral, lex.KeywordTrue, lex.KeywordFalse:
		return p.parsePrimaryExpr()
	case lex.Identifier:
		label := ast.Expr(ast.IdentifierExpr{Name: token.Lexeme})
		p.advance()
		for p.match(lex.Dot) {
			segment, err := p.expect(lex.Identifier, "expected identifier after '.' in case label")
			if err != nil {
				return nil, err
			}
			label = ast.FieldAccessExpr{Target: label, Field: segment.Lexeme}
		}
		if _, ok := label.(ast.FieldAccessExpr); !ok {
			return nil, p.errorAtCurrent("switch case enum label must be qualified as EnumName.Variant")
		}
		return label, nil
	default:
		return nil, p.errorAtCurrent("switch case must use int, float, bool, string literal, or qualified enum variant")
	}
}

func (p *parser) parseMatchExpr() (ast.Expr, error) {
	p.advance()
	subject, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lex.LeftBrace, "expected '{' to start match"); err != nil {
		return nil, err
	}
	cases := make([]ast.MatchCase, 0)
	for p.current().Kind != lex.RightBrace {
		if p.current().Kind == lex.EOF {
			return nil, p.errorAtCurrent("expected '}' to close match")
		}
		if _, err := p.expect(lex.KeywordCase, "expected 'case' in match"); err != nil {
			return nil, err
		}
		caseLabel, err := p.parseSwitchCaseLabel()
		if err != nil {
			return nil, err
		}
		labelAccess, ok := caseLabel.(ast.FieldAccessExpr)
		if !ok {
			return nil, p.errorAtCurrent("match case must use qualified enum variant")
		}
		variant := labelAccess.Field
		binding := ""
		if p.match(lex.LeftParen) {
			bindingToken, err := p.expect(lex.Identifier, "expected payload binding name")
			if err != nil {
				return nil, err
			}
			binding = bindingToken.Lexeme
			if _, err := p.expect(lex.RightParen, "expected ')' after payload binding name"); err != nil {
				return nil, err
			}
		}
		if _, err := p.expect(lex.Arrow, "expected arrow after match case"); err != nil {
			return nil, err
		}
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		cases = append(cases, ast.MatchCase{Variant: variant, Binding: binding, Value: value})
	}
	p.advance()
	return ast.MatchExpr{Subject: subject, Cases: cases}, nil
}

func (p *parser) parseArrayLiteralExpr() (ast.Expr, error) {
	if _, err := p.expect(lex.LeftBracket, "expected '[' to start array literal"); err != nil {
		return nil, err
	}
	if p.current().Kind == lex.RightBracket {
		p.advance()
		return ast.ArrayLiteralExpr{Elements: []ast.Expr{}}, nil
	}

	var elements []ast.Expr
	for {
		element, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)

		if !p.match(lex.Comma) {
			break
		}
	}

	if _, err := p.expect(lex.RightBracket, "expected ']' after array literal"); err != nil {
		return nil, err
	}

	return ast.ArrayLiteralExpr{Elements: elements}, nil
}

func (p *parser) parseVectorLiteralExpr() (ast.Expr, error) {
	if _, err := p.expect(lex.LeftBracket, "expected '[' to start vector literal"); err != nil {
		return nil, err
	}
	if p.current().Kind == lex.RightBracket {
		return nil, p.errorAtCurrent("empty vector literals are not supported")
	}
	var elements []ast.Expr
	for {
		element, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
		if !p.match(lex.Comma) {
			break
		}
	}
	if _, err := p.expect(lex.RightBracket, "expected ']' after vector literal"); err != nil {
		return nil, err
	}
	return ast.VectorLiteralExpr{Elements: elements}, nil
}

func (p *parser) parseMatrixLiteralExpr() (ast.Expr, error) {
	if _, err := p.expect(lex.LeftBracket, "expected '[' to start matrix literal"); err != nil {
		return nil, err
	}
	if p.current().Kind == lex.RightBracket {
		return nil, p.errorAtCurrent("empty matrix literals are not supported")
	}
	var rows [][]ast.Expr
	for p.current().Kind != lex.RightBracket {
		if _, err := p.expect(lex.LeftBracket, "expected '[' to start matrix row"); err != nil {
			return nil, err
		}
		if p.current().Kind == lex.RightBracket {
			return nil, p.errorAtCurrent("matrix rows must not be empty")
		}
		var row []ast.Expr
		for {
			element, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			row = append(row, element)
			if !p.match(lex.Comma) {
				break
			}
		}
		if _, err := p.expect(lex.RightBracket, "expected ']' after matrix row"); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	p.advance()
	return ast.MatrixLiteralExpr{Rows: rows}, nil
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

func isContextualIdentifierToken(kind lex.TokenKind) bool {
	switch kind {
	case lex.KeywordFlow, lex.KeywordState, lex.KeywordStep:
		return true
	default:
		return false
	}
}

func (p *parser) isIdentifierLike(kind lex.TokenKind) bool {
	return kind == lex.Identifier || isContextualIdentifierToken(kind)
}

func (p *parser) expectIdentifierLike(message string) (lex.Token, error) {
	token := p.current()
	if !p.isIdentifierLike(token.Kind) {
		return lex.Token{}, p.errorAtCurrent(message)
	}
	p.advance()
	return token, nil
}

func (p *parser) current() lex.Token {
	return p.peek(0)
}

func (p *parser) peek(offset int) lex.Token {
	position := p.position + offset
	if position >= len(p.tokens) {
		return lex.Token{Kind: lex.EOF}
	}
	return p.tokens[position]
}

func (p *parser) advance() {
	if p.position < len(p.tokens) {
		p.position++
	}
}

func (p *parser) errorAtCurrent(message string) error {
	return p.errorAtToken(p.current(), message)
}

func (p *parser) errorAtToken(token lex.Token, message string) error {
	if token.Kind == lex.EOF {
		return fmt.Errorf("%s at end of file", message)
	}
	return fmt.Errorf("%s at %d:%d near %q", message, token.Line, token.Column, token.Lexeme)
}

func binaryPrecedence(kind lex.TokenKind) (int, bool) {
	switch kind {
	case lex.KeywordOr:
		return precedenceOr, true
	case lex.KeywordAnd:
		return precedenceAnd, true
	case lex.EqualEqual, lex.BangEqual, lex.LeftAngle, lex.LeftEqual, lex.RightAngle, lex.RightEqual:
		return precedenceComparison, true
	case lex.Plus, lex.Minus:
		return precedenceAddSub, true
	case lex.At:
		return precedenceMatMul, true
	case lex.Star, lex.Slash, lex.Percent:
		return precedenceMulDiv, true
	default:
		return 0, false
	}
}

const (
	precedenceOr         = 0
	precedenceAnd        = 1
	precedenceComparison = 2
	precedenceAddSub     = 3
	precedenceMatMul     = 4
	precedenceMulDiv     = 5
)

func (p *parser) parseLiteralUnitSuffix(numberToken lex.Token) (dimension.Dimension, bool, error) {
	if p.current().Kind != lex.Identifier {
		return dimension.Zero(), false, nil
	}
	if !tokensAdjacent(numberToken, p.current()) {
		if _, ok := dimension.FromBaseName(p.current().Lexeme); !ok {
			return dimension.Zero(), false, nil
		}
	}
	dim, err := p.parseDimensionSpec()
	if err != nil {
		return dimension.Dimension{}, false, err
	}
	return dim, true, nil
}

func (p *parser) parseDimensionSpec() (dimension.Dimension, error) {
	dim, err := p.parseDimensionProduct()
	if err != nil {
		return dimension.Dimension{}, err
	}
	for p.current().Kind == lex.Slash && p.peek(1).Kind == lex.Identifier {
		p.advance()
		right, err := p.parseDimensionProduct()
		if err != nil {
			return dimension.Dimension{}, err
		}
		dim = dim.Divide(right)
	}
	return dim, nil
}

func (p *parser) parseDimensionProduct() (dimension.Dimension, error) {
	dim, err := p.parseDimensionFactor()
	if err != nil {
		return dimension.Dimension{}, err
	}
	for p.current().Kind == lex.Star && p.peek(1).Kind == lex.Identifier {
		p.advance()
		right, err := p.parseDimensionFactor()
		if err != nil {
			return dimension.Dimension{}, err
		}
		dim = dim.Multiply(right)
	}
	return dim, nil
}

func (p *parser) parseDimensionFactor() (dimension.Dimension, error) {
	unitToken, err := p.expect(lex.Identifier, "expected unit name")
	if err != nil {
		return dimension.Dimension{}, err
	}
	baseDim, ok := dimension.FromBaseName(unitToken.Lexeme)
	if !ok {
		return dimension.Dimension{}, p.errorAtToken(unitToken, fmt.Sprintf("unknown base unit: %s", unitToken.Lexeme))
	}
	exponent := 1
	if p.match(lex.Caret) {
		sign := 1
		if p.match(lex.Minus) {
			sign = -1
		} else if p.match(lex.Plus) {
			sign = 1
		}
		valueToken, err := p.expect(lex.IntLiteral, "expected integer exponent after '^'")
		if err != nil {
			return dimension.Dimension{}, err
		}
		exponent, err = strconv.Atoi(valueToken.Lexeme)
		if err != nil || exponent == 0 {
			return dimension.Dimension{}, p.errorAtToken(valueToken, "expected non-zero integer exponent")
		}
		exponent *= sign
	}
	return baseDim.Pow(exponent), nil
}

func tokensAdjacent(left lex.Token, right lex.Token) bool {
	return left.Line == right.Line && left.Column+len(left.Lexeme) == right.Column
}

func literalSuffixContinuesAsDimensionSpec(next lex.Token) bool {
	switch next.Kind {
	case lex.Star, lex.Slash, lex.Caret:
		return true
	default:
		return false
	}
}
