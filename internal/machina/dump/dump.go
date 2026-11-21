// Package dump prints a resolved Machina frame as text, one line per node in
// paint order, with each box's geometry written in the authoring vocabulary
// (Libraries/UI/DESIGN.md §5.12): the spans that place it in its parent,
// then its absolute rect. A region shows its layout-table row; a node placed
// by stack or grid sugar shows the Left/Top spans the sugar resolved to.
package dump

import (
	"fmt"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

var layerNames = map[int]string{-1: "Base", 0: "Content", 1: "Chrome", 2: "Overlay", 3: "Modal", 4: "Tooltip"}

// Format renders the frame. Output is deterministic for a given input.
func Format(resolved *layout.ResolvedLayoutDocument, lowered *lowering.Result) (string, error) {
	if resolved == nil || lowered == nil {
		return "", fmt.Errorf("dump.err.nil_frame: resolved layout and lowering result are required")
	}
	var b strings.Builder
	var walk func(id layout.NodeID, parent *layout.Rect, depth int, regionID layout.NodeID) error
	walk = func(id layout.NodeID, parent *layout.Rect, depth int, regionID layout.NodeID) error {
		node, ok := resolved.Nodes[id]
		if !ok {
			return fmt.Errorf("dump.err.missing_node: %q", id)
		}
		name := string(id)
		if node.DebugLabel != "" {
			name = node.DebugLabel
			regionID = id
		} else if regionID != "" {
			name = strings.TrimPrefix(string(id), string(regionID))
		}
		if node.Layer != 0 {
			name += " [" + layerName(node.Layer) + "]"
		}
		if node.Z != 0 {
			name += fmt.Sprintf(" z=%d", node.Z)
		}
		line := fmt.Sprintf("%s%-*s %-10s", strings.Repeat("  ", depth), 24-2*depth, name, lowered.Kinds[id])
		if parent != nil {
			x, y := spans(node, *parent)
			line += fmt.Sprintf(" %-22s %-22s", x, y)
		}
		line += fmt.Sprintf(" (%s,%s) %sx%s", num(node.Rect.X), num(node.Rect.Y), num(node.Rect.Width), num(node.Rect.Height))
		if sem, ok := lowered.Semantics[id]; ok && sem.Label != "" {
			line += fmt.Sprintf(" %q", sem.Label)
			if action, ok := lowered.Actions[id]; ok {
				line += " -> " + action.Name
			} else if sem.Disabled {
				line += " disabled"
			}
		}
		b.WriteString(strings.TrimRight(line, " ") + "\n")
		rect := node.Rect
		for _, child := range resolved.PaintOrder(id) {
			if err := walk(child, &rect, depth+1, regionID); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(resolved.RootID, nil, 0, ""); err != nil {
		return "", err
	}
	return b.String(), nil
}

func layerName(rank int) string {
	if name, ok := layerNames[rank]; ok {
		return name
	}
	return fmt.Sprintf("layer%d", rank)
}

// spans writes a node's placement in its parent. Authored span frames are
// printed as written; any other frame as the Left/Top spans it resolved to.
func spans(node layout.ResolvedLayoutNode, parent layout.Rect) (string, string) {
	if node.Frame.Kind == layout.SpanFrame {
		return span(node.Frame.XSpan, "X", "Left", "Right"), span(node.Frame.YSpan, "Y", "Top", "Bottom")
	}
	return fmt.Sprintf("Left(%spx, %spx)", num(node.Rect.X-parent.X), num(node.Rect.Width)),
		fmt.Sprintf("Top(%spx, %spx)", num(node.Rect.Y-parent.Y), num(node.Rect.Height))
}

func span(s uiir.SpanSpec, axis, near, far string) string {
	switch s.Kind {
	case uiir.SpanStart:
		return fmt.Sprintf("%s(%spx, %spx)", near, num(s.A), num(s.B))
	case uiir.SpanEnd:
		return fmt.Sprintf("%s(%spx, %spx)", far, num(s.A), num(s.B))
	case uiir.SpanStretch:
		return fmt.Sprintf("Stretch%s(%spx, %spx)", axis, num(s.A), num(s.B))
	case uiir.SpanCenter:
		return fmt.Sprintf("Center%s(%spx)", axis, num(s.B))
	default:
		return fmt.Sprintf("Fraction%s(%s ui, %s ui)", axis, num(s.A), num(s.B))
	}
}

func num(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}
