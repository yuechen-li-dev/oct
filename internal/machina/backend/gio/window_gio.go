//go:build machina_gio

package gio

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	machinalayout "github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/render"
	"github.com/yuechen-li-dev/oct/internal/machina/session"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// WindowOptions configures a native window. Width and Height are in
// device-independent units (dp), the same units the session lays out in.
type WindowOptions struct {
	Title  string
	Width  float64
	Height float64
}

// RunWindow opens a 640x480 window. Kept for the M109 smoke command.
func RunWindow(title string, s SessionAdapter) error {
	return Run(WindowOptions{Title: title, Width: 640, Height: 480}, s)
}

// Run opens a native window driven by the session and blocks until the
// window closes, then exits the process (Gio's app.Main never returns).
//
// Coordinates: the session lays out in dp. Drawing multiplies by the
// window's PxPerDp and pointer input divides by it, so the UI is sized
// correctly on scaled (HiDPI) displays.
func Run(opts WindowOptions, s SessionAdapter) error {
	if s == nil {
		return fmt.Errorf("machina.gio.err.nil_session: session adapter is required")
	}
	if opts.Width <= 0 {
		opts.Width = 640
	}
	if opts.Height <= 0 {
		opts.Height = 480
	}
	if opts.Title == "" {
		opts.Title = "Machina UI"
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title(opts.Title), app.Size(unit.Dp(opts.Width), unit.Dp(opts.Height)))
		if err := runLoop(w, s); err != nil {
			fmt.Fprintf(os.Stderr, "machina ui: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
	return nil
}

var (
	colorBackground     = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	colorFill           = color.NRGBA{R: 225, G: 228, B: 234, A: 255}
	colorButton         = color.NRGBA{R: 218, G: 226, B: 245, A: 255}
	colorButtonBorder   = color.NRGBA{R: 120, G: 140, B: 190, A: 255}
	colorButtonDisabled = color.NRGBA{R: 238, G: 239, B: 242, A: 255}
	colorButtonHover    = color.NRGBA{R: 200, G: 212, B: 240, A: 255}
	colorButtonPressed  = color.NRGBA{R: 176, G: 192, B: 232, A: 255}
	colorText           = color.NRGBA{R: 20, G: 22, B: 28, A: 255}
	colorTextMuted      = color.NRGBA{R: 150, G: 154, B: 162, A: 255}
)

// buttonInset keeps adjacent buttons visually separate; layout packs them
// edge to edge.
const buttonInset = 2.0

func runLoop(w *app.Window, s SessionAdapter) error {
	var ops op.Ops
	th := material.NewTheme()
	th.Shaper = NewShaper()
	th.Palette.Fg = colorText
	// Layout, wrapping and truncation measure with the faces drawn here.
	if err := s.SetTextMetrics(NewMetrics()); err != nil {
		fmt.Fprintf(os.Stderr, "machina ui: the frame does not fit with this font's metrics, showing it with fixed metrics: %v\n", err)
	}
	var viewport machinalayout.Rect
	tag := new(int) // pointer and keyboard event target for the whole window
	focused := false
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			scale := float64(e.Metric.PxPerDp)
			if scale <= 0 {
				scale = 1
			}

			// Follow window resizes: the session lays out in dp.
			next := machinalayout.Rect{Width: float64(e.Size.X) / scale, Height: float64(e.Size.Y) / scale}
			if next.Width > 0 && next.Height > 0 && next != viewport {
				// A size the layout cannot fit (for example a stack
				// overflow) keeps the last frame and says why, once per
				// size, instead of closing the window.
				if err := s.SetViewport(next); err != nil {
					fmt.Fprintf(os.Stderr, "machina ui: %gx%g does not fit: %v\n", next.Width, next.Height, err)
				}
				viewport = next
			}

			// The window takes keyboard focus once; the session decides
			// which control inside it has focus.
			if !focused {
				gtx.Execute(key.FocusCmd{Tag: tag})
				focused = true
			}

			// Deliver input before drawing so it is reflected in the frame
			// that follows it.
			changed, err := deliverInput(gtx, tag, s, scale)
			if err != nil {
				return err
			}

			paint.Fill(gtx.Ops, colorBackground)
			if err := draw(gtx, th, s, scale); err != nil {
				return err
			}

			// Register the window-wide pointer area last so it receives presses.
			area := clip.Rect(image.Rectangle{Max: e.Size}).Push(gtx.Ops)
			event.Op(gtx.Ops, tag)
			area.Pop()

			e.Frame(gtx.Ops)
			if changed {
				// State changed after this frame's layout; redraw once.
				// No unconditional Invalidate: an idle window must not spin.
				w.Invalidate()
			}
		}
	}
}

// keyNames maps Gio key names to the session's keys. Space arrives as text
// (an EditEvent) and becomes a key press when no text field has focus.
var keyNames = map[key.Name]session.Key{
	key.NameTab:            session.KeyTab,
	key.NameReturn:         session.KeyEnter,
	key.NameEnter:          session.KeyEnter,
	key.NameEscape:         session.KeyEscape,
	key.NameDeleteBackward: session.KeyBackspace,
	key.NameDeleteForward:  session.KeyDelete,
	key.NameLeftArrow:      session.KeyLeft,
	key.NameRightArrow:     session.KeyRight,
	key.NameUpArrow:        session.KeyUp,
	key.NameDownArrow:      session.KeyDown,
	key.NameHome:           session.KeyHome,
	key.NameEnd:            session.KeyEnd,
}

// deliverInput feeds this frame's pointer, key and text events to the
// session in order, and reports whether any changed the frame.
func deliverInput(gtx layout.Context, tag event.Tag, s SessionAdapter, scale float64) (bool, error) {
	filters := []event.Filter{
		key.FocusFilter{Target: tag},
		pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Release | pointer.Move | pointer.Drag | pointer.Leave},
	}
	for name := range keyNames {
		filters = append(filters, key.Filter{Focus: tag, Name: name, Optional: key.ModShift})
	}
	changed := false
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return changed, nil
		}
		var did bool
		var err error
		switch e := ev.(type) {
		case pointer.Event:
			at := session.PointerEvent{X: float64(e.Position.X) / scale, Y: float64(e.Position.Y) / scale}
			switch e.Kind {
			case pointer.Press:
				gtx.Execute(key.FocusCmd{Tag: tag})
				var res *session.DispatchResult
				res, err = s.PointerDown(at)
				did = res != nil
			case pointer.Release:
				did, err = s.PointerUp(at)
			case pointer.Move, pointer.Drag:
				did, err = s.PointerMove(at)
			case pointer.Leave:
				did, err = s.PointerLeave()
			}
		case key.Event:
			if e.State != key.Press {
				continue
			}
			k := keyNames[e.Name]
			if k == session.KeyTab && e.Modifiers.Contain(key.ModShift) {
				k = session.KeyShiftTab
			}
			did, err = s.KeyPress(k)
		case key.EditEvent:
			did, err = s.TextInput(e.Text)
			if err == nil && !did && e.Text == " " {
				did, err = s.KeyPress(session.KeySpace)
			}
		}
		if err != nil {
			return changed, err
		}
		changed = changed || did
	}
}

func draw(gtx layout.Context, th *material.Theme, s SessionAdapter, scale float64) error {
	cmds, err := s.Commands()
	if err != nil {
		return err
	}
	ops, err := Translate(cmds)
	if err != nil {
		return err
	}
	px := func(r machinalayout.Rect) image.Rectangle {
		return image.Rect(int(r.X*scale), int(r.Y*scale), int((r.X+r.Width)*scale), int((r.Y+r.Height)*scale))
	}
	stack := []clip.Stack{}
	for _, d := range ops {
		switch d.Kind {
		case DrawOpFillRect:
			switch d.Fill {
			case render.FillCheck:
				drawCheck(gtx, px(d.Rect), nrgba(d.Paint.Background), scale)
			case render.FillButton, render.FillButtonDisabled, render.FillButtonHover, render.FillButtonPressed:
				r := px(machinalayout.Rect{X: d.Rect.X + buttonInset, Y: d.Rect.Y + buttonInset,
					Width: d.Rect.Width - 2*buttonInset, Height: d.Rect.Height - 2*buttonInset})
				radius := int(4 * scale)
				fill := colorButton
				switch d.Fill {
				case render.FillButtonHover:
					fill = colorButtonHover
				case render.FillButtonPressed:
					fill = colorButtonPressed
				}
				if d.Fill == render.FillButtonDisabled {
					fill = colorButtonDisabled
				} else {
					// 1dp border: paint the outline, then the face inset by it.
					paint.FillShape(gtx.Ops, colorButtonBorder, clip.UniformRRect(r, radius).Op(gtx.Ops))
					b := int(scale)
					r = image.Rect(r.Min.X+b, r.Min.Y+b, r.Max.X-b, r.Max.Y-b)
				}
				paint.FillShape(gtx.Ops, fill, clip.UniformRRect(r, radius).Op(gtx.Ops))
			case render.FillStyled:
				if d.Paint != nil {
					drawPaint(gtx, px(d.Rect), *d.Paint, scale)
				}
			default:
				paint.FillShape(gtx.Ops, colorFill, clip.Rect(px(d.Rect)).Op())
			}
		case DrawOpDrawText:
			r := px(d.Rect)
			off := op.Offset(r.Min).Push(gtx.Ops)
			lgtx := gtx
			lgtx.Constraints = layout.Exact(r.Size())
			size := d.Size
			if size <= 0 {
				size = 16
			}
			label := material.Label(th, unit.Sp(size), d.Text)
			label.Font = FontFor(uiir.TextSpec{Weight: d.Weight, Mono: d.Mono})
			label.Color = colorText
			if d.Color != nil {
				label.Color = nrgba(*d.Color)
			}
			if d.Muted {
				label.Color = colorTextMuted
			}
			// One line: layout sizes text with fixed metrics, so a label a
			// little wider than its rect must not wrap out of sight.
			label.MaxLines = 1
			if d.Align == render.AlignCenter {
				// Measure, then offset to the center of the rect.
				macro := op.Record(gtx.Ops)
				mgtx := lgtx
				mgtx.Constraints = layout.Constraints{Max: image.Pt(r.Dx()*2, r.Dy())}
				dims := label.Layout(mgtx)
				call := macro.Stop()
				dx := (r.Dx() - dims.Size.X) / 2
				dy := (r.Dy() - dims.Size.Y) / 2
				c := op.Offset(image.Pt(dx, dy)).Push(gtx.Ops)
				call.Add(gtx.Ops)
				c.Pop()
			} else {
				lgtx.Constraints = layout.Constraints{Max: image.Pt(r.Dx()*2, r.Dy()*2)}
				label.Layout(lgtx)
			}
			off.Pop()
		case DrawOpPushClip:
			stack = append(stack, clip.Rect(px(d.Rect)).Push(gtx.Ops))
		case DrawOpPopClip:
			if n := len(stack); n > 0 {
				stack[n-1].Pop()
				stack = stack[:n-1]
			}
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		stack[i].Pop()
	}
	return nil
}

func nrgba(c uiir.RGBA) color.NRGBA { return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A} }

// drawPaint draws a styled node: the border color fills the rounded rect,
// then the background fills it inset by the border width.
func drawPaint(gtx layout.Context, r image.Rectangle, p render.Paint, scale float64) {
	radius := int(p.Radius * scale)
	bw := int(p.BorderWidth * scale)
	if bw > 0 && p.Border.A > 0 && p.Background.A < 255 {
		// A see-through face (a focus ring): stroke the border so what is
		// under it shows through.
		if p.Background.A > 0 {
			paint.FillShape(gtx.Ops, nrgba(p.Background), clip.UniformRRect(r, radius).Op(gtx.Ops))
		}
		half := bw / 2
		ring := image.Rect(r.Min.X+half, r.Min.Y+half, r.Max.X-half, r.Max.Y-half)
		paint.FillShape(gtx.Ops, nrgba(p.Border), clip.Stroke{Path: clip.UniformRRect(ring, max(radius-half, 0)).Path(gtx.Ops), Width: float32(bw)}.Op())
		return
	}
	if bw > 0 && p.Border.A > 0 {
		paint.FillShape(gtx.Ops, nrgba(p.Border), clip.UniformRRect(r, radius).Op(gtx.Ops))
		r = image.Rect(r.Min.X+bw, r.Min.Y+bw, r.Max.X-bw, r.Max.Y-bw)
		radius -= bw
		if radius < 0 {
			radius = 0
		}
	}
	if r.Dx() > 0 && r.Dy() > 0 && p.Background.A > 0 {
		paint.FillShape(gtx.Ops, nrgba(p.Background), clip.UniformRRect(r, radius).Op(gtx.Ops))
	}
}

// drawCheck strokes a check mark inside r.
func drawCheck(gtx layout.Context, r image.Rectangle, c color.NRGBA, scale float64) {
	x, y, w, h := float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy())
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(x+0.22*w, y+0.52*h))
	p.LineTo(f32.Pt(x+0.42*w, y+0.72*h))
	p.LineTo(f32.Pt(x+0.78*w, y+0.30*h))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: float32(2 * scale)}.Op())
}
