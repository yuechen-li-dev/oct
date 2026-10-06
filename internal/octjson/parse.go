package octjson

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// MaxDepth is how deeply arrays and objects may nest.
const MaxDepth = 512

// Parse reads one JSON value, strictly as RFC 8259 defines it: no comments,
// no trailing commas, nothing after the value. A leading UTF-8 byte order
// mark is skipped. The text must be UTF-8.
func Parse(text []byte) (*Document, *Error) {
	p := &parser{doc: &Document{text: text}, text: text}
	if len(text) >= 3 && text[0] == 0xEF && text[1] == 0xBB && text[2] == 0xBF {
		p.pos, p.doc.start = 3, 3
	}
	p.skipSpace()
	root, err := p.value(0)
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos < len(p.text) {
		return nil, p.fail(p.pos, "expected the end of the text, found %s", p.found())
	}
	p.doc.Root = root
	return p.doc, nil
}

type parser struct {
	doc  *Document
	text []byte
	pos  int
}

func (p *parser) fail(offset int, format string, arguments ...any) *Error {
	line, column := p.doc.Position(offset)
	return &Error{Line: line, Column: column, Message: fmt.Sprintf(format, arguments...)}
}

// found describes what is at the current position, for a message.
func (p *parser) found() string {
	if p.pos >= len(p.text) {
		return "the end of the text"
	}
	r, size := utf8.DecodeRune(p.text[p.pos:])
	switch {
	case r == utf8.RuneError && size <= 1:
		return "a byte that is not UTF-8"
	case r == '\n':
		return "the end of the line"
	case r < 0x20 || r == 0x7F:
		return fmt.Sprintf("the control character U+%04X", r)
	case r == '\'':
		return `"'"`
	default:
		return "'" + string(r) + "'"
	}
}

func (p *parser) skipSpace() {
	for p.pos < len(p.text) {
		switch p.text[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *parser) value(depth int) (*Node, *Error) {
	if p.pos >= len(p.text) {
		return nil, p.fail(p.pos, "expected a value, found the end of the text")
	}
	start := p.pos
	switch ch := p.text[p.pos]; {
	case ch == '{':
		return p.object(depth)
	case ch == '[':
		return p.array(depth)
	case ch == '"':
		text, err := p.stringLiteral()
		if err != nil {
			return nil, err
		}
		return &Node{Kind: NodeString, Offset: start, Text: text}, nil
	case ch == '-' || (ch >= '0' && ch <= '9'):
		return p.number()
	case p.word("true"):
		return &Node{Kind: NodeBool, Offset: start, Bool: true}, nil
	case p.word("false"):
		return &Node{Kind: NodeBool, Offset: start}, nil
	case p.word("null"):
		return &Node{Kind: NodeNull, Offset: start}, nil
	}
	return nil, p.fail(p.pos, "expected a value, found %s", p.found())
}

// word consumes a literal name if it is next. What follows it is checked by
// whoever reads on.
func (p *parser) word(name string) bool {
	if !strings.HasPrefix(string(p.text[p.pos:min(len(p.text), p.pos+len(name))]), name) {
		return false
	}
	p.pos += len(name)
	return true
}

func (p *parser) array(depth int) (*Node, *Error) {
	node := &Node{Kind: NodeArray, Offset: p.pos}
	if depth >= MaxDepth {
		return nil, p.fail(p.pos, "arrays and objects are nested more than %d deep", MaxDepth)
	}
	p.pos++
	p.skipSpace()
	if p.pos < len(p.text) && p.text[p.pos] == ']' {
		p.pos++
		return node, nil
	}
	for {
		p.skipSpace()
		element, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		node.Elements = append(node.Elements, element)
		p.skipSpace()
		if p.pos < len(p.text) && p.text[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.pos < len(p.text) && p.text[p.pos] == ']' {
			p.pos++
			return node, nil
		}
		return nil, p.fail(p.pos, "expected ',' or ']' after an array element, found %s", p.found())
	}
}

func (p *parser) object(depth int) (*Node, *Error) {
	node := &Node{Kind: NodeObject, Offset: p.pos}
	if depth >= MaxDepth {
		return nil, p.fail(p.pos, "arrays and objects are nested more than %d deep", MaxDepth)
	}
	p.pos++
	p.skipSpace()
	if p.pos < len(p.text) && p.text[p.pos] == '}' {
		p.pos++
		return node, nil
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.text) || p.text[p.pos] != '"' {
			return nil, p.fail(p.pos, "expected a key in double quotes, found %s", p.found())
		}
		keyOffset := p.pos
		key, err := p.stringLiteral()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos >= len(p.text) || p.text[p.pos] != ':' {
			return nil, p.fail(p.pos, "expected ':' after a key, found %s", p.found())
		}
		p.pos++
		p.skipSpace()
		value, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		node.Members = append(node.Members, Member{Key: key, KeyOffset: keyOffset, Value: value})
		p.skipSpace()
		if p.pos < len(p.text) && p.text[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.pos < len(p.text) && p.text[p.pos] == '}' {
			p.pos++
			return node, nil
		}
		return nil, p.fail(p.pos, "expected ',' or '}' after a member, found %s", p.found())
	}
}

// number reads `-? (0 | [1-9][0-9]*) (. [0-9]+)? ([eE] [+-]? [0-9]+)?` and
// keeps its text. Whether it is an Int or a Float, and whether it is in
// range, is for the type it is read as to say.
func (p *parser) number() (*Node, *Error) {
	start := p.pos
	digits := func() int {
		from := p.pos
		for p.pos < len(p.text) && p.text[p.pos] >= '0' && p.text[p.pos] <= '9' {
			p.pos++
		}
		return p.pos - from
	}
	if p.text[p.pos] == '-' {
		p.pos++
	}
	switch {
	case p.pos < len(p.text) && p.text[p.pos] == '0':
		p.pos++
		if p.pos < len(p.text) && p.text[p.pos] >= '0' && p.text[p.pos] <= '9' {
			return nil, p.fail(start, "a number does not begin with a zero that other digits follow")
		}
	case digits() == 0:
		return nil, p.fail(p.pos, "expected a digit, found %s", p.found())
	}
	if p.pos < len(p.text) && p.text[p.pos] == '.' {
		p.pos++
		if digits() == 0 {
			return nil, p.fail(p.pos, "expected a digit after the decimal point, found %s", p.found())
		}
	}
	if p.pos < len(p.text) && (p.text[p.pos] == 'e' || p.text[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.text) && (p.text[p.pos] == '+' || p.text[p.pos] == '-') {
			p.pos++
		}
		if digits() == 0 {
			return nil, p.fail(p.pos, "expected a digit in the exponent, found %s", p.found())
		}
	}
	return &Node{Kind: NodeNumber, Offset: start, Text: string(p.text[start:p.pos])}, nil
}

// stringLiteral reads a string and answers with its value. The string must
// be UTF-8, holds no raw control character, and every `\u` escape must be a
// character: a surrogate has to come as a pair.
func (p *parser) stringLiteral() (string, *Error) {
	start := p.pos
	p.pos++
	var value strings.Builder
	for {
		if p.pos >= len(p.text) {
			return "", p.fail(start, "this string has no closing quote")
		}
		ch := p.text[p.pos]
		switch {
		case ch == '"':
			p.pos++
			return value.String(), nil
		case ch == '\\':
			if err := p.escape(&value); err != nil {
				return "", err
			}
		case ch < 0x20:
			return "", p.fail(p.pos, "a string cannot hold %s; write it as an escape", p.found())
		case ch < utf8.RuneSelf:
			value.WriteByte(ch)
			p.pos++
		default:
			r, size := utf8.DecodeRune(p.text[p.pos:])
			if r == utf8.RuneError && size <= 1 {
				return "", p.fail(p.pos, "the text is not UTF-8 here")
			}
			value.Write(p.text[p.pos : p.pos+size])
			p.pos += size
		}
	}
}

func (p *parser) escape(value *strings.Builder) *Error {
	at := p.pos
	p.pos++
	if p.pos >= len(p.text) {
		return p.fail(at, "this string has no closing quote")
	}
	ch := p.text[p.pos]
	p.pos++
	switch ch {
	case '"', '\\', '/':
		value.WriteByte(ch)
	case 'b':
		value.WriteByte('\b')
	case 'f':
		value.WriteByte('\f')
	case 'n':
		value.WriteByte('\n')
	case 'r':
		value.WriteByte('\r')
	case 't':
		value.WriteByte('\t')
	case 'u':
		unit, err := p.hex4(at)
		if err != nil {
			return err
		}
		r := rune(unit)
		if utf16.IsSurrogate(r) {
			second := rune(-1)
			if p.pos+1 < len(p.text) && p.text[p.pos] == '\\' && p.text[p.pos+1] == 'u' {
				p.pos += 2
				low, err := p.hex4(at)
				if err != nil {
					return err
				}
				second = rune(low)
			}
			r = utf16.DecodeRune(r, second)
			if r == utf8.RuneError {
				return p.fail(at, `the escape \u%04X is half of a surrogate pair and the other half does not follow it`, unit)
			}
		}
		value.WriteRune(r)
	default:
		p.pos = at + 1
		return p.fail(at, `'\' is followed by %s; the escapes are \" \\ \/ \b \f \n \r \t and \uXXXX`, p.found())
	}
	return nil
}

func (p *parser) hex4(escapeAt int) (uint16, *Error) {
	if p.pos+4 > len(p.text) {
		return 0, p.fail(escapeAt, `\u is followed by four hexadecimal digits`)
	}
	var unit uint16
	for _, ch := range p.text[p.pos : p.pos+4] {
		var digit byte
		switch {
		case ch >= '0' && ch <= '9':
			digit = ch - '0'
		case ch >= 'a' && ch <= 'f':
			digit = ch - 'a' + 10
		case ch >= 'A' && ch <= 'F':
			digit = ch - 'A' + 10
		default:
			return 0, p.fail(escapeAt, `\u is followed by four hexadecimal digits`)
		}
		unit = unit<<4 | uint16(digit)
	}
	p.pos += 4
	return unit, nil
}
