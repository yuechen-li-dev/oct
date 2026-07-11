package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestSdslvStructuredDiagnosticsUsePreciseSpans(t *testing.T) {
	text := "[Fact][Fact]\nfn T(x: u32) -> void { let y: u32 = missing; Assert.True(1u); return; }\n"
	tokens, err := lex.Analyze(source.File{Path: "test.sdslvtest", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := Diagnostics(module)
	want := map[string]string{"SDSL-V1103": "[Fact]", "SDSL-V1105": "x: u32", "SDSL-V1501": "missing", "SDSL-V1402": "1u"}
	for code, slice := range want {
		found := false
		for _, d := range diagnostics {
			if d.Code == code {
				found = true
				if !d.Span.Known() || text[d.Span.Start.Offset:d.Span.End.Offset] != slice {
					t.Fatalf("%s span = %q, want %q", code, text[d.Span.Start.Offset:d.Span.End.Offset], slice)
				}
				if code == "SDSL-V1103" && len(d.Related) != 1 {
					t.Fatalf("duplicate has %#v related locations", d.Related)
				}
			}
		}
		if !found {
			t.Fatalf("missing diagnostic %s: %#v", code, diagnostics)
		}
	}
}

// This guard distinguishes structured ordinary source diagnostics from normal
// Go errors used by file and toolchain boundaries.
func TestSdslvValidatorHasNoStringOnlyUserErrors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("validate.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"errors.New(", "errors         []string", "strings.Join(v.errors"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("validator still has string-only diagnostic collector %q", forbidden)
		}
	}
	if !strings.Contains(text, "func Diagnostics(module ast.Module) []diagnostic.Diagnostic") {
		t.Fatal("structured validator API missing")
	}
}

func TestSdslvDiscoveryHasNoIndependentSemanticValidator(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "test", "discovery.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"discoverRegex", "func testAttributes", "func attributeLaunch", "func literalMatches"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("discovery retains semantic authority %q", forbidden)
		}
	}
	if !strings.Contains(text, "validate.ValidatedTests(module)") {
		t.Fatal("discovery does not consume validated declarations")
	}
}

func TestSdslvLegacyErrorAdapterUsesStructuredDiagnostic(t *testing.T) {
	text := "fn Bad() -> void { let x: u32 = missing; }\n"
	tokens, err := lex.Analyze(source.File{Path: "adapter.sdslv", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := Diagnostics(module)
	err = Module(module)
	if len(diagnostics) == 0 || err == nil {
		t.Fatalf("Diagnostics() = %#v, Module() = %v", diagnostics, err)
	}
	if !strings.Contains(err.Error(), diagnostics[0].Code) || !strings.Contains(err.Error(), diagnostics[0].Message) {
		t.Fatalf("adapter %q does not render authoritative diagnostic %#v", err, diagnostics[0])
	}
}

func TestSdslvM29DiagnosticCodesAndSpans(t *testing.T) {
	cases := []struct{ name, text, code, slice string }{
		{"theory rows", "[Theory]\nfn T(x: u32) -> void {}\n", "SDSL-V1107", "[Theory]"},
		{"inline arity", "[Theory]\n[InlineData(1u, 2u)]\nfn T(x: u32) -> void {}\n", "SDSL-V1202", "[InlineData(1u, 2u)]"},
		{"inline constant", "[Theory]\n[InlineData(x)]\nfn T(x: u32) -> void {}\n", "SDSL-V1204", "x"},
		{"workgroup argument", "[Fact]\n[WorkgroupSize(0u, 1u, 1u)]\nfn T() -> void {}\n", "SDSL-V1304", "0u"},
		{"assert arity", "[Fact]\nfn T() -> void { Assert.Equal(1u); }\n", "SDSL-V1401", "Assert.Equal(1u)"},
		{"assert member", "[Fact]\nfn T() -> void { Assert.Missing(1u); }\n", "SDSL-V1405", "Assert.Missing(1u)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := lex.Analyze(source.File{Path: "test.sdslvtest", Text: tc.text})
			if err != nil {
				t.Fatal(err)
			}
			module, err := parse.BuildModule(tokens)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range Diagnostics(module) {
				if d.Code == tc.code {
					if got := tc.text[d.Span.Start.Offset:d.Span.End.Offset]; got != tc.slice {
						t.Fatalf("%s span = %q, want %q", tc.code, got, tc.slice)
					}
					return
				}
			}
			t.Fatalf("missing %s in %#v", tc.code, Diagnostics(module))
		})
	}
}
