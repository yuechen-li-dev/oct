package interpret

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

func archiveWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"ZipListEntries":     (*interpreter).evalZipListEntriesBuiltin,
		"ZipExtractAll":      (*interpreter).evalZipExtractAllBuiltin,
		"ZipCreateFromFiles": (*interpreter).evalZipCreateFromFilesBuiltin,
	}
}

func (i *interpreter) evalZipListEntriesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	r, openErr := zip.OpenReader(path)
	if openErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, openErr)), nil
	}
	defer r.Close()

	names := make([]string, 0, len(r.File))
	for _, file := range r.File {
		names = append(names, file.Name)
	}
	sort.Strings(names)
	return wrapperStringArrayResult(names), nil
}

func (i *interpreter) evalZipExtractAllBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	path, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	destination, errResult, err := call.stringArg(1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	if extractErr := extractZipAll(path, destination); extractErr != nil {
		return wrapperErrorResult(callee, extractErr), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalZipCreateFromFilesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	outputPath, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	paths, errResult, err := call.stringArrayArg(1)
	if err != nil {
		return wrapperErrorResult(callee, err), nil
	}
	if errResult != nil {
		return *errResult, nil
	}
	if createErr := createZipFromFiles(outputPath, paths); createErr != nil {
		return wrapperErrorResult(callee, createErr), nil
	}
	return wrapperIntResult(0), nil
}

func createZipFromFiles(outputPath string, paths []string) error {
	file, createErr := os.Create(outputPath)
	if createErr != nil {
		return mapPathError(outputPath, createErr)
	}
	defer file.Close()
	archive := zip.NewWriter(file)
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			archive.Close()
			return mapPathError(path, readErr)
		}
		header := &zip.FileHeader{Name: filepath.Base(path), Method: zip.Deflate}
		writer, openErr := archive.CreateHeader(header)
		if openErr != nil {
			archive.Close()
			return wrapperErrorf(wrapperErrorBackendFailure, "%s: %v", path, openErr)
		}
		if _, writeErr := writer.Write(data); writeErr != nil {
			archive.Close()
			return wrapperErrorf(wrapperErrorBackendFailure, "%s: %v", path, writeErr)
		}
	}
	if closeErr := archive.Close(); closeErr != nil {
		return wrapperErrorf(wrapperErrorBackendFailure, "%v", closeErr)
	}
	return nil
}

func extractZipAll(path string, destination string) error {
	r, openErr := zip.OpenReader(path)
	if openErr != nil {
		return mapPathError(path, openErr)
	}
	defer r.Close()
	cleanDestination := filepath.Clean(destination)
	if mkErr := os.MkdirAll(cleanDestination, 0o755); mkErr != nil {
		return mapPathError(cleanDestination, mkErr)
	}
	for _, file := range r.File {
		targetPath := filepath.Join(cleanDestination, file.Name)
		if !isZipPathWithinRoot(cleanDestination, targetPath) {
			return wrapperErrorf(wrapperErrorInvalidData, "zip entry escapes destination: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if mkErr := os.MkdirAll(targetPath, 0o755); mkErr != nil {
				return mapPathError(targetPath, mkErr)
			}
			continue
		}
		if mkErr := os.MkdirAll(filepath.Dir(targetPath), 0o755); mkErr != nil {
			return mapPathError(targetPath, mkErr)
		}
		src, openErr := file.Open()
		if openErr != nil {
			return wrapperErrorf(wrapperErrorBackendFailure, "%s: %v", file.Name, openErr)
		}
		dst, createErr := os.Create(targetPath)
		if createErr != nil {
			src.Close()
			return mapPathError(targetPath, createErr)
		}
		_, copyErr := io.Copy(dst, src)
		closeDstErr := dst.Close()
		closeSrcErr := src.Close()
		if copyErr != nil {
			return mapPathError(targetPath, copyErr)
		}
		if closeDstErr != nil {
			return mapPathError(targetPath, closeDstErr)
		}
		if closeSrcErr != nil {
			return wrapperErrorf(wrapperErrorBackendFailure, "%s: %v", file.Name, closeSrcErr)
		}
	}
	return nil
}

func isZipPathWithinRoot(root string, path string) bool {
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	if cleanPath == cleanRoot {
		return true
	}
	prefix := cleanRoot + string(os.PathSeparator)
	return strings.HasPrefix(cleanPath, prefix)
}
