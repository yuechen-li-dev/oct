package lowering

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

type Action struct{ Name string }

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

// Style is a placeholder metadata slot consumed by renderer contracts.
// M112 adds Oct-facing immutable style records in Libraries/UI; wiring those
// records into lowering metadata is intentionally deferred.
type Style struct{}

type Result struct {
	Rows      []layout.LayoutRow
	Actions   map[layout.NodeID]Action
	Semantics map[layout.NodeID]Semantics
	Styles    map[layout.NodeID]Style
}

func Lower(root *uiir.Node) (*Result, error) {
	if root == nil {
		return nil, fmt.Errorf("lowering.err.nil_root: root is required")
	}
	withIDs := uiir.WithNodeIDs(root)
	res := &Result{Rows: make([]layout.LayoutRow, 0), Actions: map[layout.NodeID]Action{}, Semantics: map[layout.NodeID]Semantics{}, Styles: map[layout.NodeID]Style{}}
	if err := lowerNode(res, withIDs, nil, 0); err != nil {
		return nil, err
	}
	return res, nil
}

func lowerNode(out *Result, node *uiir.Node, parent *layout.NodeID, order int) error {
	if node == nil {
		return fmt.Errorf("lowering.err.nil_node: node is required")
	}
	id := layout.NodeID(node.NodeID)
	if id == "" {
		return fmt.Errorf("lowering.err.missing_id: node id is required")
	}
	row := layout.LayoutRow{ID: id, Parent: parent, Order: order, Frame: layout.FrameSpec{Kind: layout.FixedFrame, Width: 0, Height: 0}}
	switch node.Kind {
	case uiir.NodeText:
		row.Frame = layout.FrameSpec{Kind: layout.FixedFrame, Width: textWidth(node.Text), Height: 20}
		out.Semantics[id] = Semantics{Role: RoleText, Label: node.Text}
	case uiir.NodeButton:
		row.Frame = layout.FrameSpec{Kind: layout.FixedFrame, Width: buttonWidth(node.Label), Height: 32}
		enabled := node.Enabled
		out.Semantics[id] = Semantics{Role: RoleButton, Label: node.Label, Disabled: !enabled, Focusable: enabled}
		if enabled && node.Event != "" {
			out.Actions[id] = Action{Name: node.Event}
		}
	case uiir.NodeSpacer:
		row.Frame = layout.FrameSpec{Kind: layout.FillFrame, Weight: 1}
	case uiir.NodeColumn:
		row.Frame = inferFrame(node, parent)
		row.Arrange = &layout.ArrangeSpec{Kind: layout.StackArrange, Axis: layout.AxisVertical}
		out.Semantics[id] = Semantics{Role: RoleContainer}
	case uiir.NodeRow:
		row.Frame = inferFrame(node, parent)
		row.Arrange = &layout.ArrangeSpec{Kind: layout.StackArrange, Axis: layout.AxisHorizontal}
		out.Semantics[id] = Semantics{Role: RoleContainer}
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
			for _, c := range node.Grid.Columns {
				row.Arrange.Columns = append(row.Arrange.Columns, layout.GridTrack{Kind: layout.GridTrackKind(c.Kind), Size: c.Size, Weight: c.Weight})
			}
			for _, r := range node.Grid.Rows {
				row.Arrange.Rows = append(row.Arrange.Rows, layout.GridTrack{Kind: layout.GridTrackKind(r.Kind), Size: r.Size, Weight: r.Weight})
			}
		} else {
			row.Arrange = &layout.ArrangeSpec{Kind: layout.StackArrange, Axis: layout.AxisVertical}
		}
		out.Semantics[id] = Semantics{Role: RoleContainer}
	case uiir.NodeAbsoluteBox, uiir.NodeAnchorBox:
		if node.Box == nil {
			return fmt.Errorf("lowering.err.invalid_box: node %q requires box spec", node.NodeID)
		}
		f, err := boxFrame(node.Box)
		if err != nil {
			return err
		}
		row.Frame = f
		out.Semantics[id] = Semantics{Role: RoleContainer}
	default:
		return fmt.Errorf("lowering.err.unsupported_kind: unsupported node kind %q", node.Kind)
	}
	if parent == nil {
		row.Frame = layout.FrameSpec{Kind: layout.RootFrame}
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
	out.Rows = append(out.Rows, row)
	for i, child := range node.Children {
		p := id
		if err := lowerNode(out, child, &p, i); err != nil {
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
	default:
		return layout.FrameSpec{}, fmt.Errorf("lowering.err.invalid_box: unsupported box kind %q", box.Kind)
	}
}

func textWidth(s string) float64 {
	if w := float64(len(s) * 8); w > 0 {
		return w
	}
	return 1
}
func buttonWidth(label string) float64 {
	if w := float64(len(label)*8 + 24); w > 80 {
		return w
	}
	return 80
}
