package jsoninfer

import (
	"fmt"
	"strconv"

	"github.com/yuechen-li-dev/oct/internal/octjson"
)

// kind is what the values at one place are.
type kind int

const (
	// kindUnknown: no value says what this is. Only null was seen, or it is
	// the element of an array that is empty everywhere.
	kindUnknown kind = iota
	kindBool
	kindInt
	kindFloat
	kindString
	kindArray
	kindObject
	// kindRefused: the values here have no one declaration.
	kindRefused
)

// phrase is what a message calls a value of the kind. It is asked of the
// kinds a value can have.
func (k kind) phrase() string {
	switch k {
	case kindBool:
		return "a boolean"
	case kindInt, kindFloat:
		return "a number"
	case kindString:
		return "a string"
	case kindArray:
		return "an array"
	default:
		return "an object"
	}
}

// shape is what the values seen at one place have in common.
type shape struct {
	kind kind
	// optional: a null was seen here. A member that some objects lack is
	// optional too; that is known from the member's count, not from here.
	optional bool
	// path and node are the first value seen, for a message.
	path string
	node *octjson.Node

	// text is the strings seen, of a kindString.
	text *textValues

	// elem is the element of a kindArray; kindUnknown while every array
	// seen here is empty.
	elem *shape
	// rectangular: every array seen here was rows of numbers, at least one
	// row and one column, all rows one length.
	rectangular bool
	// rows are the objects seen as elements, of all the arrays seen here.
	rows []row

	// object is the members of a kindObject.
	object *objectShape

	// refusal is why a kindRefused has no declaration.
	refusal *Refusal
}

// row is one object of an array, and its place.
type row struct {
	path string
	node *octjson.Node
}

// objectShape is the objects seen at one place, folded into one.
type objectShape struct {
	// instances is how many objects were folded.
	instances int
	// members are the members of all of them, in the order first seen.
	members []*member
}

// member is one key of an objectShape.
type member struct {
	key string
	// count is how many of the instances have the member.
	count int
	shape *shape
}

func (o *objectShape) find(key string) *member {
	for _, existing := range o.members {
		if existing.key == key {
			return existing
		}
	}
	return nil
}

// maxEnumVariants is the most distinct strings a place may take and still
// be pointed out as a possible enum.
const maxEnumVariants = 8

// textValues is the strings seen at one place: enough to say whether they
// are few.
type textValues struct {
	total int
	// distinct holds the values in the order first seen, and counts how
	// often each was. Both stop growing one value past maxEnumVariants,
	// which is already too many.
	distinct []string
	counts   map[string]int
}

func (t *textValues) add(value string, times int) {
	t.total += times
	if _, seen := t.counts[value]; !seen {
		if len(t.distinct) > maxEnumVariants {
			return
		}
		t.distinct = append(t.distinct, value)
	}
	t.counts[value] += times
}

func (t *textValues) merge(other *textValues) {
	seen := t.total
	for _, value := range other.distinct {
		t.add(value, other.counts[value])
	}
	t.total = seen + other.total
}

// observe reads one value into a shape.
func observe(node *octjson.Node, path string) *shape {
	s := &shape{path: path, node: node}
	switch node.Kind {
	case octjson.NodeNull:
		s.optional = true
	case octjson.NodeBool:
		s.kind = kindBool
	case octjson.NodeNumber:
		s.kind = kindFloat
		if isInt(node.Text) {
			s.kind = kindInt
		}
	case octjson.NodeString:
		s.kind = kindString
		s.text = &textValues{counts: map[string]int{}}
		s.text.add(node.Text, 1)
	case octjson.NodeArray:
		s.kind = kindArray
		s.rectangular = isRectangular(node)
		s.elem = &shape{path: octjson.ElementPath(path, 0)}
		for index, element := range node.Elements {
			at := octjson.ElementPath(path, index)
			if element.Kind == octjson.NodeObject {
				s.rows = append(s.rows, row{path: at, node: element})
			}
			s.elem = unify(s.elem, observe(element, at))
		}
	default:
		s.kind = kindObject
		s.object = &objectShape{instances: 1}
		for _, written := range node.Members {
			if s.object.find(written.Key) != nil {
				// Json.Load refuses a key written twice, whatever is declared.
				return refused(octjson.MemberPath(path, written.Key), written.Value, fmt.Sprintf("the key %s is written twice in one object", strconv.Quote(written.Key)))
			}
			s.object.members = append(s.object.members, &member{key: written.Key, count: 1, shape: observe(written.Value, octjson.MemberPath(path, written.Key))})
		}
	}
	return s
}

func refused(path string, node *octjson.Node, reason string) *shape {
	return &shape{kind: kindRefused, path: path, node: node, refusal: &Refusal{Path: path, Reason: reason, node: node}}
}

// isInt reports whether the text of a number is an Int: no fraction, no
// exponent, in the 64-bit range (section 3.3 of the ladder).
func isInt(text string) bool {
	_, err := strconv.ParseInt(text, 10, 64)
	return err == nil
}

// isRectangular reports whether an array is rows of numbers, at least one
// row and one column, all rows one length: what a Matrix reads.
func isRectangular(node *octjson.Node) bool {
	if len(node.Elements) == 0 {
		return false
	}
	width := len(node.Elements[0].Elements)
	for _, rowNode := range node.Elements {
		if rowNode.Kind != octjson.NodeArray || len(rowNode.Elements) != width || width == 0 {
			return false
		}
		for _, cell := range rowNode.Elements {
			if cell.Kind != octjson.NodeNumber {
				return false
			}
		}
	}
	return true
}

// unify folds a value into what was seen before at its place. It changes
// and returns seen, except where the result is another shape altogether.
func unify(seen *shape, value *shape) *shape {
	switch {
	case seen.kind == kindRefused:
		return seen
	case value.kind == kindRefused:
		return value
	case seen.kind == kindUnknown:
		value.optional = value.optional || seen.optional
		return value
	case value.kind == kindUnknown:
		seen.optional = seen.optional || value.optional
		return seen
	}
	if (seen.kind == kindInt && value.kind == kindFloat) || (seen.kind == kindFloat && value.kind == kindInt) {
		// One number with a fraction makes the place a Float; an Int reads
		// as one.
		seen.kind = kindFloat
		value.kind = kindFloat
	}
	if seen.kind != value.kind {
		return refused(value.path, value.node, fmt.Sprintf("%s here, and %s at %s", value.kind.phrase(), seen.kind.phrase(), seen.path))
	}
	seen.optional = seen.optional || value.optional
	switch seen.kind {
	case kindString:
		seen.text.merge(value.text)
	case kindArray:
		seen.rectangular = seen.rectangular && value.rectangular
		seen.rows = append(seen.rows, value.rows...)
		seen.elem = unify(seen.elem, value.elem)
	case kindObject:
		seen.object.instances += value.object.instances
		for _, incoming := range value.object.members {
			if existing := seen.object.find(incoming.key); existing != nil {
				existing.count += incoming.count
				existing.shape = unify(existing.shape, incoming.shape)
				continue
			}
			seen.object.members = append(seen.object.members, incoming)
		}
	}
	return seen
}

// clone copies a shape and everything under it, so that it can be folded
// into another without changing the one it came from.
func clone(s *shape) *shape {
	copied := *s
	if s.text != nil {
		copied.text = &textValues{total: s.text.total, distinct: append([]string(nil), s.text.distinct...), counts: map[string]int{}}
		for value, count := range s.text.counts {
			copied.text.counts[value] = count
		}
	}
	if s.elem != nil {
		copied.elem = clone(s.elem)
	}
	copied.rows = append([]row(nil), s.rows...)
	if s.object != nil {
		copied.object = &objectShape{instances: s.object.instances}
		for _, existing := range s.object.members {
			copied.object.members = append(copied.object.members, &member{key: existing.key, count: existing.count, shape: clone(existing.shape)})
		}
	}
	return &copied
}
