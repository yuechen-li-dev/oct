package render

import (
	"fmt"
	"math"
	"sort"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
)

type CommandKind string

const (
	KindBeginFrame CommandKind = "BeginFrame"
	KindEndFrame   CommandKind = "EndFrame"
	KindFillRect   CommandKind = "FillRect"
	KindDrawText   CommandKind = "DrawText"
	KindPushClip   CommandKind = "PushClip"
	KindPopClip    CommandKind = "PopClip"
)

type Command interface{ Kind() CommandKind }

type BeginFrameCommand struct{ Root layout.Rect }

func (BeginFrameCommand) Kind() CommandKind { return KindBeginFrame }

type EndFrameCommand struct{}

func (EndFrameCommand) Kind() CommandKind { return KindEndFrame }

type FillRectCommand struct {
	NodeID layout.NodeID
	Rect   layout.Rect
}

func (FillRectCommand) Kind() CommandKind { return KindFillRect }

type DrawTextCommand struct {
	NodeID layout.NodeID
	Rect   layout.Rect
	Text   string
}

func (DrawTextCommand) Kind() CommandKind { return KindDrawText }

type PushClipCommand struct {
	NodeID layout.NodeID
	Rect   layout.Rect
}

func (PushClipCommand) Kind() CommandKind { return KindPushClip }

type PopClipCommand struct{ NodeID layout.NodeID }

func (PopClipCommand) Kind() CommandKind { return KindPopClip }

type Options struct{}

func BuildCommands(resolved *layout.ResolvedLayoutDocument, semantics map[layout.NodeID]lowering.Semantics, styles map[layout.NodeID]lowering.Style, options Options) ([]Command, error) {
	_ = styles
	_ = options
	if resolved == nil {
		return nil, fmt.Errorf("render.err.nil_resolved: resolved layout document is required")
	}
	root, ok := resolved.Nodes[resolved.RootID]
	if !ok {
		return nil, fmt.Errorf("render.err.missing_root: resolved root node %q is missing", resolved.RootID)
	}
	if err := validateRect(root.Rect); err != nil {
		return nil, fmt.Errorf("render.err.invalid_root_rect: %w", err)
	}
	if semantics == nil {
		semantics = map[layout.NodeID]lowering.Semantics{}
	}
	out := []Command{BeginFrameCommand{Root: root.Rect}}
	if err := emitNode(resolved, semantics, resolved.RootID, &out); err != nil {
		return nil, err
	}
	out = append(out, EndFrameCommand{})
	return out, nil
}

func emitNode(resolved *layout.ResolvedLayoutDocument, semantics map[layout.NodeID]lowering.Semantics, id layout.NodeID, out *[]Command) error {
	node, ok := resolved.Nodes[id]
	if !ok {
		return fmt.Errorf("render.err.missing_node: resolved tree references unknown node %q", id)
	}
	if err := validateRect(node.Rect); err != nil {
		return fmt.Errorf("render.err.invalid_rect: node %q: %w", id, err)
	}
	if sem, ok := semantics[id]; ok {
		switch sem.Role {
		case lowering.RoleText, lowering.RoleButton:
			if sem.Label != "" {
				*out = append(*out, DrawTextCommand{NodeID: id, Rect: node.Rect, Text: sem.Label})
			}
		}
	}
	children := append([]layout.NodeID(nil), resolved.Children[id]...)
	sort.SliceStable(children, func(i, j int) bool {
		left := resolved.Nodes[children[i]]
		right := resolved.Nodes[children[j]]
		if left.Z != right.Z {
			return left.Z < right.Z
		}
		if left.Order != right.Order {
			return left.Order < right.Order
		}
		return string(left.ID) < string(right.ID)
	})
	for _, child := range children {
		if err := emitNode(resolved, semantics, child, out); err != nil {
			return err
		}
	}
	return nil
}

func validateRect(r layout.Rect) error {
	if r.Width < 0 || r.Height < 0 {
		return fmt.Errorf("rect width/height must be non-negative")
	}
	if !isFinite(r.X) || !isFinite(r.Y) || !isFinite(r.Width) || !isFinite(r.Height) {
		return fmt.Errorf("rect values must be finite")
	}
	return nil
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
