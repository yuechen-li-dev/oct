package diagnostic

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestDiagnosticCarriesStructuredLocationsAndRendersSafely(t *testing.T) {
	d := Diagnostic{Path: "test.sdslv", Code: "SDSL-V1103", Severity: SeverityError, Message: "duplicate", Span: source.Span{Start: source.Position{Line: 2, Column: 1}, End: source.Position{Line: 3, Column: 2}}, Related: []Related{{Message: "first is here", Span: source.Span{Start: source.Position{Line: 1, Column: 1}}}}}
	got := Render(d)
	for _, want := range []string{"test.sdslv:2:1: error SDSL-V1103: duplicate", "note: first is here at test.sdslv:1:1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render() = %q, want %q", got, want)
		}
	}
	if !strings.Contains(Render(Diagnostic{Code: "SDSL-V1000", Message: "unknown"}), "<unknown>: error SDSL-V1000") {
		t.Fatal("unknown span was not rendered honestly")
	}
}

func TestDiagnosticOrderingIsDeterministic(t *testing.T) {
	values := []Diagnostic{{Path: "b", Code: "B", Span: source.Span{Start: source.Position{Offset: 2}}}, {Path: "a", Code: "Z", Span: source.Span{Start: source.Position{Offset: 3}}}, {Path: "a", Code: "A", Span: source.Span{Start: source.Position{Offset: 3}}}}
	Sort(values)
	if values[0].Path != "a" || values[0].Code != "A" || values[2].Path != "b" {
		t.Fatalf("unexpected order: %#v", values)
	}
}
