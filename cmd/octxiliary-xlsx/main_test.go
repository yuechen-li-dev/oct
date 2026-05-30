package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

func TestWorkbookWorkflow(t *testing.T) {
	table := newWorkbookTable()
	created, err := table.dispatch(octxiliary.Request{Family: xlsxFamily, Function: "XlsxCreateWorkbook", HasArgs: true})
	if err != nil {
		t.Fatalf("create workbook: %v", err)
	}
	if created.Kind != octxiliary.ValueHandle || created.HandleID <= 0 || created.HandleFamily != xlsxFamily || created.HandleType != workbookHandle {
		t.Fatalf("unexpected handle: %#v", created)
	}
	if _, err := table.dispatch(octxiliary.Request{Family: xlsxFamily, Function: "XlsxAddSheet", HasArgs: true, Args: []octxiliary.Value{created, {Kind: octxiliary.ValueString, String: "Data"}}}); err != nil {
		t.Fatalf("add sheet: %v", err)
	}
	if _, err := table.dispatch(octxiliary.Request{Family: xlsxFamily, Function: "XlsxSetCellString", HasArgs: true, Args: []octxiliary.Value{created, {Kind: octxiliary.ValueString, String: "Data"}, {Kind: octxiliary.ValueString, String: "A1"}, {Kind: octxiliary.ValueString, String: "Metric"}}}); err != nil {
		t.Fatalf("set string: %v", err)
	}
	if _, err := table.dispatch(octxiliary.Request{Family: xlsxFamily, Function: "XlsxSetCellFloat", HasArgs: true, Args: []octxiliary.Value{created, {Kind: octxiliary.ValueString, String: "Data"}, {Kind: octxiliary.ValueString, String: "B1"}, {Kind: octxiliary.ValueFloat, Float: 42.5}}}); err != nil {
		t.Fatalf("set float: %v", err)
	}
	path := filepath.Join(t.TempDir(), "out.xlsx")
	if _, err := table.dispatch(octxiliary.Request{Family: xlsxFamily, Function: "XlsxSaveWorkbook", HasArgs: true, Args: []octxiliary.Value{created, {Kind: octxiliary.ValueString, String: path}}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("saved file is empty")
	}
}

func TestInvalidWorkbookHandleErrors(t *testing.T) {
	table := newWorkbookTable()
	_, err := table.dispatch(octxiliary.Request{Family: xlsxFamily, Function: "XlsxAddSheet", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueHandle, HandleFamily: xlsxFamily, HandleType: workbookHandle, HandleID: 999}, {Kind: octxiliary.ValueString, String: "Data"}}})
	if err == nil || !strings.Contains(err.Error(), "unknown workbook handle") {
		t.Fatalf("expected invalid handle error, got %v", err)
	}
}

func TestInvalidSaveExtensionErrors(t *testing.T) {
	table := newWorkbookTable()
	created, err := table.dispatch(octxiliary.Request{Family: xlsxFamily, Function: "XlsxCreateWorkbook", HasArgs: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = table.dispatch(octxiliary.Request{Family: xlsxFamily, Function: "XlsxSaveWorkbook", HasArgs: true, Args: []octxiliary.Value{created, {Kind: octxiliary.ValueString, String: "bad.txt"}}})
	if err == nil || !strings.Contains(err.Error(), ".xlsx") {
		t.Fatalf("expected extension error, got %v", err)
	}
}

func TestMissingSheetWriteErrors(t *testing.T) {
	table := newWorkbookTable()
	created, err := table.dispatch(octxiliary.Request{Family: xlsxFamily, Function: "XlsxCreateWorkbook", HasArgs: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = table.dispatch(octxiliary.Request{Family: xlsxFamily, Function: "XlsxSetCellString", HasArgs: true, Args: []octxiliary.Value{created, {Kind: octxiliary.ValueString, String: "Missing"}, {Kind: octxiliary.ValueString, String: "A1"}, {Kind: octxiliary.ValueString, String: "x"}}})
	if err == nil || !strings.Contains(err.Error(), "unknown sheet") {
		t.Fatalf("expected missing sheet error, got %v", err)
	}
}
