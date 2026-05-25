//go:build machina_gio

package gio

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/app"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/yuechen-li-dev/oct/internal/machina/session"
)

func RunWindow(title string, s SessionAdapter) error {
	if s == nil {
		return fmt.Errorf("machina.gio.err.nil_session: session adapter is required")
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title(title), app.Size(unit.Dp(640), unit.Dp(480)))
		if err := runLoop(w, s); err != nil {
			panic(err)
		}
	}()
	app.Main()
	return nil
}

func runLoop(w *app.Window, s SessionAdapter) error {
	var ops op.Ops
	th := material.NewTheme()
	for e := range w.Events() {
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := layout.NewContext(&ops, e)
			pointer.InputOp{Tag: w, Kinds: pointer.Press}.Add(gtx.Ops)
			for {
				ev, ok := gtx.Event(pointer.Filter{Target: w, Kinds: pointer.Press})
				if !ok {
					break
				}
				pe, ok := ev.(pointer.Event)
				if ok {
					_, _ = s.PointerDown(session.PointerEvent{X: float64(pe.Position.X), Y: float64(pe.Position.Y)})
				}
			}
			cmds, err := s.Commands()
			if err != nil {
				return err
			}
			draw, err := Translate(cmds)
			if err != nil {
				return err
			}
			stack := []clip.Stack{}
			for _, d := range draw {
				switch d.Kind {
				case DrawOpFillRect:
					r := image.Rect(int(d.Rect.X), int(d.Rect.Y), int(d.Rect.X+d.Rect.Width), int(d.Rect.Y+d.Rect.Height))
					paint.FillShape(gtx.Ops, color.NRGBA{A: 255, R: 220, G: 220, B: 220}, clip.Rect(r).Op())
				case DrawOpDrawText:
					st := op.Offset(image.Pt(int(d.Rect.X), int(d.Rect.Y))).Push(gtx.Ops)
					material.Body1(th, d.Text).Layout(gtx)
					st.Pop()
				case DrawOpPushClip:
					r := image.Rect(int(d.Rect.X), int(d.Rect.Y), int(d.Rect.X+d.Rect.Width), int(d.Rect.Y+d.Rect.Height))
					stack = append(stack, clip.Rect(r).Push(gtx.Ops))
				case DrawOpPopClip:
					if n := len(stack); n > 0 {
						stack[n-1].Pop()
						stack = stack[:n-1]
					}
				}
			}
			e.Frame(gtx.Ops)
			w.Invalidate()
		}
	}
	return nil
}
