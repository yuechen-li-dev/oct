// Command octxiliary-json serves the Json standard-library Octxiliary wrapper.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	if req.Family != "Json" {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "JsonLoad":
		if err := expect(req.Args, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		text, err := load(req.Args[0].String)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return stringValue(text), nil
	case "JsonSave":
		if err := expect(req.Args, octxiliary.ValueString, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		if err := save(req.Args[0].String, req.Args[1].String); err != nil {
			return octxiliary.Value{}, err
		}
		return intValue(0), nil
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func load(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return normalize(contents)
}

func save(path string, input string) error {
	compact, err := normalize([]byte(input))
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(compact), 0o644)
}

func normalize(input []byte) (string, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, input); err != nil {
		return "", err
	}
	return compact.String(), nil
}

func intValue(value int) octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: value}
}

func stringValue(value string) octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueString, String: value}
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
