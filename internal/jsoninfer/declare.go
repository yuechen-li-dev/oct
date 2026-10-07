package jsoninfer

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuechen-li-dev/oct/internal/octjson"
)

// Options says what to infer for.
type Options struct {
	// Source is the path of the document as the user gave it. It is printed
	// in the `Json.Load` line, and names the root declaration when Name is
	// empty.
	Source string
	// Name is the name of the declaration the whole document loads into.
	Name string
}

// Refusal is a value that has no declaration.
type Refusal struct {
	// Path is the place of the value, spelled as a Json error spells it.
	Path string
	// Line and Column are where the value begins.
	Line   int
	Column int
	Reason string

	node *octjson.Node
}

// Result is what was inferred from one document.
type Result struct {
	// Type is the type the whole document loads into: a declared name, or
	// an expression such as `Matrix<Float>`. It is "" when the document as
	// a whole has no declaration.
	Type string
	// Refusals are the values with no declaration, in document order. When
	// there are any, the declarations do not load the document.
	Refusals []Refusal
	// Decisions are the choices made, in document order.
	Decisions []Decision

	source       string
	declarations []*declaration
	// note is said of Type, when Type is not a declaration.
	note string
}

// declaration is one `record` or `record table`.
type declaration struct {
	name   string
	table  bool
	fields []field
}

// field is one line of a declaration.
type field struct {
	name string
	// expr is the type; "" for a field whose value has no declaration.
	expr string
	// note is a comment for the line.
	note string
}

// reserved are the names a declaration cannot take: the builtin types.
var reserved = map[string]bool{
	"Int": true, "Float": true, "Complex": true, "Bool": true, "String": true, "Bytes": true, "Error": true,
	"Void": true, "UI": true, "Index": true, "Range": true, "Option": true, "Vector": true, "Matrix": true,
}

// Infer proposes the declarations a document loads into.
func Infer(document *octjson.Document, options Options) (Result, error) {
	name := options.Name
	if name == "" {
		stem := filepath.Base(options.Source)
		stem = strings.TrimSuffix(stem, filepath.Ext(stem))
		name = fieldName(stem)
		if !isDeclarationName(name) {
			name = "Document"
		}
	} else if !isDeclarationName(name) {
		return Result{}, fmt.Errorf("--name %s is not a name a record can take: it must begin with a capital letter, hold only letters and digits, and not be a builtin type", strconv.Quote(name))
	}

	// The root's name is the one the user chose or the file gave: it is
	// kept for the root, whatever is declared on the way there.
	b := &builder{document: document, root: name, names: map[string]string{name: heldForRoot}}
	root := observe(document.Root, octjson.RootPath)
	expr, note, _ := b.typeOf(root, place{name: name, tableAllowed: true}, false)
	return Result{Type: expr, Refusals: b.refusals, Decisions: b.decisions, source: options.Source, declarations: b.declarations, note: note}, nil
}

func isDeclarationName(name string) bool {
	for index, r := range name {
		if index == 0 && !unicode.IsUpper(r) {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return name != "" && !reserved[name]
}

// builder collects declarations as shapes are given types.
type builder struct {
	document *octjson.Document
	// root is the name of the declaration the whole document loads into.
	root         string
	declarations []*declaration
	// names maps each declared name to the declaration's signature.
	names     map[string]string
	refusals  []Refusal
	decisions []Decision
}

// place is where a value sits, as far as its declaration depends on it.
type place struct {
	// name is the name to declare under, when it is given: the root's.
	name string
	// key is the member the value is under; an element has its array's.
	key string
	// parent is the declaration the value is a field of.
	parent string
	// tableAllowed: a `record table` may be declared here. The cell of a
	// table cannot be a table, and neither can the element of an array.
	tableAllowed bool
}

func (b *builder) refuse(refusal *Refusal) {
	line, column := b.document.Position(refusal.node.Offset)
	b.refusals = append(b.refusals, Refusal{Path: refusal.Path, Line: line, Column: column, Reason: refusal.Reason})
}

// Placeholder notes: the document has no value that says what the type is.
const (
	noteAlwaysNull = "null everywhere in this document; String is a placeholder"
	noteEmpty      = "empty everywhere in this document; String is a placeholder"
)

// typeOf gives a shape its type, declaring what it needs. absent says the
// value is a member some objects lack. ok is false when the value has no
// declaration; the refusal is recorded.
func (b *builder) typeOf(s *shape, at place, absent bool) (expr string, note string, ok bool) {
	optional := s.optional || absent
	switch s.kind {
	case kindRefused:
		b.refuse(s.refusal)
		return "", "", false
	case kindUnknown:
		// Only null was seen: an empty array's element is handled below.
		return "Option<String>", noteAlwaysNull, true
	case kindBool:
		expr = "Bool"
	case kindInt:
		expr = "Int"
	case kindFloat:
		expr = "Float"
	case kindString:
		expr, note = "String", enumNote(s.text)
	case kindArray:
		expr, note, ok = b.arrayType(s, at)
		if !ok {
			return "", "", false
		}
	case kindObject:
		expr, ok = b.objectType(s, at)
		if !ok {
			return "", "", false
		}
	}
	if optional {
		expr = "Option<" + expr + ">"
	}
	return expr, note, true
}

// arrayType gives an array its type: a matrix, a table, or an array of its
// element's type.
func (b *builder) arrayType(s *shape, at place) (expr string, note string, ok bool) {
	element := s.elem
	switch {
	case s.rectangular:
		// Rows of numbers, all one length (3.11).
		return "Matrix<Float>", "", true
	case element.kind == kindUnknown && !element.optional:
		return "String[]", noteEmpty, true
	case element.kind != kindObject:
		inner, innerNote, ok := b.typeOf(element, place{key: at.key, parent: at.parent}, false)
		if !ok {
			return "", "", false
		}
		return inner + "[]", innerNote, true
	}

	if len(s.rows) >= 2 {
		choice := decideRows(s.rows)
		b.decisions = append(b.decisions, Decision{Path: s.path, Trace: choice.trace})
		if choice.trace.Winner == asTagged {
			b.refuse(&Refusal{Path: s.path, node: s.node, Reason: fmt.Sprintf("a tagged array: %s says which members an object has (%s), and Json reads no enum that carries a payload", strconv.Quote(choice.tag), strings.Join(choice.values, ", "))})
			return "", "", false
		}
	}
	if at.tableAllowed && !element.optional {
		// An array of objects is a table (3.11).
		name, ok := b.declare(element.object, at, true, element)
		return name, "", ok
	}
	name, ok := b.declare(element.object, at, false, element)
	if !ok {
		return "", "", false
	}
	if element.optional {
		name = "Option<" + name + ">"
	}
	return name + "[]", "", true
}

// objectType gives an object its type: a record, or a keyed table where one
// may be declared and the judgment chooses it.
func (b *builder) objectType(s *shape, at place) (expr string, ok bool) {
	if !at.tableAllowed {
		return b.declare(s.object, at, false, s)
	}
	choice, decided, reason := decideObject(s.object)
	if !decided {
		b.refuse(&Refusal{Path: s.path, node: s.node, Reason: reason})
		return "", false
	}
	b.decisions = append(b.decisions, Decision{Path: s.path, Trace: choice.trace})
	if choice.trace.Winner == asKeyedTable {
		return b.declareKeyed(choice.value, at)
	}
	return b.declare(s.object, at, false, s)
}

// declare declares a record, or a table whose rows are the record, and
// returns its name.
func (b *builder) declare(object *objectShape, at place, table bool, s *shape) (name string, ok bool) {
	names, problem := fieldNames(object)
	if problem != "" {
		b.refuse(&Refusal{Path: s.path, node: s.node, Reason: "an object that cannot be a record: " + problem})
		return "", false
	}
	if table && len(object.members) == 0 {
		b.refuse(&Refusal{Path: s.path, node: s.node, Reason: "objects with no members: a table needs a column"})
		return "", false
	}
	candidate := b.candidateName(at)
	d := &declaration{table: table}
	for index, existing := range object.members {
		expr, note, _ := b.typeOf(existing.shape, place{key: existing.key, parent: candidate, tableAllowed: !table}, existing.count < object.instances)
		d.fields = append(d.fields, field{name: names[index], expr: expr, note: note})
	}
	return b.add(d, candidate, at), true
}

// declareKeyed declares the table an object reads into when its keys are
// data: the key is the first cell, and the member's value supplies the rest
// (section 3.5 of the ladder).
func (b *builder) declareKeyed(value *shape, at place) (name string, ok bool) {
	candidate := b.candidateName(at)
	d := &declaration{table: true}
	spread := false
	if value.kind == kindObject && !value.optional && len(value.object.members) >= 2 {
		_, problem := fieldNames(value.object)
		spread = problem == ""
	}
	if spread {
		// The value is an object whose members are the other cells.
		names, _ := fieldNames(value.object)
		key := "Key"
		for _, taken := range names {
			if octjson.FoldName(taken) == "key" {
				key = "RowKey"
			}
		}
		d.fields = append(d.fields, field{name: key, expr: "String"})
		for index, existing := range value.object.members {
			expr, note, _ := b.typeOf(existing.shape, place{key: existing.key, parent: candidate}, existing.count < value.object.instances)
			d.fields = append(d.fields, field{name: names[index], expr: expr, note: note})
		}
	} else {
		// The value is the one other cell.
		expr, note, _ := b.typeOf(value, place{name: candidate + "Value", parent: candidate}, false)
		d.fields = append(d.fields, field{name: "Key", expr: "String"}, field{name: "Value", expr: expr, note: note})
	}
	return b.add(d, candidate, at), true
}

// candidateName is the name a declaration at a place asks for: the name it
// was given, or its key as a name.
func (b *builder) candidateName(at place) string {
	if at.name != "" {
		return at.name
	}
	name := fieldName(at.key)
	switch first, _ := utf8.DecodeRuneInString(name); {
	case name == "" || !unicode.IsUpper(first):
		// The key is data, or in a script with no capitals. A declaration
		// is named as the types of Oct are, with a capital first.
		return at.parent + "Item"
	case reserved[name]:
		return at.parent + name
	}
	return name
}

// heldForRoot marks the root's name as taken before the root is declared.
const heldForRoot = "\x00root"

// add records a declaration under a name no other declaration has, and
// returns the name. A declaration equal to one already made is that one.
func (b *builder) add(d *declaration, candidate string, at place) string {
	signature := d.signature()
	if at.name == b.root && b.names[b.root] == heldForRoot {
		delete(b.names, b.root)
	}
	tries := []string{candidate}
	if !strings.HasPrefix(candidate, at.parent) {
		// `Point` under `Right` is `RightPoint` when `Point` is taken. A
		// name that already begins with its parent's goes to a number.
		tries = append(tries, at.parent+candidate)
	}
	for number := 2; ; number++ {
		for _, name := range tries {
			existing, taken := b.names[name]
			if taken && existing != signature {
				continue
			}
			if !taken {
				d.name = name
				b.names[name] = signature
				b.declarations = append(b.declarations, d)
			}
			return name
		}
		tries = []string{candidate + strconv.Itoa(number)}
	}
}

func (d *declaration) signature() string {
	var text strings.Builder
	if d.table {
		text.WriteString("table")
	}
	for _, f := range d.fields {
		text.WriteString(";" + f.name + ":" + f.expr)
	}
	return text.String()
}

// enumNote points out a String that takes few values: the enum it could be,
// in a comment. Inference does not declare enums (3.11).
//
// Few means that most of the strings seen are a value seen more than once.
// A column of names with one name twice is not an enum; a column where two
// of three rows say `analyst` may be.
func enumNote(text *textValues) string {
	if len(text.distinct) > maxEnumVariants {
		return ""
	}
	repeated := 0
	for _, count := range text.counts {
		if count > 1 {
			repeated += count
		}
	}
	if repeated*2 <= text.total {
		return ""
	}
	variants := make([]string, 0, len(text.distinct))
	taken := map[string]bool{}
	for _, value := range text.distinct {
		variant := fieldName(value)
		if !isPlainWord(value) || taken[variant] {
			return ""
		}
		taken[variant] = true
		variants = append(variants, variant)
	}
	return "could be an enum: " + strings.Join(variants, ", ")
}
