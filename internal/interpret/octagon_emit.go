package interpret

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func serializeOctagonValue(value Value) (string, error) {
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
			rendered, err := serializeOctagonValue(element)
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
			rendered, err := serializeOctagonValue(fieldValue)
			if err != nil {
				return "", err
			}
			fields = append(fields, fmt.Sprintf("%s: %s", fieldName, rendered))
		}
		return fmt.Sprintf("%s { %s }", value.Record.TypeName, strings.Join(fields, " ")), nil
	case ValueEnum:
		if strings.Contains(value.Enum.TypeName, ".") {
			return "", fmt.Errorf("enum type %q is not representable in .octagon output", value.Enum.TypeName)
		}
		return fmt.Sprintf("%s.%s", value.Enum.TypeName, value.Enum.Variant), nil
	default:
		return "", fmt.Errorf("value kind %s is not representable in .octagon output", value.Kind)
	}
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
