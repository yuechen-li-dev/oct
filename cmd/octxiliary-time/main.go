// Command octxiliary-time serves the Time standard-library Octxiliary wrapper.
package main

import (
	"fmt"
	"os"
	"time"

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
	if req.Family != "Time" {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "TimeNowIso8601":
		if err := expect(req.Args); err != nil {
			return octxiliary.Value{}, err
		}
		return stringValue(time.Now().UTC().Format(time.RFC3339)), nil
	case "TimeParseIso8601", "TimeFormatIso8601":
		if err := expect(req.Args, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		parsed, err := parseIso8601(req.Args[0].String)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return stringValue(parsed.Format(time.RFC3339)), nil
	case "TimeUnixSecondsNow":
		if err := expect(req.Args); err != nil {
			return octxiliary.Value{}, err
		}
		return intValue(int(time.Now().UTC().Unix())), nil
	case "TimeFormatUnixSecond":
		if err := expect(req.Args, octxiliary.ValueInt); err != nil {
			return octxiliary.Value{}, err
		}
		return stringValue(time.Unix(int64(req.Args[0].Int), 0).UTC().Format(time.RFC3339)), nil
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func parseIso8601(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func stringValue(value string) octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueString, String: value}
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
