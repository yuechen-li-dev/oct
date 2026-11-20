package lsp

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/source"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

// Diagnostics come from two passes with different evidence quality:
//
//  1. Syntax (every keystroke, from the editor buffer): lexer and parser
//     errors carry an exact `at L:C` position, so their ranges are exact.
//  2. Package (on open/save, from disk): project loading plus type checking,
//     the same pipeline `oct run` uses. Type errors carry no source position,
//     only an enclosing declaration (`function Main: let z: cannot add m and s`).
//     locateMessage pins them to the most specific token it can justify from
//     the message and falls back to the declaration name. It never invents a
//     position the message does not support.

var (
	reAt   = regexp.MustCompile(`at (\d+):(\d+)`)
	reNear = regexp.MustCompile(`near "((?:[^"\\]|\\.)*)"`)
	// rePrefix finds `lex PATH: ` / `parse PATH: ` anywhere, since project
	// loading may wrap a file's syntax error in package context.
	rePrefix   = regexp.MustCompile(`(?:lex|parse) (.+?\.(?:oct|octest|octfail|octagon)): `)
	reDecl     = regexp.MustCompile(`^(function|record|enum|flow|concept) '?([A-Za-z_][A-Za-z0-9_.]*)'?: `)
	reLetVar   = regexp.MustCompile(`\b(?:let|var) ([A-Za-z_][A-Za-z0-9_]*):`)
	reUndef    = regexp.MustCompile(`undefined (?:variable|function|name|identifier)[: ]+'?([A-Za-z_][A-Za-z0-9_.]*)'?`)
	reQuoted   = regexp.MustCompile(`'([A-Za-z_][A-Za-z0-9_.]*)'`)
	reTypeName = regexp.MustCompile(`unknown type '?([A-Za-z_][A-Za-z0-9_.]*)'?`)
)

// fileSource supplies current text for a path: the open buffer if the editor
// has one, otherwise disk.
type fileSource func(path string) (string, bool)

// syntaxDiagnostics lexes and parses one buffer.
func syntaxDiagnostics(path, text string) []Diagnostic {
	result, err := lex.Analyze(source.File{Path: path, Text: text})
	if err == nil {
		_, err = parse.BuildFile(result)
	}
	if err == nil {
		return nil
	}
	ix := newLineIndex(text)
	var tokens []lex.Token
	if result.Tokens != nil {
		tokens = result.Tokens
	}
	return []Diagnostic{positionedDiagnostic(ix, tokens, err.Error())}
}

// positionedDiagnostic builds a diagnostic from a message containing `at L:C`.
func positionedDiagnostic(ix *lineIndex, tokens []lex.Token, raw string) Diagnostic {
	msg := cleanMessage(raw)
	m := reAt.FindStringSubmatch(raw)
	if m == nil {
		return Diagnostic{Range: ix.rangeOf(0, 0), Severity: severityError, Source: "oct", Message: msg}
	}
	line, _ := strconv.Atoi(m[1])
	col, _ := strconv.Atoi(m[2])
	start := ix.offsetOfRuneColumn(line, col)
	end := start
	if tok, ok := tokenAt(tokens, start); ok {
		end = tok.EndOffset
	} else if n := reNear.FindStringSubmatch(raw); n != nil && n[1] != "" && strings.HasPrefix(ix.text[start:], n[1]) {
		end = start + len(n[1])
	} else {
		end = wordEnd(ix.text, start)
	}
	return Diagnostic{Range: ix.rangeOf(start, end), Severity: severityError, Source: "oct", Message: msg}
}

func tokenAt(tokens []lex.Token, offset int) (lex.Token, bool) {
	for _, t := range tokens {
		if t.Kind != lex.EOF && t.Offset == offset && t.EndOffset > t.Offset {
			return t, true
		}
	}
	return lex.Token{}, false
}

func wordEnd(text string, start int) int {
	end := start
	for end < len(text) {
		c := text[end]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			break
		}
		end++
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80) {
			break
		}
	}
	if end == start && end < len(text) {
		end++
	}
	return end
}

// cleanMessage drops wrapping context up to the `lex PATH: ` / `parse PATH: `
// prefix; the editor already shows which file the diagnostic belongs to.
func cleanMessage(raw string) string {
	raw = strings.TrimSpace(raw)
	if loc := rePrefix.FindStringIndex(raw); loc != nil {
		return raw[loc[1]:]
	}
	return raw
}

// packageDiagnostics runs the `oct run`/`oct test` front end over the package
// containing path and returns diagnostics keyed by file path.
func packageDiagnostics(path string, read fileSource) map[string][]Diagnostic {
	out := map[string][]Diagnostic{}
	load := project.Load
	if strings.HasSuffix(path, ".octest") {
		load = project.LoadForTest
	}
	program, err := load(path)
	if err == nil {
		err = typecheck.CheckProgram(program)
	}
	if err == nil {
		return out
	}
	for _, line := range splitErrors(err.Error()) {
		file, diag := locateMessage(path, program, line, read)
		out[file] = append(out[file], diag)
	}
	return out
}

// splitErrors separates errors.Join output into individual messages.
func splitErrors(raw string) []string {
	var lines []string
	for _, l := range strings.Split(raw, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// locateMessage attributes one front-end error to a file and range.
func locateMessage(entry string, program project.Program, raw string, read fileSource) (string, Diagnostic) {
	// Parse/lex errors in any package file name their file and position.
	if m := rePrefix.FindStringSubmatch(raw); m != nil {
		file := m[1]
		if !filepath.IsAbs(file) {
			file = filepath.Join(filepath.Dir(entry), file)
		}
		text, _ := read(file)
		ix := newLineIndex(text)
		tokens := lexTokens(file, text)
		return file, positionedDiagnostic(ix, tokens, raw)
	}

	msg := strings.TrimSpace(raw)
	d := reDecl.FindStringSubmatch(msg)
	if d == nil {
		// Package-metadata failures belong to the package's manifest.oct.
		if strings.Contains(strings.ToLower(msg), "manifest") {
			manifest := filepath.Join(filepath.Dir(entry), "manifest.oct")
			if text, ok := read(manifest); ok {
				ix := newLineIndex(text)
				if tok, _, _, found := findDeclaration(lexTokens(manifest, text), "function", "Manifest"); found {
					return manifest, Diagnostic{Range: ix.rangeOf(tok.Offset, tok.EndOffset), Severity: severityError, Source: "oct", Message: msg}
				}
			}
		}
		// Unattributable: report at the top of the entry file, unmodified.
		text, _ := read(entry)
		return entry, Diagnostic{Range: newLineIndex(text).rangeOf(0, 0), Severity: severityError, Source: "oct", Message: msg}
	}
	declKind, declName := d[1], lastSegment(d[2])
	file := declarationFile(entry, program, declKind, declName, read)
	text, _ := read(file)
	ix := newLineIndex(text)
	tokens := lexTokens(file, text)
	nameTok, bodyStart, bodyEnd, found := findDeclaration(tokens, declKind, declName)
	if !found {
		return file, Diagnostic{Range: ix.rangeOf(0, 0), Severity: severityError, Source: "oct", Message: msg}
	}
	target, trace := chooseTarget(tokens, bodyStart, bodyEnd, nameTok, msg[len(d[0]):])
	diag := Diagnostic{Range: ix.rangeOf(target.Offset, target.EndOffset), Severity: severityError, Source: "oct", Message: msg,
		Data: map[string]any{"locator": trace}}
	if target.Offset != nameTok.Offset {
		diag.RelatedInformation = []DiagnosticRelatedInformation{{
			Location: Location{URI: pathToURI(file), Range: ix.rangeOf(nameTok.Offset, nameTok.EndOffset)},
			Message:  "in " + declKind + " " + declName,
		}}
	}
	return file, diag
}

func lastSegment(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func lexTokens(path, text string) []lex.Token {
	result, err := lex.Analyze(source.File{Path: path, Text: text})
	if err != nil {
		return nil
	}
	return result.Tokens
}

// declarationFile finds the file declaring a name: the loaded program's
// SourcePath when available, else a scan of the entry package directory.
func declarationFile(entry string, program project.Program, kind, name string, read fileSource) string {
	if kind == "function" {
		for _, pkg := range program.Packages {
			for _, fn := range pkg.Functions {
				if fn.Name == name && fn.SourcePath != "" {
					return fn.SourcePath
				}
			}
		}
	}
	dir := filepath.Dir(entry)
	candidates := []string{entry}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			n := e.Name()
			if !e.IsDir() && (strings.HasSuffix(n, ".oct") || strings.HasSuffix(n, ".octest")) {
				p := filepath.Join(dir, n)
				if !sameFile(p, entry) {
					candidates = append(candidates, p)
				}
			}
		}
	}
	for _, p := range candidates {
		text, ok := read(p)
		if !ok {
			continue
		}
		if _, _, _, found := findDeclaration(lexTokens(p, text), kind, name); found {
			return p
		}
	}
	return entry
}

var declKeyword = map[string]lex.TokenKind{
	"function": lex.KeywordFn,
	"record":   lex.KeywordRecord,
	"enum":     lex.KeywordEnum,
	"flow":     lex.KeywordFlow,
	"concept":  lex.KeywordConcept,
}

// findDeclaration returns the declaration's name token and the token index
// range of its body (through the matching closing brace).
func findDeclaration(tokens []lex.Token, kind, name string) (lex.Token, int, int, bool) {
	kw, ok := declKeyword[kind]
	if !ok {
		return lex.Token{}, 0, 0, false
	}
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i].Kind != kw || tokens[i+1].Kind != lex.Identifier || tokens[i+1].Lexeme != name {
			continue
		}
		start, end := i+1, len(tokens)
		depth := 0
		for j := i + 2; j < len(tokens); j++ {
			switch tokens[j].Kind {
			case lex.LeftBrace:
				depth++
			case lex.RightBrace:
				depth--
				if depth == 0 {
					end = j + 1
					return tokens[i+1], start, end, true
				}
			}
		}
		return tokens[i+1], start, end, true
	}
	return lex.Token{}, 0, 0, false
}

// readFromDisk is the default fileSource fallback.
func readFromDisk(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}
