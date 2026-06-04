package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

func TestPdfTextPageSaveWorkflow(t *testing.T) {
	table := newPageTable()
	page := createPageForTest(t, table)

	assertIntResult(t, table, "PdfDrawText", []octxiliary.Value{page, {Kind: octxiliary.ValueInt, Int: 12}, {Kind: octxiliary.ValueInt, Int: 16}, {Kind: octxiliary.ValueString, String: "hello pdf"}})
	assertIntResult(t, table, "PdfDrawTextStyled", []octxiliary.Value{page, {Kind: octxiliary.ValueInt, Int: 12}, {Kind: octxiliary.ValueInt, Int: 36}, {Kind: octxiliary.ValueString, String: "styled"}, validTextStyle()})

	out := filepath.Join(t.TempDir(), "out.pdf")
	assertIntResult(t, table, "PdfSave", []octxiliary.Value{page, {Kind: octxiliary.ValueString, String: out}})
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("saved pdf missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("saved pdf is empty")
	}
}

func TestPdfDrawImageBytesAndSaveWorkflow(t *testing.T) {
	table := newPageTable()
	page := createPageForTest(t, table)
	pngBytes := testPNGBytes(t)

	assertIntResult(t, table, "PdfDrawImageBytes", []octxiliary.Value{page, {Kind: octxiliary.ValueBytes, Bytes: pngBytes}, {Kind: octxiliary.ValueString, String: "png"}, {Kind: octxiliary.ValueInt, Int: 10}, {Kind: octxiliary.ValueInt, Int: 12}})
	assertIntResult(t, table, "PdfDrawImageBytesSized", []octxiliary.Value{page, {Kind: octxiliary.ValueBytes, Bytes: pngBytes}, {Kind: octxiliary.ValueString, String: "PNG"}, {Kind: octxiliary.ValueInt, Int: 40}, {Kind: octxiliary.ValueInt, Int: 30}, {Kind: octxiliary.ValueInt, Int: 60}, {Kind: octxiliary.ValueInt, Int: 40}})

	out := filepath.Join(t.TempDir(), "image-bytes.pdf")
	assertIntResult(t, table, "PdfSave", []octxiliary.Value{page, {Kind: octxiliary.ValueString, String: out}})
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("saved pdf missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("saved pdf is empty")
	}
}

func TestPdfDrawImageBytesValidationErrors(t *testing.T) {
	table := newPageTable()
	page := createPageForTest(t, table)
	pngBytes := testPNGBytes(t)

	for _, tc := range []struct {
		name     string
		function string
		args     []octxiliary.Value
		want     string
	}{
		{name: "invalid bytes", function: "PdfDrawImageBytes", args: []octxiliary.Value{page, {Kind: octxiliary.ValueBytes, Bytes: []byte("not png")}, {Kind: octxiliary.ValueString, String: "png"}, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueInt, Int: 1}}, want: "not valid png"},
		{name: "unsupported format", function: "PdfDrawImageBytes", args: []octxiliary.Value{page, {Kind: octxiliary.ValueBytes, Bytes: pngBytes}, {Kind: octxiliary.ValueString, String: "jpeg"}, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueInt, Int: 1}}, want: "unsupported image format"},
		{name: "invalid coordinate", function: "PdfDrawImageBytes", args: []octxiliary.Value{page, {Kind: octxiliary.ValueBytes, Bytes: pngBytes}, {Kind: octxiliary.ValueString, String: "png"}, {Kind: octxiliary.ValueInt, Int: -1}, {Kind: octxiliary.ValueInt, Int: 1}}, want: "coordinate must be non-negative"},
		{name: "invalid size", function: "PdfDrawImageBytesSized", args: []octxiliary.Value{page, {Kind: octxiliary.ValueBytes, Bytes: pngBytes}, {Kind: octxiliary.ValueString, String: "png"}, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueInt, Int: 0}, {Kind: octxiliary.ValueInt, Int: 10}}, want: "width must be positive"},
	} {
		_, err := table.dispatch(octxiliary.Request{Family: pdfFamily, Function: tc.function, HasArgs: true, Args: tc.args})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s expected %q error, got %v", tc.name, tc.want, err)
		}
	}

	badPage := octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: pdfFamily, HandleType: pdfPageHandleType, HandleID: 999}
	_, err := table.dispatch(octxiliary.Request{Family: pdfFamily, Function: "PdfDrawImageBytes", HasArgs: true, Args: []octxiliary.Value{badPage, {Kind: octxiliary.ValueBytes, Bytes: pngBytes}, {Kind: octxiliary.ValueString, String: "png"}, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueInt, Int: 1}}})
	if err == nil || !strings.Contains(err.Error(), "unknown page handle") {
		t.Fatalf("expected invalid page handle error, got %v", err)
	}
}

func TestPdfInvalidPageHandleErrors(t *testing.T) {
	table := newPageTable()
	bad := octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: pdfFamily, HandleType: pdfPageHandleType, HandleID: 999}
	_, err := table.dispatch(octxiliary.Request{Family: pdfFamily, Function: "PdfDrawText", HasArgs: true, Args: []octxiliary.Value{bad, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueString, String: "x"}}})
	if err == nil || !strings.Contains(err.Error(), "unknown page handle") {
		t.Fatalf("expected invalid page handle error, got %v", err)
	}
}

func TestPdfInvalidHandleFamilyTypeErrors(t *testing.T) {
	table := newPageTable()
	bad := octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: "Image", HandleType: "Image.ImageHandle", HandleID: 1}
	_, err := table.dispatch(octxiliary.Request{Family: pdfFamily, Function: "PdfSave", HasArgs: true, Args: []octxiliary.Value{bad, {Kind: octxiliary.ValueString, String: filepath.Join(t.TempDir(), "out.pdf")}}})
	if err == nil || !strings.Contains(err.Error(), "expected Pdf Pdf.PdfPage handle") {
		t.Fatalf("expected invalid handle family/type error, got %v", err)
	}
}

func TestPdfInvalidStyleColorErrors(t *testing.T) {
	table := newPageTable()
	page := createPageForTest(t, table)
	badStyle := validTextStyle()
	badStyle.Fields[1].Value.Int = 300
	_, err := table.dispatch(octxiliary.Request{Family: pdfFamily, Function: "PdfDrawTextStyled", HasArgs: true, Args: []octxiliary.Value{page, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueString, String: "x"}, badStyle}})
	if err == nil || !strings.Contains(err.Error(), "must be in [0, 255]") {
		t.Fatalf("expected invalid color error, got %v", err)
	}
}

func TestPdfInvalidStyleSizeErrors(t *testing.T) {
	table := newPageTable()
	page := createPageForTest(t, table)
	badStyle := validTextStyle()
	badStyle.Fields[0].Value.Int = 0
	_, err := table.dispatch(octxiliary.Request{Family: pdfFamily, Function: "PdfDrawTextStyled", HasArgs: true, Args: []octxiliary.Value{page, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueInt, Int: 1}, {Kind: octxiliary.ValueString, String: "x"}, badStyle}})
	if err == nil || !strings.Contains(err.Error(), "size must be positive") {
		t.Fatalf("expected invalid size error, got %v", err)
	}
}

func TestPdfInvalidPageSizeErrors(t *testing.T) {
	table := newPageTable()
	_, err := table.dispatch(octxiliary.Request{Family: pdfFamily, Function: "PdfNewPage", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueInt, Int: 0}, {Kind: octxiliary.ValueInt, Int: 80}}})
	if err == nil || !strings.Contains(err.Error(), "width must be positive") {
		t.Fatalf("expected invalid width error, got %v", err)
	}
	_, err = table.dispatch(octxiliary.Request{Family: pdfFamily, Function: "PdfNewPage", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueInt, Int: 120}, {Kind: octxiliary.ValueInt, Int: -1}}})
	if err == nil || !strings.Contains(err.Error(), "height must be positive") {
		t.Fatalf("expected invalid height error, got %v", err)
	}
}

func TestPdfInvalidSavePathErrors(t *testing.T) {
	table := newPageTable()
	page := createPageForTest(t, table)
	_, err := table.dispatch(octxiliary.Request{Family: pdfFamily, Function: "PdfSave", HasArgs: true, Args: []octxiliary.Value{page, {Kind: octxiliary.ValueString, String: filepath.Join(t.TempDir(), "missing", "out.pdf")}}})
	if err == nil {
		t.Fatalf("expected invalid save path error")
	}
}

func createPageForTest(t *testing.T, table *pageTable) octxiliary.Value {
	t.Helper()
	value, err := table.dispatch(octxiliary.Request{Family: pdfFamily, Function: "PdfNewPage", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueInt, Int: 320}, {Kind: octxiliary.ValueInt, Int: 180}}})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	if value.Kind != octxiliary.ValueHandle || value.HandleFamily != pdfFamily || value.HandleType != pdfPageHandleType || value.HandleID <= 0 {
		t.Fatalf("unexpected page handle: %#v", value)
	}
	return value
}

func assertIntResult(t *testing.T, table *pageTable, function string, args []octxiliary.Value) {
	t.Helper()
	value, err := table.dispatch(octxiliary.Request{Family: pdfFamily, Function: function, HasArgs: true, Args: args})
	if err != nil {
		t.Fatalf("%s: %v", function, err)
	}
	if value.Kind != octxiliary.ValueInt || value.Int != 0 {
		t.Fatalf("%s got %#v, want Int 0", function, value)
	}
}

func validTextStyle() octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueRecord, RecordType: pdfTextStyleRecord, Fields: []octxiliary.FieldValue{
		{Name: "Size", Value: octxiliary.Value{Kind: octxiliary.ValueInt, Int: 18}},
		{Name: "ColorR", Value: octxiliary.Value{Kind: octxiliary.ValueInt, Int: 24}},
		{Name: "ColorG", Value: octxiliary.Value{Kind: octxiliary.ValueInt, Int: 100}},
		{Name: "ColorB", Value: octxiliary.Value{Kind: octxiliary.ValueInt, Int: 220}},
	}}
}

func testPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(50 * x), G: uint8(60 * y), B: 180, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
