package build

import (
	"fmt"
	"strconv"
	"strings"
)

func emitGoValues(values []MIRValue) ([]string, error) {
	emitted := make([]string, len(values))
	for i, value := range values {
		var err error
		emitted[i], err = emitGoValue(value)
		if err != nil {
			return nil, err
		}
	}
	return emitted, nil
}

func emitGoValue(value MIRValue) (string, error) {
	if value == nil {
		return "", nil
	}
	switch v := value.(type) {
	case MIRLiteral:
		if v.Type == "String" {
			return strconv.Quote(v.Value), nil
		}
		return v.Value, nil
	case MIRLocal:
		return v.Name, nil
	case MIRFunctionRef:
		return v.Name, nil
	case MIRUnary:
		value, err := emitGoValue(v.Value)
		if err != nil {
			return "", err
		}
		return "(" + v.Op + value + ")", nil
	case MIRBinary:
		left, err := emitGoValue(v.Left)
		if err != nil {
			return "", err
		}
		right, err := emitGoValue(v.Right)
		if err != nil {
			return "", err
		}
		return "(" + left + " " + v.Op + " " + right + ")", nil
	case MIRConvert:
		inner, err := emitGoValue(v.Value)
		if err != nil {
			return "", err
		}
		if v.TargetType == "Complex" {
			return "complex(float64(" + inner + "), 0)", nil
		}
		return goType(v.TargetType) + "(" + inner + ")", nil
	case MIRIndex:
		target, err := emitGoValue(v.Target)
		if err != nil {
			return "", err
		}
		index, err := emitGoValue(v.Index)
		if err != nil {
			return "", err
		}
		return target + "[" + index + "]", nil
	case MIRFieldAccess:
		target, err := emitGoValue(v.Target)
		if err != nil {
			return "", err
		}
		return target + "." + v.Field, nil
	case MIRClone:
		inner, err := emitGoValue(v.Value)
		if err != nil {
			return "", err
		}
		return "__octClone(" + inner + ")", nil
	case MIRRangeValue:
		start, err := emitGoValue(v.Start)
		if err != nil {
			return "", err
		}
		end, err := emitGoValue(v.End)
		if err != nil {
			return "", err
		}
		step, err := emitGoValue(v.Step)
		if err != nil {
			return "", err
		}
		parts := []string{"Step: " + step}
		if v.HasStart {
			parts = append(parts, "Start: "+start, "HasStart: true")
		}
		if v.HasEnd {
			parts = append(parts, "End: "+end, "HasEnd: true")
		}
		if v.HasStep {
			parts = append(parts, "HasStep: true")
		}
		return "__octRange{" + strings.Join(parts, ", ") + "}", nil
	case MIRArrayConvert:
		inner, err := emitGoValue(v.Value)
		if err != nil {
			return "", err
		}
		if v.SourceType == "Int[]" && v.TargetType == "Float[]" {
			return "__octIntArrayToFloat(" + inner + ")", nil
		}
		return "", fmt.Errorf("unsupported array conversion %s -> %s", v.SourceType, v.TargetType)
	case MIRLength:
		inner, err := emitGoValue(v.Value)
		if err != nil {
			return "", err
		}
		return "len(" + inner + ")", nil
	case MIRMatrixColumnCount:
		inner, err := emitGoValue(v.Value)
		if err != nil {
			return "", err
		}
		return "func() int { if len(" + inner + ") == 0 { return 0 }; return len(" + inner + "[0]) }()", nil
	case MIRResultValue:
		name := goResultTypeName(v.ResultType)
		if v.IsError {
			e, err := emitGoValue(v.Error)
			if err != nil {
				return "", err
			}
			return name + "{Err: " + e + ", IsErr: true}", nil
		}
		if v.Value == nil {
			return name + "{Value: __octVoid{}}", nil
		}
		x, err := emitGoValue(v.Value)
		if err != nil {
			return "", err
		}
		return name + "{Value: " + x + "}", nil
	case MIREnumValue:
		parts := strings.Split(v.EnumType, ".")
		pkg, name := "", parts[len(parts)-1]
		if len(parts) > 1 {
			pkg = parts[0] + "_"
		}
		out := pkg + name + "{Tag: " + name + "_" + v.Variant + "_tag"
		if v.Payload != nil {
			payload, err := emitGoValue(v.Payload)
			if err != nil {
				return "", err
			}
			out += ", Payload: " + payload
		}
		return out + "}", nil
	case MIREnumPayload:
		x, err := emitGoValue(v.Value)
		if err != nil {
			return "", err
		}
		return x + ".Payload.(" + goType(v.PayloadType) + ")", nil
	case MIRIntrinsicValue:
		args, err := emitGoValues(v.Args)
		if err != nil {
			return "", err
		}
		switch v.Kind {
		case "euclidean-modulo":
			return "func(__a int, __b int) int { if __b == 0 { panic(\"runtime error: modulo by zero\") }; __r := __a % __b; if __r < 0 { if __b > 0 { __r += __b } else { __r -= __b } }; return __r }(" + strings.Join(args, ", ") + ")", nil
		case "safe-divide":
			gt := goType(v.Type)
			return "func(__a " + gt + ", __b " + gt + ") " + gt + " { return __a / __b }(" + strings.Join(args, ", ") + ")", nil
		case "stringify":
			return "fmt.Sprint(" + args[0] + ")", nil
		case "enum-tag":
			return args[0] + ".Tag", nil
		case "enum-is":
			if len(v.Metadata) != 2 {
				return "", fmt.Errorf("enum-is requires enum type and variant")
			}
			return "(" + args[0] + ".Tag == " + enumShortName(v.Metadata[0]) + "_" + v.Metadata[1] + "_tag)", nil
		default:
			return "", fmt.Errorf("unsupported MIR intrinsic %q", v.Kind)
		}
	case MIRBackendValue:
		if v.Backend != "go" {
			return "", fmt.Errorf("unsupported backend value %q", v.Backend)
		}
		return v.Expression, nil
	default:
		return "", fmt.Errorf("unsupported MIR value %T", value)
	}
}
