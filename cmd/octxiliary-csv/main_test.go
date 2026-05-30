package main

import (
	"path/filepath"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

func TestDispatchCsvWriteReadRowsPreservesRaggedAndEscapedCells(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "rows.csv")
	rows := [][]string{{"a,b", "quote \" here", "line\nbreak", ""}, {"ragged"}}

	writeValue, err := dispatch(octxiliary.Request{Family: "Csv", Function: "CsvWrite", HasArgs: true, Args: []octxiliary.Value{
		{Kind: octxiliary.ValueString, String: path},
		{Kind: octxiliary.ValueStringMatrix, Strings2: rows},
	}})
	if err != nil {
		t.Fatalf("write dispatch failed: %v", err)
	}
	if writeValue.Kind != octxiliary.ValueInt || writeValue.Int != 0 {
		t.Fatalf("write dispatch returned %#v", writeValue)
	}

	readValue, err := dispatch(octxiliary.Request{Family: "Csv", Function: "CsvRead", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: path}}})
	if err != nil {
		t.Fatalf("read dispatch failed: %v", err)
	}
	if readValue.Kind != octxiliary.ValueStringMatrix || len(readValue.Strings2) != 2 || readValue.Strings2[0][1] != "quote \" here" || readValue.Strings2[0][2] != "line\nbreak" || len(readValue.Strings2[1]) != 1 {
		t.Fatalf("read dispatch returned %#v", readValue)
	}
}

func TestDispatchCsvReadMissingFileReturnsError(t *testing.T) {
	_, err := dispatch(octxiliary.Request{Family: "Csv", Function: "CsvRead", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueString, String: filepath.Join(t.TempDir(), "missing.csv")}}})
	if err == nil {
		t.Fatal("expected missing file error")
	}
}
