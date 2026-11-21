package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testClient drives Serve over in-memory pipes with real LSP framing.
type testClient struct {
	t        *testing.T
	toServer io.WriteCloser
	in       *bufio.Reader
	nextID   int
	pending  []map[string]json.RawMessage // notifications received while waiting
	done     chan error
	inbox    chan map[string]json.RawMessage
}

func startServer(t *testing.T) *testClient {
	t.Helper()
	cr, sw := io.Pipe() // server -> client
	sr, cw := io.Pipe() // client -> server
	c := &testClient{t: t, toServer: cw, in: bufio.NewReader(cr), done: make(chan error, 1), inbox: make(chan map[string]json.RawMessage, 256)}
	go func() {
		err := Serve(sr, sw, io.Discard)
		sw.Close()
		c.done <- err
	}()
	// Drain server output continuously, as a real editor does; io.Pipe is
	// unbuffered, so an unread notification would otherwise block the server.
	go func() {
		defer close(c.inbox)
		for {
			h, err := textproto.NewReader(c.in).ReadMIMEHeader()
			if err != nil {
				return
			}
			n, _ := strconv.Atoi(h.Get("Content-Length"))
			b := make([]byte, n)
			if _, err := io.ReadFull(c.in, b); err != nil {
				return
			}
			var m map[string]json.RawMessage
			if json.Unmarshal(b, &m) == nil {
				c.inbox <- m
			}
		}
	}()
	t.Cleanup(func() {
		c.send("exit", nil, false)
		cw.Close()
		select {
		case <-c.done:
		case <-time.After(5 * time.Second):
			t.Errorf("server did not exit")
		}
	})
	return c
}

func (c *testClient) send(method string, params any, request bool) int {
	m := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		m["params"] = params
	}
	id := 0
	if request {
		c.nextID++
		id = c.nextID
		m["id"] = id
	}
	body, _ := json.Marshal(m)
	fmt.Fprintf(c.toServer, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return id
}

func (c *testClient) readMsg() map[string]json.RawMessage {
	c.t.Helper()
	select {
	case m, ok := <-c.inbox:
		if !ok {
			c.t.Fatalf("server closed the stream")
		}
		return m
	case <-time.After(60 * time.Second):
		c.t.Fatalf("timed out waiting for server message")
	}
	return nil
}

func (c *testClient) request(method string, params any) json.RawMessage {
	c.t.Helper()
	id := c.send(method, params, true)
	for {
		m := c.readMsg()
		if raw, ok := m["id"]; ok && string(raw) == strconv.Itoa(id) {
			if e, ok := m["error"]; ok {
				c.t.Fatalf("%s error: %s", method, e)
			}
			return m["result"]
		}
		c.pending = append(c.pending, m)
	}
}

// diagnosticsFor waits for the next publishDiagnostics notification for uri.
func (c *testClient) diagnosticsFor(uri string) []Diagnostic {
	c.t.Helper()
	match := func(m map[string]json.RawMessage) ([]Diagnostic, bool) {
		var method string
		_ = json.Unmarshal(m["method"], &method)
		if method != "textDocument/publishDiagnostics" {
			return nil, false
		}
		var p PublishDiagnosticsParams
		_ = json.Unmarshal(m["params"], &p)
		if p.URI != uri {
			return nil, false
		}
		return p.Diagnostics, true
	}
	for i, m := range c.pending {
		if d, ok := match(m); ok {
			c.pending = append(c.pending[:i], c.pending[i+1:]...)
			return d
		}
	}
	for {
		m := c.readMsg()
		if d, ok := match(m); ok {
			return d
		}
		c.pending = append(c.pending, m)
	}
}

// fixturePackage copies testdata/<name>/*.oct.fixture into a temp package
// directory under their real names and returns the directory and texts.
func fixturePackage(t *testing.T, name string) (string, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join("testdata", name)
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	texts := map[string]string{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".fixture") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		real := strings.TrimSuffix(e.Name(), ".fixture")
		texts[real] = string(b)
		if err := os.WriteFile(filepath.Join(dir, real), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, texts
}

func fixtureText(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (c *testClient) open(path, text string) string {
	uri := pathToURI(path)
	c.send("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": "oct", "version": 1, "text": text,
	}}, false)
	return uri
}

func initialized(t *testing.T) *testClient {
	c := startServer(t)
	res := c.request("initialize", map[string]any{"capabilities": map[string]any{}})
	if !strings.Contains(string(res), `"semanticTokensProvider"`) || !strings.Contains(string(res), `"unit"`) {
		t.Fatalf("initialize missing semantic token legend: %s", res)
	}
	c.send("initialized", map[string]any{}, false)
	return c
}

// rangeText returns the source text a diagnostic range covers.
func rangeText(text string, r Range) string {
	ix := newLineIndex(text)
	return text[ix.offsetOfPosition(r.Start):ix.offsetOfPosition(r.End)]
}

func TestUnitMismatchIsPinnedToTheBinding(t *testing.T) {
	dir, files := fixturePackage(t, "unit_mismatch")
	src := files["main.oct"]
	c := initialized(t)
	uri := c.open(filepath.Join(dir, "main.oct"), src)
	diags := c.diagnosticsFor(uri)
	if len(diags) != 1 {
		t.Fatalf("want 1 diagnostic, got %+v", diags)
	}
	d := diags[0]
	if !strings.Contains(d.Message, "cannot add m and s") {
		t.Fatalf("message = %q", d.Message)
	}
	if got := rangeText(src, d.Range); got != "z" || d.Range.Start.Line != 5 {
		t.Fatalf("range covers %q on line %d, want binding z on line 5", got, d.Range.Start.Line)
	}
	if len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Message != "in function Main" {
		t.Fatalf("related info = %+v", d.RelatedInformation)
	}
}

func TestUndefinedVariableIsPinnedToItsUse(t *testing.T) {
	dir, files := fixturePackage(t, "undefined")
	src := files["main.oct"]
	c := initialized(t)
	uri := c.open(filepath.Join(dir, "main.oct"), src)
	d := c.diagnosticsFor(uri)
	if len(d) != 1 || rangeText(src, d[0].Range) != "undefinedThing" {
		t.Fatalf("diagnostics = %+v", d)
	}
}

func TestSyntaxErrorIsExactAndLiveWithUnitSymbols(t *testing.T) {
	dir, files := fixturePackage(t, "syntax")
	good := files["main.oct"]
	c := initialized(t)
	path := filepath.Join(dir, "main.oct")
	uri := c.open(path, good)
	if d := c.diagnosticsFor(uri); len(d) != 0 {
		t.Fatalf("clean file has diagnostics: %+v", d)
	}
	// Non-ASCII before the error: a byte- or rune-based column would be off.
	bad := files["edited.oct"]
	c.send("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": 2},
		"contentChanges": []map[string]any{{"text": bad}},
	}, false)
	d := c.diagnosticsFor(uri)
	if len(d) != 1 || !strings.Contains(d[0].Message, "expected arrow before return type") {
		t.Fatalf("diagnostics = %+v", d)
	}
	if got := rangeText(bad, d[0].Range); got != "{" || d[0].Range.Start.Line != 2 {
		t.Fatalf("syntax range covers %q at %+v", got, d[0].Range)
	}
}

func TestIncrementalChangeWithMultibyteText(t *testing.T) {
	text := "let a = 20 °C\nlet b = 1\n"
	ix := newLineIndex(text)
	// Replace "1" on line 1 (UTF-16 col 8) with "2".
	got := applyChange(text, ContentChange{Range: &Range{Start: Position{1, 8}, End: Position{1, 9}}, Text: "2"})
	if got != "let a = 20 °C\nlet b = 2\n" {
		t.Fatalf("applyChange = %q", got)
	}
	// "°" is one UTF-16 unit; "C" after it is at character 12.
	if p := ix.position(strings.Index(text, "C")); p != (Position{0, 12}) {
		t.Fatalf("position of C = %+v", p)
	}
}

func TestErrorInAnotherFileIsPublishedToThatFile(t *testing.T) {
	dir, files := fixturePackage(t, "multifile")
	main, helper := files["main.oct"], files["helper.oct"]
	c := initialized(t)
	c.open(filepath.Join(dir, "main.oct"), main)
	d := c.diagnosticsFor(pathToURI(filepath.Join(dir, "helper.oct")))
	if len(d) != 1 || rangeText(helper, d[0].Range) != "x" || !strings.Contains(d[0].Message, "expected Int, got String") {
		t.Fatalf("helper diagnostics = %+v", d)
	}
}

func TestFormattingReturnsAWholeDocumentEdit(t *testing.T) {
	dir, files := fixturePackage(t, "format")
	src := files["main.oct"]
	c := initialized(t)
	uri := c.open(filepath.Join(dir, "main.oct"), src)
	c.diagnosticsFor(uri)
	res := c.request("textDocument/formatting", map[string]any{"textDocument": map[string]any{"uri": uri}, "options": map[string]any{"tabSize": 4, "insertSpaces": true}})
	var edits []TextEdit
	if err := json.Unmarshal(res, &edits); err != nil || len(edits) != 1 {
		t.Fatalf("edits = %s (%v)", res, err)
	}
	if !strings.Contains(edits[0].NewText, "    return 0") {
		t.Fatalf("formatted text = %q", edits[0].NewText)
	}
}

func decodeTokens(text string, data []uint32) map[string][]string {
	ix := newLineIndex(text)
	out := map[string][]string{}
	line, char := 0, 0
	for i := 0; i+5 <= len(data); i += 5 {
		dl, dc, n, kind := int(data[i]), int(data[i+1]), int(data[i+2]), int(data[i+3])
		if dl > 0 {
			line, char = line+dl, dc
		} else {
			char += dc
		}
		start := ix.offsetOfPosition(Position{line, char})
		end := ix.offsetOfPosition(Position{line, char + n})
		name := tokenTypeName(kind)
		out[name] = append(out[name], text[start:end])
	}
	return out
}

func TestSemanticTokensMarkUnitsParametersAndContextualKeywords(t *testing.T) {
	src := fixtureText(t, "semantic.oct.fixture")
	got := decodeTokens(src, semanticTokens(src))
	has := func(kind, text string) {
		t.Helper()
		for _, v := range got[kind] {
			if v == text {
				return
			}
		}
		t.Errorf("%s tokens %q lack %q", kind, got[kind], text)
	}
	has("unit", "m")
	has("unit", "s")
	has("unit", "K")
	has("unit", "C")
	has("keyword", "template")
	has("keyword", "record")
	has("parameter", "x")
	has("parameter", "k")
	has("function", "Scale")
	has("type", "Probe")
	has("property", "Mass")
	has("variable", "g")
	has("number", "9.81")
	// `B: 0 A: 255`: A labels the next field; it is not amperes.
	has("property", "A")
	for _, v := range got["unit"] {
		if v == "A" {
			t.Errorf("field label A misclassified as a unit")
		}
		if v == "g" || v == "T" {
			t.Errorf("variable %q misclassified as unit", v)
		}
	}
}

func TestDocumentSymbolsListTopLevelDeclarations(t *testing.T) {
	syms := documentSymbols(fixtureText(t, "symbols.oct.fixture"))
	if len(syms) != 2 || syms[0].Name != "Probe" || syms[1].Name != "Main" || syms[1].Kind != symbolKindFunction {
		t.Fatalf("symbols = %+v", syms)
	}
}

func TestUnknownMethodsAreRejectedNotDropped(t *testing.T) {
	c := initialized(t)
	id := c.send("textDocument/hover", map[string]any{}, true)
	for {
		m := c.readMsg()
		if string(m["id"]) == strconv.Itoa(id) {
			if !strings.Contains(string(m["error"]), "-32601") {
				t.Fatalf("hover reply = %v", m)
			}
			return
		}
	}
}

func TestWindowsFileURIs(t *testing.T) {
	if got := uriToPath("file:///c%3A/Users/My%20Name/oct/main.oct"); filepath.ToSlash(got) != "c:/Users/My Name/oct/main.oct" {
		t.Fatalf("uriToPath = %q", got)
	}
}

func TestManifestErrorsLandOnTheManifestFunction(t *testing.T) {
	dir, files := fixturePackage(t, "manifest")
	manifest, main := files["manifest.oct"], files["main.oct"]
	c := initialized(t)
	c.open(filepath.Join(dir, "main.oct"), main)
	d := c.diagnosticsFor(pathToURI(filepath.Join(dir, "manifest.oct")))
	if len(d) != 1 || rangeText(manifest, d[0].Range) != "Manifest" || d[0].Range.Start.Line != 2 {
		t.Fatalf("manifest diagnostics = %+v", d)
	}
}

// The locator is a judgment over competing message signals. These cases pin
// its decisions; the message texts are synthetic locator inputs, not claims
// about the type checker's wording.
func TestLocatorPrefersTheLeadingClause(t *testing.T) {
	text := fixtureText(t, "locator/main.oct.fixture")
	tokens := lexTokens("main.oct", text)
	nameTok, start, end, ok := findDeclaration(tokens, "function", "Main")
	if !ok {
		t.Fatal("Main not found")
	}
	cases := []struct {
		rest   string
		winner string
		want   string
		line   int
	}{
		// The binding leads; the callee is context.
		{"let y: function 'Sqrt' argument 1 expects Float, got String", "binding", "y", 3},
		// Only a callee is named; Sqrt is called twice, the first call wins.
		{"function 'Sqrt' expects 1 argument", "callee", "Sqrt", 3},
		// Nothing in the body matches: fall back to the declaration name.
		{"unsupported construct", "declaration", "Main", 2},
		// A named binding that does not exist is ineligible, not invented.
		{"let missing: expected Int", "declaration", "Main", 2},
	}
	for _, tc := range cases {
		tok, trace := chooseTarget(tokens, start, end, nameTok, tc.rest)
		ix := newLineIndex(text)
		r := ix.rangeOf(tok.Offset, tok.EndOffset)
		if trace.Winner != tc.winner || tok.Lexeme != tc.want || r.Start.Line != tc.line {
			t.Errorf("%q: winner %s token %q line %d; want %s %q line %d (trace %+v)",
				tc.rest, trace.Winner, tok.Lexeme, r.Start.Line, tc.winner, tc.want, tc.line, trace)
		}
		if len(trace.Candidates) != 5 {
			t.Errorf("%q: trace has %d candidates, want 5", tc.rest, len(trace.Candidates))
		}
	}
}

func TestTypeCheckDiagnosticsCarryTheLocatorTrace(t *testing.T) {
	dir, files := fixturePackage(t, "unit_mismatch")
	c := initialized(t)
	uri := c.open(filepath.Join(dir, "main.oct"), files["main.oct"])
	d := c.diagnosticsFor(uri)
	raw, _ := json.Marshal(d[0].Data)
	if !strings.Contains(string(raw), `"winner":"binding"`) {
		t.Fatalf("diagnostic data = %s", raw)
	}
}
