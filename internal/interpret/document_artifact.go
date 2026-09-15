package interpret

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/document"
)

func (i *interpreter) evalArtifactDocumentMarkdownBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalArtifactDocumentBuiltin(env, pkgName, callee, argumentExprs, ".md", "document-markdown", func(_ document.Doc, raw Value) ([]byte, error) {
		result, err := i.invokeFunctionValue(FunctionValue{Key: "Document.ToMarkdown"}, pkgName, []Value{raw})
		if err != nil {
			return nil, err
		}
		if result.hasError {
			return nil, fmt.Errorf("Document.ToMarkdown failed: %s", result.errorVal.Error.Message)
		}
		lines, err := documentStrings(result.value)
		if err != nil {
			return nil, fmt.Errorf("Document.ToMarkdown returned invalid data: %w", err)
		}
		if len(lines) == 0 {
			return []byte{}, nil
		}
		return []byte(strings.Join(lines, "\n") + "\n"), nil
	})
}

func (i *interpreter) evalArtifactDocumentDocxBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalArtifactDocumentBuiltin(env, pkgName, callee, argumentExprs, ".docx", "document-docx", func(_ document.Doc, raw Value) ([]byte, error) {
		resolved, err := i.invokeFunctionValue(FunctionValue{Key: "Document.Resolve"}, pkgName, []Value{raw})
		if err != nil {
			return nil, fmt.Errorf("Document.Resolve failed: %w", err)
		}
		if resolved.hasError {
			return nil, fmt.Errorf("Document.Resolve failed: %s", resolved.errorVal.Error.Message)
		}
		doc, err := decodeDocument(resolved.value)
		if err != nil {
			return nil, err
		}
		doc.SourceRoot = filepath.Dir(i.artifactSourcePath)
		return document.DOCX(doc)
	})
}

func (i *interpreter) evalArtifactDocumentBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr, extension, kind string, render func(document.Doc, Value) ([]byte, error)) (evalResult, error) {
	if err := i.beginArtifactWrite(); err != nil {
		return evalResult{}, err
	}
	defer i.endArtifactWrite()
	if len(argumentExprs) != 2 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 2 arguments", callee)
	}
	pathResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
	if err != nil {
		return evalResult{}, err
	}
	if pathResult.hasError {
		return pathResult, nil
	}
	docResult, err := i.evalExpr(env, pkgName, argumentExprs[1])
	if err != nil {
		return evalResult{}, err
	}
	if docResult.hasError {
		return docResult, nil
	}
	if pathResult.value.Kind != ValueString || !strings.HasSuffix(strings.ToLower(pathResult.value.Text), extension) {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s path must end with %s", callee, extension)
	}
	doc, err := decodeDocument(docResult.value)
	if err != nil {
		return evalResult{}, err
	}
	if diagnostics := document.Validate(doc); len(diagnostics) > 0 {
		return evalResult{}, fmt.Errorf("Document validation: %s", strings.Join(diagnostics, "; "))
	}
	payload, err := render(doc, docResult.value)
	if err != nil {
		return evalResult{}, err
	}
	logical := attributedOutputPath(pathResult.value.Text)
	actual, err := i.artifactCapability.StageArtifactOutput(ArtifactOutputRequest{Path: logical, Package: i.artifactPackage, Function: i.currentFunctionName, SourcePath: i.artifactSourcePath, Kind: kind})
	if err != nil {
		return evalResult{}, err
	}
	if err := os.WriteFile(actual, payload, 0o644); err != nil {
		return evalResult{}, fmt.Errorf("Artifact document write: %w", err)
	}
	i.recordArtifactWrite(logical)
	return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
}

func decodeDocument(value Value) (document.Doc, error) {
	record, err := documentRecord(value, "Doc")
	if err != nil {
		return document.Doc{}, fmt.Errorf("Artifact document renderer: %w", err)
	}
	metadataValue, err := documentField(record, "Metadata")
	if err != nil {
		return document.Doc{}, err
	}
	styleValue, err := documentField(record, "Style")
	if err != nil {
		return document.Doc{}, err
	}
	layoutValue, err := documentField(record, "Layout")
	if err != nil {
		return document.Doc{}, err
	}
	contentValue, err := documentField(record, "Content")
	if err != nil {
		return document.Doc{}, err
	}
	metadata, err := decodeDocumentMetadata(metadataValue)
	if err != nil {
		return document.Doc{}, err
	}
	style, err := decodeDocumentStyleSheet(styleValue)
	if err != nil {
		return document.Doc{}, err
	}
	layout, err := decodeDocumentPageLayout(layoutValue)
	if err != nil {
		return document.Doc{}, err
	}
	content, err := decodeDocumentBlocks(contentValue)
	if err != nil {
		return document.Doc{}, err
	}
	return document.Doc{Metadata: metadata, Style: style, Layout: layout, Content: content}, nil
}

func decodeDocumentMetadata(value Value) (document.Metadata, error) {
	r, err := documentRecord(value, "Metadata")
	if err != nil {
		return document.Metadata{}, err
	}
	title, err := documentStringField(r, "Title")
	if err != nil {
		return document.Metadata{}, err
	}
	author, err := documentStringField(r, "Author")
	if err != nil {
		return document.Metadata{}, err
	}
	subject, err := documentStringField(r, "Subject")
	if err != nil {
		return document.Metadata{}, err
	}
	return document.Metadata{Title: title, Author: author, Subject: subject}, nil
}

func decodeDocumentStyleSheet(value Value) (document.StyleSheet, error) {
	r, err := documentRecord(value, "StyleSheet")
	if err != nil {
		return document.StyleSheet{}, err
	}
	var out document.StyleSheet
	targets := []struct {
		name string
		set  func(document.ParagraphStyle)
	}{
		{"Body", func(v document.ParagraphStyle) { out.Body = v }}, {"Title", func(v document.ParagraphStyle) { out.Title = v }},
		{"Subtitle", func(v document.ParagraphStyle) { out.Subtitle = v }}, {"Heading1", func(v document.ParagraphStyle) { out.Heading1 = v }},
		{"Heading2", func(v document.ParagraphStyle) { out.Heading2 = v }}, {"Caption", func(v document.ParagraphStyle) { out.Caption = v }},
		{"Code", func(v document.ParagraphStyle) { out.Code = v }}, {"Small", func(v document.ParagraphStyle) { out.Small = v }},
	}
	for _, target := range targets {
		field, fieldErr := documentField(r, target.name)
		if fieldErr != nil {
			return out, fieldErr
		}
		style, styleErr := decodeDocumentParagraphStyle(field)
		if styleErr != nil {
			return out, styleErr
		}
		target.set(style)
	}
	return out, nil
}

func decodeDocumentParagraphStyle(value Value) (document.ParagraphStyle, error) {
	r, err := documentRecord(value, "ParagraphStyle")
	if err != nil {
		return document.ParagraphStyle{}, err
	}
	textValue, err := documentField(r, "Text")
	if err != nil {
		return document.ParagraphStyle{}, err
	}
	text, err := decodeDocumentTextStyle(textValue)
	if err != nil {
		return document.ParagraphStyle{}, err
	}
	alignment, err := documentEnumField(r, "Alignment")
	if err != nil {
		return document.ParagraphStyle{}, err
	}
	before, err := documentFloatField(r, "SpaceBeforePt")
	if err != nil {
		return document.ParagraphStyle{}, err
	}
	after, err := documentFloatField(r, "SpaceAfterPt")
	if err != nil {
		return document.ParagraphStyle{}, err
	}
	line, err := documentFloatField(r, "LineSpacing")
	if err != nil {
		return document.ParagraphStyle{}, err
	}
	return document.ParagraphStyle{Text: text, Alignment: document.Alignment(alignment), SpaceBeforePt: before, SpaceAfterPt: after, LineSpacing: line}, nil
}

func decodeDocumentTextStyle(value Value) (document.TextStyle, error) {
	r, err := documentRecord(value, "TextStyle")
	if err != nil {
		return document.TextStyle{}, err
	}
	family, err := documentStringField(r, "FontFamily")
	if err != nil {
		return document.TextStyle{}, err
	}
	size, err := documentFloatField(r, "SizePt")
	if err != nil {
		return document.TextStyle{}, err
	}
	weight, err := documentEnumField(r, "Weight")
	if err != nil {
		return document.TextStyle{}, err
	}
	italicValue, err := documentField(r, "Italic")
	if err != nil {
		return document.TextStyle{}, err
	}
	if italicValue.Kind != ValueBool {
		return document.TextStyle{}, fmt.Errorf("Document.TextStyle.Italic expects Bool")
	}
	colorValue, err := documentField(r, "Color")
	if err != nil {
		return document.TextStyle{}, err
	}
	colorRecord, err := documentRecord(colorValue, "Color")
	if err != nil {
		return document.TextStyle{}, err
	}
	color, err := documentStringField(colorRecord, "Hex")
	if err != nil {
		return document.TextStyle{}, err
	}
	return document.TextStyle{FontFamily: family, SizePt: size, Weight: document.FontWeight(weight), Italic: italicValue.Bool, Color: document.Color{Hex: color}}, nil
}

func decodeDocumentPageLayout(value Value) (document.PageLayout, error) {
	r, err := documentRecord(value, "PageLayout")
	if err != nil {
		return document.PageLayout{}, err
	}
	size, err := documentEnumField(r, "Size")
	if err != nil {
		return document.PageLayout{}, err
	}
	orientation, err := documentEnumField(r, "Orientation")
	if err != nil {
		return document.PageLayout{}, err
	}
	marginsValue, err := documentField(r, "Margins")
	if err != nil {
		return document.PageLayout{}, err
	}
	m, err := documentRecord(marginsValue, "Margins")
	if err != nil {
		return document.PageLayout{}, err
	}
	top, err := documentFloatField(m, "TopPt")
	if err != nil {
		return document.PageLayout{}, err
	}
	bottom, err := documentFloatField(m, "BottomPt")
	if err != nil {
		return document.PageLayout{}, err
	}
	left, err := documentFloatField(m, "LeftPt")
	if err != nil {
		return document.PageLayout{}, err
	}
	right, err := documentFloatField(m, "RightPt")
	if err != nil {
		return document.PageLayout{}, err
	}
	return document.PageLayout{Size: document.PageSize(size), Orientation: document.Orientation(orientation), Margins: document.Margins{TopPt: top, BottomPt: bottom, LeftPt: left, RightPt: right}}, nil
}

func decodeDocumentBlocks(value Value) ([]document.Block, error) {
	if value.Kind != ValueArray {
		return nil, fmt.Errorf("Document content expects Block[]")
	}
	out := make([]document.Block, 0, len(value.Array))
	for _, item := range value.Array {
		block, err := decodeDocumentBlock(item)
		if err != nil {
			return nil, err
		}
		out = append(out, block)
	}
	return out, nil
}

func decodeDocumentBlock(value Value) (document.Block, error) {
	if value.Kind != ValueEnum || !documentTypeNamed(value.Enum.TypeName, "Block") {
		return document.Block{}, fmt.Errorf("Document content expects Document.Block")
	}
	out := document.Block{Kind: document.BlockKind(value.Enum.Variant)}
	if value.Enum.Payload == nil {
		return out, nil
	}
	switch value.Enum.Variant {
	case "Heading":
		r, err := documentRecord(*value.Enum.Payload, "HeadingBlock")
		if err != nil {
			return out, err
		}
		level, err := documentIntField(r, "Level")
		if err != nil {
			return out, err
		}
		content, err := documentInlineField(r, "Content")
		if err != nil {
			return out, err
		}
		role, err := documentEnumField(r, "Role")
		if err != nil {
			return out, err
		}
		out.Heading = document.Heading{Level: int(level), Content: content, Role: document.ParagraphRole(role)}
	case "Paragraph":
		r, err := documentRecord(*value.Enum.Payload, "ParagraphBlock")
		if err != nil {
			return out, err
		}
		content, err := documentInlineField(r, "Content")
		if err != nil {
			return out, err
		}
		role, err := documentEnumField(r, "Role")
		if err != nil {
			return out, err
		}
		out.Paragraph = document.Paragraph{Content: content, Role: document.ParagraphRole(role)}
	case "List":
		r, err := documentRecord(*value.Enum.Payload, "ListBlock")
		if err != nil {
			return out, err
		}
		kind, err := documentEnumField(r, "Kind")
		if err != nil {
			return out, err
		}
		itemsValue, err := documentField(r, "Items")
		if err != nil {
			return out, err
		}
		if itemsValue.Kind != ValueArray {
			return out, fmt.Errorf("Document.ListBlock.Items expects Inline[][]")
		}
		items := make([][]document.Inline, 0, len(itemsValue.Array))
		for _, item := range itemsValue.Array {
			decoded, e := decodeDocumentInlines(item)
			if e != nil {
				return out, e
			}
			items = append(items, decoded)
		}
		out.List = document.List{Kind: document.ListKind(kind), Items: items}
	case "Table":
		table, err := decodeDocumentTable(*value.Enum.Payload)
		if err != nil {
			return out, err
		}
		out.Table = table
	case "LabeledTable":
		r, err := documentRecord(*value.Enum.Payload, "LabeledTableBlock")
		if err != nil {
			return out, err
		}
		id, err := documentStringField(r, "Id")
		if err != nil {
			return out, err
		}
		caption, err := documentInlineField(r, "Caption")
		if err != nil {
			return out, err
		}
		tableValue, err := documentField(r, "Table")
		if err != nil {
			return out, err
		}
		table, err := decodeDocumentTable(tableValue)
		if err != nil {
			return out, err
		}
		out.LabeledTable = document.LabeledTable{ID: id, Caption: caption, Table: table}
	case "Figure":
		figure, err := decodeDocumentFigure(*value.Enum.Payload)
		if err != nil {
			return out, err
		}
		out.Figure = figure
	case "PageChrome":
		r, err := documentRecord(*value.Enum.Payload, "PageChrome")
		if err != nil {
			return out, err
		}
		header, err := documentInlineField(r, "Header")
		if err != nil {
			return out, err
		}
		footer, err := documentInlineField(r, "Footer")
		if err != nil {
			return out, err
		}
		out.PageChrome = document.PageChrome{Header: header, Footer: footer}
	case "Code":
		r, err := documentRecord(*value.Enum.Payload, "CodeBlock")
		if err != nil {
			return out, err
		}
		language, err := documentStringField(r, "Language")
		if err != nil {
			return out, err
		}
		linesValue, err := documentField(r, "Lines")
		if err != nil {
			return out, err
		}
		lines, err := documentStrings(linesValue)
		if err != nil {
			return out, err
		}
		out.Code = document.CodeBlock{Language: language, Lines: lines}
	case "Callout":
		r, err := documentRecord(*value.Enum.Payload, "CalloutBlock")
		if err != nil {
			return out, err
		}
		kind, err := documentEnumField(r, "Kind")
		if err != nil {
			return out, err
		}
		content, err := documentInlineField(r, "Content")
		if err != nil {
			return out, err
		}
		out.Callout = document.Callout{Kind: document.CalloutKind(kind), Content: content}
	case "Group":
		children, err := decodeDocumentBlocks(*value.Enum.Payload)
		if err != nil {
			return out, err
		}
		out.Children = children
	}
	return out, nil
}

func decodeDocumentTable(value Value) (document.Table, error) {
	r, err := documentRecord(value, "TableBlock")
	if err != nil {
		return document.Table{}, err
	}
	headerValue, err := documentField(r, "Header")
	if err != nil {
		return document.Table{}, err
	}
	header, err := decodeDocumentRow(headerValue)
	if err != nil {
		return document.Table{}, err
	}
	bodyValue, err := documentField(r, "Body")
	if err != nil {
		return document.Table{}, err
	}
	if bodyValue.Kind != ValueArray {
		return document.Table{}, fmt.Errorf("Document.TableBlock.Body expects Row[]")
	}
	body := make([]document.Row, 0, len(bodyValue.Array))
	for _, rowValue := range bodyValue.Array {
		row, e := decodeDocumentRow(rowValue)
		if e != nil {
			return document.Table{}, e
		}
		body = append(body, row)
	}
	return document.Table{Header: header, Body: body}, nil
}

func decodeDocumentFigure(value Value) (document.Figure, error) {
	r, err := documentRecord(value, "FigureBlock")
	if err != nil {
		return document.Figure{}, err
	}
	id, err := documentStringField(r, "Id")
	if err != nil {
		return document.Figure{}, err
	}
	source, err := documentStringField(r, "Source")
	if err != nil {
		return document.Figure{}, err
	}
	alt, err := documentStringField(r, "AltText")
	if err != nil {
		return document.Figure{}, err
	}
	caption, err := documentInlineField(r, "Caption")
	if err != nil {
		return document.Figure{}, err
	}
	pv, err := documentField(r, "Placement")
	if err != nil {
		return document.Figure{}, err
	}
	placement, err := decodeDocumentFigurePlacement(pv)
	if err != nil {
		return document.Figure{}, err
	}
	return document.Figure{ID: id, Source: source, AltText: alt, Caption: caption, Placement: placement}, nil
}

func decodeDocumentFigurePlacement(value Value) (document.FigurePlacement, error) {
	if value.Kind != ValueEnum || !documentTypeNamed(value.Enum.TypeName, "FigurePlacement") || value.Enum.Payload == nil {
		return document.FigurePlacement{}, fmt.Errorf("expected Document.FigurePlacement")
	}
	out := document.FigurePlacement{Kind: document.FigurePlacementKind(value.Enum.Variant)}
	r, err := documentRecord(*value.Enum.Payload, value.Enum.Variant+"FigurePlacement")
	if err != nil {
		return out, err
	}
	sizeValue, err := documentField(r, "Size")
	if err != nil {
		return out, err
	}
	size, err := decodeDocumentFigureSize(sizeValue)
	if err != nil {
		return out, err
	}
	if value.Enum.Variant == "Auto" {
		alignment, err := documentEnumField(r, "Alignment")
		if err != nil {
			return out, err
		}
		keepValue, err := documentField(r, "KeepWithCaption")
		if err != nil {
			return out, err
		}
		if keepValue.Kind != ValueBool {
			return out, fmt.Errorf("Document.AutoFigurePlacement.KeepWithCaption expects Bool")
		}
		out.Auto = document.AutoFigurePlacement{Alignment: document.Alignment(alignment), KeepWithCaption: keepValue.Bool, Size: size}
		return out, nil
	}
	anchor, err := documentEnumField(r, "Anchor")
	if err != nil {
		return out, err
	}
	xv, err := documentField(r, "X")
	if err != nil {
		return out, err
	}
	x, err := decodeDocumentLength(xv)
	if err != nil {
		return out, err
	}
	yv, err := documentField(r, "Y")
	if err != nil {
		return out, err
	}
	y, err := decodeDocumentLength(yv)
	if err != nil {
		return out, err
	}
	wrap, err := documentEnumField(r, "Wrap")
	if err != nil {
		return out, err
	}
	z, err := documentIntField(r, "ZOrder")
	if err != nil {
		return out, err
	}
	out.Anchored = document.AnchoredFigurePlacement{Anchor: document.FigureAnchor(anchor), X: x, Y: y, Size: size, Wrap: document.FigureWrap(wrap), ZOrder: int(z)}
	return out, nil
}

func decodeDocumentFigureSize(value Value) (document.FigureSize, error) {
	if value.Kind != ValueEnum || !documentTypeNamed(value.Enum.TypeName, "FigureSize") || value.Enum.Payload == nil {
		return document.FigureSize{}, fmt.Errorf("expected Document.FigureSize")
	}
	if value.Enum.Variant == "Width" {
		width, err := decodeDocumentLength(*value.Enum.Payload)
		return document.FigureSize{Width: width}, err
	}
	r, err := documentRecord(*value.Enum.Payload, "ExactFigureSize")
	if err != nil {
		return document.FigureSize{}, err
	}
	wv, err := documentField(r, "Width")
	if err != nil {
		return document.FigureSize{}, err
	}
	width, err := decodeDocumentLength(wv)
	if err != nil {
		return document.FigureSize{}, err
	}
	hv, err := documentField(r, "Height")
	if err != nil {
		return document.FigureSize{}, err
	}
	height, err := decodeDocumentLength(hv)
	if err != nil {
		return document.FigureSize{}, err
	}
	return document.FigureSize{Width: width, Height: &height}, nil
}

func decodeDocumentLength(value Value) (document.Length, error) {
	if value.Kind != ValueEnum || !documentTypeNamed(value.Enum.TypeName, "Length") || value.Enum.Payload == nil || value.Enum.Payload.Kind != ValueFloat {
		return document.Length{}, fmt.Errorf("expected Document.Length")
	}
	return document.Length{Unit: document.LengthUnit(value.Enum.Variant), Value: value.Enum.Payload.Float}, nil
}

func decodeDocumentRow(value Value) (document.Row, error) {
	r, err := documentRecord(value, "Row")
	if err != nil {
		return document.Row{}, err
	}
	cellsValue, err := documentField(r, "Cells")
	if err != nil {
		return document.Row{}, err
	}
	if cellsValue.Kind != ValueArray {
		return document.Row{}, fmt.Errorf("Document.Row.Cells expects Cell[]")
	}
	cells := make([]document.Cell, 0, len(cellsValue.Array))
	for _, cellValue := range cellsValue.Array {
		cellRecord, e := documentRecord(cellValue, "Cell")
		if e != nil {
			return document.Row{}, e
		}
		content, e := documentInlineField(cellRecord, "Content")
		if e != nil {
			return document.Row{}, e
		}
		alignment, e := documentEnumField(cellRecord, "Alignment")
		if e != nil {
			return document.Row{}, e
		}
		cells = append(cells, document.Cell{Content: content, Alignment: document.Alignment(alignment)})
	}
	return document.Row{Cells: cells}, nil
}

func decodeDocumentInlines(value Value) ([]document.Inline, error) {
	if value.Kind != ValueArray {
		return nil, fmt.Errorf("Document inline content expects Inline[]")
	}
	out := make([]document.Inline, 0, len(value.Array))
	for _, inlineValue := range value.Array {
		if inlineValue.Kind != ValueEnum || !documentTypeNamed(inlineValue.Enum.TypeName, "Inline") {
			return nil, fmt.Errorf("Document inline content expects Document.Inline")
		}
		inline := document.Inline{Kind: document.InlineKind(inlineValue.Enum.Variant)}
		if inlineValue.Enum.Payload != nil {
			if inlineValue.Enum.Variant == "Link" {
				link, e := documentRecord(*inlineValue.Enum.Payload, "LinkValue")
				if e != nil {
					return nil, e
				}
				inline.Text, e = documentStringField(link, "Text")
				if e != nil {
					return nil, e
				}
				inline.URL, e = documentStringField(link, "Url")
				if e != nil {
					return nil, e
				}
			} else if inlineValue.Enum.Variant == "Reference" {
				reference, e := documentRecord(*inlineValue.Enum.Payload, "Reference")
				if e != nil {
					return nil, e
				}
				inline.Reference.Target, e = documentStringField(reference, "Target")
				if e != nil {
					return nil, e
				}
				kind, e := documentEnumField(reference, "Kind")
				if e != nil {
					return nil, e
				}
				inline.Reference.Kind = document.ReferenceKind(kind)
			} else {
				if inlineValue.Enum.Payload.Kind != ValueString {
					return nil, fmt.Errorf("Document.%s payload expects String", inlineValue.Enum.Variant)
				}
				inline.Text = inlineValue.Enum.Payload.Text
			}
		}
		out = append(out, inline)
	}
	return out, nil
}

func documentRecord(value Value, name string) (RecordValue, error) {
	if value.Kind != ValueRecord || !documentTypeNamed(value.Record.TypeName, name) {
		return RecordValue{}, fmt.Errorf("expected Document.%s", name)
	}
	return value.Record, nil
}

func documentTypeNamed(actual, name string) bool { return actual == name || actual == "Document."+name }
func documentField(record RecordValue, name string) (Value, error) {
	value, ok := record.Fields[name]
	if !ok {
		return Value{}, fmt.Errorf("Document.%s missing field %s", record.TypeName, name)
	}
	return value, nil
}
func documentStringField(record RecordValue, name string) (string, error) {
	value, err := documentField(record, name)
	if err != nil {
		return "", err
	}
	if value.Kind != ValueString {
		return "", fmt.Errorf("Document.%s.%s expects String", record.TypeName, name)
	}
	return value.Text, nil
}
func documentFloatField(record RecordValue, name string) (float64, error) {
	value, err := documentField(record, name)
	if err != nil {
		return 0, err
	}
	if value.Kind != ValueFloat {
		return 0, fmt.Errorf("Document.%s.%s expects Float", record.TypeName, name)
	}
	return value.Float, nil
}
func documentIntField(record RecordValue, name string) (int64, error) {
	value, err := documentField(record, name)
	if err != nil {
		return 0, err
	}
	if value.Kind != ValueInt {
		return 0, fmt.Errorf("Document.%s.%s expects Int", record.TypeName, name)
	}
	return value.Int, nil
}
func documentEnumField(record RecordValue, name string) (string, error) {
	value, err := documentField(record, name)
	if err != nil {
		return "", err
	}
	if value.Kind != ValueEnum {
		return "", fmt.Errorf("Document.%s.%s expects enum", record.TypeName, name)
	}
	return value.Enum.Variant, nil
}
func documentInlineField(record RecordValue, name string) ([]document.Inline, error) {
	value, err := documentField(record, name)
	if err != nil {
		return nil, err
	}
	return decodeDocumentInlines(value)
}
func documentStrings(value Value) ([]string, error) {
	if value.Kind != ValueArray {
		return nil, fmt.Errorf("expected String[]")
	}
	out := make([]string, len(value.Array))
	for i, item := range value.Array {
		if item.Kind != ValueString {
			return nil, fmt.Errorf("expected String[]")
		}
		out[i] = item.Text
	}
	return out, nil
}
