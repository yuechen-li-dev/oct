package lowering

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/textmetrics"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// Stack and grid sugar (Libraries/UI/DESIGN.md §5.5, §5.6): arrangement
// specs, child sizing, and the measured sizes behind UI.Auto, measured
// nested stacks, wrapped text and auto grid tracks. Every measurement goes
// through the session's text metrics.

// Button chrome around its label.
const (
	buttonPadX     = 24.0
	buttonPadY     = 12.0
	buttonMinWidth = 80.0
)

func stackArrange(node *uiir.Node, axis layout.Axis) *layout.ArrangeSpec {
	a := &layout.ArrangeSpec{Kind: layout.StackArrange, Axis: axis}
	if st := node.Stack; st != nil {
		a.Gap = st.Gap
		a.Padding = layout.EdgeInsets{Top: st.Padding.Top, Right: st.Padding.Right, Bottom: st.Padding.Bottom, Left: st.Padding.Left}
		a.Align = layout.ArrangeAlign(st.Align)
		a.Justify = layout.ArrangeJustify(st.Justify)
	}
	return a
}

// stackAxis reports the main axis of a stack parent.
func stackAxis(parent *uiir.Node) (layout.Axis, bool) {
	if parent == nil {
		return "", false
	}
	switch parent.Kind {
	case uiir.NodeColumn:
		return layout.AxisVertical, true
	case uiir.NodeRow:
		return layout.AxisHorizontal, true
	}
	return "", false
}

func sizingName(kind uiir.SizingKind) string {
	switch kind {
	case uiir.SizingFixed:
		return "Fixed"
	case uiir.SizingFill:
		return "Fill"
	case uiir.SizingAuto:
		return "Auto"
	default:
		return "Size"
	}
}

func textSpecOf(node *uiir.Node) uiir.TextSpec {
	if node.TextSpec == nil {
		return textmetrics.Body
	}
	spec := *node.TextSpec
	if spec.Size <= 0 {
		spec.Size = textmetrics.Body.Size
	}
	if spec.Overflow == "" {
		spec.Overflow = uiir.OverflowEllipsis
	}
	return spec
}

func isWrapped(node *uiir.Node) bool {
	return node.Kind == uiir.NodeText && node.TextSpec != nil && node.TextSpec.Wrap
}

// leafSize is a Text or Button's measured size on one line.
func leafSize(node *uiir.Node, m textmetrics.Metrics) (float64, float64) {
	line := m.LineHeight(textmetrics.Body)
	switch node.Kind {
	case uiir.NodeCheckbox:
		// The box, a gap, then the label.
		return CheckboxBox + CheckboxGap + m.Width(node.Label, textmetrics.Body), line + inputPadY
	case uiir.NodeTextField:
		return InputWidth, line + 2*TextFieldPadY
	case uiir.NodeSlider:
		return InputWidth, line + inputPadY
	}
	if node.Kind == uiir.NodeButton {
		w := m.Width(node.Label, textmetrics.Body) + buttonPadX
		if w < buttonMinWidth {
			w = buttonMinWidth
		}
		return w, m.LineHeight(textmetrics.Body) + buttonPadY
	}
	spec := textSpecOf(node)
	w := m.Width(node.Text, spec)
	if w <= 0 {
		w = 1
	}
	return w, m.LineHeight(spec)
}

// defaultSizing is how an unwrapped stack child sizes when its default
// depends on content: nested stacks are measured; wrapped text is measured
// in a vertical stack and fills a horizontal one. Other children keep their
// own frames (measured leaves; filling spacers, grids and canvases).
func defaultSizing(node *uiir.Node, axis layout.Axis) (uiir.SizingSpec, bool) {
	switch {
	case node.Kind == uiir.NodeColumn || node.Kind == uiir.NodeRow:
		return uiir.SizingSpec{Kind: uiir.SizingAuto}, true
	case isWrapped(node) && axis == layout.AxisHorizontal:
		return uiir.SizingSpec{Kind: uiir.SizingFill, Weight: 1}, true
	case isWrapped(node):
		return uiir.SizingSpec{Kind: uiir.SizingAuto}, true
	}
	return uiir.SizingSpec{}, false
}

// heightMeasure is a MeasuredFrame's measurer: the node's height at the
// width the vertical stack gives it.
type heightMeasure struct {
	node    *uiir.Node
	metrics textmetrics.Metrics
}

func (h *heightMeasure) HeightForWidth(width float64) (float64, error) {
	return heightFor(h.node, width, h.metrics)
}

// sizingFrame is a stack child's frame from its sizing. A fixed or measured
// main size leaves the cross axis to the stack (it stretches), except that a
// one-line Text or Button keeps its measured cross size and is placed by the
// stack's Align. In a vertical stack, a measured container or wrapped text
// takes the stack's width and is as tall as its content at that width.
func sizingFrame(node *uiir.Node, z uiir.SizingSpec, axis layout.Axis, m textmetrics.Metrics) (layout.FrameSpec, error) {
	switch z.Kind {
	case uiir.SizingFixed:
		if axis == layout.AxisHorizontal {
			return layout.FrameSpec{Kind: layout.FixedFrame, Width: z.Main}, nil
		}
		return layout.FrameSpec{Kind: layout.FixedFrame, Height: z.Main}, nil
	case uiir.SizingFill:
		return layout.FrameSpec{Kind: layout.FillFrame, Weight: z.Weight}, nil
	case uiir.SizingSize:
		return layout.FrameSpec{Kind: layout.FixedFrame, Width: z.Width, Height: z.Height}, nil
	}
	if (node.Kind == uiir.NodeText && !isWrapped(node)) || isControl(node) {
		w, h := leafSize(node, m)
		return layout.FrameSpec{Kind: layout.FixedFrame, Width: w, Height: h}, nil
	}
	if axis == layout.AxisVertical {
		// Fail now, with this node's path, if it cannot be measured at all.
		if _, err := heightFor(node, 1e9, m); err != nil {
			return layout.FrameSpec{}, err
		}
		return layout.FrameSpec{Kind: layout.MeasuredFrame, Measure: &heightMeasure{node: node, metrics: m}}, nil
	}
	w, _, err := intrinsic(node, m)
	if err != nil {
		return layout.FrameSpec{}, err
	}
	return layout.FrameSpec{Kind: layout.FixedFrame, Width: w}, nil
}

func unmeasurable(node *uiir.Node) error {
	return fmt.Errorf("lowering.err.measure_unsupported: node %q (%s) has no measured size; give it UI.Fixed or UI.Size", node.NodeID, kindLabel(node))
}

// intrinsic measures the size a node asks for when nothing constrains it:
// its content on one line (wrapped text measures as one line here).
func intrinsic(node *uiir.Node, m textmetrics.Metrics) (float64, float64, error) {
	if z := node.Sizing; z != nil && z.Kind == uiir.SizingSize {
		return z.Width, z.Height, nil
	}
	switch node.Kind {
	case uiir.NodeText, uiir.NodeButton, uiir.NodeCheckbox, uiir.NodeTextField, uiir.NodeSlider:
		w, h := leafSize(node, m)
		return w, h, nil
	case uiir.NodeSpacer, uiir.NodeSkip:
		return 0, 0, nil
	case uiir.NodeColumn, uiir.NodeRow:
		horizontal := node.Kind == uiir.NodeRow
		main, cross := 0.0, 0.0
		for i, child := range node.Children {
			w, h, err := intrinsic(child, m)
			if err != nil {
				return 0, 0, err
			}
			if z := child.Sizing; z != nil && z.Kind == uiir.SizingFixed {
				if horizontal {
					w = z.Main
				} else {
					h = z.Main
				}
			}
			if horizontal {
				main, cross = main+w, max(cross, h)
			} else {
				main, cross = main+h, max(cross, w)
			}
			if i > 0 && node.Stack != nil {
				main += node.Stack.Gap
			}
		}
		if st := node.Stack; st != nil {
			if horizontal {
				main += st.Padding.Left + st.Padding.Right
				cross += st.Padding.Top + st.Padding.Bottom
			} else {
				main += st.Padding.Top + st.Padding.Bottom
				cross += st.Padding.Left + st.Padding.Right
			}
		}
		if horizontal {
			return main, cross, nil
		}
		return cross, main, nil
	case uiir.NodeGrid:
		if node.Grid == nil {
			return 0, 0, fmt.Errorf("lowering.err.measure_grid: node %q is a uniform UI.GridRows grid, which has no measured size; give it UI.Fixed or UI.Size", node.NodeID)
		}
		cols, rows, err := gridTracks(node, m)
		if err != nil {
			return 0, 0, err
		}
		for _, t := range append(append([]layout.GridTrack{}, cols...), rows...) {
			if t.Kind == layout.GridTrackFill {
				return 0, 0, fmt.Errorf("lowering.err.measure_grid: grid %q has a fill track, which has no measured size; give the grid UI.Fill or UI.Fixed", node.NodeID)
			}
		}
		w := trackSum(cols, node.Grid.ColumnGap) + node.Grid.Padding.Left + node.Grid.Padding.Right
		h := trackSum(rows, node.Grid.RowGap) + node.Grid.Padding.Top + node.Grid.Padding.Bottom
		return w, h, nil
	default:
		return 0, 0, unmeasurable(node)
	}
}

// heightFor is a node's height at a given width: wrapped text wraps to it,
// and stacks pass it on to their children as layout would.
func heightFor(node *uiir.Node, width float64, m textmetrics.Metrics) (float64, error) {
	if z := node.Sizing; z != nil && z.Kind == uiir.SizingSize {
		return z.Height, nil
	}
	switch node.Kind {
	case uiir.NodeText:
		spec := textSpecOf(node)
		if spec.Wrap {
			return float64(len(textmetrics.Wrap(m, node.Text, spec, width))) * m.LineHeight(spec), nil
		}
		return m.LineHeight(spec), nil
	case uiir.NodeColumn:
		pad, gap := uiir.EdgeInsets{}, 0.0
		if node.Stack != nil {
			pad, gap = node.Stack.Padding, node.Stack.Gap
		}
		inner := width - pad.Left - pad.Right
		total := pad.Top + pad.Bottom
		for i, child := range node.Children {
			if i > 0 {
				total += gap
			}
			if z := child.Sizing; z != nil && z.Kind == uiir.SizingFixed {
				total += z.Main
				continue
			}
			h, err := heightFor(child, inner, m)
			if err != nil {
				return 0, err
			}
			total += h
		}
		return total, nil
	case uiir.NodeRow:
		pad, gap := uiir.EdgeInsets{}, 0.0
		if node.Stack != nil {
			pad, gap = node.Stack.Padding, node.Stack.Gap
		}
		inner := width - pad.Left - pad.Right
		widths := make([]float64, len(node.Children))
		weights := make([]float64, len(node.Children))
		fixed, weight := 0.0, 0.0
		for i, child := range node.Children {
			if i > 0 {
				fixed += gap
			}
			z := child.Sizing
			if z == nil {
				if d, ok := defaultSizing(child, layout.AxisHorizontal); ok {
					z = &d
				}
			}
			switch {
			case z != nil && z.Kind == uiir.SizingFixed:
				widths[i] = z.Main
			case z != nil && z.Kind == uiir.SizingSize:
				widths[i] = z.Width
			case z != nil && z.Kind == uiir.SizingFill:
				weights[i] = z.Weight
			case child.Kind == uiir.NodeSpacer || child.Kind == uiir.NodeGrid || child.Kind == uiir.NodeAbsoluteBox || child.Kind == uiir.NodeAnchorBox:
				weights[i] = 1
			default:
				w, _, err := intrinsic(child, m)
				if err != nil {
					return 0, err
				}
				widths[i] = w
			}
			fixed += widths[i]
			weight += weights[i]
		}
		remaining := inner - fixed
		tallest := 0.0
		for i, child := range node.Children {
			w := widths[i]
			if weights[i] > 0 && weight > 0 && remaining > 0 {
				w = remaining * weights[i] / weight
			}
			h, err := heightFor(child, w, m)
			if err != nil {
				return 0, err
			}
			tallest = max(tallest, h)
		}
		return tallest + pad.Top + pad.Bottom, nil
	default:
		_, h, err := intrinsic(node, m)
		return h, err
	}
}

// trackSum adds fixed track sizes and gaps.
func trackSum(tracks []layout.GridTrack, gap float64) float64 {
	total := 0.0
	for i, t := range tracks {
		if t.Kind == layout.GridTrackFixed {
			total += t.Size
		}
		if i > 0 {
			total += gap
		}
	}
	return total
}

// gridTracks lowers a grid's declared tracks, turning each auto track into
// a fixed one as wide (or tall) as its largest single-span cell.
func gridTracks(node *uiir.Node, m textmetrics.Metrics) ([]layout.GridTrack, []layout.GridTrack, error) {
	convert := func(tracks []uiir.GridTrack, column bool) ([]layout.GridTrack, error) {
		out := make([]layout.GridTrack, len(tracks))
		for i, t := range tracks {
			if t.Kind != uiir.GridTrackAuto {
				out[i] = layout.GridTrack{Kind: layout.GridTrackKind(t.Kind), Size: t.Size, Weight: t.Weight}
				continue
			}
			size := 0.0
			for _, child := range node.Children {
				c := child.Cell
				if c == nil {
					continue
				}
				index, span := c.Row, c.RowSpan
				if column {
					index, span = c.Column, c.ColumnSpan
				}
				if index != int64(i) || span != 1 {
					continue
				}
				w, h, err := intrinsic(child, m)
				if err != nil {
					return nil, err
				}
				if column {
					size = max(size, w)
				} else {
					size = max(size, h)
				}
			}
			out[i] = layout.GridTrack{Kind: layout.GridTrackFixed, Size: size}
		}
		return out, nil
	}
	cols, err := convert(node.Grid.Columns, true)
	if err != nil {
		return nil, nil, err
	}
	rows, err := convert(node.Grid.Rows, false)
	if err != nil {
		return nil, nil, err
	}
	return cols, rows, nil
}

// Control geometry, in dp. Render draws inputs inside the same boxes.
const (
	CheckboxBox   = 18.0
	CheckboxGap   = 8.0
	InputWidth    = 200.0
	TextFieldPadX = 8.0
	TextFieldPadY = 6.0
	SliderThumb   = 16.0
	inputPadY     = 4.0
)

// isControl reports a button or input: a leaf measured on both axes that
// the session can focus.
func isControl(node *uiir.Node) bool {
	switch node.Kind {
	case uiir.NodeButton, uiir.NodeCheckbox, uiir.NodeTextField, uiir.NodeSlider:
		return true
	}
	return false
}
