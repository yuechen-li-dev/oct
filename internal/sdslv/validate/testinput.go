package validate

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const TestInputABIVersion uint32 = 1

type TestInputValueKind string

const (
	TestInputKindNone  TestInputValueKind = "none"
	TestInputKindBool  TestInputValueKind = "bool"
	TestInputKindInt   TestInputValueKind = "int"
	TestInputKindUInt  TestInputValueKind = "uint"
	TestInputKindFloat TestInputValueKind = "float"
)

type ValidatedTestInput struct {
	ABIVersion    uint32             `json:"abi_version"`
	Kind          TestInputValueKind `json:"kind"`
	ElementCount  uint32             `json:"element_count"`
	PayloadWords  []uint32           `json:"payload_words"`
	AttributeSpan source.Span        `json:"-"`
	ValueSpans    []source.Span      `json:"-"`
}

func NoTestInput() ValidatedTestInput {
	return ValidatedTestInput{ABIVersion: TestInputABIVersion, Kind: TestInputKindNone}
}

func testInputAttributeKind(name string) (TestInputValueKind, bool) {
	switch name {
	case "TestInputBool":
		return TestInputKindBool, true
	case "TestInputInt":
		return TestInputKindInt, true
	case "TestInputUInt":
		return TestInputKindUInt, true
	case "TestInputFloat":
		return TestInputKindFloat, true
	default:
		return "", false
	}
}

func testInputMemberKind(name string) (TestInputValueKind, bool) {
	switch name {
	case "Bool":
		return TestInputKindBool, true
	case "Int":
		return TestInputKindInt, true
	case "UInt":
		return TestInputKindUInt, true
	case "Float":
		return TestInputKindFloat, true
	default:
		return "", false
	}
}

func validatedTestInputFromAttribute(attr ast.Attribute) (ValidatedTestInput, error) {
	kind, ok := testInputAttributeKind(attr.Name)
	if !ok {
		return NoTestInput(), fmt.Errorf("unsupported TestInput attribute %s", attr.Name)
	}
	input := ValidatedTestInput{
		ABIVersion:    TestInputABIVersion,
		Kind:          kind,
		AttributeSpan: attr.Span,
		ElementCount:  uint32(len(attr.Arguments)),
		PayloadWords:  make([]uint32, 0, len(attr.Arguments)),
		ValueSpans:    make([]source.Span, 0, len(attr.Arguments)),
	}
	for _, arg := range attr.Arguments {
		word, err := encodeTestInputWord(kind, arg)
		if err != nil {
			return NoTestInput(), err
		}
		input.PayloadWords = append(input.PayloadWords, word)
		input.ValueSpans = append(input.ValueSpans, ast.ExprSpan(arg))
	}
	return input, nil
}

func encodeTestInputWord(kind TestInputValueKind, expr ast.Expr) (uint32, error) {
	switch kind {
	case TestInputKindBool:
		value, ok := constBoolExpr(expr)
		if !ok {
			return 0, fmt.Errorf("bool")
		}
		if value {
			return 1, nil
		}
		return 0, nil
	case TestInputKindInt:
		value, ok := constI32Expr(expr)
		if !ok {
			return 0, fmt.Errorf("int")
		}
		return uint32(value), nil
	case TestInputKindUInt:
		value, ok := constU32Expr(expr)
		if !ok {
			return 0, fmt.Errorf("uint")
		}
		return value, nil
	case TestInputKindFloat:
		value, ok := constF32Expr(expr)
		if !ok {
			return 0, fmt.Errorf("float")
		}
		return math.Float32bits(value), nil
	default:
		return 0, fmt.Errorf("unsupported TestInput value kind %s", kind)
	}
}

func constBoolExpr(expr ast.Expr) (bool, bool) {
	switch e := expr.(type) {
	case ast.BoolLiteral:
		return e.Value, true
	case ast.ParenExpr:
		return constBoolExpr(e.Inner)
	default:
		return false, false
	}
}

func constI32Expr(expr ast.Expr) (int32, bool) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		if strings.HasSuffix(e.Value, "u") || strings.HasSuffix(e.Value, "U") {
			return 0, false
		}
		value, err := strconv.ParseInt(e.Value, 10, 32)
		if err != nil {
			return 0, false
		}
		return int32(value), true
	case ast.UnaryExpr:
		if e.Operator != "-" {
			return 0, false
		}
		value, ok := constI32Expr(e.Operand)
		if ok {
			return -value, true
		}
		if lit, ok := e.Operand.(ast.IntegerLiteral); ok && !strings.HasSuffix(lit.Value, "u") && !strings.HasSuffix(lit.Value, "U") {
			raw, err := strconv.ParseInt(lit.Value, 10, 32)
			if err == nil {
				return int32(-raw), true
			}
		}
		return 0, false
	case ast.ParenExpr:
		return constI32Expr(e.Inner)
	default:
		return 0, false
	}
}

func constU32Expr(expr ast.Expr) (uint32, bool) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		if !strings.HasSuffix(e.Value, "u") && !strings.HasSuffix(e.Value, "U") {
			return 0, false
		}
		value, err := strconv.ParseUint(strings.TrimRight(e.Value, "uU"), 10, 32)
		if err != nil {
			return 0, false
		}
		return uint32(value), true
	case ast.ParenExpr:
		return constU32Expr(e.Inner)
	default:
		return 0, false
	}
}

func constF32Expr(expr ast.Expr) (float32, bool) {
	switch e := expr.(type) {
	case ast.FloatLiteral:
		value, err := strconv.ParseFloat(e.Value, 32)
		if err != nil {
			return 0, false
		}
		return float32(value), true
	case ast.UnaryExpr:
		if e.Operator != "-" {
			return 0, false
		}
		value, ok := constF32Expr(e.Operand)
		if !ok {
			return 0, false
		}
		return -value, true
	case ast.ParenExpr:
		return constF32Expr(e.Inner)
	default:
		return 0, false
	}
}
