package document

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
const wordNS = `http://schemas.openxmlformats.org/wordprocessingml/2006/main`
const relNS = `http://schemas.openxmlformats.org/officeDocument/2006/relationships`
const drawingNS = `http://schemas.openxmlformats.org/drawingml/2006/main`
const wordDrawingNS = `http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing`
const pictureNS = `http://schemas.openxmlformats.org/drawingml/2006/picture`

func DOCX(doc Doc) ([]byte, error) {
	if diagnostics := Validate(doc); len(diagnostics) > 0 {
		return nil, fmt.Errorf("Document.Docx: %s", strings.Join(diagnostics, "; "))
	}
	r := &docxRenderer{doc: doc, nextRelationshipID: 3, nextDrawingID: 1, mediaByHash: map[string]mediaPart{}}
	r.findPageChrome(doc.Content)
	if len(r.chrome.Header) > 0 {
		r.headerID = r.nextRelationship()
	}
	if len(r.chrome.Footer) > 0 {
		r.footerID = r.nextRelationship()
	}
	documentXML := r.documentXML()
	if r.err != nil {
		return nil, r.err
	}
	parts := map[string][]byte{
		"[Content_Types].xml":          []byte(contentTypesXML(r)),
		"_rels/.rels":                  []byte(rootRelationshipsXML()),
		"docProps/app.xml":             []byte(appPropertiesXML()),
		"docProps/core.xml":            []byte(corePropertiesXML(doc.Metadata)),
		"word/_rels/document.xml.rels": []byte(r.documentRelationshipsXML()),
		"word/document.xml":            []byte(documentXML),
		"word/numbering.xml":           []byte(numberingXML()),
		"word/styles.xml":              []byte(stylesXML(doc.Style)),
	}
	for _, media := range r.orderedMedia() {
		parts["word/media/"+media.name] = media.bytes
	}
	if len(r.chrome.Header) > 0 {
		parts["word/header1.xml"] = []byte(r.headerFooterXML(true, r.chrome.Header))
	}
	if len(r.chrome.Footer) > 0 {
		parts["word/footer1.xml"] = []byte(r.headerFooterXML(false, r.chrome.Footer))
	}
	if len(r.headerLinks) > 0 {
		parts["word/_rels/header1.xml.rels"] = []byte(hyperlinkRelationshipsXML(r.headerLinks))
	}
	if len(r.footerLinks) > 0 {
		parts["word/_rels/footer1.xml.rels"] = []byte(hyperlinkRelationshipsXML(r.footerLinks))
	}
	return deterministicZip(parts)
}

type hyperlinkRelationship struct{ id, target string }
type imageRelationship struct{ id, target string }
type mediaPart struct {
	name, contentType string
	bytes             []byte
}

type docxRenderer struct {
	doc                 Doc
	hyperlinks          []hyperlinkRelationship
	images              []imageRelationship
	nextRelationshipID  int
	nextDrawingID       int
	mediaByHash         map[string]mediaPart
	chrome              PageChrome
	headerID            string
	footerID            string
	err                 error
	relationshipContext string
	headerLinks         []hyperlinkRelationship
	footerLinks         []hyperlinkRelationship
}

func (r *docxRenderer) documentXML() string {
	var body strings.Builder
	for _, block := range r.doc.Content {
		r.writeBlock(&body, block)
	}
	body.WriteString(sectionPropertiesXML(r.doc.Layout, r.headerID, r.footerID))
	return xmlHeader + `<w:document xmlns:w="` + wordNS + `" xmlns:r="` + relNS + `" xmlns:wp="` + wordDrawingNS + `" xmlns:a="` + drawingNS + `" xmlns:pic="` + pictureNS + `"><w:body>` + body.String() + `</w:body></w:document>`
}

func (r *docxRenderer) writeBlock(out *strings.Builder, block Block) {
	switch block.Kind {
	case HeadingBlockKind:
		r.writeParagraph(out, block.Heading.Content, block.Heading.Role, nil)
	case ParagraphBlockKind:
		r.writeParagraph(out, block.Paragraph.Content, block.Paragraph.Role, nil)
	case ListBlockKind:
		for _, item := range block.List.Items {
			numID := 1
			if block.List.Kind == NumberedList {
				numID = 2
			}
			r.writeParagraph(out, item, BodyRole, &numID)
		}
	case TableBlockKind:
		r.writeTable(out, block.Table, "")
	case LabeledTableBlockKind:
		r.writeTable(out, block.LabeledTable.Table, "")
		r.writeParagraph(out, block.LabeledTable.Caption, CaptionRole, nil)
	case FigureBlockKind:
		r.writeFigure(out, block.Figure)
	case PageChromeBlockKind:
		// Page chrome is emitted in section-owned header/footer parts.
	case CodeBlockKind:
		content := []Inline{}
		for index, line := range block.Code.Lines {
			if index > 0 {
				content = append(content, Inline{Kind: LineBreakInline})
			}
			content = append(content, Inline{Kind: TextInline, Text: line})
		}
		r.writeParagraph(out, content, CodeRole, nil)
	case CalloutBlockKind:
		shade := map[CalloutKind]string{"Note": "E7F3FF", "Info": "E7F3FF", "Warning": "FFF2CC", "Danger": "FCE4D6", "Success": "E2F0D9"}[block.Callout.Kind]
		r.writeCallout(out, block.Callout, shade)
	case HorizontalRuleBlockKind:
		out.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="8" w:space="1" w:color="808080"/></w:pBdr></w:pPr></w:p>`)
	case PageBreakBlockKind:
		out.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
	case GroupBlockKind:
		for _, child := range block.Children {
			r.writeBlock(out, child)
		}
	}
}

func (r *docxRenderer) writeParagraph(out *strings.Builder, content []Inline, role ParagraphRole, numID *int) {
	out.WriteString(`<w:p><w:pPr><w:pStyle w:val="` + styleID(role) + `"/>`)
	if numID != nil {
		out.WriteString(`<w:numPr><w:ilvl w:val="0"/><w:numId w:val="` + strconv.Itoa(*numID) + `"/></w:numPr>`)
	}
	out.WriteString(`</w:pPr>`)
	r.writeInlines(out, content, false, false, false)
	out.WriteString(`</w:p>`)
}

func (r *docxRenderer) writeInlines(out *strings.Builder, content []Inline, bold, italic, code bool) {
	for _, inline := range content {
		switch inline.Kind {
		case TextInline:
			writeRun(out, inline.Text, bold, italic, code)
		case StrongInline:
			writeRun(out, inline.Text, true, italic, code)
		case EmphasisInline:
			writeRun(out, inline.Text, bold, true, code)
		case CodeInline:
			writeRun(out, inline.Text, bold, italic, true)
		case LinkInline:
			id := ""
			if r.relationshipContext == "header" {
				id = fmt.Sprintf("rId%d", len(r.headerLinks)+1)
				r.headerLinks = append(r.headerLinks, hyperlinkRelationship{id: id, target: inline.URL})
			} else if r.relationshipContext == "footer" {
				id = fmt.Sprintf("rId%d", len(r.footerLinks)+1)
				r.footerLinks = append(r.footerLinks, hyperlinkRelationship{id: id, target: inline.URL})
			} else {
				id = r.nextRelationship()
				r.hyperlinks = append(r.hyperlinks, hyperlinkRelationship{id: id, target: inline.URL})
			}
			out.WriteString(`<w:hyperlink r:id="` + id + `"><w:r><w:rPr><w:rStyle w:val="Hyperlink"/></w:rPr><w:t xml:space="preserve">` + escapeXML(inline.Text) + `</w:t></w:r></w:hyperlink>`)
		case LineBreakInline:
			out.WriteString(`<w:r><w:br/></w:r>`)
		case PageNumberInline:
			out.WriteString(`<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>`)
		case DocumentTitleInline:
			writeRun(out, r.doc.Metadata.Title, bold, italic, code)
		case DocumentAuthorInline:
			writeRun(out, r.doc.Metadata.Author, bold, italic, code)
		case ReferenceInline:
			writeRun(out, "??", bold, italic, code)
		}
	}
}

func (r *docxRenderer) nextRelationship() string {
	id := fmt.Sprintf("rId%d", r.nextRelationshipID)
	r.nextRelationshipID++
	return id
}

func (r *docxRenderer) findPageChrome(blocks []Block) {
	for _, block := range blocks {
		if block.Kind == PageChromeBlockKind {
			r.chrome = block.PageChrome
		}
		if block.Kind == GroupBlockKind {
			r.findPageChrome(block.Children)
		}
	}
}

func (r *docxRenderer) writeFigure(out *strings.Builder, figure Figure) {
	if r.err != nil {
		return
	}
	path := figure.Source
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.doc.SourceRoot, filepath.FromSlash(path))
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		r.err = fmt.Errorf("Document figure %q: missing image %s: %w", figure.ID, figure.Source, err)
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil || (format != "png" && format != "jpeg") {
		r.err = fmt.Errorf("Document figure %q: unsupported or invalid image %s (PNG and JPEG are supported)", figure.ID, figure.Source)
		return
	}
	ext := ".png"
	contentType := "image/png"
	if format == "jpeg" {
		ext = ".jpg"
		contentType = "image/jpeg"
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	media, ok := r.mediaByHash[hash]
	if !ok {
		media = mediaPart{name: "image-" + hash[:16] + ext, contentType: contentType, bytes: payload}
		r.mediaByHash[hash] = media
	}
	rid := ""
	for _, imageRel := range r.images {
		if imageRel.target == "media/"+media.name {
			rid = imageRel.id
			break
		}
	}
	if rid == "" {
		rid = r.nextRelationship()
		r.images = append(r.images, imageRelationship{id: rid, target: "media/" + media.name})
	}
	size := figure.Placement.Auto.Size
	if figure.Placement.Kind == AnchoredPlacement {
		size = figure.Placement.Anchored.Size
	}
	cx := lengthEMU(size.Width)
	cy := int64(math.Round(float64(cx) * float64(config.Height) / float64(config.Width)))
	if size.Height != nil {
		cy = lengthEMU(*size.Height)
	}
	if cx <= 0 || cy <= 0 {
		r.err = fmt.Errorf("Document figure %q: invalid dimensions", figure.ID)
		return
	}
	drawingID := r.nextDrawingID
	r.nextDrawingID++
	out.WriteString(`<w:p><w:pPr>`)
	if figure.Placement.Kind == AutoPlacement {
		out.WriteString(`<w:jc w:val="` + alignmentValue(figure.Placement.Auto.Alignment) + `"/>`)
		if figure.Placement.Auto.KeepWithCaption {
			out.WriteString(`<w:keepNext/>`)
		}
	}
	out.WriteString(`</w:pPr><w:r><w:drawing>`)
	if figure.Placement.Kind == AutoPlacement {
		out.WriteString(`<wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="` + strconv.FormatInt(cx, 10) + `" cy="` + strconv.FormatInt(cy, 10) + `"/><wp:effectExtent l="0" t="0" r="0" b="0"/><wp:docPr id="` + strconv.Itoa(drawingID) + `" name="Figure ` + strconv.Itoa(drawingID) + `" descr="` + escapeXML(figure.AltText) + `"/><wp:cNvGraphicFramePr/>` + pictureXML(rid, media.name, drawingID, cx, cy) + `</wp:inline>`)
	} else {
		p := figure.Placement.Anchored
		behind := "0"
		if p.Wrap == BehindTextWrap {
			behind = "1"
		}
		out.WriteString(`<wp:anchor distT="0" distB="0" distL="0" distR="0" simplePos="0" relativeHeight="` + strconv.Itoa(p.ZOrder) + `" behindDoc="` + behind + `" locked="0" layoutInCell="1" allowOverlap="1"><wp:simplePos x="0" y="0"/><wp:positionH relativeFrom="` + horizontalReference(p.Anchor) + `"><wp:posOffset>` + strconv.FormatInt(lengthEMU(p.X), 10) + `</wp:posOffset></wp:positionH><wp:positionV relativeFrom="` + verticalReference(p.Anchor) + `"><wp:posOffset>` + strconv.FormatInt(lengthEMU(p.Y), 10) + `</wp:posOffset></wp:positionV><wp:extent cx="` + strconv.FormatInt(cx, 10) + `" cy="` + strconv.FormatInt(cy, 10) + `"/><wp:effectExtent l="0" t="0" r="0" b="0"/>` + wrapXML(p.Wrap) + `<wp:docPr id="` + strconv.Itoa(drawingID) + `" name="Figure ` + strconv.Itoa(drawingID) + `" descr="` + escapeXML(figure.AltText) + `"/><wp:cNvGraphicFramePr/>` + pictureXML(rid, media.name, drawingID, cx, cy) + `</wp:anchor>`)
	}
	out.WriteString(`</w:drawing></w:r></w:p>`)
	if len(figure.Caption) > 0 {
		r.writeParagraph(out, figure.Caption, CaptionRole, nil)
	}
}

func pictureXML(rid, name string, id int, cx, cy int64) string {
	return `<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:nvPicPr><pic:cNvPr id="` + strconv.Itoa(id) + `" name="` + escapeXML(name) + `"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="` + rid + `"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + strconv.FormatInt(cx, 10) + `" cy="` + strconv.FormatInt(cy, 10) + `"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic>`
}
func wrapXML(w FigureWrap) string {
	switch w {
	case SquareWrap:
		return `<wp:wrapSquare wrapText="bothSides"/>`
	case TopBottomWrap:
		return `<wp:wrapTopAndBottom/>`
	default:
		return `<wp:wrapNone/>`
	}
}
func horizontalReference(a FigureAnchor) string {
	switch a {
	case MarginAnchor:
		return "margin"
	case ParagraphAnchor:
		return "column"
	case CharacterAnchor:
		return "character"
	default:
		return "page"
	}
}
func verticalReference(a FigureAnchor) string {
	switch a {
	case MarginAnchor:
		return "margin"
	case ParagraphAnchor:
		return "paragraph"
	case CharacterAnchor:
		return "line"
	default:
		return "page"
	}
}
func lengthEMU(l Length) int64 {
	if l.Unit == Millimeter {
		return int64(math.Round(l.Value * 360000.0))
	}
	return int64(math.Round(l.Value * 12700.0))
}
func (r *docxRenderer) orderedMedia() []mediaPart {
	out := make([]mediaPart, 0, len(r.mediaByHash))
	for _, m := range r.mediaByHash {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func writeRun(out *strings.Builder, text string, bold, italic, code bool) {
	out.WriteString(`<w:r>`)
	if bold || italic || code {
		out.WriteString(`<w:rPr>`)
		if bold {
			out.WriteString(`<w:b/>`)
		}
		if italic {
			out.WriteString(`<w:i/>`)
		}
		if code {
			out.WriteString(`<w:rFonts w:ascii="Cascadia Mono" w:hAnsi="Cascadia Mono"/>`)
		}
		out.WriteString(`</w:rPr>`)
	}
	parts := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	for i, part := range parts {
		if i > 0 {
			out.WriteString(`<w:br/>`)
		}
		out.WriteString(`<w:t xml:space="preserve">` + escapeXML(part) + `</w:t>`)
	}
	out.WriteString(`</w:r>`)
}

func (r *docxRenderer) writeTable(out *strings.Builder, table Table, shade string) {
	out.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders><w:top w:val="single" w:sz="4" w:color="D9D9D9"/><w:left w:val="single" w:sz="4" w:color="D9D9D9"/><w:bottom w:val="single" w:sz="4" w:color="D9D9D9"/><w:right w:val="single" w:sz="4" w:color="D9D9D9"/><w:insideH w:val="single" w:sz="4" w:color="D9D9D9"/><w:insideV w:val="single" w:sz="4" w:color="D9D9D9"/></w:tblBorders></w:tblPr>`)
	r.writeRow(out, table.Header, true, shade)
	for _, row := range table.Body {
		r.writeRow(out, row, false, shade)
	}
	out.WriteString(`</w:tbl>`)
}

func (r *docxRenderer) writeRow(out *strings.Builder, row Row, header bool, shade string) {
	out.WriteString(`<w:tr>`)
	for _, cell := range row.Cells {
		out.WriteString(`<w:tc><w:tcPr><w:vAlign w:val="center"/><w:tcMar><w:top w:w="80" w:type="dxa"/><w:left w:w="100" w:type="dxa"/><w:bottom w:w="80" w:type="dxa"/><w:right w:w="100" w:type="dxa"/></w:tcMar>`)
		if header && shade == "" {
			shade = "D9EAF7"
		}
		if shade != "" {
			out.WriteString(`<w:shd w:val="clear" w:fill="` + shade + `"/>`)
		}
		out.WriteString(`</w:tcPr><w:p><w:pPr><w:jc w:val="` + alignmentValue(cell.Alignment) + `"/></w:pPr>`)
		r.writeInlines(out, cell.Content, header, false, false)
		out.WriteString(`</w:p></w:tc>`)
	}
	out.WriteString(`</w:tr>`)
}

func (r *docxRenderer) writeCallout(out *strings.Builder, callout Callout, shade string) {
	cell := Cell{Alignment: Left, Content: append([]Inline{{Kind: StrongInline, Text: calloutLabel(callout.Kind) + ": "}}, callout.Content...)}
	r.writeTable(out, Table{Header: Row{Cells: []Cell{cell}}}, shade)
}

func (r *docxRenderer) documentRelationshipsXML() string {
	var out strings.Builder
	out.WriteString(xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	out.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	out.WriteString(`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>`)
	if r.headerID != "" {
		out.WriteString(`<Relationship Id="` + r.headerID + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>`)
	}
	if r.footerID != "" {
		out.WriteString(`<Relationship Id="` + r.footerID + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>`)
	}
	for _, image := range r.images {
		out.WriteString(`<Relationship Id="` + image.id + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="` + image.target + `"/>`)
	}
	for _, link := range r.hyperlinks {
		out.WriteString(`<Relationship Id="` + link.id + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="` + escapeXML(link.target) + `" TargetMode="External"/>`)
	}
	out.WriteString(`</Relationships>`)
	return out.String()
}

func (r *docxRenderer) headerFooterXML(header bool, content []Inline) string {
	var body strings.Builder
	previousContext := r.relationshipContext
	if header {
		r.relationshipContext = "header"
	} else {
		r.relationshipContext = "footer"
	}
	r.writeParagraph(&body, content, SmallRole, nil)
	r.relationshipContext = previousContext
	name := "ftr"
	if header {
		name = "hdr"
	}
	return xmlHeader + `<w:` + name + ` xmlns:w="` + wordNS + `" xmlns:r="` + relNS + `">` + body.String() + `</w:` + name + `>`
}

func hyperlinkRelationshipsXML(links []hyperlinkRelationship) string {
	var out strings.Builder
	out.WriteString(xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, link := range links {
		out.WriteString(`<Relationship Id="` + link.id + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="` + escapeXML(link.target) + `" TargetMode="External"/>`)
	}
	out.WriteString(`</Relationships>`)
	return out.String()
}

func stylesXML(styles StyleSheet) string {
	ordered := []struct {
		id, name string
		style    ParagraphStyle
	}{
		{"Normal", "Normal", styles.Body}, {"Title", "Title", styles.Title}, {"Subtitle", "Subtitle", styles.Subtitle},
		{"Heading1", "heading 1", styles.Heading1}, {"Heading2", "heading 2", styles.Heading2}, {"Caption", "Caption", styles.Caption},
		{"Code", "Code", styles.Code}, {"Small", "Small", styles.Small},
	}
	var out strings.Builder
	out.WriteString(xmlHeader + `<w:styles xmlns:w="` + wordNS + `">`)
	for index, entry := range ordered {
		defaultAttr := ""
		if index == 0 {
			defaultAttr = ` w:default="1"`
		}
		out.WriteString(`<w:style w:type="paragraph" w:styleId="` + entry.id + `"` + defaultAttr + `><w:name w:val="` + entry.name + `"/>`)
		if index > 0 {
			out.WriteString(`<w:basedOn w:val="Normal"/>`)
		}
		out.WriteString(paragraphProperties(entry.style) + runProperties(entry.style.Text) + `</w:style>`)
	}
	out.WriteString(`<w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:rPr><w:color w:val="0563C1"/><w:u w:val="single"/></w:rPr></w:style>`)
	out.WriteString(`</w:styles>`)
	return out.String()
}

func paragraphProperties(style ParagraphStyle) string {
	line := int(math.Round(style.LineSpacing * 240.0))
	return `<w:pPr><w:jc w:val="` + alignmentValue(style.Alignment) + `"/><w:spacing w:before="` + twips(style.SpaceBeforePt) + `" w:after="` + twips(style.SpaceAfterPt) + `" w:line="` + strconv.Itoa(line) + `" w:lineRule="auto"/></w:pPr>`
}

func runProperties(style TextStyle) string {
	color := strings.TrimPrefix(style.Color.Hex, "#")
	if color == "" {
		color = "000000"
	}
	out := `<w:rPr><w:rFonts w:ascii="` + escapeXML(style.FontFamily) + `" w:hAnsi="` + escapeXML(style.FontFamily) + `"/><w:color w:val="` + escapeXML(color) + `"/><w:sz w:val="` + strconv.Itoa(int(math.Round(style.SizePt*2))) + `"/><w:szCs w:val="` + strconv.Itoa(int(math.Round(style.SizePt*2))) + `"/>`
	if style.Weight == Bold || style.Weight == Medium {
		out += `<w:b/>`
	}
	if style.Italic {
		out += `<w:i/>`
	}
	return out + `</w:rPr>`
}

func sectionPropertiesXML(layout PageLayout, headerID, footerID string) string {
	w, h := 12240, 15840
	if layout.Size == A4 {
		w, h = 11906, 16838
	}
	orient := ""
	if layout.Orientation == Landscape {
		w, h = h, w
		orient = ` w:orient="landscape"`
	}
	refs := ""
	if headerID != "" {
		refs += `<w:headerReference w:type="default" r:id="` + headerID + `"/>`
	}
	if footerID != "" {
		refs += `<w:footerReference w:type="default" r:id="` + footerID + `"/>`
	}
	return `<w:sectPr>` + refs + `<w:pgSz w:w="` + strconv.Itoa(w) + `" w:h="` + strconv.Itoa(h) + `"` + orient + `/><w:pgMar w:top="` + twips(layout.Margins.TopPt) + `" w:right="` + twips(layout.Margins.RightPt) + `" w:bottom="` + twips(layout.Margins.BottomPt) + `" w:left="` + twips(layout.Margins.LeftPt) + `" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`
}

func numberingXML() string {
	return xmlHeader + `<w:numbering xmlns:w="` + wordNS + `"><w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:tabs><w:tab w:val="num" w:pos="720"/></w:tabs><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum><w:abstractNum w:abstractNumId="1"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:lvlJc w:val="left"/><w:pPr><w:tabs><w:tab w:val="num" w:pos="720"/></w:tabs><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num><w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num></w:numbering>`
}

func contentTypesXML(r *docxRenderer) string {
	extra := ""
	hasPNG, hasJPEG := false, false
	for _, media := range r.mediaByHash {
		if media.contentType == "image/png" {
			hasPNG = true
		}
		if media.contentType == "image/jpeg" {
			hasJPEG = true
		}
	}
	if hasPNG {
		extra += `<Default Extension="png" ContentType="image/png"/>`
	}
	if hasJPEG {
		extra += `<Default Extension="jpg" ContentType="image/jpeg"/>`
	}
	if len(r.chrome.Header) > 0 {
		extra += `<Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>`
	}
	if len(r.chrome.Footer) > 0 {
		extra += `<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>`
	}
	return xmlHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>` + extra + `<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/><Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/></Types>`
}

func rootRelationshipsXML() string {
	return xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/></Relationships>`
}

func corePropertiesXML(metadata Metadata) string {
	return xmlHeader + `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>` + escapeXML(metadata.Title) + `</dc:title><dc:creator>` + escapeXML(metadata.Author) + `</dc:creator><dc:subject>` + escapeXML(metadata.Subject) + `</dc:subject><dcterms:created xsi:type="dcterms:W3CDTF">2000-01-01T00:00:00Z</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">2000-01-01T00:00:00Z</dcterms:modified></cp:coreProperties>`
}

func appPropertiesXML() string {
	return xmlHeader + `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes"><Application>OctCument</Application><AppVersion>1.0</AppVersion></Properties>`
}

func deterministicZip(parts map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	fixed := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: fixed}
		header.SetMode(0o644)
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := writer.Write(parts[name]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func styleID(role ParagraphRole) string {
	switch role {
	case TitleRole:
		return "Title"
	case SubtitleRole:
		return "Subtitle"
	case Heading1Role:
		return "Heading1"
	case Heading2Role:
		return "Heading2"
	case CaptionRole:
		return "Caption"
	case CodeRole:
		return "Code"
	case SmallRole:
		return "Small"
	default:
		return "Normal"
	}
}

func calloutLabel(kind CalloutKind) string {
	switch kind {
	case "Note":
		return "Note"
	case "Info":
		return "Info"
	case "Warning":
		return "Warning"
	case "Danger":
		return "Danger"
	case "Success":
		return "Success"
	default:
		return string(kind)
	}
}

func alignmentValue(alignment Alignment) string {
	switch alignment {
	case Center:
		return "center"
	case Right:
		return "right"
	case Justify:
		return "both"
	default:
		return "left"
	}
}

func twips(points float64) string { return strconv.Itoa(int(math.Round(points * 20))) }

func escapeXML(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	return strings.ReplaceAll(value, "'", "&apos;")
}
