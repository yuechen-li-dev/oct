// Package diagnostic defines compiler-owned, source-mapped diagnostics.
package diagnostic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/source"
)

type Severity uint8

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityNote
)

func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	case SeverityNote:
		return "note"
	default:
		return "error"
	}
}

type Related struct {
	Message string
	Span    source.Span
}

// Diagnostic is the canonical compiler diagnostic. Path identifies the source
// file owning Span; an empty path is reserved for source-independent failures.
type Diagnostic struct {
	Path     string
	Code     string
	Severity Severity
	Message  string
	Span     source.Span
	Related  []Related
}

// Sort establishes stable source order without depending on map iteration.
func Sort(values []Diagnostic) {
	sort.SliceStable(values, func(i, j int) bool {
		a, b := values[i], values[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Span.Start.Offset != b.Span.Start.Offset {
			return a.Span.Start.Offset < b.Span.Start.Offset
		}
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		return a.Code < b.Code
	})
}

func Render(d Diagnostic) string {
	where := d.Path
	if d.Span.Known() {
		where = fmt.Sprintf("%s:%d:%d", where, d.Span.Start.Line, d.Span.Start.Column)
	} else if where == "" {
		where = "<unknown>"
	}
	line := fmt.Sprintf("%s: %s %s: %s", where, d.Severity, d.Code, d.Message)
	for _, related := range d.Related {
		at := d.Path
		if related.Span.Known() {
			at = fmt.Sprintf("%s:%d:%d", at, related.Span.Start.Line, related.Span.Start.Column)
		}
		line += fmt.Sprintf("\nnote: %s at %s", related.Message, at)
	}
	return line
}

// Error adapts diagnostics at legacy error boundaries. Diagnostics remain the
// authority; consumers needing machine-readable data should retain the slice.
func Error(values []Diagnostic) error {
	if len(values) == 0 {
		return nil
	}
	lines := make([]string, len(values))
	for i, d := range values {
		lines[i] = Render(d)
	}
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}
