// Command octxiliary-text serves the Text standard-library Octxiliary wrapper.
package main

import (
	"fmt"
	"os"
	"regexp"

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
	if req.Family != "Text" {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "RegexIsMatch":
		if err := expect(req.Args, octxiliary.ValueString, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		compiled, err := regexp.Compile(req.Args[0].String)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return boolValue(compiled.MatchString(req.Args[1].String)), nil
	case "RegexFindAll":
		if err := expect(req.Args, octxiliary.ValueString, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		compiled, err := regexp.Compile(req.Args[0].String)
		if err != nil {
			return octxiliary.Value{}, err
		}
		matches := compiled.FindAllString(req.Args[1].String, -1)
		if matches == nil {
			matches = []string{}
		}
		return stringArrayValue(matches), nil
	case "RegexReplaceAll":
		if err := expect(req.Args, octxiliary.ValueString, octxiliary.ValueString, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		compiled, err := regexp.Compile(req.Args[0].String)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return stringValue(compiled.ReplaceAllString(req.Args[1].String, req.Args[2].String)), nil
	case "RegexSplit":
		if err := expect(req.Args, octxiliary.ValueString, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		compiled, err := regexp.Compile(req.Args[0].String)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return stringArrayValue(compiled.Split(req.Args[1].String, -1)), nil
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func boolValue(value bool) octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueBool, Bool: value}
}

func stringValue(value string) octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueString, String: value}
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
