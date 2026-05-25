package layout

import (
	"fmt"
	"sort"
)

type NodeID string

type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type FrameKind string

const (
	RootFrame     FrameKind = "root"
	AbsoluteFrame FrameKind = "absolute"
	AnchorFrame   FrameKind = "anchor"
	FixedFrame    FrameKind = "fixed"
	FillFrame     FrameKind = "fill"
	CellFrame     FrameKind = "cell"
)

type FrameSpec struct {
	Kind       FrameKind
	X          float64
	Y          float64
	Width      float64
	Height     float64
	Left       float64
	Top        float64
	Right      float64
	Bottom     float64
	Weight     float64
	Column     int
	Row        int
	ColumnSpan int
	RowSpan    int
}

type Axis string

type ArrangeKind string

type ArrangeAlign string

type ArrangeJustify string

const (
	AxisHorizontal Axis = "horizontal"
	AxisVertical   Axis = "vertical"

	StackArrange ArrangeKind = "stack"
	GridArrange  ArrangeKind = "grid"

	AlignStart  ArrangeAlign = "start"
	AlignCenter ArrangeAlign = "center"
	AlignEnd    ArrangeAlign = "end"

	JustifyStart        ArrangeJustify = "start"
	JustifyCenter       ArrangeJustify = "center"
	JustifyEnd          ArrangeJustify = "end"
	JustifySpaceBetween ArrangeJustify = "space-between"
)

type GridTrackKind string

const (
	GridTrackFixed GridTrackKind = "fixed"
	GridTrackFill  GridTrackKind = "fill"
)

type GridTrack struct {
	Kind   GridTrackKind
	Size   float64
	Weight float64
}

type EdgeInsets struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

type ArrangeSpec struct {
	Kind      ArrangeKind
	Axis      Axis
	Gap       float64
	Padding   EdgeInsets
	Align     ArrangeAlign
	Justify   ArrangeJustify
	Columns   []GridTrack
	Rows      []GridTrack
	ColumnGap float64
	RowGap    float64
}

type LayoutRow struct {
	ID         NodeID
	Parent     *NodeID
	Order      int
	Z          int
	Frame      FrameSpec
	Arrange    *ArrangeSpec
	DebugLabel string
}

type LayoutDocument struct {
	RootID      NodeID
	Nodes       map[NodeID]LayoutRow
	Children    map[NodeID][]NodeID
	inputOrders map[NodeID]int
}

func CompileRows(rows []LayoutRow) (*LayoutDocument, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("layout.err.empty_rows: at least one layout row is required")
	}
	doc := &LayoutDocument{Nodes: map[NodeID]LayoutRow{}, Children: map[NodeID][]NodeID{}, inputOrders: map[NodeID]int{}}
	rootCount := 0
	for idx, row := range rows {
		if row.ID == "" {
			return nil, fmt.Errorf("layout.err.invalid_id: row at index %d has empty id", idx)
		}
		if _, exists := doc.Nodes[row.ID]; exists {
			return nil, fmt.Errorf("layout.err.duplicate_id: duplicate row id %q", row.ID)
		}
		if err := validateFrame(row.Frame); err != nil {
			return nil, err
		}
		if row.Arrange != nil {
			if err := validateArrange(*row.Arrange); err != nil {
				return nil, err
			}
		}
		if row.Frame.Kind == RootFrame {
			rootCount++
			doc.RootID = row.ID
			if row.Parent != nil {
				return nil, fmt.Errorf("layout.err.root_parent: root row %q cannot have a parent", row.ID)
			}
		} else if row.Parent == nil {
			return nil, fmt.Errorf("layout.err.missing_parent: non-root row %q must declare a parent", row.ID)
		}
		doc.Nodes[row.ID] = row
		doc.inputOrders[row.ID] = idx
		doc.Children[row.ID] = []NodeID{}
	}
	if rootCount != 1 {
		return nil, fmt.Errorf("layout.err.missing_root: exactly one root row is required, got %d", rootCount)
	}
	for _, row := range doc.Nodes {
		if row.Frame.Kind != RootFrame && row.Parent != nil {
			if _, ok := doc.Nodes[*row.Parent]; !ok {
				return nil, fmt.Errorf("layout.err.unknown_parent: row %q references unknown parent %q", row.ID, *row.Parent)
			}
			doc.Children[*row.Parent] = append(doc.Children[*row.Parent], row.ID)
		}
	}
	for parentID := range doc.Children {
		kids := doc.Children[parentID]
		sort.SliceStable(kids, func(i, j int) bool {
			left := doc.Nodes[kids[i]]
			right := doc.Nodes[kids[j]]
			if left.Order != right.Order {
				return left.Order < right.Order
			}
			return doc.inputOrders[left.ID] < doc.inputOrders[right.ID]
		})
		doc.Children[parentID] = kids
	}
	if err := validateReachability(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func validateReachability(doc *LayoutDocument) error {
	seen := map[NodeID]bool{}
	stack := map[NodeID]bool{}
	var walk func(NodeID) error
	walk = func(id NodeID) error {
		if stack[id] {
			return fmt.Errorf("layout.err.cycle: cycle detected at node %q", id)
		}
		if seen[id] {
			return nil
		}
		stack[id] = true
		for _, child := range doc.Children[id] {
			if err := walk(child); err != nil {
				return err
			}
		}
		delete(stack, id)
		seen[id] = true
		return nil
	}
	if err := walk(doc.RootID); err != nil {
		return err
	}
	for id := range doc.Nodes {
		if !seen[id] {
			return fmt.Errorf("layout.err.unreachable: node %q is not reachable from root %q", id, doc.RootID)
		}
	}
	return nil
}
