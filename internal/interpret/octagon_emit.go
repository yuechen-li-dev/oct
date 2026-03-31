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
		return strconv.FormatFloat(value.Float, 'g', -1, 64) + formatUnitSuffix(value.Dimension), nil
	case ValueBool:
		return strconv.FormatBool(value.Bool), nil
	case ValueString:
		return strconv.Quote(value.Text), nil
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
		if strings.Contains(value.Record.TypeName, ".") {
			return "", fmt.Errorf("record type %q is not representable in .octagon output", value.Record.TypeName)
		}
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
		return fmt.Sprintf("%s {\n%s\n%s}", value.Record.TypeName, strings.Join(fields, "\n"), octagonIndent(depth)), nil
	case ValueEnum:
		if strings.Contains(value.Enum.TypeName, ".") {
			return "", fmt.Errorf("enum type %q is not representable in .octagon output", value.Enum.TypeName)
		}
		return fmt.Sprintf("%s.%s", value.Enum.TypeName, value.Enum.Variant), nil
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
