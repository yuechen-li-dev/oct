package interpret

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

func compressionWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"GzipCompressBytes":   (*interpreter).evalGzipCompressBytesBuiltin,
		"GzipDecompressBytes": (*interpreter).evalGzipDecompressBytesBuiltin,
		"GzipCompressFile":    (*interpreter).evalGzipCompressFileBuiltin,
		"GzipDecompressFile":  (*interpreter).evalGzipDecompressFileBuiltin,
	}
}

func (i *interpreter) evalGzipCompressBytesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	data, errResult, err := call.bytesArg(0)
	if err != nil {
		return wrapperErrorResult(callee, err), nil
	}
	if errResult != nil {
		return *errResult, nil
	}
	compressed, compressErr := gzipCompressBytes(data)
	if compressErr != nil {
		return wrapperErrorResult(callee, compressErr), nil
	}
	return wrapperBytesResult(compressed), nil
}

func (i *interpreter) evalGzipDecompressBytesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	data, errResult, err := call.bytesArg(0)
	if err != nil {
		return wrapperErrorResult(callee, err), nil
	}
	if errResult != nil {
		return *errResult, nil
	}
	decompressed, decompressErr := gzipDecompressBytes(data)
	if decompressErr != nil {
		return wrapperErrorResult(callee, decompressErr), nil
	}
	return wrapperBytesResult(decompressed), nil
}

func (i *interpreter) evalGzipCompressFileBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	inputPath, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	outputPath, errResult, err := call.stringArg(1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	if compressErr := gzipCompressFile(inputPath, outputPath); compressErr != nil {
		return wrapperErrorResult(callee, compressErr), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalGzipDecompressFileBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	inputPath, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	outputPath, errResult, err := call.stringArg(1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	if decompressErr := gzipDecompressFile(inputPath, outputPath); decompressErr != nil {
		return wrapperErrorResult(callee, decompressErr), nil
	}
	return wrapperIntResult(0), nil
}

func gzipCompressBytes(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(data); err != nil {
		return nil, wrapperErrorf(wrapperErrorBackendFailure, "%v", err)
	}
	if err := writer.Close(); err != nil {
		return nil, wrapperErrorf(wrapperErrorBackendFailure, "%v", err)
	}
	return buffer.Bytes(), nil
}

func gzipDecompressBytes(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, wrapperErrorf(wrapperErrorInvalidData, "%v", err)
	}
	defer reader.Close()
	decompressed, readErr := io.ReadAll(reader)
	if readErr != nil {
		return nil, wrapperErrorf(wrapperErrorInvalidData, "%v", readErr)
	}
	return decompressed, nil
}

func gzipCompressFile(inputPath string, outputPath string) error {
	data, readErr := os.ReadFile(inputPath)
	if readErr != nil {
		return mapPathError(inputPath, readErr)
	}
	compressed, compressErr := gzipCompressBytes(data)
	if compressErr != nil {
		return compressErr
	}
	if writeErr := os.WriteFile(outputPath, compressed, 0o644); writeErr != nil {
		return mapPathError(outputPath, writeErr)
	}
	return nil
}

func gzipDecompressFile(inputPath string, outputPath string) error {
	data, readErr := os.ReadFile(inputPath)
	if readErr != nil {
		return mapPathError(inputPath, readErr)
	}
	decompressed, decompressErr := gzipDecompressBytes(data)
	if decompressErr != nil {
		return decompressErr
	}
	if writeErr := os.WriteFile(outputPath, decompressed, 0o644); writeErr != nil {
		return mapPathError(outputPath, writeErr)
	}
	return nil
}
