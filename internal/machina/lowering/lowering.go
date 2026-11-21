package lowering

import (
	"fmt"
	"unicode/utf8"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// Action is what activating a node sends. Name is the tool-facing alias;
// Message, when set, is the typed value (an Oct enum value) the app's
// Update receives instead of Name.
type Action struct {
	Name    string
	Message any
}

type Role string

const (
	RoleText      Role = "text"
	RoleButton    Role = "button"
	RoleContainer Role = "container"
)

type Semantics struct {
	Role      Role
	Label     string
	Disabled  bool
	Focusable bool
}

// Style is the paint a node requested through UI.Styled (Libraries/UI
// M112 records). Nodes without a style have no entry in Result.Styles.
type Style uiir.StyleSpec

type Result struct {
	Rows      []layout.LayoutRow
	Actions   map[layout.NodeID]Action
	Semantics map[layout.NodeID]Semantics
	Styles    map[layout.NodeID]Style
	// Kinds names each node's authoring construct (Text, Button, Stack(v),
	// Canvas, At, ...) for layout dumps.
	Kinds map[layout.NodeID]string
}

func Lower(root *uiir.Node) (*Result, error) {
	if root == nil {
		return nil, fmt.Errorf("lowering.err.nil_root: root is required")
	}
	withIDs := uiir.WithNodeIDs(root)
	res := &Result{Rows: make([]layout.LayoutRow, 0), Actions: map[layout.NodeID]Action{}, Semantics: map[layout.NodeID]Semantics{}, Styles: map[layout.NodeID]Style{}, Kinds: map[layout.NodeID]string{}}
	if err := lowerNode(res, withIDs, nil, nil, 0); err != nil {
		return nil, err
	}
	return res, nil
}

// isCanvas reports a placement surface: a box-kind node with no box of its
// own (UI.Canvas). Its children must be placed boxes.
func isCanvas(node *uiir.Node) bool {
	return node != nil && (node.Kind == uiir.NodeAbsoluteBox || node.Kind == uiir.NodeAnchorBox) && node.Box == nil
}

// isPlacedBox reports a box that places exactly the content it wraps.
func isPlacedBox(node *uiir.Node) bool {
	return node != nil && (node.Kind == uiir.NodeAbsoluteBox || node.Kind == uiir.NodeAnchorBox) && node.Box != nil
}

// fillParent is the frame of a placed box's content: the whole box.
var fillParent = layout.FrameSpec{Kind: layout.SpanFrame, XSpan: uiir.SpanSpec{Kind: uiir.SpanStretch}, YSpan: uiir.SpanSpec{Kind: uiir.SpanStretch}}

func lowerNode(out *Result, node *uiir.Node, parent *layout.NodeID, parentNode *uiir.Node, order int) error {
	if node == nil {
		return fmt.Errorf("lowering.err.nil_node: node is required")
	}
	id := layout.NodeID(node.NodeID)
	if id == "" {
		return fmt.Errorf("lowering.err.missing_id: node id is required")
	}
	row := layout.LayoutRow{ID: id, Parent: parent, Order: order, Frame: layout.FrameSpec{Kind: layout.FixedFrame, Width: 0, Height: 0}}
	if node.Style != nil {
		out.Styles[id] = Style(*node.Style)
	}
	switch node.Kind {
	case uiir.NodeText:
		row.Frame = layout.FrameSpec{Kind: layout.FixedFrame, Width: textWidth(node.Text), Height: 20}
		out.Semantics[id] = Semantics{Role: RoleText, Label: node.Text}
	case uiir.NodeButton:
		row.Frame = layout.FrameSpec{Kind: layout.FixedFrame, Width: buttonWidth(node.Label), Height: 32}
		enabled := node.Enabled
		out.Semantics[id] = Semantics{Role: RoleButton, Label: node.Label, Disabled: !enabled, Focusable: enabled}
		if enabled && node.Event != "" {
			out.Actions[id] = Action{Name: node.Event, Message: node.Message}
		}
	case uiir.NodeSpacer:
		row.Frame = layout.FrameSpec{Kind: layout.FillFrame, Weight: 1}
	case uiir.NodeColumn:
		row.Frame = inferFrame(node, parent)
		row.Arrange = stackArrange(node, layout.AxisVertical)
		out.Semantics[id] = Semantics{Role: RoleContainer}
	case uiir.NodeRow:
		row.Frame = inferFrame(node, parent)
		row.Arrange = stackArrange(node, layout.AxisHorizontal)
		out.Semantics[id] = Semantics{Role: RoleContainer}
	case uiir.NodeSkip:
		return fmt.Errorf("lowering.err.skip_outside_grid: node %q is UI.Skip, which only marks a cell inside UI.Grid rows", node.NodeID)
	case uiir.NodeGrid:
		row.Frame = inferFrame(node, parent)
		if node.GridRows > 0 && node.GridCols > 0 {
			row.Arrange = &layout.ArrangeSpec{Kind: layout.GridArrange}
			for i := int64(0); i < node.GridCols; i++ {
				row.Arrange.Columns = append(row.Arrange.Columns, layout.GridTrack{Kind: layout.GridTrackFill, Weight: 1})
			}
			for i := int64(0); i < node.GridRows; i++ {
				row.Arrange.Rows = append(row.Arrange.Rows, layout.GridTrack{Kind: layout.GridTrackFill, Weight: 1})
			}
		} else if node.Grid != nil {
			row.Arrange = &layout.ArrangeSpec{Kind: layout.GridArrange, ColumnGap: node.Grid.ColumnGap, RowGap: node.Grid.RowGap, Padding: layout.EdgeInsets{Top: node.Grid.Padding.Top, Right: node.Grid.Padding.Right, Bottom: node.Grid.Padding.Bottom, Left: node.Grid.Padding.Left}}
			cols, rows, err := gridTracks(node)
			if err != nil {
				return err
			}
			row.Arrange.Columns, row.Arrange.Rows = cols, rows
		} else {
			row.Arrange = &layout.ArrangeSpec{Kind: layout.StackArrange, Axis: layout.AxisVertical}
		}
		out.Semantics[id] = Semantics{Role: RoleContainer}
	case uiir.NodeAbsoluteBox, uiir.NodeAnchorBox:
		out.Semantics[id] = Semantics{Role: RoleContainer}
		if node.Box == nil {
			// UI.Canvas: a placement surface that fills its parent.
			row.Frame = inferFrame(node, parent)
			break
		}
		f, err := boxFrame(node.Box)
		if err != nil {
			return err
		}
		row.Frame = f
		row.Z = int(node.Box.ZOrder)
		row.Layer = int(node.Box.Layer)
		row.DebugLabel = node.Box.Region
	default:
		return fmt.Errorf("lowering.err.unsupported_kind: unsupported node kind %q", node.Kind)
	}
	if isCanvas(parentNode) && !isPlacedBox(node) {
		return fmt.Errorf("lowering.err.unplaced_canvas_child: node %q (%s) is a child of a canvas but is not placed; wrap it in UI.At or place it with a layout table", node.NodeID, node.Kind)
	}
	if isPlacedBox(node) && parentNode != nil && !isCanvas(parentNode) && !isPlacedBox(parentNode) {
		return fmt.Errorf("lowering.err.box_outside_canvas: placed box %q must be a child of UI.Canvas, not %s", node.NodeID, parentNode.Kind)
	}
	if parent == nil {
		row.Frame = layout.FrameSpec{Kind: layout.RootFrame}
	} else if isPlacedBox(parentNode) && !isPlacedBox(node) {
		row.Frame = fillParent
	} else if node.Cell != nil {
		row.Frame = layout.FrameSpec{Kind: layout.CellFrame, Column: int(node.Cell.Column), Row: int(node.Cell.Row), ColumnSpan: int(node.Cell.ColumnSpan), RowSpan: int(node.Cell.RowSpan)}
	} else if parent != nil {
		for i := len(out.Rows) - 1; i >= 0; i-- {
			parentRow := out.Rows[i]
			if parentRow.ID != *parent {
				continue
			}
			if parentRow.Arrange != nil && parentRow.Arrange.Kind == layout.GridArrange {
				cols := len(parentRow.Arrange.Columns)
				if cols > 0 {
					r := order / cols
					c := order % cols
					row.Frame = layout.FrameSpec{Kind: layout.CellFrame, Column: c, Row: r, ColumnSpan: 1, RowSpan: 1}
				}
			}
			break
		}
	}
	if node.Cell != nil && (parentNode == nil || parentNode.Kind != uiir.NodeGrid) {
		return fmt.Errorf("lowering.err.span_outside_grid: node %q is a UI.Span cell, which belongs in UI.Grid rows", node.NodeID)
	}
	if node.Sizing == nil && (node.Kind == uiir.NodeColumn || node.Kind == uiir.NodeRow) {
		// An unwrapped stack inside a stack sizes to its content along the
		// parent's axis and stretches across it.
		if axis, ok := stackAxis(parentNode); ok {
			frame, err := sizingFrame(&uiir.Node{NodeID: node.NodeID, Kind: node.Kind, Stack: node.Stack, Children: node.Children, Sizing: &uiir.SizingSpec{Kind: uiir.SizingAuto}}, axis)
			if err != nil {
				return fmt.Errorf("%w (stack %q sizes to its content by default; wrap it in UI.Fill or UI.Fixed)", err, node.NodeID)
			}
			row.Frame = frame
		}
	}
	if node.Sizing != nil {
		axis, ok := stackAxis(parentNode)
		if !ok {
			return fmt.Errorf("lowering.err.sizing_outside_stack: node %q has UI.%s sizing, which applies only to a child of a stack (UI.Stack, UI.Column, UI.Row)", node.NodeID, sizingName(node.Sizing.Kind))
		}
		frame, err := sizingFrame(node, axis)
		if err != nil {
			return err
		}
		row.Frame = frame
	}
	if node.Offset != nil {
		row.OffsetX, row.OffsetY = node.Offset.X, node.Offset.Y
	}
	keys := map[string]bool{}
	for _, child := range node.Children {
		if child != nil && child.Key != "" {
			if keys[child.Key] {
				return fmt.Errorf("lowering.err.duplicate_key: two children of %q use UI.Keyed key %q; keys must be unique among siblings", node.NodeID, child.Key)
			}
			keys[child.Key] = true
		}
	}
	out.Kinds[id] = kindLabel(node)
	out.Rows = append(out.Rows, row)
	for i, child := range node.Children {
		p := id
		if err := lowerNode(out, child, &p, node, i); err != nil {
			return err
		}
	}
	return nil
}

func inferFrame(node *uiir.Node, parent *layout.NodeID) layout.FrameSpec {
	if parent == nil {
		return layout.FrameSpec{Kind: layout.RootFrame}
	}
	if len(node.Children) == 0 {
		return layout.FrameSpec{Kind: layout.FixedFrame, Width: 0, Height: 0}
	}
	return layout.FrameSpec{Kind: layout.FillFrame, Weight: 1}
}

func boxFrame(box *uiir.BoxSpec) (layout.FrameSpec, error) {
	switch box.Kind {
	case uiir.BoxAbsolute:
		return layout.FrameSpec{Kind: layout.AbsoluteFrame, X: box.X, Y: box.Y, Width: box.Width, Height: box.Height}, nil
	case uiir.BoxAnchored:
		return layout.FrameSpec{Kind: layout.AnchorFrame, Left: box.Left, Top: box.Top, Right: box.Right, Bottom: box.Bottom}, nil
	case uiir.BoxSpan:
		return layout.FrameSpec{Kind: layout.SpanFrame, XSpan: box.XSpan, YSpan: box.YSpan}, nil
	default:
		return layout.FrameSpec{}, fmt.Errorf("lowering.err.invalid_box: unsupported box kind %q", box.Kind)
	}
}

func textWidth(s string) float64 {
	if w := float64(utf8.RuneCountInString(s) * 8); w > 0 {
		return w
	}
	return 1
}
func buttonWidth(label string) float64 {
	if w := float64(utf8.RuneCountInString(label)*8 + 24); w > 80 {
		return w
	}
	return 80
}

func kindLabel(node *uiir.Node) string {
	switch node.Kind {
	case uiir.NodeColumn:
		return "Stack(v)"
	case uiir.NodeRow:
		return "Stack(h)"
	case uiir.NodeAbsoluteBox, uiir.NodeAnchorBox:
		if node.Box == nil {
			return "Canvas"
		}
		if node.Box.Region != "" {
			return "Region"
		}
		return "At"
	default:
		return string(node.Kind)
	}
}
