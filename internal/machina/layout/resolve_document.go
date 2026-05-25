package layout

import (
	"fmt"
	"math"
)

type ResolvedLayoutNode struct {
	ID         NodeID
	Rect       Rect
	Frame      FrameSpec
	Order      int
	Z          int
	DebugLabel string
}

type ResolvedLayoutDocument struct {
	RootID   NodeID
	Nodes    map[NodeID]ResolvedLayoutNode
	Children map[NodeID][]NodeID
}

func ResolveRows(rows []LayoutRow, root Rect) (*ResolvedLayoutDocument, error) {
	doc, err := CompileRows(rows)
	if err != nil {
		return nil, err
	}
	return ResolveDocument(doc, root)
}

func ResolveDocument(document *LayoutDocument, root Rect) (*ResolvedLayoutDocument, error) {
	if document == nil {
		return nil, fmt.Errorf("layout.err.nil_document: layout document is required")
	}
	if err := validateRect(root, "root"); err != nil {
		return nil, err
	}
	rootRow, ok := document.Nodes[document.RootID]
	if !ok {
		return nil, fmt.Errorf("layout.err.missing_root: root node %q is not present in document", document.RootID)
	}
	if rootRow.Frame.Kind != RootFrame {
		return nil, fmt.Errorf("layout.err.root_kind: root node %q must use root frame", rootRow.ID)
	}
	resolved := &ResolvedLayoutDocument{RootID: document.RootID, Nodes: map[NodeID]ResolvedLayoutNode{}, Children: document.Children}
	if err := resolveNode(document, resolved, document.RootID, root); err != nil {
		return nil, err
	}
	return resolved, nil
}

func resolveNode(doc *LayoutDocument, out *ResolvedLayoutDocument, nodeID NodeID, rect Rect) error {
	row := doc.Nodes[nodeID]
	out.Nodes[nodeID] = ResolvedLayoutNode{ID: nodeID, Rect: rect, Frame: row.Frame, Order: row.Order, Z: row.Z, DebugLabel: row.DebugLabel}
	children := doc.Children[nodeID]
	if len(children) == 0 {
		return nil
	}
	if row.Arrange != nil && row.Arrange.Kind == StackArrange {
		return resolveStackChildren(doc, out, nodeID, rect)
	}
	for _, childID := range children {
		child := doc.Nodes[childID]
		childRect, err := resolveFrame(child.Frame, rect)
		if err != nil {
			return fmt.Errorf("layout.err.resolve_frame: node %q: %w", childID, err)
		}
		if err := resolveNode(doc, out, childID, childRect); err != nil {
			return err
		}
	}
	return nil
}

func resolveStackChildren(doc *LayoutDocument, out *ResolvedLayoutDocument, parentID NodeID, parentRect Rect) error {
	arrange := *doc.Nodes[parentID].Arrange
	children := doc.Children[parentID]
	inner := Rect{X: parentRect.X + arrange.Padding.Left, Y: parentRect.Y + arrange.Padding.Top, Width: parentRect.Width - (arrange.Padding.Left + arrange.Padding.Right), Height: parentRect.Height - (arrange.Padding.Top + arrange.Padding.Bottom)}
	if inner.Width < 0 || inner.Height < 0 {
		return fmt.Errorf("layout.err.padding_overflow: parent %q padding exceeds parent size", parentID)
	}
	mainAvailable := inner.Height
	crossAvailable := inner.Width
	if arrange.Axis == AxisHorizontal {
		mainAvailable = inner.Width
		crossAvailable = inner.Height
	}
	if len(children) > 1 {
		mainAvailable -= float64(len(children)-1) * arrange.Gap
	}
	fixedTotal := 0.0
	fillWeight := 0.0
	for _, childID := range children {
		frame := doc.Nodes[childID].Frame
		switch frame.Kind {
		case FixedFrame:
			if arrange.Axis == AxisHorizontal {
				fixedTotal += frame.Width
			} else {
				fixedTotal += frame.Height
			}
		case FillFrame:
			w := frame.Weight
			if w == 0 {
				w = 1
			}
			fillWeight += w
		default:
			return fmt.Errorf("layout.err.stack_frame_kind: parent %q stack supports fixed/fill children only, got %q on child %q", parentID, frame.Kind, childID)
		}
	}
	remaining := mainAvailable - fixedTotal
	if remaining < 0 {
		return fmt.Errorf("layout.err.stack_negative_space: parent %q has negative remaining space %.4f", parentID, remaining)
	}
	cursor := 0.0
	for _, childID := range children {
		child := doc.Nodes[childID]
		childMain := 0.0
		if child.Frame.Kind == FixedFrame {
			if arrange.Axis == AxisHorizontal {
				childMain = child.Frame.Width
			} else {
				childMain = child.Frame.Height
			}
		} else {
			w := child.Frame.Weight
			if w == 0 {
				w = 1
			}
			if fillWeight > 0 {
				childMain = remaining * (w / fillWeight)
			}
		}
		childCross := crossAvailable
		if arrange.Axis == AxisHorizontal {
			if child.Frame.Height > 0 {
				childCross = child.Frame.Height
			}
		} else if child.Frame.Width > 0 {
			childCross = child.Frame.Width
		}
		childRect := Rect{}
		if arrange.Axis == AxisHorizontal {
			childRect = Rect{X: inner.X + cursor, Y: inner.Y, Width: childMain, Height: childCross}
		} else {
			childRect = Rect{X: inner.X, Y: inner.Y + cursor, Width: childCross, Height: childMain}
		}
		if err := validateRect(childRect, "child"); err != nil {
			return err
		}
		if err := resolveNode(doc, out, childID, childRect); err != nil {
			return err
		}
		cursor += childMain + arrange.Gap
	}
	return nil
}

func resolveFrame(frame FrameSpec, parent Rect) (Rect, error) {
	switch frame.Kind {
	case RootFrame:
		return parent, nil
	case AbsoluteFrame:
		return Rect{X: parent.X + frame.X, Y: parent.Y + frame.Y, Width: frame.Width, Height: frame.Height}, validateRect(Rect{X: parent.X + frame.X, Y: parent.Y + frame.Y, Width: frame.Width, Height: frame.Height}, "absolute")
	case AnchorFrame:
		x := parent.X + parent.Width*frame.Left
		y := parent.Y + parent.Height*frame.Top
		w := parent.Width * (frame.Right - frame.Left)
		h := parent.Height * (frame.Bottom - frame.Top)
		r := Rect{X: x, Y: y, Width: w, Height: h}
		return r, validateRect(r, "anchor")
	default:
		return Rect{}, fmt.Errorf("layout.err.unsupported_frame_kind: unsupported frame kind %q", frame.Kind)
	}
}

func validateFrame(frame FrameSpec) error {
	switch frame.Kind {
	case RootFrame:
	case AbsoluteFrame:
		if frame.Width < 0 || frame.Height < 0 {
			return fmt.Errorf("layout.err.invalid_absolute: absolute frame width/height must be non-negative")
		}
	case AnchorFrame:
		if frame.Left < 0 || frame.Left > 1 || frame.Top < 0 || frame.Top > 1 || frame.Right < 0 || frame.Right > 1 || frame.Bottom < 0 || frame.Bottom > 1 {
			return fmt.Errorf("layout.err.invalid_anchor: anchor frame fractions must be within [0,1]")
		}
		if frame.Right < frame.Left || frame.Bottom < frame.Top {
			return fmt.Errorf("layout.err.invalid_anchor_bounds: anchor frame right/bottom must be >= left/top")
		}
	case FixedFrame:
		if frame.Width < 0 || frame.Height < 0 {
			return fmt.Errorf("layout.err.invalid_fixed: fixed frame width/height must be non-negative")
		}
	case FillFrame:
		if frame.Weight < 0 {
			return fmt.Errorf("layout.err.invalid_fill_weight: fill frame weight must be positive")
		}
	default:
		return fmt.Errorf("layout.err.unsupported_frame_kind: unsupported frame kind %q", frame.Kind)
	}
	return validateFiniteFrame(frame)
}

func validateArrange(arrange ArrangeSpec) error {
	if arrange.Kind != StackArrange {
		return fmt.Errorf("layout.err.unsupported_arrange_kind: unsupported arrange kind %q", arrange.Kind)
	}
	if arrange.Axis != AxisHorizontal && arrange.Axis != AxisVertical {
		return fmt.Errorf("layout.err.invalid_axis: axis must be horizontal or vertical")
	}
	if arrange.Gap < 0 || arrange.Padding.Top < 0 || arrange.Padding.Right < 0 || arrange.Padding.Bottom < 0 || arrange.Padding.Left < 0 {
		return fmt.Errorf("layout.err.invalid_spacing: gap and padding must be non-negative")
	}
	return nil
}

func validateRect(r Rect, label string) error {
	if r.Width < 0 || r.Height < 0 {
		return fmt.Errorf("layout.err.invalid_rect: %s rect width/height must be non-negative", label)
	}
	if !isFinite(r.X) || !isFinite(r.Y) || !isFinite(r.Width) || !isFinite(r.Height) {
		return fmt.Errorf("layout.err.invalid_rect: %s rect values must be finite", label)
	}
	return nil
}

func validateFiniteFrame(frame FrameSpec) error {
	values := []float64{frame.X, frame.Y, frame.Width, frame.Height, frame.Left, frame.Top, frame.Right, frame.Bottom, frame.Weight}
	for _, value := range values {
		if !isFinite(value) {
			return fmt.Errorf("layout.err.non_finite_frame: frame values must be finite")
		}
	}
	return nil
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
