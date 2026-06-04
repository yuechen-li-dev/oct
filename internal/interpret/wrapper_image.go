package interpret

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/dimension"
)

var imagePixelDimension = mustImageDimension("px")

type wrapperImage struct {
	image  image.Image
	format string
}

func imageWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"ImageLoad":      (*interpreter).evalImageLoadBuiltin,
		"ImageSave":      (*interpreter).evalImageSaveBuiltin,
		"ImageEncodePng": (*interpreter).evalImageEncodePngBuiltin,
		"ImageWidth":     (*interpreter).evalImageWidthBuiltin,
		"ImageHeight":    (*interpreter).evalImageHeightBuiltin,
		"ImageFormat":    (*interpreter).evalImageFormatBuiltin,
	}
}

func (i *interpreter) evalImageLoadBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	path, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	handle, loadErr := i.loadImage(path)
	if loadErr != nil {
		return wrapperErrorResult(callee, loadErr), nil
	}
	return wrapperIntResult(handle), nil
}

func (i *interpreter) evalImageSaveBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	handle, errResult, err := call.intArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	path, errResult, err := call.stringArg(1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	if saveErr := i.saveImage(handle, path); saveErr != nil {
		return wrapperErrorResult(callee, saveErr), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalImageEncodePngBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	handle, errResult, err := call.intArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	wrapped, getErr := i.images.get(handle)
	if getErr != nil {
		return wrapperErrorResult(callee, getErr), nil
	}
	var encoded bytes.Buffer
	if encodeErr := png.Encode(&encoded, wrapped.image); encodeErr != nil {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorBackendFailure, "encode png failed: %v", encodeErr)), nil
	}
	return wrapperBytesResult(encoded.Bytes()), nil
}

func (i *interpreter) evalImageWidthBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	handle, errResult, err := call.intArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	wrapped, getErr := i.images.get(handle)
	if getErr != nil {
		return wrapperErrorResult(callee, getErr), nil
	}
	return wrapperIntDimensionResult(int64(wrapped.image.Bounds().Dx()), imagePixelDimension), nil
}

func (i *interpreter) evalImageHeightBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	handle, errResult, err := call.intArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	wrapped, getErr := i.images.get(handle)
	if getErr != nil {
		return wrapperErrorResult(callee, getErr), nil
	}
	return wrapperIntDimensionResult(int64(wrapped.image.Bounds().Dy()), imagePixelDimension), nil
}

func (i *interpreter) evalImageFormatBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	handle, errResult, err := call.intArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	wrapped, getErr := i.images.get(handle)
	if getErr != nil {
		return wrapperErrorResult(callee, getErr), nil
	}
	return wrapperStringResult(wrapped.format), nil
}

func (i *interpreter) loadImage(path string) (int64, error) {
	file, openErr := os.Open(path)
	if openErr != nil {
		return 0, mapPathError(path, openErr)
	}
	defer file.Close()

	decoded, format, decodeErr := image.Decode(file)
	if decodeErr != nil {
		return 0, wrapperErrorf(wrapperErrorInvalidData, "%s: %v", path, decodeErr)
	}
	handle := i.images.allocate(&wrapperImage{image: decoded, format: format})
	return handle, nil
}

func (i *interpreter) saveImage(handle int64, path string) error {
	wrapped, getErr := i.images.get(handle)
	if getErr != nil {
		return getErr
	}
	file, createErr := os.Create(path)
	if createErr != nil {
		return mapPathError(path, createErr)
	}
	defer file.Close()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		if encodeErr := png.Encode(file, wrapped.image); encodeErr != nil {
			return wrapperErrorf(wrapperErrorBackendFailure, "%s: %v", path, encodeErr)
		}
	case ".jpg", ".jpeg":
		if encodeErr := jpeg.Encode(file, wrapped.image, &jpeg.Options{Quality: 95}); encodeErr != nil {
			return wrapperErrorf(wrapperErrorBackendFailure, "%s: %v", path, encodeErr)
		}
	default:
		return wrapperErrorf(wrapperErrorInvalidArgument, "path %q must end with .png, .jpg, or .jpeg", path)
	}
	return nil
}

func mustImageDimension(name string) dimension.Dimension {
	dim, ok := dimension.FromBaseName(name)
	if !ok {
		panic("unknown image dimension " + name)
	}
	return dim
}
