// Package octfailheader reads the expectation lines that begin an .octfail
// fixture. The test runner and the formatter both need the same grammar, and
// this package is the one place it is written down.
package octfailheader

import (
	"fmt"
	"regexp"
	"strings"
)

// Phase says when the fixture is expected to fail.
type Phase string

const (
	// Compile: the source is rejected before it can run.
	//
	//	expect error: "text"
	Compile Phase = "compile"
	// Runtime: the source compiles, and running Main stops with a failure.
	//
	//	expect runtime error: "text"
	Runtime Phase = "runtime"
	// Artifact: the source is a test source, and evaluating its [Artifact]
	// entry points fails and publishes nothing.
	//
	//	expect artifact error: "text"
	Artifact Phase = "artifact"
)

// Header is the expectation block of a fixture. A fixture states one phase
// and one or more texts; the failure must contain every text.
type Header struct {
	Phase Phase
	Texts []string
	// Lines are the expectation lines as written, trimmed, in order.
	Lines []string
	// Leading is the number of blank lines before the first expectation.
	Leading int
}

var linePattern = regexp.MustCompile(`^expect (runtime |artifact )?error:\s*"(.*)"\s*$`)

func looksLikeExpectation(trimmed string) bool {
	return strings.HasPrefix(trimmed, "expect error:") ||
		strings.HasPrefix(trimmed, "expect runtime error:") ||
		strings.HasPrefix(trimmed, "expect artifact error:")
}

func phaseOf(word string) Phase {
	switch strings.TrimSpace(word) {
	case "runtime":
		return Runtime
	case "artifact":
		return Artifact
	default:
		return Compile
	}
}

// Split separates a fixture into its expectation block and the Oct source
// that follows it. The expectation lines come first, one after another, and
// all name the same phase.
func Split(content string) (Header, string, error) {
	lines := strings.Split(content, "\n")
	first := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			first = i
			break
		}
	}
	if first == -1 {
		return Header{}, "", fmt.Errorf("missing expectation header")
	}

	header := Header{Leading: first}
	end := first
	for end < len(lines) {
		trimmed := strings.TrimSpace(lines[end])
		match := linePattern.FindStringSubmatch(trimmed)
		if match == nil {
			if end > first && looksLikeExpectation(trimmed) {
				return Header{}, "", fmt.Errorf("malformed expectation header")
			}
			break
		}
		if match[2] == "" {
			return Header{}, "", fmt.Errorf("expected error substring must be non-empty")
		}
		phase := phaseOf(match[1])
		if end == first {
			header.Phase = phase
		} else if phase != header.Phase {
			return Header{}, "", fmt.Errorf("expectation lines must all name the same phase: found %q after %q", trimmed, header.Lines[0])
		}
		header.Texts = append(header.Texts, decodeExpectation(match[2]))
		header.Lines = append(header.Lines, trimmed)
		end++
	}
	if len(header.Lines) == 0 {
		return Header{}, "", fmt.Errorf("malformed expectation header")
	}
	for i := end; i < len(lines); i++ {
		if looksLikeExpectation(strings.TrimSpace(lines[i])) {
			return Header{}, "", fmt.Errorf("expectation lines must come first, one after another; found one on line %d", i+1)
		}
	}
	return header, strings.Join(lines[end:], "\n"), nil
}

// Preserve legacy unescaped quotation marks while allowing a quoted path or
// literal backslash to be stated without including the escape in the match.
func decodeExpectation(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) && (text[i+1] == '\\' || text[i+1] == '"') {
			i++
		}
		b.WriteByte(text[i])
	}
	return b.String()
}
