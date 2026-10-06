package octjson

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Admit answers whether a value meets the requirements of a refined concept:
// with "" when it does, and otherwise with what the concept says is wrong.
// The requirements are Oct expressions, so the lane that runs the program
// answers. A decode asks as it reads each value of a refined type, which is
// how the refusal comes with the place of the value in the document.
type Admit func(concept string, value Data) string

// Decode reads a document as the type a schema describes. The schema decides
// every reading: nothing here looks at the document to choose between two.
//
// There is no conversion between kinds. `"42"` is not an Int, `1` is not a
// Bool and `1.0` is not an Int. A number with no fraction and no exponent is
// a Float where a Float is declared.
//
// admit may be nil, and then no value of a refined concept is checked.
func Decode(doc *Document, schema *Schema, admit Admit) (Data, *Error) {
	d := &decoder{doc: doc, admit: admit}
	return d.value(doc.Root, schema, rootPath, "")
}

type decoder struct {
	doc   *Document
	admit Admit
}

func (d *decoder) fail(offset int, at *path, format string, arguments ...any) *Error {
	line, column := d.doc.Position(offset)
	return &Error{Path: at.String(), Line: line, Column: column, Message: fmt.Sprintf(format, arguments...)}
}

// mismatch is the error for a value of the wrong kind.
func (d *decoder) mismatch(node *Node, at *path, expects string) *Error {
	return d.fail(node.Offset, at, "expected %s, found %s", expects, node.Kind.kindName())
}

// value reads node as s, and has the value admitted when s is a refined
// concept. expects overrides what a mismatch says was wanted; an option
// passes `String or null` down to the reading of its payload.
func (d *decoder) value(node *Node, s *Schema, at *path, expects string) (Data, *Error) {
	value, err := d.read(node, s, at, expects)
	if err != nil {
		return Data{}, err
	}
	return value, d.admitted(value, s, node.Offset, at)
}

// admitted is the refusal of a value by the refined concept s names, or nil.
func (d *decoder) admitted(value Data, s *Schema, offset int, at *path) *Error {
	if s.Concept == "" || d.admit == nil {
		return nil
	}
	if refusal := d.admit(s.Concept, value); refusal != "" {
		return d.fail(offset, at, "%s", refusal)
	}
	return nil
}

func (d *decoder) read(node *Node, s *Schema, at *path, expects string) (Data, *Error) {
	if expects == "" {
		expects = s.expects()
	}
	switch s.Kind {
	case KindBool:
		if node.Kind != NodeBool {
			return Data{}, d.mismatch(node, at, expects)
		}
		return Data{Kind: DataBool, Bool: node.Bool}, nil
	case KindInt:
		return d.integer(node, s, at, expects)
	case KindFloat:
		return d.float(node, s, at, expects)
	case KindString:
		if node.Kind != NodeString {
			return Data{}, d.mismatch(node, at, expects)
		}
		return Data{Kind: DataString, Text: node.Text}, nil
	case KindEnum:
		if node.Kind != NodeString {
			return Data{}, d.mismatch(node, at, expects)
		}
		folded := FoldName(node.Text)
		for _, variant := range s.Variants {
			if FoldName(variant) == folded {
				return Data{Kind: DataEnum, EnumType: s.Name, Variant: variant}, nil
			}
		}
		return Data{}, d.fail(node.Offset, at, "expected %s, found %s", expects, quoteString(clip(node.Text)))
	case KindOption:
		if node.Kind == NodeNull {
			return none(), nil
		}
		payload, err := d.value(node, s.Elem, at, expects)
		if err != nil {
			return Data{}, err
		}
		return some(payload), nil
	case KindRecord:
		if node.Kind != NodeObject {
			return Data{}, d.mismatch(node, at, expects)
		}
		values, err := d.members(node, s.Name, s.Fields, at)
		if err != nil {
			return Data{}, err
		}
		record := Data{Kind: DataRecord, RecordType: s.Name, Fields: make([]DataField, len(s.Fields))}
		for index, field := range s.Fields {
			record.Fields[index] = DataField{Name: field.Name, Value: values[index]}
		}
		return record, nil
	case KindArray:
		if node.Kind != NodeArray {
			return Data{}, d.mismatch(node, at, expects)
		}
		return d.elements(node, s.Elem, at)
	case KindVector:
		if node.Kind != NodeArray {
			return Data{}, d.mismatch(node, at, expects)
		}
		return d.elements(node, s.Elem, at)
	case KindMatrix:
		return d.matrix(node, s, at, expects)
	case KindTable:
		return d.table(node, s, at, expects)
	default:
		return Data{}, d.fail(node.Offset, at, "%s has no JSON form", s.Name)
	}
}

func (d *decoder) elements(node *Node, element *Schema, at *path) (Data, *Error) {
	array := Data{Kind: DataArray, Array: make([]Data, len(node.Elements))}
	for index, item := range node.Elements {
		value, err := d.value(item, element, at.element(index), "")
		if err != nil {
			return Data{}, err
		}
		array.Array[index] = value
	}
	return array, nil
}

// integer reads a number with no fraction and no exponent, in the 64-bit
// range.
func (d *decoder) integer(node *Node, s *Schema, at *path, expects string) (Data, *Error) {
	if node.Kind != NodeNumber {
		return Data{}, d.mismatch(node, at, expects)
	}
	switch {
	case strings.Contains(node.Text, "."):
		return Data{}, d.fail(node.Offset, at, "expected %s, found %s, which has a fraction", expects, clip(node.Text))
	case strings.ContainsAny(node.Text, "eE"):
		return Data{}, d.fail(node.Offset, at, "expected %s, found %s, which has an exponent", expects, clip(node.Text))
	}
	value, err := strconv.ParseInt(node.Text, 10, 64)
	if err != nil {
		return Data{}, d.fail(node.Offset, at, "expected %s, found %s, which is outside the 64-bit range", expects, clip(node.Text))
	}
	return Data{Kind: DataInt, Int: value, Dimension: s.Dimension}, nil
}

// float reads any number that is finite as a 64-bit float.
func (d *decoder) float(node *Node, s *Schema, at *path, expects string) (Data, *Error) {
	if node.Kind != NodeNumber {
		return Data{}, d.mismatch(node, at, expects)
	}
	value, err := strconv.ParseFloat(node.Text, 64)
	if err != nil {
		return Data{}, d.fail(node.Offset, at, "expected %s, found %s, which is not finite as a 64-bit float", expects, clip(node.Text))
	}
	return Data{Kind: DataFloat, Float: value, Dimension: s.Dimension}, nil
}

// matrix reads an array of rows of numbers, all one length.
func (d *decoder) matrix(node *Node, s *Schema, at *path, expects string) (Data, *Error) {
	if node.Kind != NodeArray {
		return Data{}, d.mismatch(node, at, expects)
	}
	rows := Data{Kind: DataArray, Array: make([]Data, len(node.Elements))}
	for index, rowNode := range node.Elements {
		rowAt := at.element(index)
		if rowNode.Kind != NodeArray {
			return Data{}, d.mismatch(rowNode, rowAt, "an array of numbers, a row of the matrix")
		}
		if width := len(node.Elements[0].Elements); len(rowNode.Elements) != width {
			return Data{}, d.fail(rowNode.Offset, rowAt, "this row has %d numbers and the first row has %d; the rows of a matrix are one length", len(rowNode.Elements), width)
		}
		row, err := d.elements(rowNode, s.Elem, rowAt)
		if err != nil {
			return Data{}, err
		}
		rows.Array[index] = row
	}
	return rows, nil
}

// members reads an object as the fields of a record, or of one row of a
// table, and answers with a value for each field in declaration order.
//
// A key and a field match when their folded names are equal. Every field
// needs a member, except an option, which is None without one. A member
// that matches no field is an error, and so are two members for one field.
func (d *decoder) members(node *Node, typeName string, fields []Field, at *path) ([]Data, *Error) {
	byFold := make(map[string]int, len(fields))
	for index, field := range fields {
		byFold[FoldName(field.Name)] = index
	}
	matched := make([]*Member, len(fields))
	written := make(map[string]bool, len(node.Members))
	var unknown []string
	for index := range node.Members {
		member := &node.Members[index]
		if written[member.Key] {
			return nil, d.fail(member.KeyOffset, at, "the key %s is written twice", quoteString(member.Key))
		}
		written[member.Key] = true
		fieldIndex, known := byFold[FoldName(member.Key)]
		if !known {
			unknown = append(unknown, member.Key)
			continue
		}
		if earlier := matched[fieldIndex]; earlier != nil {
			return nil, d.fail(member.KeyOffset, at, "%s and %s both name the field %s", quoteString(earlier.Key), quoteString(member.Key), fields[fieldIndex].Name)
		}
		matched[fieldIndex] = member
	}
	if len(unknown) > 0 {
		names := make([]string, len(fields))
		for index, field := range fields {
			names[index] = field.Name
		}
		noun := "unknown member"
		if len(unknown) > 1 {
			noun = "unknown members"
		}
		return nil, d.fail(node.Offset, at, "%s %s; %s has %s", noun, quotedList(unknown), typeName, strings.Join(names, ", "))
	}
	var missing []string
	for index, field := range fields {
		if matched[index] == nil && field.Type.Kind != KindOption {
			missing = append(missing, field.Name)
		}
	}
	if len(missing) > 0 {
		return nil, d.fail(node.Offset, at, "missing %s", quotedList(missing))
	}
	values := make([]Data, len(fields))
	for index, field := range fields {
		member := matched[index]
		if member == nil {
			values[index] = none()
			continue
		}
		value, err := d.value(member.Value, field.Type, at.member(member.Key), "")
		if err != nil {
			return nil, err
		}
		values[index] = value
	}
	return values, nil
}

// table reads a record table from an array of row objects, or from a keyed
// object when its first column is a String. The Data is the table's Octagon
// literal: a record whose fields are whole columns.
func (d *decoder) table(node *Node, s *Schema, at *path, expects string) (Data, *Error) {
	columns := make([][]Data, len(s.Fields))
	appendRow := func(cells []Data, from int) {
		for index, cell := range cells {
			columns[from+index] = append(columns[from+index], cell)
		}
	}
	switch {
	case node.Kind == NodeArray:
		for index, rowNode := range node.Elements {
			rowAt := at.element(index)
			if rowNode.Kind != NodeObject {
				return Data{}, d.mismatch(rowNode, rowAt, "an object, a row of "+s.Name)
			}
			cells, err := d.members(rowNode, s.Name, s.Fields, rowAt)
			if err != nil {
				return Data{}, err
			}
			appendRow(cells, 0)
		}
	case node.Kind == NodeObject && s.keyed():
		rest := s.Fields[1:]
		// One other column takes the member's value as its cell, unless the
		// value is an object and that column is not itself read from one;
		// then the object holds the cell under its name, as it does for
		// several columns.
		single := len(rest) == 1
		written := make(map[string]bool, len(node.Members))
		for index := range node.Members {
			member := &node.Members[index]
			if written[member.Key] {
				return Data{}, d.fail(member.KeyOffset, at, "the key %s is written twice", quoteString(member.Key))
			}
			written[member.Key] = true
			rowAt := at.member(member.Key)
			key := Data{Kind: DataString, Text: member.Key}
			if err := d.admitted(key, s.Fields[0].Type, member.KeyOffset, rowAt); err != nil {
				return Data{}, err
			}
			columns[0] = append(columns[0], key)
			if single && (member.Value.Kind != NodeObject || rest[0].Type.readsFromObject()) {
				cell, err := d.value(member.Value, rest[0].Type, rowAt, "")
				if err != nil {
					return Data{}, err
				}
				appendRow([]Data{cell}, 1)
				continue
			}
			if member.Value.Kind != NodeObject {
				return Data{}, d.mismatch(member.Value, rowAt, "an object, a row of "+s.Name)
			}
			cells, err := d.members(member.Value, s.Name, rest, rowAt)
			if err != nil {
				return Data{}, err
			}
			appendRow(cells, 1)
		}
	default:
		return Data{}, d.mismatch(node, at, expects)
	}
	table := Data{Kind: DataRecord, RecordType: s.Name, Fields: make([]DataField, len(s.Fields))}
	for index, field := range s.Fields {
		column := columns[index]
		if column == nil {
			column = []Data{}
		}
		table.Fields[index] = DataField{Name: field.Name, Value: Data{Kind: DataArray, Array: column}}
	}
	return table, nil
}

// clip shortens a long piece of the document for a message.
func clip(text string) string {
	const limit = 40
	if len(text) <= limit {
		return text
	}
	cut := limit
	for !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "…"
}
