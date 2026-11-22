package render

import (
	"fmt"
	"math"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/textmetrics"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// buttonLabelInset is the horizontal room a button's chrome keeps around
// its label (lowering adds the same 24 dp when it measures a button).
const buttonLabelInset = 24.0

// emitText draws a Text node inside its rect under its overflow policy
// (DESIGN.md §5.3, law 9). Text is wrapped and truncated here with the same
// metrics layout measured it with, so a backend only draws lines that fit:
//
//   - Ellipsis shortens a line that is too wide, and ends the last visible
//     line of wrapped text with an ellipsis when lines are left over;
//   - Clip draws what fits and clips the rest to the rect;
//   - Error fails the frame, naming the node and the shortfall.
func emitText(m textmetrics.Metrics, id layout.NodeID, rect layout.Rect, sem lowering.Semantics, styled *uiir.RGBA, out *[]Command) error {
	spec := textmetrics.Body
	if sem.Text != nil {
		spec = *sem.Text
	}
	color := styled
	if spec.Color != nil {
		color = spec.Color
	}
	base := DrawTextCommand{NodeID: id, Color: color}
	if spec.Size != textmetrics.Body.Size {
		base.Size = spec.Size
	}
	if spec.Weight != textmetrics.Body.Weight {
		base.Weight = spec.Weight
	}
	base.Mono = spec.Mono
	lineHeight := m.LineHeight(spec)

	lines := []string{sem.Label}
	if spec.Wrap {
		lines = textmetrics.Wrap(m, sem.Label, spec, rect.Width)
	}
	visible := len(lines)
	if spec.Wrap {
		if room := int(math.Floor(rect.Height/lineHeight + 1e-9)); room < visible {
			visible = max(room, 1)
		}
	}
	tooWide := false
	for _, line := range lines[:visible] {
		if m.Width(line, spec) > rect.Width+1e-9 {
			tooWide = true
		}
	}
	cut := visible < len(lines)
	if spec.Overflow == uiir.OverflowError && (tooWide || cut || (!spec.Wrap && lineHeight > rect.Height+1e-9)) {
		if cut {
			return fmt.Errorf("render.err.text_overflow: node %q wraps to %d lines but its %gx%g box has room for %d; give it more room or set Overflow to Ellipsis or Clip", id, len(lines), rect.Width, rect.Height, visible)
		}
		return fmt.Errorf("render.err.text_overflow: node %q text %q needs %g x %g but its box is %g x %g; give it more room or set Overflow to Ellipsis or Clip", id, sem.Label, m.Width(sem.Label, spec), lineHeight, rect.Width, rect.Height)
	}
	clip := spec.Overflow == uiir.OverflowClip && (tooWide || cut)
	if clip {
		*out = append(*out, PushClipCommand{NodeID: id, Rect: rect})
	}
	for i, line := range lines[:visible] {
		if spec.Overflow == uiir.OverflowEllipsis {
			if cut && i == visible-1 {
				line = textmetrics.Fit(m, line+textmetrics.Ellipsis, spec, rect.Width)
			} else {
				line = textmetrics.Fit(m, line, spec, rect.Width)
			}
		}
		cmd := base
		cmd.Text = line
		cmd.Rect = rect
		if spec.Wrap {
			cmd.Rect = layout.Rect{X: rect.X, Y: rect.Y + float64(i)*lineHeight, Width: rect.Width, Height: lineHeight}
		}
		*out = append(*out, cmd)
	}
	if clip {
		*out = append(*out, PopClipCommand{NodeID: id})
	}
	return nil
}
