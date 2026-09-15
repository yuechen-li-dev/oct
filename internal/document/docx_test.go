package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDOCXPackageIsDeterministicAndParseable(t *testing.T) {
	doc := docxPackageFixture()
	first, err := DOCX(doc)
	if err != nil {
		t.Fatalf("first DOCX render: %v", err)
	}
	second, err := DOCX(doc)
	if err != nil {
		t.Fatalf("second DOCX render: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("equal semantic documents must produce byte-identical DOCX packages")
	}

	reader, err := zip.NewReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatalf("open DOCX ZIP: %v", err)
	}
	required := map[string]bool{
		"[Content_Types].xml":          false,
		"_rels/.rels":                  false,
		"docProps/app.xml":             false,
		"docProps/core.xml":            false,
		"word/_rels/document.xml.rels": false,
		"word/document.xml":            false,
		"word/numbering.xml":           false,
		"word/styles.xml":              false,
	}
	parts := map[string]string{}
	for _, file := range reader.File {
		if _, ok := required[file.Name]; !ok {
			t.Fatalf("unexpected package part %q", file.Name)
		}
		required[file.Name] = true
		if file.Modified.Year() != 1980 || file.Modified.Month() != 1 || file.Modified.Day() != 1 {
			t.Fatalf("part %s has non-canonical timestamp %s", file.Name, file.Modified)
		}
		rc, openErr := file.Open()
		if openErr != nil {
			t.Fatalf("open %s: %v", file.Name, openErr)
		}
		payload, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatalf("read %s: %v", file.Name, readErr)
		}
		if closeErr != nil {
			t.Fatalf("close %s: %v", file.Name, closeErr)
		}
		parts[file.Name] = string(payload)
		decoder := xml.NewDecoder(bytes.NewReader(payload))
		for {
			_, parseErr := decoder.Token()
			if parseErr == io.EOF {
				break
			}
			if parseErr != nil {
				t.Fatalf("parse XML part %s: %v", file.Name, parseErr)
			}
		}
	}
	for name, found := range required {
		if !found {
			t.Fatalf("required package part %s is missing", name)
		}
	}
	documentXML := parts["word/document.xml"]
	for _, expected := range []string{`w:pStyle w:val="Title"`, `w:numId w:val="1"`, `w:numId w:val="2"`, `w:type="page"`, `w:pgSz w:w="16838" w:h="11906" w:orient="landscape"`, `w:pgMar w:top="720"`, `r:id="rId3"`} {
		if !strings.Contains(documentXML, expected) {
			t.Errorf("document.xml missing %s", expected)
		}
	}
	if !strings.Contains(parts["word/_rels/document.xml.rels"], `Target="https://example.test/evidence" TargetMode="External"`) {
		t.Error("document relationships missing external hyperlink")
	}
	if !strings.Contains(parts["word/styles.xml"], `w:styleId="Code"`) {
		t.Error("styles.xml missing semantic Code style")
	}
}

func TestDOCXFiguresAnchorsChromeAndDeterminism(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "plot.png")
	file, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{B: 255, A: 255})
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	doc := docxPackageFixture()
	doc.SourceRoot = dir
	doc.Content = append(doc.Content,
		Block{Kind: FigureBlockKind, Figure: Figure{ID: "auto", Source: "plot.png", AltText: "flow plot", Caption: []Inline{{Kind: TextInline, Text: "Figure 1 — Flow"}}, Placement: FigurePlacement{Kind: AutoPlacement, Auto: AutoFigurePlacement{Alignment: Center, KeepWithCaption: true, Size: FigureSize{Width: Length{Unit: Millimeter, Value: 120}}}}}},
		Block{Kind: FigureBlockKind, Figure: Figure{ID: "manual", Source: "plot.png", AltText: "anchored plot", Caption: []Inline{{Kind: TextInline, Text: "Figure 2 — Manual"}}, Placement: FigurePlacement{Kind: AnchoredPlacement, Anchored: AnchoredFigurePlacement{Anchor: PageAnchor, X: Length{Unit: Millimeter, Value: 20}, Y: Length{Unit: Millimeter, Value: 30}, Size: FigureSize{Width: Length{Unit: Millimeter, Value: 120}}, Wrap: SquareWrap, ZOrder: 7}}}},
		Block{Kind: FigureBlockKind, Figure: Figure{ID: "exact", Source: "plot.png", AltText: "exact plot", Caption: []Inline{{Kind: TextInline, Text: "Figure 3 — Exact"}}, Placement: FigurePlacement{Kind: AutoPlacement, Auto: AutoFigurePlacement{Alignment: Center, Size: FigureSize{Width: Length{Unit: Millimeter, Value: 120}, Height: lengthPointer(Length{Unit: Millimeter, Value: 80})}}}}},
		Block{Kind: PageChromeBlockKind, PageChrome: PageChrome{Header: []Inline{{Kind: DocumentTitleInline}, {Kind: TextInline, Text: " — "}, {Kind: LinkInline, Text: "evidence", URL: "https://example.test/header"}}, Footer: []Inline{{Kind: TextInline, Text: "Page "}, {Kind: PageNumberInline}}}},
	)
	first, err := DOCX(doc)
	if err != nil {
		t.Fatal(err)
	}
	second, err := DOCX(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("image DOCX must be byte-identical")
	}
	parts := unzipParts(t, first)
	if got := countPrefix(parts, "word/media/"); got != 1 {
		t.Fatalf("expected one content-deduplicated media part, got %d", got)
	}
	documentXML := parts["word/document.xml"]
	for _, expected := range []string{`<wp:inline`, `<wp:anchor`, `<wp:posOffset>7200000</wp:posOffset>`, `<wp:posOffset>10800000</wp:posOffset>`, `cx="43200000"`, `cy="21600000"`, `cy="28800000"`, `relativeFrom="page"`, `<wp:wrapSquare`, `wp:docPr id="1"`, `wp:docPr id="2"`, `wp:docPr id="3"`, `w:headerReference`, `w:footerReference`} {
		if !strings.Contains(documentXML, expected) {
			t.Errorf("document.xml missing %s", expected)
		}
	}
	if !strings.Contains(parts["word/footer1.xml"], `PAGE`) {
		t.Error("footer does not contain semantic page-number field")
	}
	if !strings.Contains(parts["word/header1.xml"], `Package proof`) {
		t.Error("header does not contain document title")
	}
	if !strings.Contains(parts["word/_rels/header1.xml.rels"], `Target="https://example.test/header"`) {
		t.Error("header rich-inline hyperlink relationship is missing")
	}
	rels := parts["word/_rels/document.xml.rels"]
	if strings.Count(rels, `relationships/image`) != 1 {
		t.Fatalf("expected stable reused image relationship, got %s", rels)
	}
	if !strings.Contains(rels, `Id="rId5"`) {
		t.Errorf("image relationship ID is not deterministic: %s", rels)
	}
}

func TestDocumentLengthConversionsAreExact(t *testing.T) {
	if got := lengthEMU(Length{Unit: Millimeter, Value: 20}); got != 7_200_000 {
		t.Fatalf("20 mm = %d EMU", got)
	}
	if got := lengthEMU(Length{Unit: Point, Value: 36}); got != 457_200 {
		t.Fatalf("36 pt = %d EMU", got)
	}
}

func TestDOCXFigureFailuresAreDiagnostic(t *testing.T) {
	doc := docxPackageFixture()
	doc.SourceRoot = t.TempDir()
	doc.Content = []Block{{Kind: FigureBlockKind, Figure: Figure{ID: "missing", Source: "missing.png", AltText: "missing", Placement: FigurePlacement{Kind: AutoPlacement, Auto: AutoFigurePlacement{Size: FigureSize{Width: Length{Unit: Millimeter, Value: 10}}}}}}}
	_, err := DOCX(doc)
	if err == nil || !strings.Contains(err.Error(), "missing image") {
		t.Fatalf("expected missing image diagnostic, got %v", err)
	}
}

func TestDOCXSupportsJPEGFigures(t *testing.T) {
	dir := t.TempDir(); path := filepath.Join(dir, "photo.jpeg")
	file, err := os.Create(path); if err != nil { t.Fatal(err) }
	if err := jpeg.Encode(file, image.NewRGBA(image.Rect(0, 0, 3, 2)), nil); err != nil { t.Fatal(err) }
	if err := file.Close(); err != nil { t.Fatal(err) }
	doc := docxPackageFixture(); doc.SourceRoot = dir
	doc.Content = []Block{{Kind: FigureBlockKind, Figure: Figure{Source: "photo.jpeg", AltText: "JPEG", Placement: FigurePlacement{Kind: AutoPlacement, Auto: AutoFigurePlacement{Size: FigureSize{Width: Length{Unit: Point, Value: 72}}}}}}}
	payload, err := DOCX(doc); if err != nil { t.Fatal(err) }
	parts := unzipParts(t, payload)
	if countPrefix(parts, "word/media/image-") != 1 { t.Fatal("JPEG media part missing") }
	if !strings.Contains(parts["[Content_Types].xml"], `Extension="jpg" ContentType="image/jpeg"`) { t.Error("JPEG content type missing") }
}

func unzipParts(t *testing.T, payload []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		if err := rc.Close(); err != nil {
			t.Fatal(err)
		}
		out[f.Name] = string(data)
	}
	return out
}

func countPrefix(parts map[string]string, prefix string) int {
	count := 0
	for name := range parts {
		if strings.HasPrefix(name, prefix) {
			count++
		}
	}
	return count
}

func lengthPointer(value Length) *Length { return &value }

func docxPackageFixture() Doc {
	text := TextStyle{FontFamily: "Inter", SizePt: 10.5, Weight: Normal, Color: Color{Hex: "#202124"}}
	paragraph := ParagraphStyle{Text: text, Alignment: Left, SpaceAfterPt: 6, LineSpacing: 1.08}
	styles := StyleSheet{Body: paragraph, Title: paragraph, Subtitle: paragraph, Heading1: paragraph, Heading2: paragraph, Caption: paragraph, Code: paragraph, Small: paragraph}
	return Doc{
		Metadata: Metadata{Title: "Package proof", Author: "Oct", Subject: "DOCX"},
		Style:    styles,
		Layout:   PageLayout{Size: A4, Orientation: Landscape, Margins: Margins{TopPt: 36, BottomPt: 36, LeftPt: 42, RightPt: 42}},
		Content: []Block{
			{Kind: HeadingBlockKind, Heading: Heading{Level: 1, Role: TitleRole, Content: []Inline{{Kind: TextInline, Text: "Package proof"}}}},
			{Kind: ParagraphBlockKind, Paragraph: Paragraph{Role: BodyRole, Content: []Inline{{Kind: StrongInline, Text: "bold"}, {Kind: TextInline, Text: " / "}, {Kind: EmphasisInline, Text: "italic"}, {Kind: TextInline, Text: " / "}, {Kind: CodeInline, Text: "code"}, {Kind: TextInline, Text: " / "}, {Kind: LinkInline, Text: "evidence", URL: "https://example.test/evidence"}}}},
			{Kind: ListBlockKind, List: List{Kind: BulletList, Items: [][]Inline{{{Kind: TextInline, Text: "bullet"}}}}},
			{Kind: ListBlockKind, List: List{Kind: NumberedList, Items: [][]Inline{{{Kind: TextInline, Text: "numbered"}}}}},
			{Kind: TableBlockKind, Table: Table{Header: Row{Cells: []Cell{{Content: []Inline{{Kind: TextInline, Text: "Key"}}, Alignment: Left}}}, Body: []Row{{Cells: []Cell{{Content: []Inline{{Kind: TextInline, Text: "Value"}}, Alignment: Right}}}}}},
			{Kind: CalloutBlockKind, Callout: Callout{Kind: "Warning", Content: []Inline{{Kind: TextInline, Text: "bounded"}}}},
			{Kind: CodeBlockKind, Code: CodeBlock{Language: "oct", Lines: []string{"let x = 1", "return x"}}},
			{Kind: HorizontalRuleBlockKind},
			{Kind: PageBreakBlockKind},
		},
	}
}
