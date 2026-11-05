package interpret

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

func WriteOctagon(path string, value Value) error {
	if !strings.HasSuffix(path, ".octagon") {
		return fmt.Errorf("WriteOctagon path must end with .octagon")
	}
	path = attributedOutputPath(path)
	return writeOctagonPath(path, value)
}

func writeOctagonPath(path string, value Value) error {
	rendered, err := serializeOctagonValue(value)
	if err != nil {
		return fmt.Errorf("WriteOctagon cannot serialize value: %w", err)
	}
	if err := os.WriteFile(path, []byte(rendered+"\n"), 0o644); err != nil {
		return fmt.Errorf("WriteOctagon write %s: %w", path, err)
	}
	return nil
}

func serializeOctagonValue(value Value) (string, error) {
	return serializeOctagonValueAtDepth(value, 0)
}

func serializeOctagonValueAtDepth(value Value, depth int) (string, error) {
	switch value.Kind {
	case ValueInt:
		return strconv.FormatInt(value.Int, 10) + formatUnitSuffix(value.Dimension), nil
	case ValueFloat:
		text := strconv.FormatFloat(value.Float, 'g', -1, 64)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return text + formatUnitSuffix(value.Dimension), nil
	case ValueBool:
		return strconv.FormatBool(value.Bool), nil
	case ValueString:
		return strconv.Quote(value.Text), nil
	case ValueMatrix:
		rows := make([]Value, value.Matrix.Rows)
		for row := range rows {
			rows[row] = Value{Kind: ValueArray, Array: value.Matrix.Elements[row*value.Matrix.Cols : (row+1)*value.Matrix.Cols]}
		}
		return serializeOctagonValueAtDepth(Value{Kind: ValueArray, Array: rows}, depth)
	case ValueVector:
		return serializeOctagonValueAtDepth(Value{Kind: ValueArray, Array: value.Vector}, depth)
	case ValueArray:
		parts := make([]string, 0, len(value.Array))
		for _, element := range value.Array {
			rendered, err := serializeOctagonValueAtDepth(element, depth)
			if err != nil {
				return "", err
			}
			parts = append(parts, rendered)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	case ValueRecord:
		fieldOrder := recordFieldOrder(value.Record)
		fields := make([]string, 0, len(fieldOrder))
		for _, fieldName := range fieldOrder {
			fieldValue, ok := value.Record.Fields[fieldName]
			if !ok {
				return "", fmt.Errorf("record %q missing field %q", value.Record.TypeName, fieldName)
			}
			rendered, err := serializeOctagonValueAtDepth(fieldValue, depth+1)
			if err != nil {
				return "", err
			}
			fieldRendered := rendered
			if octagonNeedsFieldParens(fieldValue) {
				fieldRendered = "(" + rendered + ")"
			}
			fields = append(fields, fmt.Sprintf("%s%s: %s", octagonIndent(depth+1), fieldName, fieldRendered))
		}
		return fmt.Sprintf("%s {\n%s\n%s}", octagonTypeName(value.Record.TypeName), strings.Join(fields, "\n"), octagonIndent(depth)), nil
	case ValueEnum:
		name := fmt.Sprintf("%s.%s", octagonTypeName(value.Enum.TypeName), value.Enum.Variant)
		if value.Enum.Payload == nil {
			return name, nil
		}
		payload, err := serializeOctagonValueAtDepth(*value.Enum.Payload, depth)
		if err != nil {
			return "", err
		}
		return name + "(" + payload + ")", nil
	default:
		return "", fmt.Errorf("value kind %s is not representable in .octagon output", value.Kind)
	}
}

func octagonIndent(depth int) string {
	return strings.Repeat("    ", depth)
}

func octagonNeedsFieldParens(value Value) bool {
	return value.Kind == ValueInt || value.Kind == ValueFloat
}

func recordFieldOrder(record RecordValue) []string {
	if len(record.FieldOrder) > 0 {
		return append([]string(nil), record.FieldOrder...)
	}
	order := make([]string, 0, len(record.Fields))
	for fieldName := range record.Fields {
		order = append(order, fieldName)
	}
	sort.Strings(order)
	return order
}

// octagonTypeName is the name a record or an enum is written under: its own,
// without the package. A value that came from another package carries that
// package in its type name; the file is loaded as a declared type, which
// says which package is meant, and the compiled writer writes the same.
func octagonTypeName(typeName string) string {
	if dot := strings.LastIndex(typeName, "."); dot >= 0 {
		return typeName[dot+1:]
	}
	return typeName
}
