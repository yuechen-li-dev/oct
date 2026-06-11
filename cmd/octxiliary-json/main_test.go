package main

import (
	"path/filepath"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

func TestDispatchJsonParseNormalizesInlineDocument(t *testing.T) {
	value, err := dispatch(octxiliary.Request{Family: "Json", Function: "JsonParse", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: "{ \"ok\": true }"}}})
	if err != nil {
		t.Fatalf("JsonParse dispatch failed: %v", err)
	}
	if value.Kind != octxiliary.ValueString || value.String != `{"ok":true}` {
		t.Fatalf("JsonParse returned %#v", value)
	}
}

func TestDispatchJsonParseRejectsInvalidDocument(t *testing.T) {
	_, err := dispatch(octxiliary.Request{Family: "Json", Function: "JsonParse", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: "{"}}})
	if err == nil {
		t.Fatalf("expected JsonParse to reject invalid JSON")
	}
}

func TestDispatchJsonLoadMissingFileReturnsError(t *testing.T) {
	_, err := dispatch(octxiliary.Request{Family: "Json", Function: "JsonLoad", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: filepath.Join(t.TempDir(), "missing.json")}}})
	if err == nil {
		t.Fatalf("expected JsonLoad to reject missing files")
	}
}
