// Command octxiliary-archive serves the Archive standard-library Octxiliary wrapper.
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

func main() {
	if err := octxiliary.ReadHandshake(os.Stdin); err != nil {
		return
	}
	if err := octxiliary.WriteHandshake(os.Stdout); err != nil {
		return
	}
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
			_ = octxiliary.WriteFrame(os.Stdout, octxiliary.EncodeResponse(resp))
			continue
		}
		value, err := dispatch(req)
		if err != nil {
			resp.OK = false
			resp.Error = err.Error()
		} else {
			resp.OK = true
			resp.Value = value
			resp.HasValue = true
		}
		if err := octxiliary.WriteFrame(os.Stdout, octxiliary.EncodeResponse(resp)); err != nil {
			return
		}
	}
}

func dispatch(req octxiliary.Request) (octxiliary.Value, error) {
	if req.Family != "Archive" {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "ZipListEntries":
		if err := expect(req.Args, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		names, err := listEntries(req.Args[0].String)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return stringArrayValue(names), nil
	case "ZipExtractAll":
		if err := expect(req.Args, octxiliary.ValueString, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		if err := extractAll(req.Args[0].String, req.Args[1].String); err != nil {
			return octxiliary.Value{}, err
		}
		return intValue(0), nil
	case "ZipCreateFromFiles":
		if err := expect(req.Args, octxiliary.ValueString, octxiliary.ValueStringArray); err != nil {
			return octxiliary.Value{}, err
		}
		if err := createFromFiles(req.Args[0].String, req.Args[1].Strings); err != nil {
			return octxiliary.Value{}, err
		}
		return intValue(0), nil
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func listEntries(path string) ([]string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	names := make([]string, 0, len(r.File))
	for _, file := range r.File {
		names = append(names, file.Name)
	}
	sort.Strings(names)
	return names, nil
}

func createFromFiles(outputPath string, paths []string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(file)
	for _, path := range paths {
		if err := addFile(archive, path); err != nil {
			_ = archive.Close()
			_ = file.Close()
			return err
		}
	}
	if err := archive.Close(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func addFile(archive *zip.Writer, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	writer, err := archive.CreateHeader(&zip.FileHeader{Name: filepath.Base(path), Method: zip.Deflate})
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func extractAll(path string, destination string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	root := filepath.Clean(destination)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for _, file := range r.File {
		target := filepath.Join(root, file.Name)
		if !withinRoot(root, target) {
			return fmt.Errorf("zip entry escapes destination: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractFile(file, target); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(file *zip.File, target string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func withinRoot(root string, path string) bool {
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	if cleanPath == cleanRoot {
		return true
	}
	return strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator))
}

func intValue(value int) octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: value}
}

func stringArrayValue(value []string) octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueStringArray, Strings: value}
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
