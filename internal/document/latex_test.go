package document

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLatexRendersDeterministicPortableAcademicBundle(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "plot.png")
	file, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 4, 2))
	canvas.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(file, canvas); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references.bib"), []byte("@article{smith2024, title={Stable Evidence}, author={Smith, Ada}, year={2024}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := latexFixture(dir)
	first, err := Latex(doc)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Latex(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Source, second.Source) {
		t.Fatal("LaTeX source must be byte-identical")
	}
	if len(first.Files) != 2 || first.Files[0].Path >= first.Files[1].Path {
		t.Fatalf("bundle files are not stable and sorted: %#v", first.Files)
	}
	source := string(first.Source)
	for _, expected := range []string{
		`\documentclass[11pt]{article}`,
		`a4paper,landscape,top=36.000pt,bottom=42.000pt,left=48.000pt,right=54.000pt`,
		`\title{{`, `Escaped \& Deterministic}}`, `\begin{abstract}`, `\RaggedRight`, `\section*{{`, `1\quad Method}}\label{oct:sec:method}`,
		`\href{\detokenize{https://example.test/a_b?x=1%25}}{source}`, `\textbackslash{} \{ \} \$ \& \# \% \_ \textasciicircum{} \textasciitilde{}`,
		`\begin{equation}`, `\tag{1}`, `\label{oct:eq:energy}`, `\cite{smith2024}`,
		`\begin{tabularx}{\linewidth}{>{\raggedright\arraybackslash}X>{\raggedleft\arraybackslash}X}`, `Table 1 — Results}}`, `\includegraphics[width=40.000mm]{assets/image-`,
		`\bibliographystyle{plain}`, `\bibliography{assets/references-`, `\fancyfoot[C]{Page \thepage}`,
	} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated LaTeX missing %q\n%s", expected, source)
		}
	}
	if strings.Contains(source, dir) || strings.Contains(source, `C:\`) {
		t.Fatalf("generated LaTeX contains a host path: %s", source)
	}
}

func TestLatexCapabilityValidationIsConservative(t *testing.T) {
	doc := latexFixture(t.TempDir())
	doc.Content = []Block{
		{Kind: EquationBlockKind, Equation: Equation{Latex: `x + \input{secrets}`, Numbered: true, ID: "bad"}},
		{Kind: FigureBlockKind, Figure: Figure{Source: "x.png", AltText: "x", Placement: FigurePlacement{Kind: AnchoredPlacement, Anchored: AnchoredFigurePlacement{Anchor: ParagraphAnchor, Size: FigureSize{Width: Length{Unit: Point, Value: 10}}, ZOrder: 2}}}},
	}
	diagnostics := strings.Join(ValidateLatex(doc), " | ")
	if !strings.Contains(diagnostics, "document-level") || !strings.Contains(diagnostics, "Page and Margin") || !strings.Contains(diagnostics, "z-order") {
		t.Fatalf("unexpected diagnostics: %s", diagnostics)
	}
}

func TestPDFFailureWhenConfiguredEngineIsMissingIsConcise(t *testing.T) {
	t.Setenv("OCT_LATEX_ENGINE", "oct-definitely-missing-latex-engine")
	_, err := PDF(LatexBundle{Source: []byte("\\documentclass{article}\\begin{document}x\\end{document}\n")})
	if err == nil || !strings.Contains(err.Error(), "configured LaTeX engine") {
		t.Fatalf("expected missing-engine diagnostic, got %v", err)
	}
}

func TestLatexEscapingCoversReservedCharacters(t *testing.T) {
	got := escapeLatexText(`\{}$&#%_^~`)
	want := `\textbackslash{}\{\}\$\&\#\%\_\textasciicircum{}\textasciitilde{}`
	if got != want {
		t.Fatalf("escape mismatch\nwant %q\n got %q", want, got)
	}
}

func latexFixture(sourceRoot string) Doc {
	style := ParagraphStyle{Text: TextStyle{FontFamily: "Inter", SizePt: 10.5, Weight: Normal, Color: Color{Hex: "#202124"}}, Alignment: Left, LineSpacing: 1.08}
	styles := StyleSheet{Body: style, Title: style, Subtitle: style, Heading1: style, Heading2: style, Caption: style, Code: style, Small: style}
	paragraph := Block{Kind: ParagraphBlockKind, Paragraph: Paragraph{Role: BodyRole, Content: []Inline{
		{Kind: TextInline, Text: `\ { } $ & # % _ ^ ~`},
		{Kind: TextInline, Text: " via "},
		{Kind: LinkInline, Text: "source", URL: "https://example.test/a_b?x=1%"},
		{Kind: TextInline, Text: " "},
		{Kind: CitationInline, Citation: []string{"smith2024"}},
	}}}
	figure := Block{Kind: FigureBlockKind, Figure: Figure{
		ID: "plot", Source: "plot.png", AltText: "plot", Caption: []Inline{{Kind: TextInline, Text: "Figure 1 — Response"}},
		Placement: FigurePlacement{Kind: AutoPlacement, Auto: AutoFigurePlacement{Alignment: Center, Size: FigureSize{Width: Length{Unit: Millimeter, Value: 40}}}},
	}}
	table := Block{Kind: LabeledTableBlockKind, LabeledTable: LabeledTable{
		ID: "results", Caption: []Inline{{Kind: TextInline, Text: "Table 1 — Results"}},
		Table: Table{
			Header: Row{Cells: []Cell{
				{Alignment: Left, Content: []Inline{{Kind: TextInline, Text: "Name"}}},
				{Alignment: Right, Content: []Inline{{Kind: TextInline, Text: "Value"}}},
			}},
			Body: []Row{{Cells: []Cell{
				{Alignment: Left, Content: []Inline{{Kind: TextInline, Text: "A"}}},
				{Alignment: Right, Content: []Inline{{Kind: TextInline, Text: "1"}}},
			}}},
		},
	}}
	return Doc{
		SourceRoot: sourceRoot,
		Metadata:   Metadata{Title: "Escaped & Deterministic", Author: "Ada Author", Subject: "M3"},
		Style:      styles,
		Layout:     PageLayout{Size: A4, Orientation: Landscape, Margins: Margins{TopPt: 36, BottomPt: 42, LeftPt: 48, RightPt: 54}},
		Content: []Block{
			{Kind: HeadingBlockKind, Heading: Heading{Level: 1, Role: TitleRole, Content: []Inline{{Kind: TextInline, Text: "Escaped & Deterministic"}}}},
			{Kind: AbstractBlockKind, Abstract: Abstract{Children: []Block{{Kind: ParagraphBlockKind, Paragraph: Paragraph{Role: BodyRole, Content: []Inline{{Kind: TextInline, Text: "Evidence."}}}}}}},
			{Kind: PageChromeBlockKind, PageChrome: PageChrome{Header: []Inline{{Kind: DocumentTitleInline}}, Footer: []Inline{{Kind: TextInline, Text: "Page "}, {Kind: PageNumberInline}}}},
			{Kind: SectionBlockKind, Section: Section{ID: "method", Level: 1, Title: []Inline{{Kind: TextInline, Text: "Method"}}, Children: []Block{
				paragraph,
				{Kind: EquationBlockKind, Equation: Equation{ID: "energy", Latex: "E = mc^2", Numbered: true}},
				figure,
				table,
			}}},
			{Kind: BibliographyBlockKind, Bibliography: Bibliography{Source: "references.bib"}},
		},
	}
}
