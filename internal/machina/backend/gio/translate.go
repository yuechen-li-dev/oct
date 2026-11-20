package gio

import (
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/render"
)

type DrawOpKind string

const (
	DrawOpBeginFrame DrawOpKind = "BeginFrame"
	DrawOpEndFrame   DrawOpKind = "EndFrame"
	DrawOpFillRect   DrawOpKind = "FillRect"
	DrawOpDrawText   DrawOpKind = "DrawText"
	DrawOpPushClip   DrawOpKind = "PushClip"
	DrawOpPopClip    DrawOpKind = "PopClip"
)

type DrawOp struct {
	Kind  DrawOpKind
	Rect  layout.Rect
	Text  string
	Fill  render.FillKind
	Paint *render.Paint
	Align render.TextAlign
	Muted bool
	Color *uiir.RGBA
}

func Translate(commands []render.Command) ([]DrawOp, error) {
	out := make([]DrawOp, 0, len(commands))
	clipDepth := 0
	for i, cmd := range commands {
		switch c := cmd.(type) {
		case render.BeginFrameCommand:
			out = append(out, DrawOp{Kind: DrawOpBeginFrame, Rect: c.Root})
		case render.EndFrameCommand:
			if clipDepth != 0 {
				return nil, fmt.Errorf("machina.gio.err.unbalanced_clip: end frame with clip depth %d", clipDepth)
			}
			out = append(out, DrawOp{Kind: DrawOpEndFrame})
		case render.FillRectCommand:
			out = append(out, DrawOp{Kind: DrawOpFillRect, Rect: c.Rect, Fill: c.Fill, Paint: c.Paint})
		case render.DrawTextCommand:
			out = append(out, DrawOp{Kind: DrawOpDrawText, Rect: c.Rect, Text: c.Text, Align: c.Align, Muted: c.Muted, Color: c.Color})
		case render.PushClipCommand:
			clipDepth++
			out = append(out, DrawOp{Kind: DrawOpPushClip, Rect: c.Rect})
		case render.PopClipCommand:
			if clipDepth == 0 {
				return nil, fmt.Errorf("machina.gio.err.unbalanced_clip: pop without push at %d", i)
			}
			clipDepth--
			out = append(out, DrawOp{Kind: DrawOpPopClip})
		default:
			return nil, fmt.Errorf("machina.gio.err.unsupported_command: %T", cmd)
		}
	}
	return out, nil
}
