package ocfmt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestFormatSourceIdempotent(t *testing.T) {
	input := "package Main\n\nfn main()->Int{\n    let x=1+2\n    return x\n}\n"
	first, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format first: %v", err)
	}
	second, err := FormatSource(first)
	if err != nil {
		t.Fatalf("format second: %v", err)
	}
	if first != second {
		t.Fatalf("format should be idempotent\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestFormatSourceConvergence(t *testing.T) {
	left := "package Main\nfn sum(a:Int,b:Int)->Int{\nreturn a+b\n}\n"
	right := "package   Main\nfn sum( a : Int, b : Int ) -> Int {\n    return a + b\n}\n"
	outLeft, err := FormatSource(left)
	if err != nil {
		t.Fatalf("format left: %v", err)
	}
	outRight, err := FormatSource(right)
	if err != nil {
		t.Fatalf("format right: %v", err)
	}
	if outLeft != outRight {
		t.Fatalf("expected convergence\nleft:\n%s\nright:\n%s", outLeft, outRight)
	}
}

func TestFormatSourcePreservesComments(t *testing.T) {
	input := "package Main\n\n///   Adds values\nfn add(a:Int,b:Int)->Int{ // inline comment\n// ordinary comment\nreturn a+b\n}\n"
	out, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	mustContain(t, out, "///   Adds values")
	mustContain(t, out, "// inline comment")
	mustContain(t, out, "// ordinary comment")
}

// `->` and `=>` are one token. The formatter has no opinion about which one
// an author writes unless it is asked for one.
func TestFormatSourceKeepsEachArrowAsWritten(t *testing.T) {
	input := "package Main\nfn Main()=>Int{return switch 1 { case 1 => 2 else -> 3 }}\nfn Other()->Int{return 1}\n"
	for _, options := range []Options{{}, {Arrows: ArrowsKeep}, {Mode: ModeEnLLMCompact}} {
		out, err := FormatSourceWithOptions(input, options)
		if err != nil {
			t.Fatalf("%+v: format: %v", options, err)
		}
		compact := strings.ReplaceAll(out, " ", "")
		for _, want := range []string{"fnMain()=>Int", "case1=>2", "else->3", "fnOther()->Int"} {
			if !strings.Contains(compact, want) {
				t.Errorf("%+v: expected %q to survive, got:\n%s", options, want, out)
			}
		}
	}
}

func TestFormatSourceWritesOneArrowSpellingWhenAsked(t *testing.T) {
	input := "package Main\nfn Main()=>Int{return switch 1 { case 1 => 2 else -> 3 }}\n"
	cases := []struct {
		arrows       Arrows
		want, absent string
	}{
		{ArrowsThin, "fn Main() -> Int { return switch 1 { case 1 -> 2 else -> 3 } }", "=>"},
		{ArrowsFat, "fn Main() => Int { return switch 1 { case 1 => 2 else => 3 } }", "->"},
	}
	for _, c := range cases {
		out, err := FormatSourceWithOptions(input, Options{Arrows: c.arrows})
		if err != nil {
			t.Fatalf("%s: format: %v", c.arrows, err)
		}
		mustContain(t, out, c.want)
		if strings.Contains(out, c.absent) {
			t.Errorf("%s: %q is still present:\n%s", c.arrows, c.absent, out)
		}
		again, err := FormatSourceWithOptions(out, Options{})
		if err != nil || again != out {
			t.Errorf("%s: the default mode changed the result: %v\n%s", c.arrows, err, again)
		}
	}
}

func TestFormatSourceRejectsAnUnknownArrowsSetting(t *testing.T) {
	_, err := FormatSourceWithOptions("package Main\n", Options{Arrows: "double"})
	if err == nil || !strings.Contains(err.Error(), "expected keep|thin|fat") {
		t.Fatalf("expected the arrows setting to be rejected, got %v", err)
	}
}

// A comparison written next to an assignment is not an arrow, and an arrow
// setting does not touch it.
func TestArrowsSettingLeavesComparisonsAlone(t *testing.T) {
	input := "package Main\nfn Main() -> Bool {\n    let a = 1\n    return a >= 0 and a <= 2\n}\n"
	for _, arrows := range []Arrows{ArrowsKeep, ArrowsThin, ArrowsFat} {
		out, err := FormatSourceWithOptions(input, Options{Arrows: arrows})
		if err != nil {
			t.Fatalf("%s: %v", arrows, err)
		}
		mustContain(t, out, "return a >= 0 and a <= 2")
	}
}

func TestFormatPathDirectory(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.oct")
	f2 := filepath.Join(dir, "b.octest")
	f3 := filepath.Join(dir, "c.octfail")
	f4 := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(f1, []byte("package Main\nfn main()->Int{return 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("package Main\n[Fact]\nfn test()->Void{return}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f3, []byte("expect error: \"boom\"\n\npackage Main\nfn Main()->Int{return 1kg + 2s}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f4, []byte("no change"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := FormatPath(dir); err != nil {
		t.Fatalf("format path: %v", err)
	}
	got1, _ := os.ReadFile(f1)
	got2, _ := os.ReadFile(f2)
	got3, _ := os.ReadFile(f3)
	got4, _ := os.ReadFile(f4)
	mustContain(t, string(got1), "fn main() -> Int")
	mustContain(t, string(got2), "[Fact]")
	mustContain(t, string(got2), "fn test() -> Void")
	mustContain(t, string(got3), "expect error: \"boom\"")
	mustContain(t, string(got3), "fn Main() -> Int")
	if string(got4) != "no change" {
		t.Fatalf("non-oct file changed")
	}
}

func TestFormatPathSingleFileRespectsExtension(t *testing.T) {
	dir := t.TempDir()
	octPath := filepath.Join(dir, "a.oct")
	octestPath := filepath.Join(dir, "b.octest")
	octfailPath := filepath.Join(dir, "c.octfail")

	if err := os.WriteFile(octPath, []byte("package Main\nfn main()->Int{return 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(octestPath, []byte("package Main\n[Fact]\nfn test()->Void{return}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(octfailPath, []byte("expect error: \"bad\"\n\npackage Main\nfn Main()->Int{return 1kg+2s}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{octPath, octestPath, octfailPath} {
		if err := FormatPath(path); err != nil {
			t.Fatalf("format %s: %v", path, err)
		}
	}

	gotOct, _ := os.ReadFile(octPath)
	gotOctest, _ := os.ReadFile(octestPath)
	gotOctfail, _ := os.ReadFile(octfailPath)
	mustContain(t, string(gotOct), "fn main() -> Int")
	mustContain(t, string(gotOctest), "[Fact]")
	mustContain(t, string(gotOctest), "fn test() -> Void")
	mustContain(t, string(gotOctfail), "expect error: \"bad\"")
	mustContain(t, string(gotOctfail), "fn Main() -> Int")
}

func TestFormatPathDirectoryIdempotentForMixedTypes(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.oct":     "package Main\nfn main() -> Int {\n    return 1\n}\n",
		"b.octest":  "package Main\n[Fact]\nfn test() -> Void {\n    return\n}\n",
		"c.octfail": "expect error: \"boom\"\n\npackage Main\nfn Main() -> Int {\n    return 1kg + 2s\n}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := FormatPath(dir); err != nil {
		t.Fatalf("first format path: %v", err)
	}

	first := map[string]string{}
	for name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		first[name] = string(data)
	}

	if err := FormatPath(dir); err != nil {
		t.Fatalf("second format path: %v", err)
	}
	for name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if first[name] != string(data) {
			t.Fatalf("expected %s to be stable after second format", name)
		}
	}
}

func TestFormatSourceRejectsInvalidSource(t *testing.T) {
	if _, err := FormatSource("package Main\nfn bad( {\n"); err == nil {
		t.Fatalf("expected parse/lex error")
	}
}

func mustContain(t *testing.T, s string, sub string) {
	t.Helper()
	if !contains(s, sub) {
		t.Fatalf("expected output to contain %q\n%s", sub, s)
	}
}

func contains(s string, sub string) bool {
	return strings.Contains(s, sub)
}

func parseBuild(path, text string) error {
	lexed, err := lex.Analyze(source.File{Path: path, Text: text})
	if err != nil {
		return err
	}
	_, err = parse.BuildFile(lexed)
	return err
}

func TestFormatSourceModesRoundTripParse(t *testing.T) {
	input := "package Main\nfn main()->Int{return sum(1+2,3)}\n"
	for _, mode := range []Mode{ModeReadable, ModeCompact, ModeEnLLM, ModeEnLLMCompact} {
		out, err := FormatSourceWithOptions(input, Options{Mode: mode})
		if err != nil {
			t.Fatalf("format mode %s: %v", mode, err)
		}
		if err := parseBuild("<test>.oct", out); err != nil {
			t.Fatalf("parse after format mode %s: %v\n%s", mode, err, out)
		}
	}
}

func TestFormatPathCheck(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.oct")
	if err := os.WriteFile(f, []byte("package Main\nfn main()->Int{return 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := FormatPathWithOptions(f, Options{Mode: ModeReadable, Check: true}); err == nil {
		t.Fatalf("expected check failure for unformatted file")
	}
	if err := FormatPathWithOptions(f, Options{Mode: ModeReadable}); err != nil {
		t.Fatal(err)
	}
	if err := FormatPathWithOptions(f, Options{Mode: ModeReadable, Check: true}); err != nil {
		t.Fatal(err)
	}
}

func TestEnLLMDoesNotAutoWrapHeavyCalls(t *testing.T) {
	input := "package Main\nfn main()->Void{\nlet report=Markdown.Report([Markdown.H1(\"t\"),Markdown.Section(\"Metrics\",[Markdown.Table(rows)]),Markdown.Callout(\"warning\",[\"a\",\"b\"])])\nreturn\n}\n"
	out, err := FormatSourceWithOptions(input, Options{Mode: ModeEnLLM})
	if err != nil {
		t.Fatalf("format en-llm: %v", err)
	}
	mustContain(t, out, "let report = Markdown.Report([Markdown.H1(\"t\"), Markdown.Section(\"Metrics\"")
	if strings.Contains(out, "let report = Markdown.Report(\n") {
		t.Fatalf("expected no multiline rewrite:\n%s", out)
	}
}

func TestModeIdempotenceReadableAndCompact(t *testing.T) {
	input := "package Main\nfn main()->Int{return Markdown.Report([Markdown.H1(\"t\"),Markdown.Section(\"S\",[Markdown.Table(rows)])])}\n"
	for _, mode := range []Mode{ModeEnLLM, ModeEnLLMCompact} {
		first, err := FormatSourceWithOptions(input, Options{Mode: mode})
		if err != nil {
			t.Fatalf("first format mode %s: %v", mode, err)
		}
		second, err := FormatSourceWithOptions(first, Options{Mode: mode})
		if err != nil {
			t.Fatalf("second format mode %s: %v", mode, err)
		}
		if first != second {
			t.Fatalf("mode %s is not idempotent", mode)
		}
	}
}

func TestEnLLMSimpleCallStaysInline(t *testing.T) {
	input := "package Main\nfn main()->Void{\nlet x=Markdown.H1(\"Title\")\nreturn\n}\n"
	out, err := FormatSourceWithOptions(input, Options{Mode: ModeEnLLM})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, out, "let x = Markdown.H1(\"Title\")")
}

func TestEnLLMCommentPreservedWithoutAutoWrap(t *testing.T) {
	input := "package Main\nfn main()->Void{\nlet report=Markdown.Report([Markdown.H1(\"t\"),Markdown.Section(\"Metrics\",[Markdown.Table(rows)])]) // keep\nreturn\n}\n"
	out, _, err := formatSourceWithDiagnostics(input, Options{Mode: ModeEnLLM})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, out, "// keep")
	mustContain(t, out, "let report = Markdown.Report(")
	mustContain(t, out, "// keep")
	if strings.Contains(out, "let report = Markdown.Report(\n") {
		t.Fatalf("expected no multiline rewrite:\n%s", out)
	}
}

func TestNoJudgmentTraceWhenAutoWrapDisabled(t *testing.T) {
	input := "package Main\nfn main()->Void{\nlet report=Markdown.Report([Markdown.H1(\"t\"),Markdown.Section(\"Metrics\",[Markdown.Table(rows)]),Markdown.Callout(\"warning\",[\"m\"])])\nreturn\n}\n"
	_, diag, err := formatSourceWithDiagnostics(input, Options{Mode: ModeEnLLM})
	if err != nil {
		t.Fatal(err)
	}
	if len(diag.Traces) != 0 {
		t.Fatalf("expected no traces while auto-wrap is disabled")
	}
}

func TestParametricSyntaxKeepsTypeAnglesAndContextualSelectorReadable(t *testing.T) {
	input := "package Main\nrecord Job{ID:String}\ntemplate record Keyed<Record,Key>{KeyOf:Selector<Record,Key>}\nfn Main()->Void{let x=Keyed<Job,String>{KeyOf:.ID}}\n"
	out, err := FormatSource(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"template record Keyed<Record, Key>", "Selector<Record, Key>", "Keyed<Job, String> {", "KeyOf: .ID"} {
		mustContain(t, out, want)
	}
	if strings.Contains(out, "Keyed <") || strings.Contains(out, "Selector <") {
		t.Fatalf("parametric angles formatted as comparisons:\n%s", out)
	}
}

func TestOctXMLMarkupIndentationIsReadableAndIdempotent(t *testing.T) {
	input := "package Main\nrecord Node{Text:String Children:Node[]}\nfn MarkupTextNode(value:String)->Node{return Node{Text:value Children:[]}}\nfn Panel(children:Node[])->Node{return Node{Text:\"\" Children:children}}\nfn Main()->Node{\nreturn <Panel>\nHello\n<Panel />\n</Panel>\n}\n"
	first, err := FormatSource(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FormatSource(first)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("Oct-XML formatting is not idempotent:\n%s", first)
	}
	mustContain(t, first, "        Hello\n        <Panel />\n    </Panel>")
}

// An .octfail may be a contract for a parser error. It cannot be formatted,
// and it must not stop the files around it from being formatted or checked.
func TestFormatPathLeavesUnparsableOctFailAlone(t *testing.T) {
	dir := t.TempDir()
	unparsable := "expect error: \"expected parameter name\"\n\npackage Main\nfn bad( {\n"
	bad := filepath.Join(dir, "a_parse_contract.octfail")
	good := filepath.Join(dir, "b.oct")
	if err := os.WriteFile(bad, []byte(unparsable), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, []byte("package Main\nfn main()->Int{return 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := FormatPath(dir); err != nil {
		t.Fatalf("format directory: %v", err)
	}
	if got, _ := os.ReadFile(bad); string(got) != unparsable {
		t.Fatalf("unparsable .octfail was rewritten:\n%s", got)
	}
	got, _ := os.ReadFile(good)
	mustContain(t, string(got), "fn main() -> Int { return 1 }")
	if err := FormatPathWithOptions(dir, Options{Check: true}); err != nil {
		t.Fatalf("check after format: %v", err)
	}
	if err := FormatPath(bad); err != nil {
		t.Fatalf("format the .octfail by itself: %v", err)
	}
}

// A source file that does not parse is refused, but every other file in the
// directory is still formatted and every refusal is reported.
func TestFormatPathDirectoryFormatsTheRestAndReportsEveryRefusal(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a_broken.oct": "package Main\nfn bad( {\n",
		"b_good.oct":   "package Main\nfn main()->Int{return 1}\n",
		"c_broken.oct": "package Main\nfn worse( {\n",
		"d_good.oct":   "package Main\nfn other()->Int{return 2}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := FormatPath(dir)
	if err == nil {
		t.Fatal("expected the broken files to be reported")
	}
	for _, name := range []string{"a_broken.oct", "c_broken.oct"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("refusal of %s is not reported: %v", name, err)
		}
		if got, _ := os.ReadFile(filepath.Join(dir, name)); string(got) != files[name] {
			t.Errorf("%s was rewritten", name)
		}
	}
	for _, name := range []string{"b_good.oct", "d_good.oct"} {
		got, _ := os.ReadFile(filepath.Join(dir, name))
		mustContain(t, string(got), "-> Int { return")
	}
}

// The formatter writes "\n" line endings and keeps trailing blank lines.
func TestFormatSourceLineEndings(t *testing.T) {
	out, err := FormatSource("package Main\r\nfn main()->Int{\r\nreturn 1\r\n}\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "package Main\nfn main() -> Int {\n    return 1\n}\n\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
	out, err = FormatSource("package Main\nfn main()->Int{return 1}")
	if err != nil {
		t.Fatal(err)
	}
	if want := "package Main\nfn main() -> Int { return 1 }\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// The formatter re-reads its own output and refuses to return it unless the
// tokens are the ones it was given. Handing it tokens that do not belong to
// the text makes that check fire.
func TestFormatLayoutRefusesOutputWithDifferentTokens(t *testing.T) {
	text := "package Main\nfn a() -> Int { return 1 }\n"
	other, err := lex.Analyze(source.File{Path: "<test>.oct", Text: "package Main\nfn b() -> Int { return 1 }\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := formatLayout(text, other.Tokens, nil, settings{}); err == nil || !strings.Contains(err.Error(), "internal error") {
		t.Fatalf("expected an internal error, got %v", err)
	}
	own, err := lex.Analyze(source.File{Path: "<test>.oct", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := formatLayout(text, own.Tokens, nil, settings{}); err != nil {
		t.Fatalf("the text's own tokens were refused: %v", err)
	}
}

func TestVerifyComparesTokensLinesAndUnitJunctions(t *testing.T) {
	text := "package Main\nfn a() -> Float<m> { return 2.0m - 1.0 m }\n"
	lexed, err := lex.Analyze(source.File{Path: "<test>.oct", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	l := &layout{src: text}
	for _, tok := range lexed.Tokens {
		if tok.Kind != lex.EOF {
			l.toks = append(l.toks, tok)
		}
	}
	cases := []struct {
		name string
		out  string
		ok   bool
	}{
		{"unchanged", text, true},
		{"respaced", "package Main\nfn a()->Float<m>{return 2.0m-1.0 m}\n", true},
		{"arrow spelling", strings.Replace(text, "->", "=>", 1), false},
		{"operator changed", strings.Replace(text, " - ", " + ", 1), false},
		{"token dropped", strings.Replace(text, "return ", "", 1), false},
		{"token moved to the next line", strings.Replace(text, "{ return", "{\nreturn", 1), false},
		{"unit joined to its number", strings.Replace(text, "1.0 m", "1.0m", 1), false},
		{"unit split from its number", strings.Replace(text, "2.0m", "2.0 m", 1), false},
		{"does not lex", strings.Replace(text, "2.0m", "\"2.0m", 1), false},
	}
	for _, c := range cases {
		err := l.verify(c.out)
		if c.ok && err != nil {
			t.Errorf("%s: refused: %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}

	// With a spelling asked for, that spelling is the only one accepted.
	l.arrow = "=>"
	if err := l.verify(strings.Replace(text, "->", "=>", 1)); err != nil {
		t.Errorf("the requested arrow spelling was refused: %v", err)
	}
	if err := l.verify(text); err == nil {
		t.Errorf("an arrow left in the other spelling was accepted")
	}
}
