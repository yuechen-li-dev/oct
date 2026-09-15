package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
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
