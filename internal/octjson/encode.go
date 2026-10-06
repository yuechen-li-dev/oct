package octjson

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Encode writes a value of the type a schema describes.
//
// A record is an object whose keys are the field names as declared, in
// declaration order. A record table is an array of row objects.
// `Option.None` is `null`, and its member is written. The text is UTF-8 with
// two-space indentation and one final newline; an array whose elements are
// scalars is on one line. A Float always has a fraction or an exponent, in
// the shortest form that reads back as the same value.
//
// For every representable value v of a schema s, decoding Encode(v, s) as s
// gives v.
func Encode(value Data, schema *Schema) ([]byte, *Error) {
	e := &encoder{}
	if err := e.value(value, schema, rootPath, 0); err != nil {
		return nil, err
	}
	e.out.WriteByte('\n')
	return e.out.Bytes(), nil
}

type encoder struct {
	out bytes.Buffer
}

// unwritable is the error for a value that JSON cannot hold.
func unwritable(at *path, format string, arguments ...any) *Error {
	return &Error{Path: at.String(), Message: fmt.Sprintf(format, arguments...), Unwritable: true}
}

// shape is the error for a value that is not what its schema says. A lane
// that hands over such a value has a defect; no program can cause it.
func shape(at *path, s *Schema, value Data) *Error {
	return &Error{Path: at.String(), Message: fmt.Sprintf("internal error: a value of kind %d was given for %s", value.Kind, s.expects())}
}

func (e *encoder) indent(depth int) {
	e.out.WriteByte('\n')
	for ; depth > 0; depth-- {
		e.out.WriteString("  ")
	}
}

func (e *encoder) value(value Data, s *Schema, at *path, depth int) *Error {
	switch s.Kind {
	case KindBool:
		if value.Kind != DataBool {
			return shape(at, s, value)
		}
		e.out.WriteString(strconv.FormatBool(value.Bool))
	case KindInt:
		if value.Kind != DataInt {
			return shape(at, s, value)
		}
		e.out.WriteString(strconv.FormatInt(value.Int, 10))
	case KindFloat:
		if value.Kind != DataFloat {
			return shape(at, s, value)
		}
		switch {
		case math.IsNaN(value.Float):
			return unwritable(at, "NaN has no JSON form")
		case math.IsInf(value.Float, 0):
			return unwritable(at, "an infinity has no JSON form")
		}
		e.out.WriteString(formatFloat(value.Float))
	case KindString:
		if value.Kind != DataString {
			return shape(at, s, value)
		}
		if !utf8.ValidString(value.Text) {
			return unwritable(at, "this String holds bytes that are not UTF-8")
		}
		e.out.WriteString(quoteString(value.Text))
	case KindEnum:
		if value.Kind != DataEnum || value.Payload != nil {
			return shape(at, s, value)
		}
		e.out.WriteString(quoteString(value.Variant))
	case KindOption:
		if value.Kind != DataEnum || value.EnumType != OptionType {
			return shape(at, s, value)
		}
		if value.Variant == OptionNone {
			e.out.WriteString("null")
			return nil
		}
		if value.Payload == nil {
			return shape(at, s, value)
		}
		return e.value(*value.Payload, s.Elem, at, depth)
	case KindRecord:
		if value.Kind != DataRecord {
			return shape(at, s, value)
		}
		cells := make([]Data, len(s.Fields))
		for index, field := range s.Fields {
			cell, found := value.Field(field.Name)
			if !found {
				return shape(at, s, value)
			}
			cells[index] = cell
		}
		return e.object(s.Fields, cells, at, depth)
	case KindArray, KindVector:
		if value.Kind != DataArray {
			return shape(at, s, value)
		}
		return e.array(value.Array, s.Elem, at, depth)
	case KindMatrix:
		if value.Kind != DataArray {
			return shape(at, s, value)
		}
		return e.array(value.Array, &Schema{Kind: KindVector, Elem: s.Elem}, at, depth)
	case KindTable:
		return e.table(value, s, at, depth)
	default:
		return &Error{Path: at.String(), Message: s.Name + " has no JSON form"}
	}
	return nil
}

// scalar reports whether every value of the schema is written without a
// line break. The layout of an array follows from its type, not from the
// values it happens to hold.
func (s *Schema) scalar() bool {
	switch s.Kind {
	case KindBool, KindInt, KindFloat, KindString, KindEnum:
		return true
	case KindOption:
		return s.Elem.scalar()
	default:
		return false
	}
}

func (e *encoder) array(elements []Data, element *Schema, at *path, depth int) *Error {
	if len(elements) == 0 {
		e.out.WriteString("[]")
		return nil
	}
	oneLine := element.scalar()
	e.out.WriteByte('[')
	for index, item := range elements {
		switch {
		case !oneLine:
			if index > 0 {
				e.out.WriteByte(',')
			}
			e.indent(depth + 1)
		case index > 0:
			e.out.WriteString(", ")
		}
		if err := e.value(item, element, at.element(index), depth+1); err != nil {
			return err
		}
	}
	if !oneLine {
		e.indent(depth)
	}
	e.out.WriteByte(']')
	return nil
}

func (e *encoder) object(fields []Field, cells []Data, at *path, depth int) *Error {
	if len(fields) == 0 {
		e.out.WriteString("{}")
		return nil
	}
	e.out.WriteByte('{')
	for index, field := range fields {
		if index > 0 {
			e.out.WriteByte(',')
		}
		e.indent(depth + 1)
		e.out.WriteString(quoteString(field.Name))
		e.out.WriteString(": ")
		if err := e.value(cells[index], field.Type, at.member(field.Name), depth+1); err != nil {
			return err
		}
	}
	e.indent(depth)
	e.out.WriteByte('}')
	return nil
}

// table writes the columns of a record table as an array of row objects.
func (e *encoder) table(value Data, s *Schema, at *path, depth int) *Error {
	if value.Kind != DataRecord {
		return shape(at, s, value)
	}
	columns := make([][]Data, len(s.Fields))
	rows := 0
	for index, field := range s.Fields {
		column, found := value.Field(field.Name)
		if !found || column.Kind != DataArray || (index > 0 && len(column.Array) != rows) {
			return shape(at, s, value)
		}
		columns[index], rows = column.Array, len(column.Array)
	}
	if rows == 0 {
		e.out.WriteString("[]")
		return nil
	}
	e.out.WriteByte('[')
	cells := make([]Data, len(s.Fields))
	for row := 0; row < rows; row++ {
		if row > 0 {
			e.out.WriteByte(',')
		}
		e.indent(depth + 1)
		for index := range s.Fields {
			cells[index] = columns[index][row]
		}
		if err := e.object(s.Fields, cells, at.element(row), depth+1); err != nil {
			return err
		}
	}
	e.indent(depth)
	e.out.WriteByte(']')
	return nil
}

// formatFloat is the shortest text that reads back as the same Float, with a
// fraction or an exponent so that it reads back as a Float and not an Int.
// Digits are written out between 1e-6 and 1e21, and an exponent is used
// outside that range, where the digits would be mostly zeros.
func formatFloat(value float64) string {
	if magnitude := math.Abs(value); magnitude != 0 && (magnitude < 1e-6 || magnitude >= 1e21) {
		return strconv.FormatFloat(value, 'e', -1, 64)
	}
	text := strconv.FormatFloat(value, 'f', -1, 64)
	if !strings.Contains(text, ".") {
		text += ".0"
	}
	return text
}

// quoteString writes a JSON string. It escapes `"`, `\` and the control
// characters below U+0020, and nothing else. A byte that is not UTF-8 is
// written as U+FFFD; Encode refuses such a String before it gets here, and a
// message that quotes a key has to print something.
func quoteString(text string) string {
	const hex = "0123456789abcdef"
	var quoted strings.Builder
	quoted.Grow(len(text) + 2)
	quoted.WriteByte('"')
	for _, r := range text {
		switch {
		case r == '"':
			quoted.WriteString(`\"`)
		case r == '\\':
			quoted.WriteString(`\\`)
		case r == '\b':
			quoted.WriteString(`\b`)
		case r == '\f':
			quoted.WriteString(`\f`)
		case r == '\n':
			quoted.WriteString(`\n`)
		case r == '\r':
			quoted.WriteString(`\r`)
		case r == '\t':
			quoted.WriteString(`\t`)
		case r < 0x20:
			quoted.WriteString(`\u00`)
			quoted.WriteByte(hex[r>>4])
			quoted.WriteByte(hex[r&0xF])
		default:
			quoted.WriteRune(r)
		}
	}
	quoted.WriteByte('"')
	return quoted.String()
}
