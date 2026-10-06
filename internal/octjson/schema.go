package octjson

import (
	"fmt"
	"strings"
	"unicode"
)

// Kind is what a Schema describes.
type Kind int

const (
	// KindUnsupported is a type with no JSON form. Name says which.
	KindUnsupported Kind = iota
	KindBool
	KindInt
	KindFloat
	KindString
	// KindEnum is an enum whose variants carry no payload.
	KindEnum
	// KindOption is Option<Elem>.
	KindOption
	KindRecord
	// KindArray is Elem[].
	KindArray
	// KindVector is Vector<Elem>, and KindMatrix is Matrix<Elem>.
	KindVector
	KindMatrix
	// KindTable is a record table. Fields are its columns, each with the
	// type of one cell.
	KindTable
)

// Schema describes an Oct type as far as JSON is concerned. A lane builds one
// from the type argument of a Json call. A type that names itself, such as a
// record with an array of itself, is a Schema that points back to itself.
type Schema struct {
	Kind Kind
	// Name is the type's name as a message should give it: the record, the
	// record table or the enum, and for KindUnsupported the type that has
	// no JSON form. It is also the type name of the Data a decode makes.
	Name string
	// Dimension is the dimension of an Int or a Float, as Octagon data
	// writes it, or "" for a plain number.
	Dimension string
	// Elem is the payload of an option and the element of an array, a
	// vector or a matrix.
	Elem *Schema
	// Fields are the fields of a record or the columns of a table, in
	// declaration order.
	Fields []Field
	// Variants are the variants of an enum, in declaration order.
	Variants []string
}

// Field is a field of a record or a column of a table.
type Field struct {
	Name string
	Type *Schema
}

// FoldName is the form in which a JSON key and a field name are compared:
// without `_`, `-`, `.` and spaces, and in lower case. `read_timeout_ms`,
// `readTimeoutMs` and `ReadTimeoutMs` fold to one name.
func FoldName(name string) string {
	var folded strings.Builder
	for _, r := range name {
		switch r {
		case '_', '-', '.', ' ':
		default:
			folded.WriteRune(unicode.ToLower(r))
		}
	}
	return folded.String()
}

// Check reports why a type is not JSON-representable, naming the part that
// is not, or nil when it is. A lane calls it when it compiles a Json call.
func Check(schema *Schema) error {
	return (&checker{done: map[*Schema]bool{}}).check(schema, "")
}

type checker struct {
	done map[*Schema]bool
}

func (c *checker) check(s *Schema, at string) error {
	if s == nil {
		return fmt.Errorf("internal error: a JSON schema is missing at %q", at)
	}
	if c.done[s] {
		return nil
	}
	c.done[s] = true
	prefix := ""
	if at != "" {
		prefix = at + ": "
	}
	switch s.Kind {
	case KindUnsupported:
		return fmt.Errorf("%s%s has no JSON form", prefix, s.Name)
	case KindBool, KindInt, KindFloat, KindString:
		return nil
	case KindEnum:
		seen := map[string]string{}
		for _, variant := range s.Variants {
			folded := FoldName(variant)
			if other, clash := seen[folded]; clash {
				return fmt.Errorf("%sthe variants %s and %s of %s are one name in JSON, where case and separators are ignored", prefix, other, variant, s.Name)
			}
			seen[folded] = variant
		}
		return nil
	case KindOption:
		if s.Elem != nil && s.Elem.Kind == KindOption {
			return fmt.Errorf("%san Option of an Option has no JSON form: `null` could be either None", prefix)
		}
		return c.check(s.Elem, at)
	case KindArray:
		return c.check(s.Elem, at)
	case KindVector, KindMatrix:
		if s.Elem == nil || (s.Elem.Kind != KindInt && s.Elem.Kind != KindFloat) {
			return fmt.Errorf("%sa vector or a matrix of anything but Int or Float has no JSON form", prefix)
		}
		return nil
	case KindRecord, KindTable:
		seen := map[string]string{}
		for _, field := range s.Fields {
			folded := FoldName(field.Name)
			if other, clash := seen[folded]; clash {
				return fmt.Errorf("%sthe fields %s and %s of %s are one name in JSON, where case and separators are ignored", prefix, other, field.Name, s.Name)
			}
			seen[folded] = field.Name
		}
		for _, field := range s.Fields {
			// A part of the type is named by its record and field.
			if err := c.check(field.Type, s.Name+"."+field.Name); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("internal error: unknown JSON schema kind %d", s.Kind)
	}
}

// expects is what a message says a schema wants: `String`, `Int or null`,
// `an object`.
func (s *Schema) expects() string {
	switch s.Kind {
	case KindBool:
		return "Bool"
	case KindInt:
		return "Int"
	case KindFloat:
		return "Float"
	case KindString:
		return "String"
	case KindEnum:
		return "one of " + quotedList(s.Variants)
	case KindOption:
		return s.Elem.expects() + " or null"
	case KindRecord:
		return "an object"
	case KindArray:
		return "an array"
	case KindVector:
		return "an array of numbers"
	case KindMatrix:
		return "an array of arrays of numbers"
	case KindTable:
		if s.keyed() {
			return "an array of objects or a keyed object"
		}
		return "an array of objects"
	default:
		return s.Name
	}
}

// keyed reports whether a table can be read from a keyed object: its first
// column is the key, so that column is a String.
func (s *Schema) keyed() bool {
	return s.Kind == KindTable && len(s.Fields) >= 2 && s.Fields[0].Type.Kind == KindString
}

// readsFromObject reports whether a value of the schema is written as a JSON
// object.
func (s *Schema) readsFromObject() bool {
	switch s.Kind {
	case KindRecord:
		return true
	case KindTable:
		return s.keyed()
	case KindOption:
		return s.Elem.readsFromObject()
	default:
		return false
	}
}
