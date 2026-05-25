package hittest

import (
	"fmt"
	"math"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
)

type Point struct {
	X float64
	Y float64
}

type Result struct {
	NodeID    layout.NodeID
	Rect      layout.Rect
	Action    lowering.Action
	Semantics *lowering.Semantics
}

type Index struct {
	candidates []candidate
}

type candidate struct {
	nodeID layout.NodeID
	rect   layout.Rect
	action lowering.Action
}

func BuildIndex(resolved *layout.ResolvedLayoutDocument, actions map[layout.NodeID]lowering.Action, semantics map[layout.NodeID]lowering.Semantics) (*Index, error) {
	_ = semantics
	if resolved == nil {
		return nil, fmt.Errorf("hittest.err.nil_resolved: resolved layout document is required")
	}
	if actions == nil {
		actions = map[layout.NodeID]lowering.Action{}
	}
	ordered := make([]layout.NodeID, 0)
	if err := collectPreOrder(resolved, resolved.RootID, &ordered); err != nil {
		return nil, err
	}
	candidates := make([]candidate, 0, len(actions))
	for _, id := range ordered {
		action, ok := actions[id]
		if !ok {
			continue
		}
		node, exists := resolved.Nodes[id]
		if !exists {
			return nil, fmt.Errorf("hittest.err.missing_node: action metadata references unknown node %q", id)
		}
		if node.Rect.Width <= 0 || node.Rect.Height <= 0 {
			continue
		}
		candidates = append(candidates, candidate{nodeID: id, rect: node.Rect, action: action})
	}
	for id := range actions {
		if _, ok := resolved.Nodes[id]; !ok {
			return nil, fmt.Errorf("hittest.err.missing_node: action metadata references unknown node %q", id)
		}
	}
	return &Index{candidates: candidates}, nil
}

func (idx *Index) HitTest(point Point, semantics map[layout.NodeID]lowering.Semantics) (*Result, bool, error) {
	if idx == nil {
		return nil, false, fmt.Errorf("hittest.err.nil_index: index is required")
	}
	if !isFinite(point.X) || !isFinite(point.Y) {
		return nil, false, fmt.Errorf("hittest.err.invalid_point: point coordinates must be finite")
	}
	for i := len(idx.candidates) - 1; i >= 0; i-- {
		c := idx.candidates[i]
		if contains(c.rect, point) {
			result := &Result{NodeID: c.nodeID, Rect: c.rect, Action: c.action}
			if semantics != nil {
				if sem, ok := semantics[c.nodeID]; ok {
					copy := sem
					result.Semantics = &copy
				}
			}
			return result, true, nil
		}
	}
	return nil, false, nil
}

func HitTest(resolved *layout.ResolvedLayoutDocument, actions map[layout.NodeID]lowering.Action, semantics map[layout.NodeID]lowering.Semantics, point Point) (*Result, bool, error) {
	idx, err := BuildIndex(resolved, actions, semantics)
	if err != nil {
		return nil, false, err
	}
	return idx.HitTest(point, semantics)
}

func collectPreOrder(resolved *layout.ResolvedLayoutDocument, id layout.NodeID, out *[]layout.NodeID) error {
	if _, ok := resolved.Nodes[id]; !ok {
		return fmt.Errorf("hittest.err.missing_node: resolved tree references unknown node %q", id)
	}
	*out = append(*out, id)
	for _, child := range resolved.Children[id] {
		if err := collectPreOrder(resolved, child, out); err != nil {
			return err
		}
	}
	return nil
}

func contains(rect layout.Rect, point Point) bool {
	return point.X >= rect.X && point.X < rect.X+rect.Width && point.Y >= rect.Y && point.Y < rect.Y+rect.Height
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
