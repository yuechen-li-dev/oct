package render

import (
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
	"math"

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

// FillKind tells a backend what a filled rectangle represents, so control
// chrome is decided once here rather than re-derived by every backend.
type FillKind string

const (
	FillPlain          FillKind = ""
	FillButton         FillKind = "button"
	FillButtonDisabled FillKind = "button-disabled"
	// FillStyled carries explicit Paint from UI.Styled and replaces the
	// default chrome for that node.
	FillStyled FillKind = "styled"
)

// Paint is explicit fill and border for a styled node.
type Paint struct {
	Background  uiir.RGBA
	Border      uiir.RGBA
	BorderWidth float64
	Radius      float64
}

type FillRectCommand struct {
	NodeID layout.NodeID
	Rect   layout.Rect
	Fill   FillKind
	Paint  *Paint // set iff Fill == FillStyled
}

func (FillRectCommand) Kind() CommandKind { return KindFillRect }

// TextAlign positions text inside its rect.
type TextAlign string

const (
	AlignStart  TextAlign = ""
	AlignCenter TextAlign = "center"
)

type DrawTextCommand struct {
	NodeID layout.NodeID
	Rect   layout.Rect
	Text   string
	Align  TextAlign
	// Muted marks text a backend should de-emphasize (a disabled control).
	Muted bool
	// Color is the node's styled foreground; nil means the backend default.
	Color *uiir.RGBA
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
	if err := emitNode(resolved, semantics, styles, resolved.RootID, &out); err != nil {
		return nil, err
	}
	out = append(out, EndFrameCommand{})
	return out, nil
}

func emitNode(resolved *layout.ResolvedLayoutDocument, semantics map[layout.NodeID]lowering.Semantics, styles map[layout.NodeID]lowering.Style, id layout.NodeID, out *[]Command) error {
	node, ok := resolved.Nodes[id]
	if !ok {
		return fmt.Errorf("render.err.missing_node: resolved tree references unknown node %q", id)
	}
	if err := validateRect(node.Rect); err != nil {
		return fmt.Errorf("render.err.invalid_rect: node %q: %w", id, err)
	}
	style, styled := styles[id]
	var textColor *uiir.RGBA
	if styled {
		// Explicit paint comes first so the node's own text and its
		// children draw over it.
		*out = append(*out, FillRectCommand{NodeID: id, Rect: node.Rect, Fill: FillStyled, Paint: &Paint{
			Background: style.Background, Border: style.BorderColor, BorderWidth: style.BorderWidth, Radius: style.Radius,
		}})
		fg := style.Foreground
		textColor = &fg
	}
	if sem, ok := semantics[id]; ok {
		switch sem.Role {
		case lowering.RoleText:
			if sem.Label != "" {
				*out = append(*out, DrawTextCommand{NodeID: id, Rect: node.Rect, Text: sem.Label, Color: textColor})
			}
		case lowering.RoleButton:
			// A button is chrome plus a centered label; without the fill a
			// backend has no way to distinguish it from text. A styled
			// button already has its paint.
			if !styled {
				fill := FillButton
				if sem.Disabled {
					fill = FillButtonDisabled
				}
				*out = append(*out, FillRectCommand{NodeID: id, Rect: node.Rect, Fill: fill})
			}
			if sem.Label != "" {
				*out = append(*out, DrawTextCommand{NodeID: id, Rect: node.Rect, Text: sem.Label, Align: AlignCenter, Muted: sem.Disabled, Color: textColor})
			}
		}
	}
	for _, child := range resolved.PaintOrder(id) {
		if err := emitNode(resolved, semantics, styles, child, out); err != nil {
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
