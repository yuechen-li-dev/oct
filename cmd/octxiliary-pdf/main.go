package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"codeberg.org/go-pdf/fpdf"
	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

const (
	pdfFamily          = "Pdf"
	pdfPageHandleType  = "Pdf.PdfPage"
	pdfTextStyleRecord = "Pdf.TextStyle"
	pdfPixelsPerInch   = 96.0
	pdfPointsPerInch   = 72.0
	pdfDefaultFontName = "Inter"
)

type pdfPage struct {
	doc                 *fpdf.Fpdf
	defaultFontFamily   string
	defaultFontSizePx   int
	defaultTextColorRGB [3]int
	fontFallbackUsed    bool
}

type textStyle struct {
	size   int
	colorR int
	colorG int
	colorB int
}

type pageTable struct {
	next  int
	pages map[int]*pdfPage
}

func newPageTable() *pageTable {
	return &pageTable{next: 1, pages: map[int]*pdfPage{}}
}

func (t *pageTable) allocate(page *pdfPage) int {
	id := t.next
	t.next++
	t.pages[id] = page
	return id
}

func (t *pageTable) get(value octxiliary.Value) (*pdfPage, error) {
	if value.Kind != octxiliary.ValueHandle {
		return nil, fmt.Errorf("expected page handle, got %s", value.Kind)
	}
	if value.HandleFamily != pdfFamily || value.HandleType != pdfPageHandleType {
		return nil, fmt.Errorf("expected %s %s handle", pdfFamily, pdfPageHandleType)
	}
	page, ok := t.pages[value.HandleID]
	if !ok {
		return nil, fmt.Errorf("unknown page handle %d", value.HandleID)
	}
	return page, nil
}

func main() {
	if err := octxiliary.ReadHandshake(os.Stdin); err != nil {
		return
	}
	if err := octxiliary.WriteHandshake(os.Stdout); err != nil {
		return
	}
	table := newPageTable()
	for {
		frame, err := octxiliary.ReadFrame(os.Stdin)
		if err != nil {
			return
		}
		req, parseErr := octxiliary.ParseRequest(frame)
		resp := octxiliary.Response{ID: req.ID}
		if parseErr != nil {
			resp.OK = false
			resp.Error = parseErr.Error()
			_ = octxiliary.WriteResponseFrame(os.Stdout, resp)
			continue
		}
		value, err := table.dispatch(req)
		if err != nil {
			resp.OK = false
			resp.Error = err.Error()
		} else {
			resp.OK = true
			resp.Value = value
			resp.HasValue = true
		}
		if err := octxiliary.WriteResponseFrame(os.Stdout, resp); err != nil {
			return
		}
	}
}

func (t *pageTable) dispatch(req octxiliary.Request) (octxiliary.Value, error) {
	if req.Family != pdfFamily {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "PdfNewPage":
		if err := expect(req.Args, octxiliary.ValueInt, octxiliary.ValueInt); err != nil {
			return octxiliary.Value{}, err
		}
		page, err := newPdfPage(req.Args[0].Int, req.Args[1].Int)
		if err != nil {
			return octxiliary.Value{}, err
		}
		id := t.allocate(page)
		return octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: pdfFamily, HandleType: pdfPageHandleType, HandleID: id}, nil
	case "PdfDrawText":
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueInt, octxiliary.ValueInt, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		page, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		if err := validateCoordinate(req.Args[1].Int, "x"); err != nil {
			return octxiliary.Value{}, err
		}
		if err := validateCoordinate(req.Args[2].Int, "y"); err != nil {
			return octxiliary.Value{}, err
		}
		if err := page.drawText(req.Args[3].String, req.Args[1].Int, req.Args[2].Int, page.defaultFontSizePx, page.defaultTextColorRGB); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
	case "PdfDrawTextStyled":
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueInt, octxiliary.ValueInt, octxiliary.ValueString, octxiliary.ValueRecord); err != nil {
			return octxiliary.Value{}, err
		}
		page, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		if err := validateCoordinate(req.Args[1].Int, "x"); err != nil {
			return octxiliary.Value{}, err
		}
		if err := validateCoordinate(req.Args[2].Int, "y"); err != nil {
			return octxiliary.Value{}, err
		}
		style, err := decodeTextStyle(req.Args[4])
		if err != nil {
			return octxiliary.Value{}, err
		}
		if err := page.drawText(req.Args[3].String, req.Args[1].Int, req.Args[2].Int, style.size, [3]int{style.colorR, style.colorG, style.colorB}); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
	case "PdfSave":
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		page, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		if err := page.save(req.Args[1].String); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func newPdfPage(widthPx int, heightPx int) (*pdfPage, error) {
	if widthPx <= 0 {
		return nil, fmt.Errorf("page width must be positive")
	}
	if heightPx <= 0 {
		return nil, fmt.Errorf("page height must be positive")
	}
	size := fpdf.SizeType{Wd: pxToPt(widthPx), Ht: pxToPt(heightPx)}
	doc := fpdf.NewCustom(&fpdf.InitType{UnitStr: "pt", Size: size})
	doc.SetAutoPageBreak(false, 0)
	doc.SetCompression(false)
	doc.AddPage()

	page := &pdfPage{
		doc:                 doc,
		defaultFontFamily:   pdfDefaultFontName,
		defaultFontSizePx:   16,
		defaultTextColorRGB: [3]int{0, 0, 0},
	}
	if err := page.configureDefaultFont(); err != nil {
		return nil, err
	}
	return page, nil
}

func (p *pdfPage) configureDefaultFont() error {
	fontBytes, fontErr := loadInterRegularTTF()
	if fontErr != nil {
		p.defaultFontFamily = "Helvetica"
		p.fontFallbackUsed = true
	} else {
		p.doc.AddUTF8FontFromBytes(pdfDefaultFontName, "", fontBytes)
		if pdfErr := p.doc.Error(); pdfErr != nil {
			p.defaultFontFamily = "Helvetica"
			p.fontFallbackUsed = true
			p.doc.SetError(nil)
		}
	}
	p.doc.SetFont(p.defaultFontFamily, "", pxToPt(p.defaultFontSizePx))
	p.doc.SetTextColor(p.defaultTextColorRGB[0], p.defaultTextColorRGB[1], p.defaultTextColorRGB[2])
	if pdfErr := p.doc.Error(); pdfErr != nil {
		return fmt.Errorf("pdf init failed: %v", pdfErr)
	}
	return nil
}

func loadInterRegularTTF() ([]byte, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("unable to resolve wrapper source path")
	}
	fontPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "internal", "interpret", "assets", "fonts", "Inter-Regular.ttf")
	return os.ReadFile(fontPath)
}

func (p *pdfPage) drawText(text string, xPx int, yPx int, fontSizePx int, colorRGB [3]int) error {
	p.doc.SetFont(p.defaultFontFamily, "", pxToPt(fontSizePx))
	p.doc.SetTextColor(colorRGB[0], colorRGB[1], colorRGB[2])
	lineHeightPt := pxToPt(fontSizePx)
	p.doc.SetXY(pxToPt(xPx), pxToPt(yPx))
	p.doc.CellFormat(0, lineHeightPt, text, "", 0, "", false, 0, "")
	if pdfErr := p.doc.Error(); pdfErr != nil {
		return fmt.Errorf("draw text failed: %v", pdfErr)
	}
	return nil
}

func (p *pdfPage) save(path string) error {
	if err := p.doc.OutputFileAndClose(path); err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	return nil
}

func decodeTextStyle(value octxiliary.Value) (textStyle, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != pdfTextStyleRecord {
		return textStyle{}, fmt.Errorf("expected %s record", pdfTextStyleRecord)
	}
	want := []string{"Size", "ColorR", "ColorG", "ColorB"}
	if len(value.Fields) != len(want) {
		return textStyle{}, fmt.Errorf("%s fields must be Size, ColorR, ColorG, ColorB", pdfTextStyleRecord)
	}
	decoded := make([]int, len(want))
	for i, name := range want {
		field := value.Fields[i]
		if field.Name != name {
			return textStyle{}, fmt.Errorf("%s fields must be Size, ColorR, ColorG, ColorB", pdfTextStyleRecord)
		}
		if field.Value.Kind != octxiliary.ValueInt {
			return textStyle{}, fmt.Errorf("%s field %s must be Int", pdfTextStyleRecord, name)
		}
		decoded[i] = field.Value.Int
	}
	if decoded[0] <= 0 {
		return textStyle{}, fmt.Errorf("text style size must be positive")
	}
	for i, channel := range decoded[1:] {
		if channel < 0 || channel > 255 {
			return textStyle{}, fmt.Errorf("text style color channel %s must be in [0, 255]", want[i+1])
		}
	}
	return textStyle{size: decoded[0], colorR: decoded[1], colorG: decoded[2], colorB: decoded[3]}, nil
}

func validateCoordinate(value int, name string) error {
	if value < 0 {
		return fmt.Errorf("%s coordinate must be non-negative", name)
	}
	return nil
}

func pxToPt(px int) float64 {
	return float64(px) * (pdfPointsPerInch / pdfPixelsPerInch)
}

func expect(args []octxiliary.Value, kinds ...octxiliary.ValueKind) error {
	if len(args) != len(kinds) {
		return fmt.Errorf("expected %d args, got %d", len(kinds), len(args))
	}
	for i, kind := range kinds {
		if args[i].Kind != kind {
			return fmt.Errorf("arg %d expected %s, got %s", i+1, kind, args[i].Kind)
		}
	}
	return nil
}
