// Command octxiliary-csv serves the Csv standard-library Octxiliary wrapper.
package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"

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
	if req.Family != "Csv" {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "CsvRead", "CsvReadRows":
		if err := expect(req.Args, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		rows, err := readRows(req.Args[0].String)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueStringMatrix, Strings2: rows}, nil
	case "CsvWrite", "CsvWriteRows":
		if err := expect(req.Args, octxiliary.ValueString, octxiliary.ValueStringMatrix); err != nil {
			return octxiliary.Value{}, err
		}
		if err := writeRows(req.Args[0].String, req.Args[1].Strings2); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func readRows(path string) ([][]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	return reader.ReadAll()
}

func writeRows(path string, rows [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		if mkdirErr := ensureParentDir(path); mkdirErr == nil {
			file, err = os.Create(path)
		}
	}
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.WriteAll(rows)
	return writer.Error()
}

func ensureParentDir(path string) error {
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		return nil
	}
	return os.MkdirAll(parent, 0o755)
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
