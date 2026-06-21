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
			_ = octxiliary.WriteResponseFrame(os.Stdout, resp)
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
		if err := octxiliary.WriteResponseFrame(os.Stdout, resp); err != nil {
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
	case "TestEchoStringRows":
		if err := expect(req.Args, octxiliary.ValueStringMatrix); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueStringMatrix, Strings2: req.Args[0].Strings2}, nil
	case "TestRowCount":
		if err := expect(req.Args, octxiliary.ValueStringMatrix); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: len(req.Args[0].Strings2)}, nil
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
	case "TestSumFloats":
		if err := expect(req.Args, octxiliary.ValueFloatArray); err != nil {
			return octxiliary.Value{}, err
		}
		total := 0.0
		for _, value := range req.Args[0].Floats {
			total += value
		}
		return octxiliary.Value{Kind: octxiliary.ValueFloat, Float: total}, nil
	case "TestDescribeOptions":
		if err := expect(req.Args, octxiliary.ValueRecord); err != nil {
			return octxiliary.Value{}, err
		}
		if req.Args[0].RecordType != "Main.TestOptions" || len(req.Args[0].Fields) != 2 || req.Args[0].Fields[0].Name != "Count" || req.Args[0].Fields[1].Name != "Name" {
			return octxiliary.Value{}, fmt.Errorf("unexpected record payload %#v", req.Args[0])
		}
		if req.Args[0].Fields[0].Value.Kind != octxiliary.ValueInt || req.Args[0].Fields[1].Value.Kind != octxiliary.ValueString {
			return octxiliary.Value{}, fmt.Errorf("unexpected record field kinds")
		}
		return octxiliary.Value{Kind: octxiliary.ValueString, String: fmt.Sprintf("%s:%d", req.Args[0].Fields[1].Value.String, req.Args[0].Fields[0].Value.Int)}, nil
	case "TestReturnOptions":
		if err := expect(req.Args); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueRecord, RecordType: "Main.TestOptions", Fields: []octxiliary.FieldValue{
			{Name: "Count", Value: octxiliary.Value{Kind: octxiliary.ValueInt, Int: 11}},
			{Name: "Name", Value: octxiliary.Value{Kind: octxiliary.ValueString, String: "returned"}},
		}}, nil
	case "TestTouch", "TestTouchDirect":
		if err := expect(req.Args); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueVoid}, nil
	case "TestAlwaysFails":
		if err := expect(req.Args); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{}, fmt.Errorf("test wrapper forced failure")
	case "TestCreateHandle":
		if err := expect(req.Args); err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: "TestWrapper", HandleType: "Main.TestHandle", HandleID: 77}, nil
	case "TestUseHandle":
		if err := expect(req.Args, octxiliary.ValueHandle); err != nil {
			return octxiliary.Value{}, err
		}
		if req.Args[0].HandleFamily != "TestWrapper" || req.Args[0].HandleType != "Main.TestHandle" || req.Args[0].HandleID <= 0 {
			return octxiliary.Value{}, fmt.Errorf("unexpected handle payload %#v", req.Args[0])
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: req.Args[0].HandleID}, nil
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
