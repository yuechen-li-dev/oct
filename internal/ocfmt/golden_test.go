package ocfmt

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/source"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata from the formatter's current output")

// Each testdata/<name>.oct.in is unformatted source. The formatter's output
// for it is pinned in <name>.oct.golden (readable mode) and
// <name>.oct.compact.golden (compact mode). The fixtures use neutral
// extensions so that no tool mistakes them for package sources.
func goldenCases(t *testing.T) []string {
	t.Helper()
	inputs, err := filepath.Glob(filepath.Join("testdata", "*.oct.in"))
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no golden inputs found: %v", err)
	}
	return inputs
}

func TestGoldenOutput(t *testing.T) {
	modes := []struct {
		mode   Mode
		suffix string
	}{
		{ModeReadable, ".oct.golden"},
		{ModeCompact, ".oct.compact.golden"},
	}
	for _, input := range goldenCases(t) {
		name := strings.TrimSuffix(filepath.Base(input), ".oct.in")
		src, err := os.ReadFile(input)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range modes {
			t.Run(name+"/"+string(m.mode), func(t *testing.T) {
				got, err := FormatSourceWithOptions(string(src), Options{Mode: m.mode})
				if err != nil {
					t.Fatalf("format: %v", err)
				}
				goldenPath := filepath.Join("testdata", name+m.suffix)
				if *updateGolden {
					if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(goldenPath)
				if err != nil {
					t.Fatalf("read golden (run with -update to create it): %v", err)
				}
				// The fixtures' extensions are not covered by .gitattributes, so a
				// checkout that converts line endings hands them over as CRLF.
				if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
					t.Errorf("output differs from %s\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, want)
				}

				again, err := FormatSourceWithOptions(got, Options{Mode: m.mode})
				if err != nil {
					t.Fatalf("format the formatted output: %v", err)
				}
				if again != got {
					t.Errorf("formatting is not idempotent\n--- first ---\n%s\n--- second ---\n%s", got, again)
				}
				assertSameProgram(t, string(src), got)
			})
		}
	}
}

// Readable and compact output are two spellings of one program, so each
// formats to the other.
func TestGoldenModesConverge(t *testing.T) {
	for _, input := range goldenCases(t) {
		name := strings.TrimSuffix(filepath.Base(input), ".oct.in")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			readable, err := FormatSourceWithOptions(string(src), Options{Mode: ModeReadable})
			if err != nil {
				t.Fatal(err)
			}
			compact, err := FormatSourceWithOptions(string(src), Options{Mode: ModeCompact})
			if err != nil {
				t.Fatal(err)
			}
			fromCompact, err := FormatSourceWithOptions(compact, Options{Mode: ModeReadable})
			if err != nil {
				t.Fatalf("format compact output as readable: %v", err)
			}
			if fromCompact != readable {
				t.Errorf("readable(compact(src)) differs from readable(src)\n--- via compact ---\n%s\n--- direct ---\n%s", fromCompact, readable)
			}
		})
	}
}

// assertSameProgram is the formatter's safety contract, checked from outside
// it: the same tokens on the same lines, and no number gains or loses a
// touching unit name.
func assertSameProgram(t *testing.T, before string, after string) {
	t.Helper()
	tokens := func(text string) []lex.Token {
		lexed, err := lex.Analyze(source.File{Path: "<golden>.oct", Text: strings.ReplaceAll(text, "\r\n", "\n")})
		if err != nil {
			t.Fatalf("lex: %v", err)
		}
		return lexed.Tokens
	}
	a, b := tokens(before), tokens(after)
	if len(a) != len(b) {
		t.Fatalf("token count changed from %d to %d", len(a), len(b))
	}
	isNumber := func(tok lex.Token) bool { return tok.Kind == lex.IntLiteral || tok.Kind == lex.FloatLiteral }
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].Line != b[i].Line || (a[i].Lexeme != b[i].Lexeme && a[i].Kind != lex.Arrow) {
			t.Fatalf("token %d changed: %s %q on line %d became %s %q on line %d", i, a[i].Kind, a[i].Lexeme, a[i].Line, b[i].Kind, b[i].Lexeme, b[i].Line)
		}
		if i > 0 && a[i].Kind == lex.Identifier && isNumber(a[i-1]) {
			if (a[i-1].EndOffset == a[i].Offset) != (b[i-1].EndOffset == b[i].Offset) {
				t.Fatalf("the unit name %q on line %d was joined to or split from its number", a[i].Lexeme, a[i].Line)
			}
		}
	}
}
