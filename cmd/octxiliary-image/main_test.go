package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

func TestImageLoadInspectAndSaveWorkflow(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "rect.png")
	jpgPath := filepath.Join(dir, "rect.jpg")
	writeTestPNG(t, pngPath)
	writeTestJPEG(t, jpgPath)

	table := newImageTable()
	pngHandle := loadImageForTest(t, table, pngPath, "png")
	assertIntResult(t, table, "ImageWidth", pngHandle, 3)
	assertIntResult(t, table, "ImageHeight", pngHandle, 2)
	assertStringResult(t, table, "ImageFormat", pngHandle, "png")

	savedPNG := filepath.Join(dir, "saved.png")
	if value, err := table.dispatch(octxiliary.Request{Family: imageFamily, Function: "ImageSave", HasArgs: true, Args: []octxiliary.Value{pngHandle, {Kind: octxiliary.ValueString, String: savedPNG}}}); err != nil {
		t.Fatalf("save png: %v", err)
	} else if value.Kind != octxiliary.ValueInt || value.Int != 0 {
		t.Fatalf("unexpected save png result: %#v", value)
	}
	assertNonEmptyFile(t, savedPNG)

	jpgHandle := loadImageForTest(t, table, jpgPath, "jpeg")
	assertIntResult(t, table, "ImageWidth", jpgHandle, 3)
	assertIntResult(t, table, "ImageHeight", jpgHandle, 2)
	assertStringResult(t, table, "ImageFormat", jpgHandle, "jpeg")

	savedJPG := filepath.Join(dir, "saved.jpg")
	if _, err := table.dispatch(octxiliary.Request{Family: imageFamily, Function: "ImageSave", HasArgs: true, Args: []octxiliary.Value{jpgHandle, {Kind: octxiliary.ValueString, String: savedJPG}}}); err != nil {
		t.Fatalf("save jpeg: %v", err)
	}
	assertNonEmptyFile(t, savedJPG)
}

func TestImageEncodePngReturnsDecodableBytes(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "rect.png")
	jpgPath := filepath.Join(dir, "rect.jpg")
	writeTestPNG(t, pngPath)
	writeTestJPEG(t, jpgPath)

	table := newImageTable()
	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "png", path: pngPath},
		{name: "jpeg", path: jpgPath},
	} {
		handle := loadImageForTest(t, table, tc.path, tc.name)
		value, err := table.dispatch(octxiliary.Request{Family: imageFamily, Function: "ImageEncodePng", HasArgs: true, Args: []octxiliary.Value{handle}})
		if err != nil {
			t.Fatalf("encode %s: %v", tc.name, err)
		}
		if value.Kind != octxiliary.ValueBytes || len(value.Bytes) == 0 {
			t.Fatalf("encode %s got %#v, want non-empty Bytes", tc.name, value)
		}
		decoded, err := png.Decode(bytes.NewReader(value.Bytes))
		if err != nil {
			t.Fatalf("encoded %s bytes should decode as png: %v", tc.name, err)
		}
		if decoded.Bounds().Dx() != 3 || decoded.Bounds().Dy() != 2 {
			t.Fatalf("encoded %s bounds = %v, want 3x2", tc.name, decoded.Bounds())
		}
	}
}

func TestImageLoadMissingFileErrors(t *testing.T) {
	table := newImageTable()
	_, err := table.dispatch(octxiliary.Request{Family: imageFamily, Function: "ImageLoad", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: filepath.Join(t.TempDir(), "missing.png")}}})
	if err == nil {
		t.Fatalf("expected missing file error")
	}
}

func TestImageLoadCorruptImageErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.img")
	if err := os.WriteFile(path, []byte("not an image payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	table := newImageTable()
	_, err := table.dispatch(octxiliary.Request{Family: imageFamily, Function: "ImageLoad", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: path}}})
	if err == nil {
		t.Fatalf("expected corrupt image error")
	}
}

func TestImageSaveUnsupportedExtensionErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rect.png")
	writeTestPNG(t, path)
	table := newImageTable()
	handle := loadImageForTest(t, table, path, "png")
	_, err := table.dispatch(octxiliary.Request{Family: imageFamily, Function: "ImageSave", HasArgs: true, Args: []octxiliary.Value{handle, {Kind: octxiliary.ValueString, String: filepath.Join(t.TempDir(), "bad.bmp")}}})
	if err == nil || !strings.Contains(err.Error(), ".png, .jpg, or .jpeg") {
		t.Fatalf("expected extension error, got %v", err)
	}
}

func TestImageInvalidHandleErrors(t *testing.T) {
	table := newImageTable()
	badHandle := octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: imageFamily, HandleType: imageHandle, HandleID: 999}
	_, err := table.dispatch(octxiliary.Request{Family: imageFamily, Function: "ImageWidth", HasArgs: true, Args: []octxiliary.Value{badHandle}})
	if err == nil || !strings.Contains(err.Error(), "unknown image handle") {
		t.Fatalf("expected invalid handle error, got %v", err)
	}
	_, err = table.dispatch(octxiliary.Request{Family: imageFamily, Function: "ImageEncodePng", HasArgs: true, Args: []octxiliary.Value{badHandle}})
	if err == nil || !strings.Contains(err.Error(), "unknown image handle") {
		t.Fatalf("expected encode invalid handle error, got %v", err)
	}
}

func loadImageForTest(t *testing.T, table *imageTable, path string, format string) octxiliary.Value {
	t.Helper()
	value, err := table.dispatch(octxiliary.Request{Family: imageFamily, Function: "ImageLoad", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: path}}})
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	if value.Kind != octxiliary.ValueHandle || value.HandleFamily != imageFamily || value.HandleType != imageHandle || value.HandleID <= 0 {
		t.Fatalf("unexpected %s handle: %#v", format, value)
	}
	return value
}

func assertIntResult(t *testing.T, table *imageTable, function string, handle octxiliary.Value, want int) {
	t.Helper()
	value, err := table.dispatch(octxiliary.Request{Family: imageFamily, Function: function, HasArgs: true, Args: []octxiliary.Value{handle}})
	if err != nil {
		t.Fatalf("%s: %v", function, err)
	}
	if value.Kind != octxiliary.ValueInt || value.Int != want {
		t.Fatalf("%s got %#v, want Int %d", function, value, want)
	}
}

func assertStringResult(t *testing.T, table *imageTable, function string, handle octxiliary.Value, want string) {
	t.Helper()
	value, err := table.dispatch(octxiliary.Request{Family: imageFamily, Function: function, HasArgs: true, Args: []octxiliary.Value{handle}})
	if err != nil {
		t.Fatalf("%s: %v", function, err)
	}
	if value.Kind != octxiliary.ValueString || value.String != want {
		t.Fatalf("%s got %#v, want String %q", function, value, want)
	}
}

func assertNonEmptyFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("%s is empty", path)
	}
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, testRect()); err != nil {
		t.Fatal(err)
	}
}

func writeTestJPEG(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := jpeg.Encode(file, testRect(), &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
}

func testRect() image.Image {
	rect := image.NewRGBA(image.Rect(0, 0, 3, 2))
	colors := []color.RGBA{
		{R: 255, A: 255},
		{G: 255, A: 255},
		{B: 255, A: 255},
		{R: 255, G: 255, A: 255},
		{R: 255, B: 255, A: 255},
		{G: 255, B: 255, A: 255},
	}
	index := 0
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			rect.SetRGBA(x, y, colors[index])
			index++
		}
	}
	return rect
}
