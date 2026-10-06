package octjson

import (
	"sort"
	"unicode/utf8"
)

// NodeKind is the kind of a JSON value.
type NodeKind int

const (
	NodeNull NodeKind = iota
	NodeBool
	NodeNumber
	NodeString
	NodeArray
	NodeObject
)

// Node is one JSON value of a Document.
type Node struct {
	Kind NodeKind
	// Offset is the byte offset of the value's first character.
	Offset int
	// Bool is the value of a NodeBool.
	Bool bool
	// Text is the text of a NodeNumber exactly as written, and the decoded
	// value of a NodeString.
	Text string
	// Elements are the elements of a NodeArray, in order.
	Elements []*Node
	// Members are the members of a NodeObject, in document order. A key
	// that is written twice is two members.
	Members []Member
}

// Member is one `"key": value` of an object.
type Member struct {
	Key string
	// KeyOffset is the byte offset of the key's opening quote.
	KeyOffset int
	Value     *Node
}

// Document is parsed JSON text.
type Document struct {
	Root *Node
	text []byte
	// lineStarts holds the byte offset at which each line begins.
	lineStarts []int
	// start is the offset of the first character: 3 after a byte order
	// mark, which is not a column of the first line.
	start int
}

// Position is the line and column of a byte offset, both counted from one.
// A column counts characters, not bytes.
func (d *Document) Position(offset int) (line int, column int) {
	if d.lineStarts == nil {
		d.lineStarts = []int{d.start}
		for index, b := range d.text {
			if b == '\n' {
				d.lineStarts = append(d.lineStarts, index+1)
			}
		}
	}
	if offset > len(d.text) {
		offset = len(d.text)
	}
	lineIndex := sort.Search(len(d.lineStarts), func(i int) bool { return d.lineStarts[i] > offset }) - 1
	return lineIndex + 1, utf8.RuneCount(d.text[d.lineStarts[lineIndex]:offset]) + 1
}

// kindName is what an error calls a value of the kind.
func (k NodeKind) kindName() string {
	switch k {
	case NodeNull:
		return "null"
	case NodeBool:
		return "a boolean"
	case NodeNumber:
		return "a number"
	case NodeString:
		return "a string"
	case NodeArray:
		return "an array"
	default:
		return "an object"
	}
}
