package source

import (
	"fmt"
	"os"
)

type File struct {
	Path string
	Text string
}

// Position is a byte position in a UTF-8 source file. Offset is zero based and
// is suitable for slicing File.Text; Line and Column are one based and count
// Unicode code points (not bytes). The zero value is an unknown position.
type Position struct {
	Offset uint32
	Line   uint32
	Column uint32
}

// Span identifies the half-open source interval [Start, End). A zero Span is
// unknown. Spans are compiler data, never reconstructed by consumers.
type Span struct {
	Start Position
	End   Position
}

func (s Span) Known() bool { return s.Start.Line != 0 || s.End.Line != 0 }

// Contains reports whether other is wholly within s. Unknown spans are never
// considered contained; callers must consciously exempt synthetic nodes.
func (s Span) Contains(other Span) bool {
	if !s.Known() || !other.Known() {
		return false
	}
	return s.Start.Offset <= other.Start.Offset && other.End.Offset <= s.End.Offset
}

// Merge returns the smallest known span enclosing both spans. If only one is
// known it is returned; if neither is known the result is unknown.
func (s Span) Merge(other Span) Span {
	if !s.Known() {
		return other
	}
	if !other.Known() {
		return s
	}
	if other.Start.Offset < s.Start.Offset {
		s.Start = other.Start
	}
	if s.End.Offset < other.End.Offset {
		s.End = other.End
	}
	return s
}

func Load(path string) (File, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, fmt.Errorf("source file not found: %s", path)
		}
		return File{}, fmt.Errorf("load source %s: %w", path, err)
	}

	return File{
		Path: path,
		Text: string(text),
	}, nil
}
