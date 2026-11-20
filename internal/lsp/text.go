package lsp

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// lineIndex converts between the three coordinate systems in play:
//   - byte offsets (lexer Token.Offset / EndOffset),
//   - 1-based line : 1-based rune column (lexer Line/Column, error "at L:C"),
//   - 0-based line : UTF-16 code unit (LSP Position).
//
// Oct source routinely contains non-ASCII unit symbols (°C, Ω, µ), so treating
// any of these as interchangeable shifts every later range on the line.
type lineIndex struct {
	text   string
	starts []int // byte offset of the start of each line
}

func newLineIndex(text string) *lineIndex {
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &lineIndex{text: text, starts: starts}
}

func (ix *lineIndex) lineCount() int { return len(ix.starts) }

func (ix *lineIndex) lineEnd(line int) int {
	if line+1 < len(ix.starts) {
		end := ix.starts[line+1] - 1
		if end > ix.starts[line] && ix.text[end-1] == '\r' {
			end--
		}
		return end
	}
	return len(ix.text)
}

// position converts a byte offset to an LSP position.
func (ix *lineIndex) position(offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(ix.text) {
		offset = len(ix.text)
	}
	line := 0
	lo, hi := 0, len(ix.starts)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if ix.starts[mid] <= offset {
			line = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return Position{Line: line, Character: utf16Len(ix.text[ix.starts[line]:offset])}
}

func (ix *lineIndex) rangeOf(start, end int) Range {
	return Range{Start: ix.position(start), End: ix.position(end)}
}

// offsetOfRuneColumn converts a 1-based line and 1-based rune column into a
// byte offset, clamping to the line.
func (ix *lineIndex) offsetOfRuneColumn(line, column int) int {
	if line < 1 {
		line = 1
	}
	if line > len(ix.starts) {
		return len(ix.text)
	}
	offset := ix.starts[line-1]
	end := ix.lineEnd(line - 1)
	for col := 1; col < column && offset < end; col++ {
		_, size := utf8.DecodeRuneInString(ix.text[offset:])
		offset += size
	}
	return offset
}

// offsetOfPosition converts an LSP position to a byte offset.
func (ix *lineIndex) offsetOfPosition(p Position) int {
	if p.Line < 0 {
		return 0
	}
	if p.Line >= len(ix.starts) {
		return len(ix.text)
	}
	offset := ix.starts[p.Line]
	end := ix.lineEnd(p.Line)
	units := 0
	for offset < end && units < p.Character {
		r, size := utf8.DecodeRuneInString(ix.text[offset:])
		units += utf16.RuneLen(r)
		offset += size
	}
	return offset
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r1 := utf16.RuneLen(r); r1 > 0 {
			n += r1
		} else {
			n++ // invalid UTF-8 decodes as U+FFFD, one unit
		}
	}
	return n
}

// applyChange applies one incremental or full content change.
func applyChange(text string, change ContentChange) string {
	if change.Range == nil {
		return change.Text
	}
	ix := newLineIndex(text)
	start := ix.offsetOfPosition(change.Range.Start)
	end := ix.offsetOfPosition(change.Range.End)
	if end < start {
		start, end = end, start
	}
	return text[:start] + change.Text + text[end:]
}

// uriToPath converts a file:// URI to a local path, handling the Windows
// drive-letter form file:///c%3A/Users/... that VS Code sends.
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	p := u.Path
	if runtime.GOOS == "windows" || looksLikeWindowsDrivePath(p) {
		p = strings.TrimPrefix(p, "/")
		return filepath.FromSlash(p)
	}
	return filepath.FromSlash(p)
}

func looksLikeWindowsDrivePath(p string) bool {
	return len(p) >= 3 && p[0] == '/' && p[2] == ':' &&
		((p[1] >= 'a' && p[1] <= 'z') || (p[1] >= 'A' && p[1] <= 'Z'))
}

// pathToURI converts a local path to a file:// URI.
func pathToURI(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // C:/x -> /C:/x
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// sameFile compares paths case-insensitively on Windows.
func sameFile(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
