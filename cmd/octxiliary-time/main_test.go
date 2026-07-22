package main

import (
	"strings"
	"testing"
	"time"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

func TestGeneratedTimeDispatch(t *testing.T) {
	tests := []struct {
		name string
		req  octxiliary.Request
		want octxiliary.ValueKind
	}{
		{"now", octxiliary.Request{Family: "Time", Function: "TimeNowIso8601", HasArgs: true}, octxiliary.ValueString},
		{"parse", octxiliary.Request{Family: "Time", Function: "TimeParseIso8601", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: "2024-01-02T03:04:05-05:00"}}}, octxiliary.ValueString},
		{"format", octxiliary.Request{Family: "Time", Function: "TimeFormatIso8601", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: "2024-01-02T03:04:05Z"}}}, octxiliary.ValueString},
		{"unix", octxiliary.Request{Family: "Time", Function: "TimeUnixSecondsNow", HasArgs: true}, octxiliary.ValueInt},
		{"format unix", octxiliary.Request{Family: "Time", Function: "TimeFormatUnixSecond", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueInt, Int: 0}}}, octxiliary.ValueString},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value, err := dispatch(tc.req)
			if err != nil {
				t.Fatalf("dispatch: %v", err)
			}
			if value.Kind != tc.want {
				t.Fatalf("kind = %s, want %s", value.Kind, tc.want)
			}
		})
	}

	value, err := dispatch(octxiliary.Request{Family: "Time", Function: "TimeFormatUnixSecond", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueInt, Int: 0}}})
	if err != nil || value.String != time.Unix(0, 0).UTC().Format(time.RFC3339) {
		t.Fatalf("unix formatting = %#v, %v", value, err)
	}
}

func TestGeneratedTimeDispatchRejectsInvalidRequest(t *testing.T) {
	_, err := dispatch(octxiliary.Request{Family: "Time", Function: "TimeFormatUnixSecond", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: "zero"}}})
	if err == nil || !strings.Contains(err.Error(), "expected Int") {
		t.Fatalf("wrong kind error = %v", err)
	}
}
