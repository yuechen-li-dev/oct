// Package document owns renderer-private projections of the ordinary Oct
// Document values. It deliberately exposes no OOXML vocabulary to Oct source.
package document

import (
	"fmt"
	"strings"
)

type FontWeight string
type Alignment string
type ParagraphRole string
type PageSize string
type Orientation string
type ListKind string
type CalloutKind string
type InlineKind string
type BlockKind string
type LengthUnit string
type FigurePlacementKind string
type FigureAnchor string
type FigureWrap string
type ReferenceKind string

const (
	Normal FontWeight = "Normal"
	Medium FontWeight = "Medium"
	Bold   FontWeight = "Bold"

	Left    Alignment = "Left"
	Center  Alignment = "Center"
	Right   Alignment = "Right"
	Justify Alignment = "Justify"

	BodyRole     ParagraphRole = "Body"
	TitleRole    ParagraphRole = "Title"
	SubtitleRole ParagraphRole = "Subtitle"
	Heading1Role ParagraphRole = "Heading1"
	Heading2Role ParagraphRole = "Heading2"
	CaptionRole  ParagraphRole = "Caption"
	CodeRole     ParagraphRole = "Code"
	SmallRole    ParagraphRole = "Small"

	Letter    PageSize    = "Letter"
	A4        PageSize    = "A4"
	Portrait  Orientation = "Portrait"
	Landscape Orientation = "Landscape"

	BulletList   ListKind = "Bullet"
	NumberedList ListKind = "Numbered"

	TextInline           InlineKind = "Text"
	StrongInline         InlineKind = "Strong"
	EmphasisInline       InlineKind = "Emphasis"
	CodeInline           InlineKind = "Code"
	LinkInline           InlineKind = "Link"
	LineBreakInline      InlineKind = "LineBreak"
	ReferenceInline      InlineKind = "Reference"
	CitationInline       InlineKind = "Citation"
	PageNumberInline     InlineKind = "PageNumber"
	DocumentTitleInline  InlineKind = "DocumentTitle"
	DocumentAuthorInline InlineKind = "DocumentAuthor"

	HeadingBlockKind        BlockKind = "Heading"
	ParagraphBlockKind      BlockKind = "Paragraph"
	ListBlockKind           BlockKind = "List"
	TableBlockKind          BlockKind = "Table"
	CodeBlockKind           BlockKind = "Code"
	CalloutBlockKind        BlockKind = "Callout"
	HorizontalRuleBlockKind BlockKind = "HorizontalRule"
	PageBreakBlockKind      BlockKind = "PageBreak"
	GroupBlockKind          BlockKind = "Group"
	FigureBlockKind         BlockKind = "Figure"
	LabeledTableBlockKind   BlockKind = "LabeledTable"
	PageChromeBlockKind     BlockKind = "PageChrome"
	EquationBlockKind       BlockKind = "Equation"
	BibliographyBlockKind   BlockKind = "Bibliography"
	SectionBlockKind        BlockKind = "Section"
	AbstractBlockKind       BlockKind = "Abstract"

	Point             LengthUnit          = "Pt"
	Millimeter        LengthUnit          = "Mm"
	AutoPlacement     FigurePlacementKind = "Auto"
	AnchoredPlacement FigurePlacementKind = "Anchored"
	PageAnchor        FigureAnchor        = "Page"
	MarginAnchor      FigureAnchor        = "Margin"
	ParagraphAnchor   FigureAnchor        = "Paragraph"
	CharacterAnchor   FigureAnchor        = "Character"
	SquareWrap        FigureWrap          = "Square"
	TopBottomWrap     FigureWrap          = "TopBottom"
	BehindTextWrap    FigureWrap          = "BehindText"
	InFrontOfTextWrap FigureWrap          = "InFrontOfText"
	FigureReference   ReferenceKind       = "Figure"
	TableReference    ReferenceKind       = "Table"
	SectionReference  ReferenceKind       = "Section"
	EquationReference ReferenceKind       = "Equation"
)

type Color struct{ Hex string }

type TextStyle struct {
	FontFamily string
	SizePt     float64
	Weight     FontWeight
	Italic     bool
	Color      Color
}

type ParagraphStyle struct {
	Text          TextStyle
	Alignment     Alignment
	SpaceBeforePt float64
	SpaceAfterPt  float64
	LineSpacing   float64
}

type StyleSheet struct {
	Body, Title, Subtitle, Heading1, Heading2, Caption, Code, Small ParagraphStyle
}

type Margins struct{ TopPt, BottomPt, LeftPt, RightPt float64 }

type PageLayout struct {
	Size        PageSize
	Orientation Orientation
	Margins     Margins
}

type Metadata struct{ Title, Author, Subject string }

type Inline struct {
	Kind      InlineKind
	Text      string
	URL       string
	Reference Reference
	Citation  []string
}

type Reference struct {
	Target string
	Kind   ReferenceKind
}

type Heading struct {
	Level   int
	Content []Inline
	Role    ParagraphRole
}

type Paragraph struct {
	Content []Inline
	Role    ParagraphRole
}

type List struct {
	Kind  ListKind
	Items [][]Inline
}

type Cell struct {
	Content   []Inline
	Alignment Alignment
}

type Row struct{ Cells []Cell }

type Table struct {
	Header Row
	Body   []Row
}

type Length struct {
	Unit  LengthUnit
	Value float64
}
type FigureSize struct {
	Width  Length
	Height *Length
}
type AutoFigurePlacement struct {
	Alignment       Alignment
	KeepWithCaption bool
	Size            FigureSize
}
type AnchoredFigurePlacement struct {
	Anchor FigureAnchor
	X, Y   Length
	Size   FigureSize
	Wrap   FigureWrap
	ZOrder int
}
type FigurePlacement struct {
	Kind     FigurePlacementKind
	Auto     AutoFigurePlacement
	Anchored AnchoredFigurePlacement
}
type Figure struct {
	ID, Source, AltText string
	Caption             []Inline
	Placement           FigurePlacement
}
type LabeledTable struct {
	ID      string
	Caption []Inline
	Table   Table
}
type PageChrome struct{ Header, Footer []Inline }

type Equation struct {
	ID       string
	Latex    string
	Numbered bool
}

type Bibliography struct{ Source string }

type Section struct {
	ID       string
	Level    int
	Title    []Inline
	Children []Block
}

type Abstract struct{ Children []Block }

type CodeBlock struct {
	Language string
	Lines    []string
}

type Callout struct {
	Kind    CalloutKind
	Content []Inline
}

type Block struct {
	Kind         BlockKind
	Heading      Heading
	Paragraph    Paragraph
	List         List
	Table        Table
	Code         CodeBlock
	Callout      Callout
	Children     []Block
	Figure       Figure
	LabeledTable LabeledTable
	PageChrome   PageChrome
	Equation     Equation
	Bibliography Bibliography
	Section      Section
	Abstract     Abstract
}

type Doc struct {
	Metadata   Metadata
	Style      StyleSheet
	Layout     PageLayout
	Content    []Block
	SourceRoot string
}

func Validate(doc Doc) []string {
	var diagnostics []string
	ids := map[string]ReferenceKind{}
	var visit func([]Block)
	visit = func(blocks []Block) {
		for _, block := range blocks {
			switch block.Kind {
			case HeadingBlockKind:
				if block.Heading.Level < 1 || block.Heading.Level > 2 {
					diagnostics = append(diagnostics, "Document heading level must be 1 or 2 in M0")
				}
			case TableBlockKind:
				diagnostics = validateTable(block.Table, diagnostics)
			case LabeledTableBlockKind:
				diagnostics = validateTable(block.LabeledTable.Table, diagnostics)
				diagnostics = registerID(block.LabeledTable.ID, TableReference, ids, diagnostics)
			case FigureBlockKind:
				diagnostics = registerID(block.Figure.ID, FigureReference, ids, diagnostics)
				diagnostics = validateFigure(block.Figure, diagnostics)
			case EquationBlockKind:
				if block.Equation.Latex == "" {
					diagnostics = append(diagnostics, "Document equation payload must not be empty")
				}
				if block.Equation.Numbered {
					diagnostics = registerID(block.Equation.ID, EquationReference, ids, diagnostics)
				}
			case BibliographyBlockKind:
				if block.Bibliography.Source == "" {
					diagnostics = append(diagnostics, "Document bibliography source must not be empty")
				}
			case SectionBlockKind:
				diagnostics = registerID(block.Section.ID, SectionReference, ids, diagnostics)
				visit(block.Section.Children)
			case AbstractBlockKind:
				visit(block.Abstract.Children)
			case GroupBlockKind:
				visit(block.Children)
			}
		}
	}
	visit(doc.Content)
	var validateReferences func([]Block)
	validateReferences = func(blocks []Block) {
		for _, block := range blocks {
			for _, inlines := range blockInlineCollections(block) {
				for _, inline := range inlines {
					if inline.Kind == CitationInline {
						if len(inline.Citation) == 0 {
							diagnostics = append(diagnostics, "Document citation must contain at least one key")
						}
						for _, key := range inline.Citation {
							if key == "" {
								diagnostics = append(diagnostics, "Document citation key must not be empty")
							}
							if strings.ContainsAny(key, "{}\\,\r\n") {
								diagnostics = append(diagnostics, "Document citation key contains unsupported characters")
							}
						}
					}
					if inline.Kind != ReferenceInline {
						continue
					}
					kind, ok := ids[inline.Reference.Target]
					if !ok {
						diagnostics = append(diagnostics, "Document reference target not found: "+inline.Reference.Target)
					} else if kind != inline.Reference.Kind {
						diagnostics = append(diagnostics, fmt.Sprintf("Document reference kind mismatch for %s: expected %s, found %s", inline.Reference.Target, inline.Reference.Kind, kind))
					}
				}
			}
			if block.Kind == GroupBlockKind {
				validateReferences(block.Children)
			} else if block.Kind == SectionBlockKind {
				validateReferences(block.Section.Children)
			} else if block.Kind == AbstractBlockKind {
				validateReferences(block.Abstract.Children)
			}
		}
	}
	validateReferences(doc.Content)
	return diagnostics
}

func validateTable(table Table, diagnostics []string) []string {
	columns := len(table.Header.Cells)
	if columns == 0 {
		diagnostics = append(diagnostics, "Document table header must contain at least one cell")
	}
	for _, row := range table.Body {
		if len(row.Cells) != columns {
			diagnostics = append(diagnostics, "Document table rows must match the header cell count")
		}
	}
	return diagnostics
}

func registerID(id string, kind ReferenceKind, ids map[string]ReferenceKind, diagnostics []string) []string {
	if id == "" {
		return diagnostics
	}
	if _, exists := ids[id]; exists {
		return append(diagnostics, "Document identifier is duplicated: "+id)
	}
	ids[id] = kind
	return diagnostics
}

func validateFigure(figure Figure, diagnostics []string) []string {
	if figure.Source == "" {
		diagnostics = append(diagnostics, "Document figure source must not be empty")
	}
	if figure.AltText == "" {
		diagnostics = append(diagnostics, "Document figure alt text must not be empty")
	}
	placement := figure.Placement
	var size FigureSize
	if placement.Kind == AutoPlacement {
		size = placement.Auto.Size
	} else if placement.Kind == AnchoredPlacement {
		size = placement.Anchored.Size
		if lengthPoints(placement.Anchored.X) < 0 || lengthPoints(placement.Anchored.Y) < 0 {
			diagnostics = append(diagnostics, "Document anchored figure coordinates must be non-negative")
		}
		if placement.Anchored.ZOrder < 0 {
			diagnostics = append(diagnostics, "Document anchored figure z-order must be non-negative")
		}
		switch placement.Anchored.Anchor {
		case PageAnchor, MarginAnchor, ParagraphAnchor, CharacterAnchor:
		default:
			diagnostics = append(diagnostics, "Document anchored figure has unsupported anchor: "+string(placement.Anchored.Anchor))
		}
		switch placement.Anchored.Wrap {
		case SquareWrap, TopBottomWrap, BehindTextWrap, InFrontOfTextWrap:
		default:
			diagnostics = append(diagnostics, "Document anchored figure has unsupported wrap: "+string(placement.Anchored.Wrap))
		}
	} else {
		diagnostics = append(diagnostics, "Document figure has unsupported placement: "+string(placement.Kind))
		return diagnostics
	}
	if lengthPoints(size.Width) <= 0 {
		diagnostics = append(diagnostics, "Document figure width must be positive")
	}
	if size.Height != nil && lengthPoints(*size.Height) <= 0 {
		diagnostics = append(diagnostics, "Document figure height must be positive")
	}
	return diagnostics
}

func lengthPoints(length Length) float64 {
	if length.Unit == Millimeter {
		return length.Value * 72.0 / 25.4
	}
	return length.Value
}

func blockInlineCollections(block Block) [][]Inline {
	switch block.Kind {
	case HeadingBlockKind:
		return [][]Inline{block.Heading.Content}
	case ParagraphBlockKind:
		return [][]Inline{block.Paragraph.Content}
	case ListBlockKind:
		return block.List.Items
	case TableBlockKind:
		return tableInlineCollections(block.Table)
	case LabeledTableBlockKind:
		return append([][]Inline{block.LabeledTable.Caption}, tableInlineCollections(block.LabeledTable.Table)...)
	case FigureBlockKind:
		return [][]Inline{block.Figure.Caption}
	case CalloutBlockKind:
		return [][]Inline{block.Callout.Content}
	case PageChromeBlockKind:
		return [][]Inline{block.PageChrome.Header, block.PageChrome.Footer}
	case SectionBlockKind:
		return [][]Inline{block.Section.Title}
	default:
		return nil
	}
}

func tableInlineCollections(table Table) [][]Inline {
	var out [][]Inline
	for _, cell := range table.Header.Cells {
		out = append(out, cell.Content)
	}
	for _, row := range table.Body {
		for _, cell := range row.Cells {
			out = append(out, cell.Content)
		}
	}
	return out
}
