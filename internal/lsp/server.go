package lsp

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/ocfmt"
)

type document struct {
	uri     string
	path    string
	text    string
	version int
}

// Server is one LSP session over a single connection.
type Server struct {
	conn     *conn
	log      io.Writer
	docs     map[string]*document // by URI
	pkgDiags map[string][]Diagnostic
	// published records every path we have published package diagnostics
	// for, so a fixed error is cleared even in files that are not open.
	published map[string]bool
	shutdown  bool
}

// Serve runs the language server until the client sends `exit` or closes
// the stream. log receives human-readable server logs (stderr in `oct lsp`).
func Serve(in io.Reader, out io.Writer, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}
	s := &Server{
		conn:      newConn(in, out),
		log:       log,
		docs:      map[string]*document{},
		pkgDiags:  map[string][]Diagnostic{},
		published: map[string]bool{},
	}
	for {
		msg, err := s.conn.read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			fmt.Fprintf(log, "oct lsp: %v\n", err)
			return err
		}
		if msg.Method == "exit" {
			return nil
		}
		s.dispatch(msg)
	}
}

func (s *Server) dispatch(msg message) {
	isRequest := len(msg.ID) > 0
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(s.log, "oct lsp: panic in %s: %v\n%s\n", msg.Method, r, debug.Stack())
			if isRequest {
				_ = s.conn.replyError(msg.ID, codeInternalError, fmt.Sprintf("internal error in %s: %v", msg.Method, r))
			}
		}
	}()
	if msg.Method == "" {
		return // a response to a server->client request; none are sent in M0
	}
	result, err, handled := s.handle(msg)
	if !isRequest {
		if err != nil {
			fmt.Fprintf(s.log, "oct lsp: %s: %v\n", msg.Method, err)
		}
		return
	}
	switch {
	case !handled:
		_ = s.conn.replyError(msg.ID, codeMethodNotFound, "method not supported: "+msg.Method)
	case err != nil:
		_ = s.conn.replyError(msg.ID, codeInvalidParams, err.Error())
	default:
		_ = s.conn.reply(msg.ID, result)
	}
}

func (s *Server) handle(msg message) (any, error, bool) {
	switch msg.Method {
	case "initialize":
		return s.initialize(), nil, true
	case "initialized":
		return nil, nil, true
	case "shutdown":
		s.shutdown = true
		return nil, nil, true
	case "textDocument/didOpen":
		var p DidOpenParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, err, true
		}
		s.didOpen(p)
		return nil, nil, true
	case "textDocument/didChange":
		var p DidChangeParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, err, true
		}
		s.didChange(p)
		return nil, nil, true
	case "textDocument/didSave":
		var p DidSaveParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, err, true
		}
		s.didSave(p)
		return nil, nil, true
	case "textDocument/didClose":
		var p DidCloseParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, err, true
		}
		s.didClose(p)
		return nil, nil, true
	case "textDocument/formatting":
		var p DocumentParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, err, true
		}
		edits, err := s.formatting(p)
		return edits, err, true
	case "textDocument/semanticTokens/full":
		var p DocumentParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, err, true
		}
		doc := s.docs[p.TextDocument.URI]
		if doc == nil {
			return SemanticTokens{Data: []uint32{}}, nil, true
		}
		return SemanticTokens{Data: semanticTokens(doc.text)}, nil, true
	case "textDocument/documentSymbol":
		var p DocumentParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, err, true
		}
		doc := s.docs[p.TextDocument.URI]
		if doc == nil {
			return []DocumentSymbol{}, nil, true
		}
		return documentSymbols(doc.text), nil, true
	case "$/cancelRequest", "$/setTrace", "workspace/didChangeConfiguration", "workspace/didChangeWatchedFiles":
		return nil, nil, true
	}
	return nil, nil, false
}

func (s *Server) initialize() any {
	return map[string]any{
		"capabilities": map[string]any{
			"positionEncoding": "utf-16",
			"textDocumentSync": map[string]any{
				"openClose": true,
				"change":    2, // incremental
				"save":      map[string]any{"includeText": false},
			},
			"documentFormattingProvider": true,
			"documentSymbolProvider":     true,
			"semanticTokensProvider": map[string]any{
				"legend": map[string]any{
					"tokenTypes":     semanticTokenTypes,
					"tokenModifiers": semanticTokenModifiers,
				},
				"full": true,
			},
		},
		"serverInfo": map[string]any{"name": "oct-lsp", "version": "0.1.0"},
	}
}

func isOctSource(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".oct" || ext == ".octest"
}

// isPackageEntry reports whether a file can be loaded as a compilation entry.
// manifest.oct is package metadata evaluated by the package manager, not a
// program; loading it as an entry reports a misleading metadata error, so it
// gets the syntax pass only.
func isPackageEntry(path string) bool {
	return isOctSource(path) && !strings.EqualFold(filepath.Base(path), "manifest.oct")
}

func (s *Server) didOpen(p DidOpenParams) {
	doc := &document{
		uri:     p.TextDocument.URI,
		path:    uriToPath(p.TextDocument.URI),
		text:    p.TextDocument.Text,
		version: p.TextDocument.Version,
	}
	s.docs[doc.uri] = doc
	if !isOctSource(doc.path) {
		return
	}
	if isPackageEntry(doc.path) {
		s.runPackagePass(doc.path)
	}
	s.publishFor(doc)
}

func (s *Server) didChange(p DidChangeParams) {
	doc := s.docs[p.TextDocument.URI]
	if doc == nil {
		return
	}
	for _, c := range p.ContentChanges {
		doc.text = applyChange(doc.text, c)
	}
	doc.version = p.TextDocument.Version
	if isOctSource(doc.path) {
		s.publishFor(doc)
	}
}

func (s *Server) didSave(p DidSaveParams) {
	doc := s.docs[p.TextDocument.URI]
	if doc == nil || !isOctSource(doc.path) {
		return
	}
	if p.Text != nil {
		doc.text = *p.Text
	}
	if isPackageEntry(doc.path) {
		s.runPackagePass(doc.path)
	}
	s.publishFor(doc)
}

func (s *Server) didClose(p DidCloseParams) {
	delete(s.docs, p.TextDocument.URI)
	_ = s.conn.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{URI: p.TextDocument.URI, Diagnostics: []Diagnostic{}})
}

// runPackagePass type-checks the package of path from disk and publishes the
// result for every affected file that is not open (open files are published
// by publishFor, merged with their live syntax diagnostics).
func (s *Server) runPackagePass(path string) {
	dir := filepath.Dir(path)
	result := packageDiagnostics(path, s.readFile)
	// Forget previous results for this package directory.
	for p := range s.pkgDiags {
		if sameFile(filepath.Dir(p), dir) {
			delete(s.pkgDiags, p)
		}
	}
	for p, ds := range result {
		s.pkgDiags[filepath.Clean(p)] = ds
	}
	// Publish closed files: new errors, and clears for fixed ones.
	targets := map[string]bool{}
	for p := range result {
		targets[filepath.Clean(p)] = true
	}
	for p := range s.published {
		if sameFile(filepath.Dir(p), dir) {
			targets[p] = true
		}
	}
	for p := range targets {
		if s.openDoc(p) != nil {
			continue
		}
		ds := s.pkgDiags[p]
		if ds == nil {
			ds = []Diagnostic{}
		}
		_ = s.conn.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{URI: pathToURI(p), Diagnostics: ds})
		if len(ds) > 0 {
			s.published[p] = true
		} else {
			delete(s.published, p)
		}
	}
}

// publishFor sends an open document's diagnostics. A syntax error in the live
// buffer supersedes stale package results for that file; otherwise the last
// package pass is shown.
func (s *Server) publishFor(doc *document) {
	diags := syntaxDiagnostics(doc.path, doc.text)
	if len(diags) == 0 {
		for p, ds := range s.pkgDiags {
			if sameFile(p, doc.path) {
				diags = append(diags, ds...)
			}
		}
	}
	if diags == nil {
		diags = []Diagnostic{}
	}
	version := doc.version
	_ = s.conn.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{URI: doc.uri, Version: &version, Diagnostics: diags})
}

func (s *Server) openDoc(path string) *document {
	for _, d := range s.docs {
		if sameFile(d.path, path) {
			return d
		}
	}
	return nil
}

// readFile prefers the editor buffer, falling back to disk.
func (s *Server) readFile(path string) (string, bool) {
	if d := s.openDoc(path); d != nil {
		return d.text, true
	}
	return readFromDisk(path)
}

func (s *Server) formatting(p DocumentParams) ([]TextEdit, error) {
	doc := s.docs[p.TextDocument.URI]
	if doc == nil || !isOctSource(doc.path) {
		return []TextEdit{}, nil
	}
	formatted, err := ocfmt.FormatSource(doc.text)
	if err != nil {
		// Unformattable source (usually a syntax error) is not a protocol
		// failure; the syntax diagnostic already explains it.
		fmt.Fprintf(s.log, "oct lsp: format %s: %v\n", doc.path, err)
		return []TextEdit{}, nil
	}
	if formatted == doc.text {
		return []TextEdit{}, nil
	}
	ix := newLineIndex(doc.text)
	return []TextEdit{{Range: ix.rangeOf(0, len(doc.text)), NewText: formatted}}, nil
}

// documentSymbols lists top-level declarations for the outline view.
func documentSymbols(text string) []DocumentSymbol {
	tokens := lexTokens("buffer.oct", text)
	if tokens == nil {
		return []DocumentSymbol{}
	}
	ix := newLineIndex(text)
	kinds := map[lex.TokenKind]struct {
		symbol int
		label  string
	}{
		lex.KeywordFn:      {symbolKindFunction, "fn"},
		lex.KeywordRecord:  {symbolKindStruct, "record"},
		lex.KeywordEnum:    {symbolKindEnum, "enum"},
		lex.KeywordConcept: {symbolKindInterface, "concept"},
		lex.KeywordFlow:    {symbolKindEvent, "flow"},
	}
	var out []DocumentSymbol
	depth := 0
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		switch t.Kind {
		case lex.LeftBrace:
			depth++
			continue
		case lex.RightBrace:
			depth--
			continue
		}
		k, ok := kinds[t.Kind]
		if !ok || depth != 0 || i+1 >= len(tokens) || tokens[i+1].Kind != lex.Identifier {
			continue
		}
		name := tokens[i+1]
		end := name.EndOffset
		d := 0
		for j := i + 2; j < len(tokens); j++ {
			if tokens[j].Kind == lex.LeftBrace {
				d++
			} else if tokens[j].Kind == lex.RightBrace {
				d--
				if d == 0 {
					end = tokens[j].EndOffset
					break
				}
			}
		}
		out = append(out, DocumentSymbol{
			Name:           name.Lexeme,
			Detail:         k.label,
			Kind:           k.symbol,
			Range:          ix.rangeOf(t.Offset, end),
			SelectionRange: ix.rangeOf(name.Offset, name.EndOffset),
		})
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Range.Start.Line != out[b].Range.Start.Line {
			return out[a].Range.Start.Line < out[b].Range.Start.Line
		}
		return out[a].Range.Start.Character < out[b].Range.Start.Character
	})
	if out == nil {
		out = []DocumentSymbol{}
	}
	return out
}
