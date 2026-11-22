//go:build machina_gio

package gio

import (
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/textmetrics"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func TestGioMetricsMeasureTheFontTheWindowDraws(t *testing.T) {
	m := NewMetrics()
	body := textmetrics.Body
	short, long := m.Width("Raise", body), m.Width("Raise the setpoint", body)
	if short <= 0 || long <= short {
		t.Fatalf("widths %g, %g: longer text must be wider", short, long)
	}
	if big := m.Width("Raise", uiir.TextSpec{Size: 32}); big < 1.8*short {
		t.Fatalf("32 dp text %g should be about twice 16 dp text %g", big, short)
	}
	if mono, prop := m.Width("iiii", uiir.TextSpec{Size: 16, Mono: true}), m.Width("iiii", body); mono <= prop {
		t.Fatalf("monospace i (%g) should be wider than proportional i (%g)", mono, prop)
	}
	if h := m.LineHeight(body); h < 16 || h > 24 {
		t.Fatalf("16 dp line height %g", h)
	}
	t.Logf("body: %q=%g, line=%g; fixed would be %g", "Raise the setpoint", long, m.LineHeight(body), textmetrics.Fixed{}.Width("Raise the setpoint", body))
}
