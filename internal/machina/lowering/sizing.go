package lowering

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// Stack and grid sugar (Libraries/UI/DESIGN.md §5.5, §5.6): arrangement
// specs, child sizing, and the measured sizes behind UI.Auto and auto grid
// tracks. Measurement uses the same fixed text metrics as leaf frames.

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

// sizingFrame is a stack child's frame from its UI.Fixed/Fill/Auto/Size.
// A fixed or measured main size leaves the cross axis to the stack (it
// stretches), except that a measured Text or Button keeps its measured
// cross size and is placed by the stack's Align.
func sizingFrame(node *uiir.Node, axis layout.Axis) (layout.FrameSpec, error) {
	z := node.Sizing
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
	default:
		w, h, err := intrinsic(node)
		if err != nil {
			return layout.FrameSpec{}, err
		}
		if node.Kind == uiir.NodeText || node.Kind == uiir.NodeButton {
			return layout.FrameSpec{Kind: layout.FixedFrame, Width: w, Height: h}, nil
		}
		// A measured container takes its content's main size and the
		// stack's cross size, like UI.Fixed.
		if axis == layout.AxisHorizontal {
			return layout.FrameSpec{Kind: layout.FixedFrame, Width: w}, nil
		}
		return layout.FrameSpec{Kind: layout.FixedFrame, Height: h}, nil
	}
}

// intrinsic measures the size a node asks for when nothing constrains it.
func intrinsic(node *uiir.Node) (float64, float64, error) {
	if z := node.Sizing; z != nil && z.Kind == uiir.SizingSize {
		return z.Width, z.Height, nil
	}
	switch node.Kind {
	case uiir.NodeText:
		return textWidth(node.Text), 20, nil
	case uiir.NodeButton:
		return buttonWidth(node.Label), 32, nil
	case uiir.NodeSpacer, uiir.NodeSkip:
		return 0, 0, nil
	case uiir.NodeColumn, uiir.NodeRow:
		horizontal := node.Kind == uiir.NodeRow
		main, cross := 0.0, 0.0
		for i, child := range node.Children {
			w, h, err := intrinsic(child)
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
		cols, rows, err := gridTracks(node)
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
		return 0, 0, fmt.Errorf("lowering.err.measure_unsupported: node %q (%s) has no measured size; give it UI.Fixed or UI.Size", node.NodeID, kindLabel(node))
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
func gridTracks(node *uiir.Node) ([]layout.GridTrack, []layout.GridTrack, error) {
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
				w, h, err := intrinsic(child)
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
