//go:build machina_gio

package gio

import (
	"image"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// measureScale shapes text at 4 px per dp so measurements keep quarter-dp
// precision; Gio reports whole pixels.
const measureScale = 4

// Metrics measures text with Gio's shaper and the Go font collection, the
// same faces the window draws with, so layout, wrapping and truncation use
// the widths the backend renders.
type Metrics struct {
	shaper  *text.Shaper
	widths  map[metricsKey]float64
	heights map[metricsKey]float64
}

type metricsKey struct {
	text   string
	size   float64
	weight int
	mono   bool
}

// NewShaper is the shaper both drawing and measuring use.
func NewShaper() *text.Shaper {
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
}

func NewMetrics() *Metrics {
	return &Metrics{shaper: NewShaper(), widths: map[metricsKey]float64{}, heights: map[metricsKey]float64{}}
}

// FontFor is the Gio font for a text spec.
func FontFor(spec uiir.TextSpec) font.Font {
	f := font.Font{}
	if spec.Mono {
		f.Typeface = "Go Mono"
	}
	if spec.Weight > 0 {
		f.Weight = font.Weight(spec.Weight - 400)
	}
	return f
}

func specSize(spec uiir.TextSpec) float64 {
	if spec.Size > 0 {
		return spec.Size
	}
	return 16
}

func (m *Metrics) dims(s string, spec uiir.TextSpec) (float64, float64) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: measureScale, PxPerSp: measureScale},
		Constraints: layout.Constraints{Max: image.Pt(1<<24, 1<<24)},
	}
	l := widget.Label{MaxLines: 1}
	d := l.Layout(gtx, m.shaper, FontFor(spec), unit.Sp(specSize(spec)), s, op.CallOp{})
	return float64(d.Size.X) / measureScale, float64(d.Size.Y) / measureScale
}

func (m *Metrics) Width(s string, spec uiir.TextSpec) float64 {
	if s == "" {
		return 0
	}
	k := metricsKey{text: s, size: specSize(spec), weight: spec.Weight, mono: spec.Mono}
	if w, ok := m.widths[k]; ok {
		return w
	}
	w, _ := m.dims(s, spec)
	m.widths[k] = w
	return w
}

func (m *Metrics) LineHeight(spec uiir.TextSpec) float64 {
	k := metricsKey{size: specSize(spec), weight: spec.Weight, mono: spec.Mono}
	if h, ok := m.heights[k]; ok {
		return h
	}
	_, h := m.dims("Ag", spec)
	m.heights[k] = h
	return h
}
