package main

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"codeberg.org/go-pdf/fpdf"
	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

const (
	pdfFamily          = "Pdf"
	pdfPageHandleType  = "Pdf.PdfPage"
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
	imageCounter        int
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
	if value.HandleID <= 0 {
		return nil, fmt.Errorf("page handle ID must be positive")
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
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueInt, octxiliary.ValueInt, octxiliary.ValueString, octxiliary.ValueInt, octxiliary.ValueInt, octxiliary.ValueInt, octxiliary.ValueInt); err != nil {
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
		style, err := newTextStyle(req.Args[4].Int, req.Args[5].Int, req.Args[6].Int, req.Args[7].Int)
		if err != nil {
			return octxiliary.Value{}, err
		}
		if err := page.drawText(req.Args[3].String, req.Args[1].Int, req.Args[2].Int, style.size, [3]int{style.colorR, style.colorG, style.colorB}); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
	case "PdfDrawImageBytes":
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueBytes, octxiliary.ValueString, octxiliary.ValueInt, octxiliary.ValueInt); err != nil {
			return octxiliary.Value{}, err
		}
		page, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		if err := validateCoordinate(req.Args[3].Int, "x"); err != nil {
			return octxiliary.Value{}, err
		}
		if err := validateCoordinate(req.Args[4].Int, "y"); err != nil {
			return octxiliary.Value{}, err
		}
		width, height, err := imageBytesDimensions(req.Args[1].Bytes, req.Args[2].String)
		if err != nil {
			return octxiliary.Value{}, err
		}
		if err := page.drawImageBytes(req.Args[1].Bytes, req.Args[2].String, req.Args[3].Int, req.Args[4].Int, width, height); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
	case "PdfDrawImageBytesSized":
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueBytes, octxiliary.ValueString, octxiliary.ValueInt, octxiliary.ValueInt, octxiliary.ValueInt, octxiliary.ValueInt); err != nil {
			return octxiliary.Value{}, err
		}
		page, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		if err := validateCoordinate(req.Args[3].Int, "x"); err != nil {
			return octxiliary.Value{}, err
		}
		if err := validateCoordinate(req.Args[4].Int, "y"); err != nil {
			return octxiliary.Value{}, err
		}
		if err := validatePositiveSize(req.Args[5].Int, "width"); err != nil {
			return octxiliary.Value{}, err
		}
		if err := validatePositiveSize(req.Args[6].Int, "height"); err != nil {
			return octxiliary.Value{}, err
		}
		if _, _, err := imageBytesDimensions(req.Args[1].Bytes, req.Args[2].String); err != nil {
			return octxiliary.Value{}, err
		}
		if err := page.drawImageBytes(req.Args[1].Bytes, req.Args[2].String, req.Args[3].Int, req.Args[4].Int, req.Args[5].Int, req.Args[6].Int); err != nil {
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

func (p *pdfPage) drawImageBytes(data []byte, format string, xPx int, yPx int, widthPx int, heightPx int) error {
	imageType, err := pdfImageType(format)
	if err != nil {
		return err
	}
	p.imageCounter++
	alias := fmt.Sprintf("img_%d", p.imageCounter)
	options := fpdf.ImageOptions{ImageType: imageType, ReadDpi: false}
	if info := p.doc.RegisterImageOptionsReader(alias, options, bytes.NewReader(data)); info == nil {
		if pdfErr := p.doc.Error(); pdfErr != nil {
			return fmt.Errorf("register image bytes failed: %v", pdfErr)
		}
		return fmt.Errorf("register image bytes failed")
	}
	p.doc.ImageOptions(alias, pxToPt(xPx), pxToPt(yPx), pxToPt(widthPx), pxToPt(heightPx), false, options, 0, "")
	if pdfErr := p.doc.Error(); pdfErr != nil {
		return fmt.Errorf("draw image bytes failed: %v", pdfErr)
	}
	return nil
}

func imageBytesDimensions(data []byte, format string) (int, int, error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("image bytes must be non-empty")
	}
	imageType, err := pdfImageType(format)
	if err != nil {
		return 0, 0, err
	}
	switch imageType {
	case "PNG":
		decoded, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return 0, 0, fmt.Errorf("image bytes are not valid png: %v", err)
		}
		bounds := decoded.Bounds()
		return bounds.Dx(), bounds.Dy(), nil
	default:
		return 0, 0, fmt.Errorf("unsupported image format %q", format)
	}
}

func pdfImageType(format string) (string, error) {
	switch strings.ToLower(format) {
	case "png":
		return "PNG", nil
	default:
		return "", fmt.Errorf("unsupported image format %q; only png is supported", format)
	}
}

func (p *pdfPage) save(path string) error {
	if err := p.doc.OutputFileAndClose(path); err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	return nil
}

// newTextStyle checks the four style arguments of PdfDrawTextStyled, which
// arrive as the builtin passes them: size, then red, green and blue.
func newTextStyle(size int, colorR int, colorG int, colorB int) (textStyle, error) {
	if size <= 0 {
		return textStyle{}, fmt.Errorf("text style size must be positive")
	}
	for _, channel := range []struct {
		name  string
		value int
	}{{"ColorR", colorR}, {"ColorG", colorG}, {"ColorB", colorB}} {
		if channel.value < 0 || channel.value > 255 {
			return textStyle{}, fmt.Errorf("text style color channel %s must be in [0, 255]", channel.name)
		}
	}
	return textStyle{size: size, colorR: colorR, colorG: colorG, colorB: colorB}, nil
}

func validateCoordinate(value int, name string) error {
	if value < 0 {
		return fmt.Errorf("%s coordinate must be non-negative", name)
	}
	return nil
}

func validatePositiveSize(value int, name string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be positive", name)
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
