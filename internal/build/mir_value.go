package build

import (
	"fmt"
	"strconv"
	"strings"
)

// MIRValue is the backend-neutral representation of an ordinary computed value.
// Names and type identities remain strings; expression structure does not.
type MIRValue interface{ mirValue() }

type MIRLiteral struct {
	Type  string
	Value string
}

func (MIRLiteral) mirValue() {}

type MIRLocal struct {
	Name string
	Type string
}

func (MIRLocal) mirValue() {}

type MIRFunctionRef struct {
	Name string
	Type string
}

func (MIRFunctionRef) mirValue() {}

type MIRUnary struct {
	Op    string
	Value MIRValue
	Type  string
}

func (MIRUnary) mirValue() {}

type MIRBinary struct {
	Op          string
	Left, Right MIRValue
	Type        string
}

func (MIRBinary) mirValue() {}

type MIRConvert struct {
	TargetType string
	Value      MIRValue
}

func (MIRConvert) mirValue() {}

type MIRIndex struct {
	Target, Index MIRValue
	Type          string
}

func (MIRIndex) mirValue() {}

type MIRFieldAccess struct {
	Target      MIRValue
	Field, Type string
}

func (MIRFieldAccess) mirValue() {}

type MIRClone struct {
	Value MIRValue
	Type  string
}

func (MIRClone) mirValue() {}

type MIRRangeValue struct {
	Start, End, Step          MIRValue
	HasStart, HasEnd, HasStep bool
}

func (MIRRangeValue) mirValue() {}

type MIRArrayConvert struct {
	Value                  MIRValue
	SourceType, TargetType string
}

func (MIRArrayConvert) mirValue() {}

type MIRLength struct{ Value MIRValue }

func (MIRLength) mirValue() {}

type MIRMatrixColumnCount struct{ Value MIRValue }

func (MIRMatrixColumnCount) mirValue() {}

type MIRResultValue struct {
	ResultType string
	Value      MIRValue
	Error      MIRValue
	IsError    bool
}

func (MIRResultValue) mirValue() {}

type MIREnumValue struct {
	EnumType, Variant string
	Payload           MIRValue
}

func (MIREnumValue) mirValue() {}

type MIREnumPayload struct {
	Value       MIRValue
	PayloadType string
}

func (MIREnumPayload) mirValue() {}

// MIRIntrinsicValue names a bounded Oct semantic operation whose implementation
// is supplied by a backend (for example Euclidean modulo or checked table access).
// Kind is never an emitted helper/function name.
type MIRIntrinsicValue struct {
	Kind, Type string
	Args       []MIRValue
	Metadata   []string
}

func (MIRIntrinsicValue) mirValue() {}

// MIRBackendValue is the intentionally isolated compatibility seam for the few
// existing closure/table expressions not yet expressible by the bounded value
// model. Ordinary scalar, aggregate, access, conversion, and runtime-helper
// values must never use it.
type MIRBackendValue struct{ Backend, Expression, Type, Reason string }

func (MIRBackendValue) mirValue() {}

func mirLocal(name, typ string) MIRValue { return MIRLocal{Name: name, Type: typ} }
func mirBool(value bool) MIRValue        { return MIRLiteral{Type: "Bool", Value: strconv.FormatBool(value)} }
func mirInt(value string) MIRValue       { return MIRLiteral{Type: "Int", Value: value} }
func mirString(value string) MIRValue    { return MIRLiteral{Type: "String", Value: value} }

func mirValueName(value MIRValue) (string, bool) {
	switch v := value.(type) {
	case MIRLocal:
		return v.Name, true
	case MIRFunctionRef:
		return v.Name, true
	default:
		return "", false
	}
}

func mirValueContains(value MIRValue, legacyNeedle string) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case MIRClone:
		return legacyNeedle == "__octClone(" || mirValueContains(v.Value, legacyNeedle)
	case MIRRangeValue:
		return legacyNeedle == "__octRange{" || mirValueContains(v.Start, legacyNeedle) || mirValueContains(v.End, legacyNeedle) || mirValueContains(v.Step, legacyNeedle)
	case MIRArrayConvert:
		return legacyNeedle == "__octIntArrayToFloat(" || mirValueContains(v.Value, legacyNeedle)
	case MIRUnary:
		return mirValueContains(v.Value, legacyNeedle)
	case MIRBinary:
		return mirValueContains(v.Left, legacyNeedle) || mirValueContains(v.Right, legacyNeedle)
	case MIRConvert:
		return mirValueContains(v.Value, legacyNeedle)
	case MIRIndex:
		return mirValueContains(v.Target, legacyNeedle) || mirValueContains(v.Index, legacyNeedle)
	case MIRFieldAccess:
		return mirValueContains(v.Target, legacyNeedle)
	case MIRLength:
		return mirValueContains(v.Value, legacyNeedle)
	case MIRMatrixColumnCount:
		return mirValueContains(v.Value, legacyNeedle)
	case MIRResultValue:
		return mirValueContains(v.Value, legacyNeedle) || mirValueContains(v.Error, legacyNeedle)
	case MIREnumValue:
		return mirValueContains(v.Payload, legacyNeedle)
	case MIREnumPayload:
		return mirValueContains(v.Value, legacyNeedle)
	case MIRIntrinsicValue:
		for _, arg := range v.Args {
			if mirValueContains(arg, legacyNeedle) {
				return true
			}
		}
		return false
	case MIRBackendValue:
		return strings.Contains(v.Expression, legacyNeedle)
	default:
		return false
	}
}

func rewriteMIRLocal(value MIRValue, from, to string) MIRValue {
	if value == nil {
		return nil
	}
	rewriteType := func(t string) string { return strings.ReplaceAll(t, from, to) }
	switch v := value.(type) {
	case MIRLocal:
		if v.Name == from {
			v.Name = to
		}
		v.Type = rewriteType(v.Type)
		return v
	case MIRFunctionRef:
		if v.Name == from {
			v.Name = to
		}
		v.Type = rewriteType(v.Type)
		return v
	case MIRUnary:
		v.Value = rewriteMIRLocal(v.Value, from, to)
		v.Type = rewriteType(v.Type)
		return v
	case MIRBinary:
		v.Left = rewriteMIRLocal(v.Left, from, to)
		v.Right = rewriteMIRLocal(v.Right, from, to)
		v.Type = rewriteType(v.Type)
		return v
	case MIRConvert:
		v.Value = rewriteMIRLocal(v.Value, from, to)
		v.TargetType = rewriteType(v.TargetType)
		return v
	case MIRIndex:
		v.Target = rewriteMIRLocal(v.Target, from, to)
		v.Index = rewriteMIRLocal(v.Index, from, to)
		v.Type = rewriteType(v.Type)
		return v
	case MIRFieldAccess:
		v.Target = rewriteMIRLocal(v.Target, from, to)
		v.Type = rewriteType(v.Type)
		return v
	case MIRClone:
		v.Value = rewriteMIRLocal(v.Value, from, to)
		v.Type = rewriteType(v.Type)
		return v
	case MIRRangeValue:
		v.Start = rewriteMIRLocal(v.Start, from, to)
		v.End = rewriteMIRLocal(v.End, from, to)
		v.Step = rewriteMIRLocal(v.Step, from, to)
		return v
	case MIRArrayConvert:
		v.Value = rewriteMIRLocal(v.Value, from, to)
		v.SourceType = rewriteType(v.SourceType)
		v.TargetType = rewriteType(v.TargetType)
		return v
	case MIRLength:
		v.Value = rewriteMIRLocal(v.Value, from, to)
		return v
	case MIRMatrixColumnCount:
		v.Value = rewriteMIRLocal(v.Value, from, to)
		return v
	case MIRResultValue:
		v.Value = rewriteMIRLocal(v.Value, from, to)
		v.Error = rewriteMIRLocal(v.Error, from, to)
		v.ResultType = rewriteType(v.ResultType)
		return v
	case MIREnumValue:
		v.Payload = rewriteMIRLocal(v.Payload, from, to)
		return v
	case MIREnumPayload:
		v.Value = rewriteMIRLocal(v.Value, from, to)
		return v
	case MIRIntrinsicValue:
		for i := range v.Args {
			v.Args[i] = rewriteMIRLocal(v.Args[i], from, to)
		}
		return v
	case MIRBackendValue:
		v.Expression = strings.ReplaceAll(v.Expression, from, to)
		return v
	default:
		return value
	}
}

func dumpMIRValue(value MIRValue) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case MIRLiteral:
		if v.Type == "String" {
			return strconv.Quote(v.Value)
		}
		return v.Value
	case MIRLocal:
		return v.Name
	case MIRFunctionRef:
		return "function(" + v.Name + ")"
	case MIRUnary:
		return "(" + v.Op + dumpMIRValue(v.Value) + ")"
	case MIRBinary:
		return "(" + dumpMIRValue(v.Left) + " " + v.Op + " " + dumpMIRValue(v.Right) + ")"
	case MIRConvert:
		return "convert<" + v.TargetType + ">(" + dumpMIRValue(v.Value) + ")"
	case MIRIndex:
		return dumpMIRValue(v.Target) + "[" + dumpMIRValue(v.Index) + "]"
	case MIRFieldAccess:
		return dumpMIRValue(v.Target) + "." + v.Field
	case MIRClone:
		return "clone(" + dumpMIRValue(v.Value) + ")"
	case MIRRangeValue:
		return fmt.Sprintf("range(start=%s,has_start=%t,end=%s,has_end=%t,step=%s,has_step=%t)", dumpMIRValue(v.Start), v.HasStart, dumpMIRValue(v.End), v.HasEnd, dumpMIRValue(v.Step), v.HasStep)
	case MIRArrayConvert:
		return "array_convert<" + v.TargetType + ">(" + dumpMIRValue(v.Value) + ")"
	case MIRLength:
		return "length(" + dumpMIRValue(v.Value) + ")"
	case MIRMatrixColumnCount:
		return "matrix_columns(" + dumpMIRValue(v.Value) + ")"
	case MIRResultValue:
		if v.IsError {
			return "error<" + v.ResultType + ">(" + dumpMIRValue(v.Error) + ")"
		}
		return "ok<" + v.ResultType + ">(" + dumpMIRValue(v.Value) + ")"
	case MIREnumValue:
		if v.Payload == nil {
			return v.EnumType + "." + v.Variant
		}
		return v.EnumType + "." + v.Variant + "(" + dumpMIRValue(v.Payload) + ")"
	case MIREnumPayload:
		return "enum_payload<" + v.PayloadType + ">(" + dumpMIRValue(v.Value) + ")"
	case MIRIntrinsicValue:
		args := make([]string, len(v.Args))
		for i := range v.Args {
			args[i] = dumpMIRValue(v.Args[i])
		}
		return v.Kind + "(" + strings.Join(args, ", ") + ")"
	case MIRBackendValue:
		return fmt.Sprintf("backend[%s:%s]", v.Backend, v.Reason)
	default:
		return fmt.Sprintf("unsupported-value(%T)", value)
	}
}
