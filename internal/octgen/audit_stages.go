package octgen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strconv"

	experimental "github.com/yuechen-li-dev/oct/experimental/octgen"
)

// AuditStages is the M1 package-local declaration model. Unlike the M0 Time
// dispatch, it contains no host behavior modes: Oct has already derived all
// concrete values and the host only validates and renders composite literals.
type AuditStages struct {
	Package string
	Noise   []AuditStage
	Context []AuditStage
}

type AuditStage struct {
	ID       int64
	Name     string
	Policy   string
	Source   string
	Base     int64
	Count    int64
	Layout   string
	Capture  string
	Lifetime string
	Keys     []int64
}

func decodeAuditStages(value experimental.Value, provenance string) (AuditStages, error) {
	if value.Kind != experimental.ValueRecord || value.Record.TypeName != "GeneratedAuditStages" {
		return AuditStages{}, modelError(provenance, "Generate must return GeneratedAuditStages")
	}
	packageName, err := requiredString(value.Record.Fields, "Package", provenance)
	if err != nil {
		return AuditStages{}, err
	}
	if packageName != "main" {
		return AuditStages{}, modelError(provenance, "GeneratedAuditStages.Package must be main, got %q", packageName)
	}
	noise, err := decodeAuditStageList(value.Record.Fields, "Noise", provenance)
	if err != nil {
		return AuditStages{}, err
	}
	context, err := decodeAuditStageList(value.Record.Fields, "Context", provenance)
	if err != nil {
		return AuditStages{}, err
	}
	return AuditStages{Package: packageName, Noise: noise, Context: context}, nil
}

func decodeAuditStageList(fields map[string]experimental.Value, name, provenance string) ([]AuditStage, error) {
	value, ok := fields[name]
	if !ok || value.Kind != experimental.ValueArray {
		return nil, modelError(provenance, "GeneratedAuditStages.%s must be AuditStage[]", name)
	}
	if len(value.Array) < 3 {
		return nil, modelError(provenance, "GeneratedAuditStages.%s requires at least three stages", name)
	}
	stages := make([]AuditStage, 0, len(value.Array))
	seenNames := map[string]struct{}{}
	for index, item := range value.Array {
		stage, err := decodeAuditStage(item, provenance, name, index)
		if err != nil {
			return nil, err
		}
		if stage.ID != int64(index+1) {
			return nil, modelError(provenance, "GeneratedAuditStages.%s[%d].ID must be derived sequentially as %d, got %d", name, index, index+1, stage.ID)
		}
		if _, exists := seenNames[stage.Name]; exists {
			return nil, modelError(provenance, "GeneratedAuditStages.%s[%d].Name %q is duplicated", name, index, stage.Name)
		}
		seenNames[stage.Name] = struct{}{}
		stages = append(stages, stage)
	}
	return stages, nil
}

func decodeAuditStage(value experimental.Value, provenance, family string, index int) (AuditStage, error) {
	if value.Kind != experimental.ValueRecord || value.Record.TypeName != "AuditStage" {
		return AuditStage{}, modelError(provenance, "GeneratedAuditStages.%s[%d] must be AuditStage", family, index)
	}
	strings := map[string]string{}
	for _, name := range []string{"Name", "Policy", "Source", "Layout", "Capture", "Lifetime"} {
		text, err := requiredString(value.Record.Fields, name, provenance)
		if err != nil {
			return AuditStage{}, err
		}
		if text == "" {
			return AuditStage{}, modelError(provenance, "GeneratedAuditStages.%s[%d].%s must not be empty", family, index, name)
		}
		strings[name] = text
	}
	ints := map[string]int64{}
	for _, name := range []string{"ID", "Base", "Count"} {
		item, ok := value.Record.Fields[name]
		if !ok || item.Kind != experimental.ValueInt {
			return AuditStage{}, modelError(provenance, "GeneratedAuditStages.%s[%d].%s must be Int", family, index, name)
		}
		if item.Int < 0 || (name == "Count" && item.Int == 0) {
			return AuditStage{}, modelError(provenance, "GeneratedAuditStages.%s[%d].%s must be positive or zero as applicable", family, index, name)
		}
		ints[name] = item.Int
	}
	keysValue, ok := value.Record.Fields["Keys"]
	if !ok || keysValue.Kind != experimental.ValueArray {
		return AuditStage{}, modelError(provenance, "GeneratedAuditStages.%s[%d].Keys must be Int[]", family, index)
	}
	keys := make([]int64, 0, len(keysValue.Array))
	seenKeys := map[int64]struct{}{}
	for keyIndex, key := range keysValue.Array {
		if key.Kind != experimental.ValueInt || key.Int < 0 || key.Int >= ints["Count"] {
			return AuditStage{}, modelError(provenance, "GeneratedAuditStages.%s[%d].Keys[%d] must be an in-range Int", family, index, keyIndex)
		}
		if _, exists := seenKeys[key.Int]; exists {
			return AuditStage{}, modelError(provenance, "GeneratedAuditStages.%s[%d].Keys[%d] duplicates %d", family, index, keyIndex, key.Int)
		}
		seenKeys[key.Int] = struct{}{}
		keys = append(keys, key.Int)
	}
	if len(keys) == 0 || len(keys) > 15 {
		return AuditStage{}, modelError(provenance, "GeneratedAuditStages.%s[%d].Keys must contain 1 through 15 values", family, index)
	}
	return AuditStage{ID: ints["ID"], Name: strings["Name"], Policy: strings["Policy"], Source: strings["Source"], Base: ints["Base"], Count: ints["Count"], Layout: strings["Layout"], Capture: strings["Capture"], Lifetime: strings["Lifetime"], Keys: keys}, nil
}

func renderAuditStages(model AuditStages) ([]byte, error) {
	fset := token.NewFileSet()
	positionFile := fset.AddFile("octgen-audit-stages.generated.go", -1, 100000)
	positionContent := make([]byte, 100000)
	for index := 9; index < len(positionContent); index += 10 {
		positionContent[index] = '\n'
	}
	positionFile.SetLinesForContent(positionContent)
	positions := auditPositions{next: token.Pos(positionFile.Base())}
	file := &ast.File{Name: positionedIdent(model.Package, &positions), Package: positions.take(), Decls: []ast.Decl{auditStageVariable("generatedNoiseAuditStages", model.Noise, &positions), auditStageVariable("generatedContextAuditStages", model.Context, &positions)}}
	var formatted bytes.Buffer
	if err := format.Node(&formatted, fset, file); err != nil {
		return nil, fmt.Errorf("format structured audit-stage model: %w", err)
	}
	return append([]byte(generatedHeader), formatted.Bytes()...), nil
}

type auditPositions struct{ next token.Pos }

func (p *auditPositions) take() token.Pos {
	position := p.next
	p.next++
	return position
}

func (p *auditPositions) nextLine() {
	offset := int(p.next - 1)
	p.next = token.Pos(((offset/10)+1)*10 + 1)
}

func positionedIdent(name string, positions *auditPositions) *ast.Ident {
	return &ast.Ident{Name: name, NamePos: positions.take()}
}

func auditStageVariable(name string, stages []AuditStage, positions *auditPositions) *ast.GenDecl {
	declarationPosition := positions.take()
	variableName := positionedIdent(name, positions)
	arrayType := &ast.ArrayType{Lbrack: positions.take(), Elt: positionedIdent("auditStageSpec", positions)}
	literal := &ast.CompositeLit{Type: arrayType, Lbrace: positions.take()}
	positions.nextLine()
	values := make([]ast.Expr, 0, len(stages))
	for _, stage := range stages {
		values = append(values, auditStageLiteral(stage, positions))
	}
	literal.Elts = values
	return &ast.GenDecl{TokPos: declarationPosition, Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names:  []*ast.Ident{variableName},
		Type:   arrayType,
		Values: []ast.Expr{literal},
	}}}
}

func auditStageLiteral(stage AuditStage, positions *auditPositions) *ast.CompositeLit {
	literal := &ast.CompositeLit{Type: positionedIdent("auditStageSpec", positions), Lbrace: positions.take(), Elts: []ast.Expr{
		intField("ID", stage.ID, positions),
		stringField("Name", stage.Name, positions),
		stringField("Policy", stage.Policy, positions),
		stringField("Source", stage.Source, positions),
		intField("Base", stage.Base, positions),
		intField("Count", stage.Count, positions),
		stringField("Layout", stage.Layout, positions),
		stringField("Capture", stage.Capture, positions),
		stringField("Lifetime", stage.Lifetime, positions),
		keysField(stage.Keys, positions),
	}}
	positions.nextLine()
	return literal
}

func intField(name string, value int64, positions *auditPositions) *ast.KeyValueExpr {
	field := &ast.KeyValueExpr{Key: positionedIdent(name, positions), Colon: positions.take(), Value: integerLiteral64(value, positions)}
	positions.nextLine()
	return field
}

func stringField(name, value string, positions *auditPositions) *ast.KeyValueExpr {
	field := &ast.KeyValueExpr{Key: positionedIdent(name, positions), Colon: positions.take(), Value: stringLiteralAt(value, positions)}
	positions.nextLine()
	return field
}

func keysField(values []int64, positions *auditPositions) *ast.KeyValueExpr {
	field := &ast.KeyValueExpr{Key: positionedIdent("Keys", positions), Colon: positions.take(), Value: int64SliceLiteral(values, positions)}
	positions.nextLine()
	return field
}

func int64SliceLiteral(values []int64, positions *auditPositions) *ast.CompositeLit {
	elements := make([]ast.Expr, 0, len(values))
	for _, value := range values {
		elements = append(elements, integerLiteral64(value, positions))
	}
	return &ast.CompositeLit{Type: &ast.ArrayType{Lbrack: positions.take(), Elt: positionedIdent("uint32", positions)}, Lbrace: positions.take(), Elts: elements}
}

func integerLiteral64(value int64, positions *auditPositions) *ast.BasicLit {
	return &ast.BasicLit{ValuePos: positions.take(), Kind: token.INT, Value: strconv.FormatInt(value, 10)}
}

func stringLiteralAt(value string, positions *auditPositions) *ast.BasicLit {
	return &ast.BasicLit{ValuePos: positions.take(), Kind: token.STRING, Value: strconv.Quote(value)}
}
