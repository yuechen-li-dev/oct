// Command octxiliary-compression serves the Compression standard-library Octxiliary wrapper.
package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"

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
	if req.Family != "Compression" {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "GzipCompressBytes":
		if err := expect(req.Args, octxiliary.ValueBytes); err != nil {
			return octxiliary.Value{}, err
		}
		compressed, err := gzipCompressBytes(req.Args[0].Bytes)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return bytesValue(compressed), nil
	case "GzipDecompressBytes":
		if err := expect(req.Args, octxiliary.ValueBytes); err != nil {
			return octxiliary.Value{}, err
		}
		decompressed, err := gzipDecompressBytes(req.Args[0].Bytes)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return bytesValue(decompressed), nil
	case "GzipCompressFile":
		if err := expect(req.Args, octxiliary.ValueString, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		if err := gzipCompressFile(req.Args[0].String, req.Args[1].String); err != nil {
			return octxiliary.Value{}, err
		}
		return intValue(0), nil
	case "GzipDecompressFile":
		if err := expect(req.Args, octxiliary.ValueString, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		if err := gzipDecompressFile(req.Args[0].String, req.Args[1].String); err != nil {
			return octxiliary.Value{}, err
		}
		return intValue(0), nil
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func gzipCompressBytes(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func gzipDecompressBytes(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return decompressed, nil
}

func gzipCompressFile(inputPath string, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	compressed, err := gzipCompressBytes(data)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, compressed, 0o644)
}

func gzipDecompressFile(inputPath string, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	decompressed, err := gzipDecompressBytes(data)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, decompressed, 0o644)
}

func bytesValue(value []byte) octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueBytes, Bytes: value}
}

func intValue(value int) octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: value}
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
