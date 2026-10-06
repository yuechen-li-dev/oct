package octjson

import (
	"strconv"
	"strings"
)

// Error is a failure to parse, decode or encode. Its text is made in one
// place, Text, so that it is the same wherever it is reported.
type Error struct {
	// Path is the place in the document or the value, as `$.service.http`
	// or `$[1].assignee`. A syntax error has none.
	Path string
	// Line and Column are where in the text, counted from one. They are
	// zero for an error that has no text behind it, as when writing.
	Line, Column int
	// Message says what is wrong.
	Message string
	// Unwritable is true when a value cannot be written as JSON at all, as
	// a NaN cannot. Json.Save stops the program on it; it is not an Error
	// a program handles.
	Unwritable bool
}

// Text is the full message: the operation, the file when there is one, the
// path, the position and what is wrong.
//
//	Json.Load: tickets.json: $[1].assignee (line 9, column 17): expected String or null, found a number
//	Json.Parse: (line 1, column 7): expected a value, found '}'
//	Json.Load: tickets.json: the file does not exist
func (e *Error) Text(operation string, source string) string {
	var b strings.Builder
	b.WriteString(operation)
	b.WriteString(": ")
	if source != "" {
		b.WriteString(source)
		b.WriteString(": ")
	}
	if e.Path != "" {
		b.WriteString(e.Path)
		if e.Line > 0 {
			b.WriteByte(' ')
		}
	}
	if e.Line > 0 {
		b.WriteString("(line ")
		b.WriteString(strconv.Itoa(e.Line))
		b.WriteString(", column ")
		b.WriteString(strconv.Itoa(e.Column))
		b.WriteByte(')')
	}
	if e.Path != "" || e.Line > 0 {
		b.WriteString(": ")
	}
	b.WriteString(e.Message)
	return b.String()
}

// Error makes *Error an error. Callers that report to a user use Text.
func (e *Error) Error() string { return e.Text("Json", "") }

// path builds the place of a value as the decoder and the encoder descend.
type path struct {
	parent *path
	// key is a member name, or "" for an element.
	key   string
	index int
}

var rootPath = &path{}

func (p *path) member(key string) *path { return &path{parent: p, key: key, index: -1} }
func (p *path) element(index int) *path { return &path{parent: p, index: index} }
func (p *path) isRoot() bool            { return p.parent == nil }
func (p *path) String() string {
	if p.isRoot() {
		return "$"
	}
	prefix := p.parent.String()
	if p.index >= 0 {
		return prefix + "[" + strconv.Itoa(p.index) + "]"
	}
	if isPlainKey(p.key) {
		return prefix + "." + p.key
	}
	return prefix + "[" + quoteString(p.key) + "]"
}

// isPlainKey reports whether a key reads unquoted after a dot.
func isPlainKey(key string) bool {
	if key == "" {
		return false
	}
	for index := 0; index < len(key); index++ {
		ch := key[index]
		letter := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
		if !letter && (index == 0 || ch < '0' || ch > '9') {
			return false
		}
	}
	return true
}

// quotedList is `"a", "b", "c"`.
func quotedList(names []string) string {
	quoted := make([]string, len(names))
	for index, name := range names {
		quoted[index] = quoteString(name)
	}
	return strings.Join(quoted, ", ")
}
