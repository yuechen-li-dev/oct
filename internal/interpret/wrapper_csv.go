package interpret

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"oct/internal/ast"
)

func csvWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"CsvRead":       (*interpreter).evalCSVReadBuiltin,
		"CsvReadRows":   (*interpreter).evalCSVReadBuiltin,
		"CsvReadTable":  (*interpreter).evalCSVReadTableBuiltin,
		"CsvReadMatrix": (*interpreter).evalCSVReadMatrixBuiltin,
		"CsvWrite":      (*interpreter).evalCSVWriteBuiltin,
		"CsvWriteRows":  (*interpreter).evalCSVWriteBuiltin,
		"CsvWriteTable": (*interpreter).evalCSVWriteTableBuiltin,
		"CsvWriteMatrix": (*interpreter).evalCSVWriteMatrixBuiltin,
	}
}

func (i *interpreter) evalCSVReadBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	path, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	file, openErr := os.Open(path)
	if openErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, openErr)), nil
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = 0
	rows, readErr := reader.ReadAll()
	if readErr != nil {
		return wrapperErrorResult(callee, mapCSVError(readErr)), nil
	}
	if len(rows) > 1 {
		expectedColumns := len(rows[0])
		for rowIndex := 1; rowIndex < len(rows); rowIndex++ {
			if len(rows[rowIndex]) != expectedColumns {
				return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "inconsistent column count at row %d", rowIndex+1)), nil
			}
		}
	}
	return wrapperStringMatrixResult(rows), nil
}

func (i *interpreter) evalCSVWriteBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	path, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	rowsValue, errResult, err := call.evalArg(1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	rows, decodeErr := stringMatrixArg(rowsValue)
	if decodeErr != nil {
		return wrapperErrorResult(callee, decodeErr), nil
	}
	file, createErr := os.Create(path)
	if createErr != nil {
		if mkdirErr := ensureParentDir(path); mkdirErr == nil {
			file, createErr = os.Create(path)
		}
	}
	if createErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, createErr)), nil
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.WriteAll(rows)
	if writeErr := writer.Error(); writeErr != nil {
		return wrapperErrorResult(callee, mapCSVError(writeErr)), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalCSVReadTableBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	rowsResult, err := i.evalCSVReadBuiltin(env, pkgName, "CsvReadRows", argumentExprs)
	if err != nil {
		return rowsResult, err
	}
	rows := rowsResult.value.Array
	if len(rows) == 0 {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "csv table requires at least one header row")), nil
	}
	headerRow := rows[0].Array
	if len(headerRow) == 0 {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "csv table header row cannot be empty")), nil
	}
	seen := map[string]int{}
	columns := map[string]Value{}
	fieldOrder := make([]string, 0, len(headerRow))
	for idx, headerValue := range headerRow {
		header := headerValue.Text
		if header == "" {
			return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "csv table header %d is empty", idx+1)), nil
		}
		if prior, ok := seen[header]; ok {
			return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "duplicate csv table header %q at columns %d and %d", header, prior+1, idx+1)), nil
		}
		seen[header] = idx
		fieldOrder = append(fieldOrder, header)
		columns[header] = Value{Kind: ValueArray, Array: []Value{}}
	}
	for rowIndex := 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex].Array
		if len(row) != len(headerRow) {
			return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "inconsistent column count at row %d", rowIndex+1)), nil
		}
		for colIndex, cell := range row {
			header := headerRow[colIndex].Text
			column := columns[header]
			column.Array = append(column.Array, Value{Kind: ValueString, Text: cell.Text})
			columns[header] = column
		}
	}
	return evalResult{value: Value{Kind: ValueRecord, Record: RecordValue{TypeName: "Csv.Table", FieldOrder: fieldOrder, Fields: columns}}}, nil
}

func (i *interpreter) evalCSVReadMatrixBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	rowsResult, err := i.evalCSVReadBuiltin(env, pkgName, "CsvReadRows", argumentExprs)
	if err != nil {
		return rowsResult, err
	}
	rows := rowsResult.value.Array
	if len(rows) == 0 {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "csv matrix requires at least one row")), nil
	}
	matrix := make([]Value, 0, len(rows))
	for rowIndex, rowValue := range rows {
		row := rowValue.Array
		floatRow := make([]Value, 0, len(row))
		for colIndex, cell := range row {
			parsed, parseErr := strconv.ParseFloat(cell.Text, 64)
			if parseErr != nil {
				return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "non-numeric cell at row %d column %d: %q", rowIndex+1, colIndex+1, cell.Text)), nil
			}
			floatRow = append(floatRow, Value{Kind: ValueFloat, Float: parsed})
		}
		matrix = append(matrix, Value{Kind: ValueArray, Array: floatRow})
	}
	return wrapperArrayResult(matrix), nil
}

func (i *interpreter) evalCSVWriteTableBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidArgument, "Csv.WriteTable is not implemented in M0; use Csv.WriteRows/Csv.Write")), nil
}

func (i *interpreter) evalCSVWriteMatrixBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	path, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	matrixValue, errResult, err := call.evalArg(1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	if matrixValue.Kind != ValueArray {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument expects Float[][]")), nil
	}
	rows := make([][]string, 0, len(matrixValue.Array))
	for rowIndex, rowValue := range matrixValue.Array {
		if rowValue.Kind != ValueArray {
			return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument expects Float[][] (row %d)", rowIndex)), nil
		}
		row := make([]string, 0, len(rowValue.Array))
		for colIndex, colValue := range rowValue.Array {
			if colValue.Kind != ValueFloat && colValue.Kind != ValueInt {
				return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument expects Float[][] (row %d column %d)", rowIndex, colIndex)), nil
			}
			if colValue.Kind == ValueFloat {
				row = append(row, fmt.Sprintf("%g", colValue.Float))
			} else {
				row = append(row, fmt.Sprintf("%d", colValue.Int))
			}
		}
		rows = append(rows, row)
	}
	return i.writeCSVRows(path, rows, callee), nil
}

func (i *interpreter) writeCSVRows(path string, rows [][]string, callee string) evalResult {
	file, createErr := os.Create(path)
	if createErr != nil {
		if mkdirErr := ensureParentDir(path); mkdirErr == nil {
			file, createErr = os.Create(path)
		}
	}
	if createErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, createErr))
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	writer.WriteAll(rows)
	if writeErr := writer.Error(); writeErr != nil {
		return wrapperErrorResult(callee, mapCSVError(writeErr))
	}
	return wrapperIntResult(0)
}

func stringMatrixArg(value Value) ([][]string, error) {
	if value.Kind != ValueArray {
		return nil, wrapperErrorf(wrapperErrorInvalidArgument, "argument expects String[][]")
	}
	rows := make([][]string, 0, len(value.Array))
	for rowIndex, rowValue := range value.Array {
		if rowValue.Kind != ValueArray {
			return nil, wrapperErrorf(wrapperErrorInvalidArgument, "argument expects String[][] (row %d)", rowIndex)
		}
		row := make([]string, 0, len(rowValue.Array))
		for colIndex, colValue := range rowValue.Array {
			if colValue.Kind != ValueString {
				return nil, wrapperErrorf(wrapperErrorInvalidArgument, "argument expects String[][] (row %d column %d)", rowIndex, colIndex)
			}
			row = append(row, colValue.Text)
		}
		rows = append(rows, row)
	}
	return rows, nil
}
