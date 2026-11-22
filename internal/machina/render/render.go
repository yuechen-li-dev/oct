package render

import (
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/machina/textmetrics"
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
	// Default button chrome under the pointer and while pressed.
	FillButtonHover   FillKind = "button-hover"
	FillButtonPressed FillKind = "button-pressed"
	// FillCheck is a check mark drawn in Paint.Background inside Rect (a
	// checked checkbox). Backends stroke it; fonts need not have a glyph.
	FillCheck FillKind = "check"
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
	Paint  *Paint // set iff Fill is FillStyled or FillCheck
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
	// Size, Weight and Mono set the font; zero values are the body font
	// (16 dp, regular, proportional). Text is already wrapped and truncated
	// to fit Rect with the same metrics layout used.
	Size   float64
	Weight int
	Mono   bool
}

func (DrawTextCommand) Kind() CommandKind { return KindDrawText }

type PushClipCommand struct {
	NodeID layout.NodeID
	Rect   layout.Rect
}

func (PushClipCommand) Kind() CommandKind { return KindPushClip }

type PopClipCommand struct{ NodeID layout.NodeID }

func (PopClipCommand) Kind() CommandKind { return KindPopClip }

// Options configures command building. Metrics measures text for wrapping
// and truncation; nil is the deterministic fixed metric. Interaction is
// the session's transient state, which decides state styles, focus rings
// and the caret. States are the nodes' state styles.
type Options struct {
	Metrics     textmetrics.Metrics
	Interaction Interaction
	States      map[layout.NodeID]uiir.StateStyles
}

// Interaction is what the session tracks between frames, keyed by node id:
// the control under the pointer, the one being pressed, the one with
// keyboard focus, and the caret (a rune index) in a focused text field.
type Interaction struct {
	Hover   layout.NodeID
	Pressed layout.NodeID
	Focus   layout.NodeID
	Caret   int
}

type emitter struct {
	resolved  *layout.ResolvedLayoutDocument
	semantics map[layout.NodeID]lowering.Semantics
	styles    map[layout.NodeID]lowering.Style
	options   Options
	out       *[]Command
}

func BuildCommands(resolved *layout.ResolvedLayoutDocument, semantics map[layout.NodeID]lowering.Semantics, styles map[layout.NodeID]lowering.Style, options Options) ([]Command, error) {
	if options.Metrics == nil {
		options.Metrics = textmetrics.Fixed{}
	}
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
	e := emitter{resolved: resolved, semantics: semantics, styles: styles, options: options, out: &out}
	if err := e.emitNode(resolved.RootID); err != nil {
		return nil, err
	}
	out = append(out, EndFrameCommand{})
	return out, nil
}

func (e emitter) emitNode(id layout.NodeID) error {
	resolved, semantics, m, out := e.resolved, e.semantics, e.options.Metrics, e.out
	node, ok := resolved.Nodes[id]
	if !ok {
		return fmt.Errorf("render.err.missing_node: resolved tree references unknown node %q", id)
	}
	if err := validateRect(node.Rect); err != nil {
		return fmt.Errorf("render.err.invalid_rect: node %q: %w", id, err)
	}
	style, styled := e.styleFor(id)
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
				if err := emitText(m, id, node.Rect, sem, textColor, out); err != nil {
					return err
				}
			}
		case lowering.RoleButton:
			// A button is chrome plus a centered label; without the fill a
			// backend has no way to distinguish it from text. A styled
			// button already has its paint.
			if !styled {
				fill := FillButton
				switch {
				case sem.Disabled:
					fill = FillButtonDisabled
				case e.options.Interaction.Pressed == id:
					fill = FillButtonPressed
				case e.options.Interaction.Hover == id:
					fill = FillButtonHover
				}
				*out = append(*out, FillRectCommand{NodeID: id, Rect: node.Rect, Fill: fill})
			}
			if sem.Label != "" {
				// A label wider than its button is shortened, never spilled.
				label := textmetrics.Fit(m, sem.Label, textmetrics.Body, node.Rect.Width-buttonLabelInset)
				*out = append(*out, DrawTextCommand{NodeID: id, Rect: node.Rect, Text: label, Align: AlignCenter, Muted: sem.Disabled, Color: textColor})
			}
			e.focusRing(id, node.Rect, styled)
		case lowering.RoleCheckbox, lowering.RoleTextField, lowering.RoleSlider:
			e.emitInput(id, node.Rect, sem, styled, textColor)
		}
	}
	for _, child := range resolved.PaintOrder(id) {
		if err := e.emitNode(child); err != nil {
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
