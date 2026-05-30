// Command octxiliary-xlsx serves the Xlsx standard-library Octxiliary wrapper.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

const (
	xlsxFamily     = "Xlsx"
	workbookHandle = "IO.Workbook"
)

type xlsxWorkbook struct {
	file *excelize.File
}

type workbookTable struct {
	next      int
	workbooks map[int]*xlsxWorkbook
}

func newWorkbookTable() *workbookTable {
	return &workbookTable{next: 1, workbooks: map[int]*xlsxWorkbook{}}
}

func (t *workbookTable) allocate(workbook *xlsxWorkbook) int {
	id := t.next
	t.next++
	t.workbooks[id] = workbook
	return id
}

func (t *workbookTable) get(value octxiliary.Value) (*xlsxWorkbook, error) {
	if value.Kind != octxiliary.ValueHandle {
		return nil, fmt.Errorf("expected workbook handle, got %s", value.Kind)
	}
	if value.HandleFamily != xlsxFamily || value.HandleType != workbookHandle {
		return nil, fmt.Errorf("expected %s %s handle", xlsxFamily, workbookHandle)
	}
	workbook, ok := t.workbooks[value.HandleID]
	if !ok {
		return nil, fmt.Errorf("unknown workbook handle %d", value.HandleID)
	}
	return workbook, nil
}

func main() {
	if err := octxiliary.ReadHandshake(os.Stdin); err != nil {
		return
	}
	if err := octxiliary.WriteHandshake(os.Stdout); err != nil {
		return
	}
	table := newWorkbookTable()
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
		value, err := table.dispatch(req)
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

func (t *workbookTable) dispatch(req octxiliary.Request) (octxiliary.Value, error) {
	if req.Family != xlsxFamily {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "XlsxCreateWorkbook":
		if err := expect(req.Args); err != nil {
			return octxiliary.Value{}, err
		}
		file := excelize.NewFile()
		file.DeleteSheet("Sheet1")
		id := t.allocate(&xlsxWorkbook{file: file})
		return octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: xlsxFamily, HandleType: workbookHandle, HandleID: id}, nil
	case "XlsxAddSheet":
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		workbook, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		return addSheet(workbook, req.Args[1].String)
	case "XlsxSetCellString":
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueString, octxiliary.ValueString, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		workbook, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		return setCellString(workbook, req.Args[1].String, req.Args[2].String, req.Args[3].String)
	case "XlsxSetCellFloat":
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueString, octxiliary.ValueString, octxiliary.ValueFloat); err != nil {
			return octxiliary.Value{}, err
		}
		workbook, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		return setCellFloat(workbook, req.Args[1].String, req.Args[2].String, req.Args[3].Float)
	case "XlsxSaveWorkbook":
		if err := expect(req.Args, octxiliary.ValueHandle, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		workbook, err := t.get(req.Args[0])
		if err != nil {
			return octxiliary.Value{}, err
		}
		return saveWorkbook(workbook, req.Args[1].String)
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func addSheet(workbook *xlsxWorkbook, sheetName string) (octxiliary.Value, error) {
	sheetIndex, sheetErr := workbook.file.GetSheetIndex(sheetName)
	if sheetErr != nil {
		return octxiliary.Value{}, sheetErr
	}
	if sheetIndex != -1 {
		return octxiliary.Value{}, fmt.Errorf("sheet %q already exists", sheetName)
	}
	if _, err := workbook.file.NewSheet(sheetName); err != nil {
		return octxiliary.Value{}, err
	}
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
}

func setCellString(workbook *xlsxWorkbook, sheetName string, cell string, value string) (octxiliary.Value, error) {
	if err := requireSheet(workbook, sheetName); err != nil {
		return octxiliary.Value{}, err
	}
	if err := workbook.file.SetCellStr(sheetName, cell, value); err != nil {
		return octxiliary.Value{}, err
	}
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
}

func setCellFloat(workbook *xlsxWorkbook, sheetName string, cell string, value float64) (octxiliary.Value, error) {
	if err := requireSheet(workbook, sheetName); err != nil {
		return octxiliary.Value{}, err
	}
	if err := workbook.file.SetCellFloat(sheetName, cell, value, -1, 64); err != nil {
		return octxiliary.Value{}, err
	}
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
}

func saveWorkbook(workbook *xlsxWorkbook, path string) (octxiliary.Value, error) {
	if !strings.HasSuffix(path, ".xlsx") {
		return octxiliary.Value{}, fmt.Errorf("path must end with .xlsx")
	}
	if len(workbook.file.GetSheetList()) == 0 {
		return octxiliary.Value{}, fmt.Errorf("workbook has no sheets")
	}
	if err := workbook.file.SaveAs(path); err != nil {
		return octxiliary.Value{}, fmt.Errorf("%s: %v", path, err)
	}
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
}

func requireSheet(workbook *xlsxWorkbook, sheetName string) error {
	sheetIndex, sheetErr := workbook.file.GetSheetIndex(sheetName)
	if sheetErr != nil {
		return sheetErr
	}
	if sheetIndex == -1 {
		return fmt.Errorf("unknown sheet %q", sheetName)
	}
	return nil
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
