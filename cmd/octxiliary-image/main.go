// Command octxiliary-image serves the Image standard-library Octxiliary wrapper.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

const (
	imageFamily = "Image"
	imageHandle = "Image.ImageHandle"
)

type storedImage struct {
	image  image.Image
	format string
}

type imageTable struct {
	next   int
	images map[int]*storedImage
}

func newImageTable() *imageTable {
	return &imageTable{next: 1, images: map[int]*storedImage{}}
}

func (t *imageTable) allocate(image *storedImage) int {
	id := t.next
	t.next++
	t.images[id] = image
	return id
}

func (t *imageTable) get(value octxiliary.Value) (*storedImage, error) {
	if value.Kind != octxiliary.ValueHandle {
		return nil, fmt.Errorf("expected image handle, got %s", value.Kind)
	}
	if value.HandleFamily != imageFamily || value.HandleType != imageHandle {
		return nil, fmt.Errorf("expected %s %s handle", imageFamily, imageHandle)
	}
	if value.HandleID <= 0 {
		return nil, fmt.Errorf("image handle ID must be positive")
	}
	image, ok := t.images[value.HandleID]
	if !ok {
		return nil, fmt.Errorf("unknown image handle %d", value.HandleID)
	}
	return image, nil
}

func main() {
	if err := octxiliary.ReadHandshake(os.Stdin); err != nil {
		return
	}
	if err := octxiliary.WriteHandshake(os.Stdout); err != nil {
		return
	}
	table := newImageTable()
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

func (t *imageTable) dispatch(req octxiliary.Request) (octxiliary.Value, error) {
	if req.Family != imageFamily {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "ImageLoad":
		if err := expect(req.Args, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		return t.load(req.Args[0].String)
	case "ImageSave":
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		image, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		return save(image, req.Args[1].String)
	case "ImageEncodePng":
		if err := expect(req.Args, octxiliary.ValueHandle); err != nil {
			return octxiliary.Value{}, err
		}
		image, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		return encodePng(image)
	case "ImageWidth":
		if err := expect(req.Args, octxiliary.ValueHandle); err != nil {
			return octxiliary.Value{}, err
		}
		image, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: image.image.Bounds().Dx()}, nil
	case "ImageHeight":
		if err := expect(req.Args, octxiliary.ValueHandle); err != nil {
			return octxiliary.Value{}, err
		}
		image, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: image.image.Bounds().Dy()}, nil
	case "ImageFormat":
		if err := expect(req.Args, octxiliary.ValueHandle); err != nil {
			return octxiliary.Value{}, err
		}
		image, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueString, String: image.format}, nil
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func (t *imageTable) load(path string) (octxiliary.Value, error) {
	file, err := os.Open(path)
	if err != nil {
		return octxiliary.Value{}, fmt.Errorf("%s: %v", path, err)
	}
	defer file.Close()

	decoded, format, err := image.Decode(file)
	if err != nil {
		return octxiliary.Value{}, fmt.Errorf("%s: %v", path, err)
	}
	id := t.allocate(&storedImage{image: decoded, format: format})
	return octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: imageFamily, HandleType: imageHandle, HandleID: id}, nil
}

func encodePng(image *storedImage) (octxiliary.Value, error) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.image); err != nil {
		return octxiliary.Value{}, fmt.Errorf("encode png failed: %v", err)
	}
	return octxiliary.Value{Kind: octxiliary.ValueBytes, Bytes: encoded.Bytes()}, nil
}

func save(image *storedImage, path string) (octxiliary.Value, error) {
	file, err := os.Create(path)
	if err != nil {
		return octxiliary.Value{}, fmt.Errorf("%s: %v", path, err)
	}
	defer file.Close()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		if err := png.Encode(file, image.image); err != nil {
			return octxiliary.Value{}, fmt.Errorf("%s: %v", path, err)
		}
	case ".jpg", ".jpeg":
		if err := jpeg.Encode(file, image.image, &jpeg.Options{Quality: 95}); err != nil {
			return octxiliary.Value{}, fmt.Errorf("%s: %v", path, err)
		}
	default:
		return octxiliary.Value{}, fmt.Errorf("path %q must end with .png, .jpg, or .jpeg", path)
	}
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
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
