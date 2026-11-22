package render

import (
	"math"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/textmetrics"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// Default input chrome. Inputs are drawn from plain fills and text, so a
// backend needs no widget code of its own beyond FillCheck.
var (
	colorAccent     = uiir.RGBA{R: 59, G: 110, B: 214, A: 255}
	colorFieldBG    = uiir.RGBA{R: 255, G: 255, B: 255, A: 255}
	colorFieldEdge  = uiir.RGBA{R: 160, G: 166, B: 178, A: 255}
	colorHoverEdge  = uiir.RGBA{R: 110, G: 118, B: 134, A: 255}
	colorTrack      = uiir.RGBA{R: 214, G: 218, B: 225, A: 255}
	colorWhite      = uiir.RGBA{R: 255, G: 255, B: 255, A: 255}
	colorText       = uiir.RGBA{R: 20, G: 22, B: 28, A: 255}
	focusRingWidth  = 2.0
	sliderTrack     = 4.0
	sliderThumb     = lowering.SliderThumb
	caretWidth      = 1.5
	textFieldRadius = 4.0
)

// styleFor is the style a node paints with in the current interaction:
// its state style when it declares one for a state that holds (disabled,
// then pressed, then focused, then hovered), else its normal style.
func (e emitter) styleFor(id layout.NodeID) (lowering.Style, bool) {
	style, styled := e.styles[id]
	states, ok := e.options.States[id]
	if !ok {
		return style, styled
	}
	in := e.options.Interaction
	switch {
	case e.semantics[id].Disabled:
		return lowering.Style(states.Disabled), true
	case in.Pressed == id:
		return lowering.Style(states.Pressed), true
	case in.Focus == id:
		return lowering.Style(states.Focused), true
	case in.Hover == id:
		return lowering.Style(states.Hover), true
	}
	return style, styled
}

func (e emitter) fill(id layout.NodeID, r layout.Rect, p Paint) {
	*e.out = append(*e.out, FillRectCommand{NodeID: id, Rect: r, Fill: FillStyled, Paint: &p})
}

// focusRing outlines the focused control, unless a state style already
// paints its focus.
func (e emitter) focusRing(id layout.NodeID, r layout.Rect, styled bool) {
	if e.options.Interaction.Focus != id {
		return
	}
	if _, ok := e.options.States[id]; ok {
		return
	}
	e.fill(id, r, Paint{Border: colorAccent, BorderWidth: focusRingWidth, Radius: textFieldRadius})
}

func (e emitter) emitInput(id layout.NodeID, r layout.Rect, sem lowering.Semantics, styled bool, textColor *uiir.RGBA) {
	in := e.options.Interaction
	m := e.options.Metrics
	edge := colorFieldEdge
	if in.Hover == id || in.Pressed == id {
		edge = colorHoverEdge
	}
	switch sem.Role {
	case lowering.RoleCheckbox:
		box := layout.Rect{X: r.X, Y: r.Y + (r.Height-lowering.CheckboxBox)/2, Width: lowering.CheckboxBox, Height: lowering.CheckboxBox}
		if sem.Input.Checked {
			e.fill(id, box, Paint{Background: colorAccent, Radius: 3})
			*e.out = append(*e.out, FillRectCommand{NodeID: id, Rect: box, Fill: FillCheck, Paint: &Paint{Background: colorWhite}})
		} else {
			e.fill(id, box, Paint{Background: colorFieldBG, Border: edge, BorderWidth: 1.5, Radius: 3})
		}
		if sem.Label != "" {
			labelX := lowering.CheckboxBox + lowering.CheckboxGap
			rect := layout.Rect{X: r.X + labelX, Y: r.Y, Width: math.Max(0, r.Width-labelX), Height: r.Height}
			label := textmetrics.Fit(m, sem.Label, textmetrics.Body, rect.Width)
			*e.out = append(*e.out, DrawTextCommand{NodeID: id, Rect: centeredLine(rect, m), Text: label, Color: textColor})
		}
		// Outset, so the ring shows around a checked (accent) box.
		e.focusRing(id, layout.Rect{X: box.X - 3, Y: box.Y - 3, Width: box.Width + 6, Height: box.Height + 6}, styled)
	case lowering.RoleTextField:
		focused := in.Focus == id
		if !styled {
			p := Paint{Background: colorFieldBG, Border: edge, BorderWidth: 1, Radius: textFieldRadius}
			if focused {
				p.Border, p.BorderWidth = colorAccent, focusRingWidth
			}
			e.fill(id, r, p)
		}
		inner := layout.Rect{X: r.X + lowering.TextFieldPadX, Y: r.Y, Width: math.Max(0, r.Width-2*lowering.TextFieldPadX), Height: r.Height}
		value := sem.Input.Value
		caret := clampCaret(in.Caret, value)
		caretX := m.Width(string([]rune(value)[:caret]), textmetrics.Body)
		// Scroll so the caret stays visible in a field narrower than its
		// text; otherwise show the text from its start.
		scroll := 0.0
		if focused && caretX > inner.Width-caretWidth {
			scroll = caretX - inner.Width + caretWidth
		}
		*e.out = append(*e.out, PushClipCommand{NodeID: id, Rect: inner})
		if value != "" {
			line := centeredLine(inner, m)
			line.X -= scroll
			line.Width = m.Width(value, textmetrics.Body) + 1
			*e.out = append(*e.out, DrawTextCommand{NodeID: id, Rect: line, Text: value, Color: textColor})
		}
		if focused {
			line := centeredLine(inner, m)
			color := colorText
			if textColor != nil {
				color = *textColor
			}
			e.fill(id, layout.Rect{X: inner.X + caretX - scroll, Y: line.Y, Width: caretWidth, Height: line.Height}, Paint{Background: color})
		}
		*e.out = append(*e.out, PopClipCommand{NodeID: id})
	case lowering.RoleSlider:
		in := sem.Input
		t := (in.Number - in.Min) / (in.Max - in.Min)
		usable := math.Max(0, r.Width-sliderThumb)
		cx := r.X + sliderThumb/2 + t*usable
		cy := r.Y + r.Height/2
		track := layout.Rect{X: r.X + sliderThumb/2, Y: cy - sliderTrack/2, Width: usable, Height: sliderTrack}
		e.fill(id, track, Paint{Background: colorTrack, Radius: sliderTrack / 2})
		e.fill(id, layout.Rect{X: track.X, Y: track.Y, Width: cx - track.X, Height: sliderTrack}, Paint{Background: colorAccent, Radius: sliderTrack / 2})
		thumb := layout.Rect{X: cx - sliderThumb/2, Y: cy - sliderThumb/2, Width: sliderThumb, Height: sliderThumb}
		thumbEdge := colorWhite
		if e.options.Interaction.Hover == id || e.options.Interaction.Pressed == id {
			thumbEdge = colorTrack
		}
		e.fill(id, thumb, Paint{Background: colorAccent, Border: thumbEdge, BorderWidth: 2, Radius: sliderThumb / 2})
		e.focusRing(id, r, styled)
	}
}

// centeredLine is one body line vertically centered in r.
func centeredLine(r layout.Rect, m textmetrics.Metrics) layout.Rect {
	h := m.LineHeight(textmetrics.Body)
	return layout.Rect{X: r.X, Y: r.Y + (r.Height-h)/2, Width: r.Width, Height: h}
}

func clampCaret(caret int, value string) int {
	n := len([]rune(value))
	if caret < 0 {
		return 0
	}
	if caret > n {
		return n
	}
	return caret
}
