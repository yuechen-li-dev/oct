package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/octapp"
	"github.com/yuechen-li-dev/oct/internal/machina/render"
	"github.com/yuechen-li-dev/oct/internal/machina/session"
)

// uiWindowRunner opens a native window for a session. It is nil in default
// builds and set by ui_gio.go in builds tagged machina_gio, so the default
// oct binary keeps zero native windowing dependencies.
var uiWindowRunner func(opts uiOptions, s *session.Session[interpret.Value]) error

type uiOptions struct {
	sub    string
	path   string
	title  string
	width  float64
	height float64
	steps  []uiStep
	dump   bool
}

// uiStep is one scripted input for `oct ui check`, applied in order.
type uiStep struct {
	kind  string // "click", "type" or "key"
	value string
}

const uiUsage = "usage: oct ui <run|check> <app.oct> [--title <text>] [--width <dp>] [--height <dp>] [--click <label>] [--type <text>] [--key <key>]... [--dump]"

func writeUIHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, uiUsage+`
Run an Oct program as a Machina UI app. An app package declares:
  fn Init() -> Model
  fn Update(model: Model, msg: Msg) -> Model     (Msg: the app's message enum)
  fn View(model: Model) -> UI<Msg>
Subcommands:
  run    Open the app in a native window. Requires an oct binary built with
         the native backend: go build -tags machina_gio ./cmd/oct
  check  Load the app headlessly and print its first frame. Then, in
         order, each --click presses the control with that exact label, or
         the one sending that message or callback (for example
         'Msg.Inspect("harbor-board")' or 'Msg.Rename'), at its center;
         each --type types text into the focused text field; each --key
         presses a key (Tab, Shift+Tab, Enter, Space, Escape, Backspace,
         Delete, Left, Right, Up, Down, Home, End). Each step prints the
         next frame.
         --dump prints each frame as a layout dump instead: one line per
         node in paint order, with the spans that place it and its rect.
Width and height are device-independent units (default 640x480).`)
	return err
}

func parseUIOptions(args []string) (uiOptions, error) {
	opts := uiOptions{width: 640, height: 480}
	if len(args) < 2 {
		return opts, errors.New(uiUsage)
	}
	opts.sub = args[0]
	if opts.sub != "run" && opts.sub != "check" {
		return opts, fmt.Errorf("unknown ui subcommand %q; %s", opts.sub, uiUsage)
	}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		value := func() (string, error) {
			if i+1 >= len(rest) {
				return "", fmt.Errorf("%s requires a value", arg)
			}
			i++
			return rest[i], nil
		}
		switch arg {
		case "--title":
			v, err := value()
			if err != nil {
				return opts, err
			}
			opts.title = v
		case "--width", "--height":
			v, err := value()
			if err != nil {
				return opts, err
			}
			n, err := strconv.ParseFloat(v, 64)
			if err != nil || n <= 0 {
				return opts, fmt.Errorf("%s must be a positive number, got %q", arg, v)
			}
			if arg == "--width" {
				opts.width = n
			} else {
				opts.height = n
			}
		case "--dump":
			opts.dump = true
		case "--click", "--type", "--key":
			v, err := value()
			if err != nil {
				return opts, err
			}
			opts.steps = append(opts.steps, uiStep{kind: strings.TrimPrefix(arg, "--"), value: v})
		default:
			if strings.HasPrefix(arg, "--") {
				return opts, fmt.Errorf("unknown option %s; %s", arg, uiUsage)
			}
			if opts.path != "" {
				return opts, fmt.Errorf("unexpected argument %q; %s", arg, uiUsage)
			}
			opts.path = arg
		}
	}
	if opts.path == "" {
		return opts, errors.New(uiUsage)
	}
	if opts.sub == "run" && len(opts.steps) > 0 {
		return opts, fmt.Errorf("--click, --type and --key apply to `oct ui check` only")
	}
	if opts.sub == "run" && opts.dump {
		return opts, fmt.Errorf("--dump applies to `oct ui check` only")
	}
	return opts, nil
}

func runUICommand(opts uiOptions, stdout io.Writer) error {
	if opts.sub == "run" && uiWindowRunner == nil {
		// Fail before loading: the build, not the program, is the problem.
		return errors.New("this oct binary was built without the native UI backend.\n" +
			"Rebuild it with:  go build -tags machina_gio ./cmd/oct\n" +
			"Or check the app headlessly with:  oct ui check " + opts.path)
	}
	s, err := octapp.NewSession(opts.path, layout.Rect{Width: opts.width, Height: opts.height}, stdout)
	if err != nil {
		return err
	}
	if opts.sub == "run" {
		if opts.title == "" {
			opts.title = opts.path
		}
		return uiWindowRunner(opts, s)
	}
	fmt.Fprintf(stdout, "Machina UI app OK: %s (%gx%g)\n", opts.path, opts.width, opts.height)
	if err := writeUIFrame(stdout, s, opts.dump); err != nil {
		return err
	}
	for _, step := range opts.steps {
		line, err := runUIStep(s, step)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "\n%s\n", line)
		if err := writeUIFrame(stdout, s, opts.dump); err != nil {
			return err
		}
	}
	return nil
}

// runUIStep applies one scripted input and describes what it did.
func runUIStep(s *session.Session[interpret.Value], step uiStep) (string, error) {
	switch step.kind {
	case "type":
		if s.Focused() == "" {
			return "", fmt.Errorf("--type %q: nothing has focus; --click a text field first", step.value)
		}
		if _, err := s.TextInput(step.value); err != nil {
			return "", err
		}
		return fmt.Sprintf("type %q", step.value), nil
	case "key":
		key := session.Key(step.value)
		known := false
		for _, k := range session.Keys {
			known = known || k == key
		}
		if !known {
			return "", fmt.Errorf("--key %q: unknown key", step.value)
		}
		if _, err := s.KeyPress(key); err != nil {
			return "", err
		}
		return fmt.Sprintf("key %s (focus %s)", key, s.Focused()), nil
	}
	cmds, err := s.Commands()
	if err != nil {
		return "", err
	}
	target, ok := findLabel(cmds, step.value)
	if !ok {
		target, ok = s.ActionRect(step.value)
	}
	if !ok {
		return "", fmt.Errorf("--click %q: no element with that exact label or message in the current frame", step.value)
	}
	at := session.PointerEvent{X: target.X + target.Width/2, Y: target.Y + target.Height/2}
	res, err := s.PointerDown(at)
	if err != nil {
		return "", err
	}
	if !res.Hit {
		return "", fmt.Errorf("--click %q: the label is visible but not interactive", step.value)
	}
	if _, err := s.PointerUp(at); err != nil {
		return "", err
	}
	if res.Action == nil {
		return fmt.Sprintf("click %q -> focus", step.value), nil
	}
	return fmt.Sprintf("click %q -> %s", step.value, res.Action.Name), nil
}

func findLabel(cmds []render.Command, label string) (layout.Rect, bool) {
	for _, c := range cmds {
		if t, ok := c.(render.DrawTextCommand); ok && t.Text == label {
			return t.Rect, true
		}
	}
	return layout.Rect{}, false
}

// writeUIFrame prints the frame's text in reading order with positions.
func writeUIFrame(out io.Writer, s *session.Session[interpret.Value], asDump bool) error {
	if asDump {
		text, err := s.Dump()
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "dump:")
		_, err = io.WriteString(out, text)
		return err
	}
	cmds, err := s.Commands()
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "frame:")
	for _, c := range cmds {
		if t, ok := c.(render.DrawTextCommand); ok {
			fmt.Fprintf(out, "  %-28q at (%g, %g) size %gx%g\n", t.Text, t.Rect.X, t.Rect.Y, t.Rect.Width, t.Rect.Height)
		}
	}
	return nil
}
