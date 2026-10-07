package interpret

import (
	"fmt"
	"strconv"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

func (i interpreter) materializeOctagonValue(currentPkg string, expectedType ast.TypeRef, expr ast.Expr) (Value, error) {
	expr = unwrapParenExpr(expr)
	// Octagon is data-only, but signed numeric literals parse as a unary minus
	// over a literal. Materialize that narrow scalar form before checking the
	// declared data type; arbitrary expressions remain rejected.
	if unary, ok := expr.(ast.UnaryExpr); ok && unary.Operator == "-" {
		switch operand := unwrapParenExpr(unary.Operand).(type) {
		case ast.IntegerLiteral:
			if expectedType.Name == "Int" && !expectedType.IsArray && expectedType.Package == "" && operand.Dimension == expectedType.Dimension {
				value, err := strconv.ParseInt(operand.Value, 10, 64)
				if err != nil {
					return Value{}, fmt.Errorf("invalid Int literal %q", operand.Value)
				}
				return Value{Kind: ValueInt, Int: -value, Dimension: operand.Dimension}, nil
			}
		case ast.FloatLiteral:
			if expectedType.Name == "Float" && !expectedType.IsArray && expectedType.Package == "" && operand.Dimension == expectedType.Dimension {
				value, err := strconv.ParseFloat(operand.Value, 64)
				if err != nil {
					return Value{}, fmt.Errorf("invalid Float literal %q", operand.Value)
				}
				return Value{Kind: ValueFloat, Float: -value, Dimension: operand.Dimension}, nil
			}
		}
	}
	if expectedType.IsArray && expectedType.ArrayDepth == 0 {
		expectedType.ArrayDepth = 1
	}
	if expectedType.Function != nil {
		return Value{}, fmt.Errorf("unsupported expected type %s", expectedTypeString(expectedType))
	}
	if expectedType.VectorOf != nil && !expectedType.IsArray {
		return i.materializeOctagonVector(currentPkg, expectedType, expr)
	}
	if expectedType.MatrixOf != nil && !expectedType.IsArray {
		return i.materializeOctagonMatrix(currentPkg, expectedType, expr)
	}
	if expectedType.IsArray {
		arrayExpr, ok := expr.(ast.ArrayLiteralExpr)
		if !ok {
			return Value{}, fmt.Errorf("expected %s, got %T", expectedTypeString(expectedType), expr)
		}
		elementType := expectedType
		elementType.ArrayDepth--
		elementType.IsArray = elementType.ArrayDepth > 0
		elements := make([]Value, 0, len(arrayExpr.Elements))
		for idx, elementExpr := range arrayExpr.Elements {
			element, err := i.materializeOctagonValue(currentPkg, elementType, elementExpr)
			if err != nil {
				return Value{}, fmt.Errorf("array element %d mismatch: %w", idx, err)
			}
			elements = append(elements, element)
		}
		return Value{Kind: ValueArray, Array: elements}, nil
	}

	if ast.IsOptionType(expectedType) {
		return i.materializeOctagonOption(currentPkg, expectedType, expr)
	}

	if refinement, refinementPkg, ok := i.lookupRefinementDecl(currentPkg, expectedTypeName(expectedType)); ok {
		base, err := i.materializeOctagonValue(refinementPkg, refinement.Target, expr)
		if err != nil {
			return Value{}, fmt.Errorf("expected refined concept %s: %w", expectedTypeString(expectedType), err)
		}
		constructor, exists := i.functions[refinementPkg+".__oct_refine_"+refinement.Name]
		if !exists {
			return Value{}, fmt.Errorf("runtime invariant violation: missing refinement constructor for %s", expectedTypeString(expectedType))
		}
		checked, err := i.executeFunction(constructor, refinementPkg, []Value{base})
		if err != nil {
			return Value{}, err
		}
		if checked.hasError {
			return Value{}, fmt.Errorf("%s", checked.errorVal.Error.Message)
		}
		return checked.value, nil
	}

	if expectedType.Package == "" {
		switch expectedType.Name {
		case "Int":
			intExpr, ok := expr.(ast.IntegerLiteral)
			if !ok {
				return Value{}, fmt.Errorf("expected %s, got %T", expectedTypeString(expectedType), expr)
			}
			if intExpr.Dimension != expectedType.Dimension {
				return Value{}, fmt.Errorf("expected %s, got Int<%s>", expectedTypeString(expectedType), intExpr.Dimension)
			}
			value, err := strconv.ParseInt(intExpr.Value, 10, 64)
			if err != nil {
				return Value{}, fmt.Errorf("invalid Int literal %q", intExpr.Value)
			}
			return Value{Kind: ValueInt, Int: value, Dimension: intExpr.Dimension}, nil
		case "Float":
			floatExpr, ok := expr.(ast.FloatLiteral)
			if !ok {
				return Value{}, fmt.Errorf("expected %s, got %T", expectedTypeString(expectedType), expr)
			}
			if floatExpr.Dimension != expectedType.Dimension {
				return Value{}, fmt.Errorf("expected %s, got Float<%s>", expectedTypeString(expectedType), floatExpr.Dimension)
			}
			value, err := strconv.ParseFloat(floatExpr.Value, 64)
			if err != nil {
				return Value{}, fmt.Errorf("invalid Float literal %q", floatExpr.Value)
			}
			return Value{Kind: ValueFloat, Float: value, Dimension: floatExpr.Dimension}, nil
		case "Bool":
			boolExpr, ok := expr.(ast.BoolLiteral)
			if !ok {
				return Value{}, fmt.Errorf("expected Bool, got %T", expr)
			}
			return Value{Kind: ValueBool, Bool: boolExpr.Value}, nil
		case "String":
			stringExpr, ok := expr.(ast.StringLiteralExpr)
			if !ok {
				return Value{}, fmt.Errorf("expected String, got %T", expr)
			}
			return Value{Kind: ValueString, Text: stringExpr.Value}, nil
		}
	}

	recordDecl, resolvedRecordName, hasRecord := i.lookupRecordDecl(currentPkg, expectedTypeName(expectedType))
	if hasRecord {
		recordExpr, ok := expr.(ast.RecordLiteralExpr)
		if !ok {
			return Value{}, fmt.Errorf("expected %s, got %T", expectedTypeString(expectedType), expr)
		}
		if recordExpr.TypeName != expectedType.Name && recordExpr.TypeName != expectedType.Package+"."+expectedType.Name {
			return Value{}, fmt.Errorf("expected record %s, got %s", expectedTypeString(expectedType), recordExpr.TypeName)
		}
		fields := make(map[string]ast.Expr, len(recordExpr.Fields))
		for _, field := range recordExpr.Fields {
			if _, exists := fields[field.Name]; exists {
				return Value{}, fmt.Errorf("record %s field %s specified more than once", expectedTypeString(expectedType), field.Name)
			}
			fields[field.Name] = field.Value
		}
		values := make(map[string]Value, len(recordDecl.Fields))
		fieldOrder := make([]string, 0, len(recordDecl.Fields))
		for _, declaredField := range recordDecl.Fields {
			fieldExpr, ok := fields[declaredField.Name]
			if !ok {
				return Value{}, fmt.Errorf("record %s missing field %s", expectedTypeString(expectedType), declaredField.Name)
			}
			fieldType := declaredField.Type
			if recordDecl.IsTable {
				fieldType.ArrayDepth++
				fieldType.IsArray = true
			}
			value, err := i.materializeOctagonValueOf(currentPkg, resolvedRecordName, fieldType, fieldExpr)
			if err != nil {
				if recordDecl.IsTable {
					return Value{}, fmt.Errorf("record table %s column %s mismatch: %w", expectedTypeString(expectedType), declaredField.Name, err)
				}
				return Value{}, fmt.Errorf("record field %s mismatch: %w", declaredField.Name, err)
			}
			values[declaredField.Name] = value
			fieldOrder = append(fieldOrder, declaredField.Name)
			delete(fields, declaredField.Name)
		}
		for extra := range fields {
			return Value{}, fmt.Errorf("record %s has unexpected field %s", expectedTypeString(expectedType), extra)
		}
		record := RecordValue{TypeName: resolvedRecordName, Fields: values, FieldOrder: fieldOrder}
		if recordDecl.IsTable && i.staticProofs != nil {
			record.StaticSubject = i.staticProofs.newSubject(resolvedRecordName)
		}
		return Value{Kind: ValueRecord, Record: record}, nil
	}

	enumDecl, resolvedEnumName, hasEnum := i.lookupEnumDecl(currentPkg, expectedTypeName(expectedType))
	if hasEnum {
		enumName := ""
		enumVariant := ""
		var payload []ast.Expr
		hasPayloadForm := false
		switch enumExpr := expr.(type) {
		case ast.CallExpr:
			hasPayloadForm = true
			payload = enumExpr.Arguments
			expr = enumExpr.Callee
		}
		switch enumExpr := expr.(type) {
		case ast.EnumValueExpr:
			enumName = enumExpr.EnumName
			enumVariant = enumExpr.Variant
		case ast.FieldAccessExpr:
			if flattened, ok := flattenDirectCallName(enumExpr); ok {
				dot := -1
				for idx := range flattened {
					if flattened[idx] == '.' {
						dot = idx
						break
					}
				}
				if dot > 0 && dot < len(flattened)-1 {
					enumName = flattened[:dot]
					enumVariant = flattened[dot+1:]
				}
			}
		}
		if enumName == "" {
			return Value{}, fmt.Errorf("expected %s, got %T", expectedTypeString(expectedType), expr)
		}
		if enumName != expectedType.Name && enumName != expectedType.Package+"."+expectedType.Name {
			return Value{}, fmt.Errorf("expected enum %s, got %s", expectedTypeString(expectedType), enumName)
		}
		for _, declaredVariant := range enumDecl.Variants {
			if declaredVariant.Name == enumVariant {
				if declaredVariant.Payload != nil {
					if len(payload) != 1 {
						return Value{}, fmt.Errorf("enum %s variant %s requires exactly 1 payload argument, got %d", expectedTypeString(expectedType), enumVariant, len(payload))
					}
					value, err := i.materializeOctagonValueOf(currentPkg, resolvedEnumName, *declaredVariant.Payload, payload[0])
					if err != nil {
						return Value{}, fmt.Errorf("enum %s variant %s payload mismatch: %w", expectedTypeString(expectedType), enumVariant, err)
					}
					return Value{Kind: ValueEnum, Enum: EnumValue{TypeName: resolvedEnumName, Variant: enumVariant, Payload: &value}}, nil
				}
				if hasPayloadForm {
					return Value{}, fmt.Errorf("enum %s variant %s does not accept a payload", expectedTypeString(expectedType), enumVariant)
				}
				return Value{Kind: ValueEnum, Enum: EnumValue{TypeName: resolvedEnumName, Variant: enumVariant}}, nil
			}
		}
		return Value{}, fmt.Errorf("enum %s has no variant %s", expectedTypeString(expectedType), enumVariant)
	}

	return Value{}, fmt.Errorf("unsupported expected type %s", expectedTypeString(expectedType))
}

// materializeOctagonVector loads an array of numbers as the declared
// `Vector<T>`. Octagon data has no vector of its own: an array is one where
// the declared type says so.
func (i interpreter) materializeOctagonVector(currentPkg string, expectedType ast.TypeRef, expr ast.Expr) (Value, error) {
	arrayExpr, ok := expr.(ast.ArrayLiteralExpr)
	if !ok {
		return Value{}, fmt.Errorf("expected %s, got %s", expectedTypeString(expectedType), octagonDataDescription(expr))
	}
	elements := make([]Value, 0, len(arrayExpr.Elements))
	for idx, elementExpr := range arrayExpr.Elements {
		element, err := i.materializeOctagonValue(currentPkg, *expectedType.VectorOf, elementExpr)
		if err != nil {
			return Value{}, fmt.Errorf("vector element %d mismatch: %w", idx, err)
		}
		elements = append(elements, element)
	}
	return Value{Kind: ValueVector, Vector: elements}, nil
}

// materializeOctagonMatrix loads an array of rows of numbers, all one
// length, as the declared `Matrix<T>`.
func (i interpreter) materializeOctagonMatrix(currentPkg string, expectedType ast.TypeRef, expr ast.Expr) (Value, error) {
	rowsExpr, ok := expr.(ast.ArrayLiteralExpr)
	if !ok {
		return Value{}, fmt.Errorf("expected %s, got %s", expectedTypeString(expectedType), octagonDataDescription(expr))
	}
	matrix := MatrixValue{Rows: len(rowsExpr.Elements)}
	for rowIndex, rowExpr := range rowsExpr.Elements {
		row, ok := rowExpr.(ast.ArrayLiteralExpr)
		if !ok {
			return Value{}, fmt.Errorf("matrix row %d mismatch: expected an array, got %s", rowIndex, octagonDataDescription(rowExpr))
		}
		if rowIndex == 0 {
			matrix.Cols = len(row.Elements)
		}
		if len(row.Elements) != matrix.Cols {
			return Value{}, fmt.Errorf("matrix row %d has %d elements, and row 0 has %d", rowIndex, len(row.Elements), matrix.Cols)
		}
		for columnIndex, elementExpr := range row.Elements {
			element, err := i.materializeOctagonValue(currentPkg, *expectedType.MatrixOf, elementExpr)
			if err != nil {
				return Value{}, fmt.Errorf("matrix element %d, %d mismatch: %w", rowIndex, columnIndex, err)
			}
			matrix.Elements = append(matrix.Elements, element)
		}
	}
	return Value{Kind: ValueMatrix, Matrix: matrix}, nil
}

// materializeOctagonValueOf loads a field of a record, or the payload of an
// enum variant, whose declaring type is ownerType. The field's type was
// written in the owner's package, so it is resolved there, and a value of a
// type from that package is named as any value that leaves its package is.
func (i interpreter) materializeOctagonValueOf(currentPkg string, ownerType string, expectedType ast.TypeRef, expr ast.Expr) (Value, error) {
	ownerPkg := packageForTypeName(currentPkg, ownerType)
	value, err := i.materializeOctagonValue(ownerPkg, expectedType, expr)
	if err != nil || ownerPkg == currentPkg {
		return value, err
	}
	return qualifyCrossPackageValue(value, ownerPkg), nil
}

func (i interpreter) lookupRefinementDecl(currentPackage string, typeName string) (ast.ConceptDecl, string, bool) {
	if pkgName, localName, ok := splitQualifiedTypeName(typeName); ok {
		decl, exists := i.refinements[pkgName+"."+localName]
		return decl, pkgName, exists
	}
	decl, exists := i.refinements[currentPackage+"."+typeName]
	return decl, currentPackage, exists
}

func unwrapParenExpr(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.Inner
	}
}

func expectedTypeName(typeRef ast.TypeRef) string {
	if typeRef.Package == "" {
		return typeRef.Name
	}
	return typeRef.Package + "." + typeRef.Name
}

func expectedTypeString(typeRef ast.TypeRef) string {
	if typeRef.IsArray && typeRef.ArrayDepth == 0 {
		typeRef.ArrayDepth = 1
	}
	name := expectedTypeName(typeRef)
	if typeRef.Package == "" && typeRef.Name == ast.OptionTypeName && len(typeRef.TypeArguments) == 1 {
		name += "<" + expectedTypeString(typeRef.TypeArguments[0]) + ">"
	}
	if (name == "Int" || name == "Float") && !typeRef.Dimension.IsDimensionless() {
		name += "<" + typeRef.Dimension.String() + ">"
	}
	if typeRef.IsArray {
		for idx := 0; idx < typeRef.ArrayDepth; idx++ {
			name += "[]"
		}
	}
	if typeRef.Function != nil {
		return "fn(...)"
	}
	if typeRef.VectorOf != nil {
		return "Vector<...>"
	}
	if typeRef.MatrixOf != nil {
		return "Matrix<...>"
	}
	return name
}
