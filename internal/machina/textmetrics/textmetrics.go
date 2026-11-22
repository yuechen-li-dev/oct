// Package textmetrics is the seam between text and layout: how wide a
// string is and how tall a line is for a text spec, plus the wrapping and
// truncation that layout and render must agree on.
//
// Headless tools use Fixed, which is deterministic so dumps and golden tests
// are stable. A native backend supplies metrics from its real font shaper;
// layout, wrapping and truncation then all use the same numbers the backend
// draws with.
package textmetrics

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// Metrics measures text in dp.
type Metrics interface {
	Width(text string, spec uiir.TextSpec) float64
	LineHeight(spec uiir.TextSpec) float64
}

// Body is the spec of unstyled text and button labels.
var Body = uiir.TextSpec{Size: 16, Weight: 400, Overflow: uiir.OverflowEllipsis}

// Ellipsis is appended to truncated text.
const Ellipsis = "…"

// Fixed is the deterministic headless metric: every rune is half the font
// size wide (0.6 for monospace) and a line is 1.25 times the size. At the
// default 16 dp size that is 8 dp per character and 20 dp per line.
type Fixed struct{}

func (Fixed) Width(text string, spec uiir.TextSpec) float64 {
	per := 0.5
	if spec.Mono {
		per = 0.6
	}
	return float64(utf8.RuneCountInString(text)) * size(spec) * per
}

func (Fixed) LineHeight(spec uiir.TextSpec) float64 { return size(spec) * 1.25 }

func size(spec uiir.TextSpec) float64 {
	if spec.Size > 0 {
		return spec.Size
	}
	return Body.Size
}

// Wrap breaks text into lines no wider than width, at spaces where it can
// and inside a word only when the word alone is wider than width. Explicit
// newlines are kept. Wrapping is greedy and deterministic for given metrics.
func Wrap(m Metrics, text string, spec uiir.TextSpec, width float64) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.FieldsFunc(paragraph, unicode.IsSpace)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		line := ""
		for _, word := range words {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if m.Width(candidate, spec) <= width {
				line = candidate
				continue
			}
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			// A word wider than the line is broken by rune.
			for m.Width(word, spec) > width {
				cut := fitRunes(m, word, spec, width)
				if cut == 0 {
					cut = len(string([]rune(word)[0]))
				}
				lines = append(lines, word[:cut])
				word = word[cut:]
			}
			line = word
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// fitRunes returns the byte length of the longest prefix of s that fits.
func fitRunes(m Metrics, s string, spec uiir.TextSpec, width float64) int {
	best := 0
	for i := range s {
		if i > 0 && m.Width(s[:i], spec) > width {
			return best
		}
		best = i
	}
	if m.Width(s, spec) <= width {
		return len(s)
	}
	return best
}

// Fit returns s unchanged if it fits width, or its longest prefix that fits
// with an ellipsis appended. When not even the ellipsis fits it returns "".
func Fit(m Metrics, s string, spec uiir.TextSpec, width float64) string {
	if m.Width(s, spec) <= width {
		return s
	}
	room := width - m.Width(Ellipsis, spec)
	if room < 0 {
		return ""
	}
	cut := fitRunes(m, s, spec, room)
	return strings.TrimRightFunc(s[:cut], unicode.IsSpace) + Ellipsis
}
