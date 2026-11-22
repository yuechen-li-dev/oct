package textmetrics

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func TestFixedMetricsMatchTheHistoricalTextMetric(t *testing.T) {
	if w := (Fixed{}).Width("Raise", Body); w != 40 {
		t.Fatalf("body width = %g, want 8 dp per rune", w)
	}
	if h := (Fixed{}).LineHeight(Body); h != 20 {
		t.Fatalf("body line height = %g, want 20", h)
	}
	if w := (Fixed{}).Width("°C", Body); w != 16 {
		t.Fatalf("width counts runes, not bytes: %g", w)
	}
	if w := (Fixed{}).Width("ab", uiir.TextSpec{Size: 10, Mono: true}); w != 12 {
		t.Fatalf("mono width = %g", w)
	}
}

func TestWrapBreaksAtSpacesAndInsideOverlongWords(t *testing.T) {
	m := Fixed{}
	got := Wrap(m, "one two three four", Body, 80) // 10 runes per line
	if strings.Join(got, "|") != "one two|three four" {
		t.Fatalf("wrap = %q", got)
	}
	got = Wrap(m, "abcdefghijklmnop qr", Body, 64) // 8 runes per line
	if strings.Join(got, "|") != "abcdefgh|ijklmnop|qr" {
		t.Fatalf("long word = %q", got)
	}
	got = Wrap(m, "a\n\nb", Body, 64)
	if strings.Join(got, "|") != "a||b" {
		t.Fatalf("newlines = %q", got)
	}
	if got := Wrap(m, "xyz", Body, 1); strings.Join(got, "|") != "x|y|z" {
		t.Fatalf("narrower than one rune = %q", got)
	}
}

func TestFitAddsAnEllipsisOnlyWhenItMustCut(t *testing.T) {
	m := Fixed{}
	if got := Fit(m, "short", Body, 80); got != "short" {
		t.Fatalf("fits = %q", got)
	}
	if got := Fit(m, "a long label", Body, 64); got != "a long…" || m.Width(got, Body) > 64 {
		t.Fatalf("cut = %q", got)
	}
	if got := Fit(m, "abc", Body, 4); got != "" {
		t.Fatalf("no room for an ellipsis = %q", got)
	}
}
