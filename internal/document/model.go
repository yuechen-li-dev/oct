// Package document owns renderer-private projections of the ordinary Oct
// Document values. It deliberately exposes no OOXML vocabulary to Oct source.
package document

type FontWeight string
type Alignment string
type ParagraphRole string
type PageSize string
type Orientation string
type ListKind string
type CalloutKind string
type InlineKind string
type BlockKind string

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

	TextInline      InlineKind = "Text"
	StrongInline    InlineKind = "Strong"
	EmphasisInline  InlineKind = "Emphasis"
	CodeInline      InlineKind = "Code"
	LinkInline      InlineKind = "Link"
	LineBreakInline InlineKind = "LineBreak"

	HeadingBlockKind        BlockKind = "Heading"
	ParagraphBlockKind      BlockKind = "Paragraph"
	ListBlockKind           BlockKind = "List"
	TableBlockKind          BlockKind = "Table"
	CodeBlockKind           BlockKind = "Code"
	CalloutBlockKind        BlockKind = "Callout"
	HorizontalRuleBlockKind BlockKind = "HorizontalRule"
	PageBreakBlockKind      BlockKind = "PageBreak"
	GroupBlockKind          BlockKind = "Group"
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
	Kind InlineKind
	Text string
	URL  string
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

type CodeBlock struct {
	Language string
	Lines    []string
}

type Callout struct {
	Kind    CalloutKind
	Content []Inline
}

type Block struct {
	Kind      BlockKind
	Heading   Heading
	Paragraph Paragraph
	List      List
	Table     Table
	Code      CodeBlock
	Callout   Callout
	Children  []Block
}

type Doc struct {
	Metadata Metadata
	Style    StyleSheet
	Layout   PageLayout
	Content  []Block
}

func Validate(doc Doc) []string {
	var diagnostics []string
	var visit func([]Block)
	visit = func(blocks []Block) {
		for _, block := range blocks {
			switch block.Kind {
			case HeadingBlockKind:
				if block.Heading.Level < 1 || block.Heading.Level > 2 {
					diagnostics = append(diagnostics, "Document heading level must be 1 or 2 in M0")
				}
			case TableBlockKind:
				columns := len(block.Table.Header.Cells)
				if columns == 0 {
					diagnostics = append(diagnostics, "Document table header must contain at least one cell")
				}
				for _, row := range block.Table.Body {
					if len(row.Cells) != columns {
						diagnostics = append(diagnostics, "Document table rows must match the header cell count")
					}
				}
			case GroupBlockKind:
				visit(block.Children)
			}
		}
	}
	visit(doc.Content)
	return diagnostics
}
