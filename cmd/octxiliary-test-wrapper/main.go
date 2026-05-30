// Command octxiliary-test-wrapper is a deterministic test sidecar for generic Octxiliary wrapper lowering.
package main

import (
	"fmt"
	"os"
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
	if req.Family != "TestWrapper" {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "TestEchoString":
		if err := expect(req.Args, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueString, String: req.Args[0].String}, nil
	case "TestJoinStrings":
		if err := expect(req.Args, octxiliary.ValueStringArray); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueString, String: strings.Join(req.Args[0].Strings, ",")}, nil
	case "TestBytesLen":
		if err := expect(req.Args, octxiliary.ValueBytes); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: len(req.Args[0].Bytes)}, nil
	case "TestNot":
		if err := expect(req.Args, octxiliary.ValueBool); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueBool, Bool: !req.Args[0].Bool}, nil
	case "TestAddInt":
		if err := expect(req.Args, octxiliary.ValueInt, octxiliary.ValueInt); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: req.Args[0].Int + req.Args[1].Int}, nil
	case "TestHalf":
		if err := expect(req.Args, octxiliary.ValueFloat); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueFloat, Float: req.Args[0].Float / 2}, nil
	case "TestTouch":
		if err := expect(req.Args); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueVoid}, nil
	case "TestAlwaysFails":
		if err := expect(req.Args); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{}, fmt.Errorf("test wrapper forced failure")
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
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
