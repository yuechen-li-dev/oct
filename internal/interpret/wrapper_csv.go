package interpret

import (
	"encoding/csv"
	"os"

	"oct/internal/ast"
)

func csvWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"CsvRead":  (*interpreter).evalCSVReadBuiltin,
		"CsvWrite": (*interpreter).evalCSVWriteBuiltin,
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
