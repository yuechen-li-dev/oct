package interpret

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"codeberg.org/go-pdf/fpdf"
	"github.com/yuechen-li-dev/oct/internal/ast"
)

const (
	pdfPixelsPerInch   = 96.0
	pdfPointsPerInch   = 72.0
	pdfDefaultFontName = "Inter"
)

type wrapperPDFPage struct {
	doc                 *fpdf.Fpdf
	widthPx             int64
	heightPx            int64
	defaultFontFamily   string
	defaultFontSizePx   int64
	defaultTextColorRGB [3]int
	fontFallbackUsed    bool
	imageCounter        int64
}

func pdfWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"PdfNewPage":             (*interpreter).evalPdfNewPageBuiltin,
		"PdfDrawText":            (*interpreter).evalPdfDrawTextBuiltin,
		"PdfDrawTextStyled":      (*interpreter).evalPdfDrawTextStyledBuiltin,
		"PdfDrawImage":           (*interpreter).evalPdfDrawImageBuiltin,
		"PdfDrawImageSized":      (*interpreter).evalPdfDrawImageSizedBuiltin,
		"PdfDrawImageBytes":      (*interpreter).evalPdfDrawImageBytesBuiltin,
		"PdfDrawImageBytesSized": (*interpreter).evalPdfDrawImageBytesSizedBuiltin,
		"PdfSave":                (*interpreter).evalPdfSaveBuiltin,
	}
}

func (i *interpreter) evalPdfNewPageBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	width, errResult, err := pxArg(call, 0, true)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	height, errResult, err := pxArg(call, 1, true)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	page, newErr := newWrapperPDFPage(width, height)
	if newErr != nil {
		return wrapperErrorResult(callee, newErr), nil
	}
	handle := i.pdfPages.allocate(page)
	return wrapperIntResult(handle), nil
}

func (i *interpreter) evalPdfDrawTextBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(4); err != nil {
		return evalResult{}, err
	}
	page, text, x, y, errResult, err := i.evalPdfTextArgs(call)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	if drawErr := page.drawText(text, x, y, page.defaultFontSizePx, page.defaultTextColorRGB); drawErr != nil {
		return wrapperErrorResult(callee, drawErr), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalPdfDrawTextStyledBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(8); err != nil {
		return evalResult{}, err
	}
	page, text, x, y, errResult, err := i.evalPdfTextArgs(call)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	fontSize, errResult, err := pxArg(call, 4, true)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	r, errResult, err := colorChannelArg(call, 5)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	g, errResult, err := colorChannelArg(call, 6)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	b, errResult, err := colorChannelArg(call, 7)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	if drawErr := page.drawText(text, x, y, fontSize, [3]int{r, g, b}); drawErr != nil {
		return wrapperErrorResult(callee, drawErr), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalPdfDrawImageBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(4); err != nil {
		return evalResult{}, err
	}
	page, sourceImage, x, y, errResult, err := i.evalPdfImageArgs(call)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	width := int64(sourceImage.Bounds().Dx())
	height := int64(sourceImage.Bounds().Dy())
	if drawErr := page.drawImage(sourceImage, x, y, width, height); drawErr != nil {
		return wrapperErrorResult(callee, drawErr), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalPdfDrawImageSizedBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(6); err != nil {
		return evalResult{}, err
	}
	page, sourceImage, x, y, errResult, err := i.evalPdfImageArgs(call)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	width, errResult, err := pxArg(call, 4, true)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	height, errResult, err := pxArg(call, 5, true)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	if drawErr := page.drawImage(sourceImage, x, y, width, height); drawErr != nil {
		return wrapperErrorResult(callee, drawErr), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalPdfDrawImageBytesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(5); err != nil {
		return evalResult{}, err
	}
	page, data, format, x, y, errResult, err := i.evalPdfImageBytesArgs(call)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	decoded, decodeErr := decodePdfImageBytes(data, format)
	if decodeErr != nil {
		return wrapperErrorResult(callee, decodeErr), nil
	}
	width := int64(decoded.Bounds().Dx())
	height := int64(decoded.Bounds().Dy())
	if drawErr := page.drawImage(decoded, x, y, width, height); drawErr != nil {
		return wrapperErrorResult(callee, drawErr), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalPdfDrawImageBytesSizedBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(7); err != nil {
		return evalResult{}, err
	}
	page, data, format, x, y, errResult, err := i.evalPdfImageBytesArgs(call)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	width, errResult, err := pxArg(call, 5, true)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	height, errResult, err := pxArg(call, 6, true)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	decoded, decodeErr := decodePdfImageBytes(data, format)
	if decodeErr != nil {
		return wrapperErrorResult(callee, decodeErr), nil
	}
	if drawErr := page.drawImage(decoded, x, y, width, height); drawErr != nil {
		return wrapperErrorResult(callee, drawErr), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalPdfSaveBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	pageHandle, errResult, err := call.intArg(0)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	path, errResult, err := call.stringArg(1)
	if err != nil || errResult != nil {
		return unwrapWrapperArgResult(errResult, err)
	}
	page, getErr := i.pdfPages.get(pageHandle)
	if getErr != nil {
		return wrapperErrorResult(callee, getErr), nil
	}
	if saveErr := page.save(path); saveErr != nil {
		return wrapperErrorResult(callee, saveErr), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalPdfTextArgs(call wrapperCall) (*wrapperPDFPage, string, int64, int64, *evalResult, error) {
	pageHandle, errResult, err := call.intArg(0)
	if err != nil || errResult != nil {
		return nil, "", 0, 0, errResult, err
	}
	page, getErr := i.pdfPages.get(pageHandle)
	if getErr != nil {
		result := wrapperErrorResult(call.callee, getErr)
		return nil, "", 0, 0, &result, nil
	}
	x, errResult, err := pxArg(call, 1, false)
	if err != nil || errResult != nil {
		return nil, "", 0, 0, errResult, err
	}
	y, errResult, err := pxArg(call, 2, false)
	if err != nil || errResult != nil {
		return nil, "", 0, 0, errResult, err
	}
	text, errResult, err := call.stringArg(3)
	if err != nil || errResult != nil {
		return nil, "", 0, 0, errResult, err
	}
	return page, text, x, y, nil, nil
}

func (i *interpreter) evalPdfImageArgs(call wrapperCall) (*wrapperPDFPage, image.Image, int64, int64, *evalResult, error) {
	pageHandle, errResult, err := call.intArg(0)
	if err != nil || errResult != nil {
		return nil, nil, 0, 0, errResult, err
	}
	page, getErr := i.pdfPages.get(pageHandle)
	if getErr != nil {
		result := wrapperErrorResult(call.callee, getErr)
		return nil, nil, 0, 0, &result, nil
	}
	imageHandle, errResult, err := call.intArg(1)
	if err != nil || errResult != nil {
		return nil, nil, 0, 0, errResult, err
	}
	wrappedImage, imageErr := i.images.get(imageHandle)
	if imageErr != nil {
		result := wrapperErrorResult(call.callee, imageErr)
		return nil, nil, 0, 0, &result, nil
	}
	x, errResult, err := pxArg(call, 2, false)
	if err != nil || errResult != nil {
		return nil, nil, 0, 0, errResult, err
	}
	y, errResult, err := pxArg(call, 3, false)
	if err != nil || errResult != nil {
		return nil, nil, 0, 0, errResult, err
	}
	return page, wrappedImage.image, x, y, nil, nil
}

func (i *interpreter) evalPdfImageBytesArgs(call wrapperCall) (*wrapperPDFPage, []byte, string, int64, int64, *evalResult, error) {
	pageHandle, errResult, err := call.intArg(0)
	if err != nil || errResult != nil {
		return nil, nil, "", 0, 0, errResult, err
	}
	page, getErr := i.pdfPages.get(pageHandle)
	if getErr != nil {
		result := wrapperErrorResult(call.callee, getErr)
		return nil, nil, "", 0, 0, &result, nil
	}
	data, errResult, err := call.bytesArg(1)
	if err != nil || errResult != nil {
		return nil, nil, "", 0, 0, errResult, err
	}
	format, errResult, err := call.stringArg(2)
	if err != nil || errResult != nil {
		return nil, nil, "", 0, 0, errResult, err
	}
	x, errResult, err := pxArg(call, 3, false)
	if err != nil || errResult != nil {
		return nil, nil, "", 0, 0, errResult, err
	}
	y, errResult, err := pxArg(call, 4, false)
	if err != nil || errResult != nil {
		return nil, nil, "", 0, 0, errResult, err
	}
	return page, data, format, x, y, nil, nil
}

func decodePdfImageBytes(data []byte, format string) (image.Image, error) {
	if len(data) == 0 {
		return nil, wrapperErrorf(wrapperErrorInvalidArgument, "image bytes must be non-empty")
	}
	if strings.ToLower(format) != "png" {
		return nil, wrapperErrorf(wrapperErrorInvalidArgument, "unsupported image format %q; only png is supported", format)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, wrapperErrorf(wrapperErrorInvalidData, "image bytes are not valid png: %v", err)
	}
	return decoded, nil
}

func newWrapperPDFPage(widthPx int64, heightPx int64) (*wrapperPDFPage, error) {
	size := fpdf.SizeType{Wd: pxToPt(widthPx), Ht: pxToPt(heightPx)}
	doc := fpdf.NewCustom(&fpdf.InitType{UnitStr: "pt", Size: size})
	doc.SetAutoPageBreak(false, 0)
	doc.SetCompression(false)
	doc.AddPage()

	page := &wrapperPDFPage{
		doc:                 doc,
		widthPx:             widthPx,
		heightPx:            heightPx,
		defaultFontFamily:   pdfDefaultFontName,
		defaultFontSizePx:   16,
		defaultTextColorRGB: [3]int{0, 0, 0},
	}
	if fontErr := page.configureDefaultFont(); fontErr != nil {
		return nil, fontErr
	}
	return page, nil
}

func (p *wrapperPDFPage) configureDefaultFont() error {
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
		return wrapperErrorf(wrapperErrorBackendFailure, "pdf init failed: %v", pdfErr)
	}
	return nil
}

func loadInterRegularTTF() ([]byte, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("unable to resolve wrapper source path")
	}
	fontPath := filepath.Join(filepath.Dir(currentFile), "assets", "fonts", "Inter-Regular.ttf")
	bytes, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (p *wrapperPDFPage) drawText(text string, xPx int64, yPx int64, fontSizePx int64, colorRGB [3]int) error {
	p.doc.SetFont(p.defaultFontFamily, "", pxToPt(fontSizePx))
	p.doc.SetTextColor(colorRGB[0], colorRGB[1], colorRGB[2])
	lineHeightPt := pxToPt(fontSizePx)
	p.doc.SetXY(pxToPt(xPx), pxToPt(yPx))
	p.doc.CellFormat(0, lineHeightPt, text, "", 0, "", false, 0, "")
	if pdfErr := p.doc.Error(); pdfErr != nil {
		return wrapperErrorf(wrapperErrorBackendFailure, "draw text failed: %v", pdfErr)
	}
	return nil
}

func (p *wrapperPDFPage) drawImage(source image.Image, xPx int64, yPx int64, widthPx int64, heightPx int64) error {
	alias, registerErr := p.registerImage(source)
	if registerErr != nil {
		return registerErr
	}
	options := fpdf.ImageOptions{ImageType: "PNG"}
	p.doc.ImageOptions(alias, pxToPt(xPx), pxToPt(yPx), pxToPt(widthPx), pxToPt(heightPx), false, options, 0, "")
	if pdfErr := p.doc.Error(); pdfErr != nil {
		return wrapperErrorf(wrapperErrorBackendFailure, "draw image failed: %v", pdfErr)
	}
	return nil
}

func (p *wrapperPDFPage) registerImage(source image.Image) (string, error) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		return "", wrapperErrorf(wrapperErrorBackendFailure, "encode image for pdf failed: %v", err)
	}
	p.imageCounter++
	alias := fmt.Sprintf("img_%d", p.imageCounter)
	options := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
	if info := p.doc.RegisterImageOptionsReader(alias, options, bytes.NewReader(encoded.Bytes())); info == nil {
		if pdfErr := p.doc.Error(); pdfErr != nil {
			return "", wrapperErrorf(wrapperErrorBackendFailure, "register image failed: %v", pdfErr)
		}
		return "", wrapperErrorf(wrapperErrorBackendFailure, "register image failed")
	}
	return alias, nil
}

func (p *wrapperPDFPage) save(path string) error {
	if err := p.doc.OutputFileAndClose(path); err != nil {
		return mapPathError(path, err)
	}
	return nil
}

func pxArg(call wrapperCall, index int, requirePositive bool) (int64, *evalResult, error) {
	argument, errResult, err := call.evalArg(index)
	if err != nil || errResult != nil {
		return 0, errResult, err
	}
	if argument.Kind != ValueInt || argument.Dimension != imagePixelDimension {
		result := wrapperErrorResult(call.callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument %d expects Int<px>", index+1))
		return 0, &result, nil
	}
	if requirePositive && argument.Int <= 0 {
		result := wrapperErrorResult(call.callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument %d expects a positive Int<px>", index+1))
		return 0, &result, nil
	}
	if !requirePositive && argument.Int < 0 {
		result := wrapperErrorResult(call.callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument %d expects Int<px> >= 0", index+1))
		return 0, &result, nil
	}
	return argument.Int, nil, nil
}

func colorChannelArg(call wrapperCall, index int) (int, *evalResult, error) {
	channel, errResult, err := call.intArg(index)
	if err != nil || errResult != nil {
		return 0, errResult, err
	}
	if channel < 0 || channel > 255 {
		result := wrapperErrorResult(call.callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument %d expects color channel in [0,255]", index+1))
		return 0, &result, nil
	}
	return int(channel), nil, nil
}

func unwrapWrapperArgResult(errResult *evalResult, err error) (evalResult, error) {
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	return evalResult{}, nil
}

func pxToPt(px int64) float64 {
	return (float64(px) / pdfPixelsPerInch) * pdfPointsPerInch
}
