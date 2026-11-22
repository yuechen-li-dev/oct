package session

import (
	"fmt"
	"math"

	"github.com/yuechen-li-dev/oct/internal/machina/hittest"
	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/textmetrics"
)

// Key is a key the session handles. Printable text arrives through
// TextInput instead.
type Key string

const (
	KeyTab       Key = "Tab"
	KeyShiftTab  Key = "Shift+Tab"
	KeyEnter     Key = "Enter"
	KeySpace     Key = "Space"
	KeyEscape    Key = "Escape"
	KeyBackspace Key = "Backspace"
	KeyDelete    Key = "Delete"
	KeyLeft      Key = "Left"
	KeyRight     Key = "Right"
	KeyUp        Key = "Up"
	KeyDown      Key = "Down"
	KeyHome      Key = "Home"
	KeyEnd       Key = "End"
)

// Keys lists the keys KeyPress accepts.
var Keys = []Key{KeyTab, KeyShiftTab, KeyEnter, KeySpace, KeyEscape, KeyBackspace, KeyDelete, KeyLeft, KeyRight, KeyUp, KeyDown, KeyHome, KeyEnd}

// sliderSteps is how many key presses cross a slider's range.
const sliderSteps = 20

// Focused is the node with keyboard focus, or "".
func (s *Session[TState]) Focused() layout.NodeID {
	if s == nil {
		return ""
	}
	return s.interaction.Focus
}

// Caret is the caret's rune index in the focused text field.
func (s *Session[TState]) Caret() int {
	if s == nil {
		return 0
	}
	return s.interaction.Caret
}

// settleInteraction keeps interaction state that still names a control in
// the new frame and drops the rest; a node keeps its id while the tree
// keeps its shape (or its UI.Keyed key).
func (s *Session[TState]) settleInteraction() {
	targets := map[layout.NodeID]bool{}
	for _, id := range s.index.Targets() {
		targets[id] = true
	}
	for _, id := range []*layout.NodeID{&s.interaction.Hover, &s.interaction.Pressed, &s.interaction.Focus, &s.dragging} {
		if !targets[*id] {
			*id = ""
		}
	}
	if sem, ok := s.lowered.Semantics[s.interaction.Focus]; ok && sem.Role == lowering.RoleTextField {
		s.interaction.Caret = min(max(s.interaction.Caret, 0), len([]rune(sem.Input.Value)))
	} else {
		s.interaction.Caret = 0
	}
}

// PointerMove tracks the control under the pointer and drags a pressed
// slider. It reports whether the frame changed.
func (s *Session[TState]) PointerMove(event PointerEvent) (bool, error) {
	if s == nil {
		return false, fmt.Errorf("session.err.nil_session: session is required")
	}
	if s.dragging != "" {
		sem := s.lowered.Semantics[s.dragging]
		if v := s.sliderValueAt(s.dragging, event.X); v != sem.Input.Number {
			action := s.lowered.Actions[s.dragging]
			action.Value = v
			return true, s.send(action)
		}
		return false, nil
	}
	hover := layout.NodeID("")
	hit, ok, err := s.index.HitTest(hittest.Point{X: event.X, Y: event.Y}, nil)
	if err != nil {
		return false, fmt.Errorf("session.err.hit_test: %w", err)
	}
	if ok {
		hover = hit.NodeID
	}
	if hover == s.interaction.Hover {
		return false, nil
	}
	s.interaction.Hover = hover
	return true, s.repaint()
}

// PointerLeave clears hover when the pointer leaves the window; a drag in
// progress continues.
func (s *Session[TState]) PointerLeave() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("session.err.nil_session: session is required")
	}
	if s.interaction.Hover == "" {
		return false, nil
	}
	s.interaction.Hover = ""
	return true, s.repaint()
}

// PointerUp ends a press and a slider drag.
func (s *Session[TState]) PointerUp(event PointerEvent) (bool, error) {
	if s == nil {
		return false, fmt.Errorf("session.err.nil_session: session is required")
	}
	if s.interaction.Pressed == "" && s.dragging == "" {
		return false, nil
	}
	s.interaction.Pressed, s.dragging = "", ""
	return true, s.repaint()
}

// KeyPress handles a key for the focused control: Tab and Shift+Tab move
// focus in paint order, Escape clears it, Enter and Space press a button,
// Space toggles a checkbox, the arrows, Home and End move a slider or a
// text field's caret, and Backspace and Delete edit a text field. It
// reports whether the frame changed.
func (s *Session[TState]) KeyPress(key Key) (bool, error) {
	if s == nil {
		return false, fmt.Errorf("session.err.nil_session: session is required")
	}
	switch key {
	case KeyTab, KeyShiftTab:
		return true, s.moveFocus(key == KeyTab)
	case KeyEscape:
		if s.interaction.Focus == "" {
			return false, nil
		}
		s.interaction.Focus = ""
		return true, s.repaint()
	}
	id := s.interaction.Focus
	if id == "" {
		return false, nil
	}
	sem := s.lowered.Semantics[id]
	action := s.lowered.Actions[id]
	switch sem.Role {
	case lowering.RoleButton:
		if key == KeyEnter || key == KeySpace {
			return true, s.send(action)
		}
	case lowering.RoleCheckbox:
		if key == KeySpace || key == KeyEnter {
			action.Value = !sem.Input.Checked
			return true, s.send(action)
		}
	case lowering.RoleSlider:
		in := sem.Input
		step := (in.Max - in.Min) / sliderSteps
		v := in.Number
		switch key {
		case KeyLeft, KeyDown:
			v -= step
		case KeyRight, KeyUp:
			v += step
		case KeyHome:
			v = in.Min
		case KeyEnd:
			v = in.Max
		default:
			return false, nil
		}
		v = math.Min(math.Max(v, in.Min), in.Max)
		if v == in.Number {
			return false, nil
		}
		action.Value = v
		return true, s.send(action)
	case lowering.RoleTextField:
		runes := []rune(sem.Input.Value)
		caret := s.interaction.Caret
		switch key {
		case KeyLeft:
			s.interaction.Caret = max(caret-1, 0)
		case KeyRight:
			s.interaction.Caret = min(caret+1, len(runes))
		case KeyHome:
			s.interaction.Caret = 0
		case KeyEnd:
			s.interaction.Caret = len(runes)
		case KeyBackspace:
			if caret == 0 {
				return false, nil
			}
			return true, s.editText(id, string(runes[:caret-1])+string(runes[caret:]), caret-1)
		case KeyDelete:
			if caret >= len(runes) {
				return false, nil
			}
			return true, s.editText(id, string(runes[:caret])+string(runes[caret+1:]), caret)
		default:
			return false, nil
		}
		return true, s.repaint()
	}
	return false, nil
}

// TextInput types text into the focused text field at its caret.
func (s *Session[TState]) TextInput(text string) (bool, error) {
	if s == nil {
		return false, fmt.Errorf("session.err.nil_session: session is required")
	}
	id := s.interaction.Focus
	sem, ok := s.lowered.Semantics[id]
	if !ok || sem.Role != lowering.RoleTextField || text == "" {
		return false, nil
	}
	runes := []rune(sem.Input.Value)
	caret := s.interaction.Caret
	inserted := []rune(text)
	next := string(runes[:caret]) + text + string(runes[caret:])
	return true, s.editText(id, next, caret+len(inserted))
}

// editText proposes a new value for a text field; the caret goes where the
// edit left it, clamped to whatever value the model keeps.
func (s *Session[TState]) editText(id layout.NodeID, value string, caret int) error {
	action := s.lowered.Actions[id]
	action.Value = value
	s.interaction.Caret = caret
	return s.send(action)
}

func (s *Session[TState]) moveFocus(forward bool) error {
	targets := s.index.Targets()
	if len(targets) == 0 {
		return nil
	}
	at := -1
	for i, id := range targets {
		if id == s.interaction.Focus {
			at = i
		}
	}
	switch {
	case at < 0 && forward:
		at = 0
	case at < 0:
		at = len(targets) - 1
	case forward:
		at = (at + 1) % len(targets)
	default:
		at = (at - 1 + len(targets)) % len(targets)
	}
	s.interaction.Focus = targets[at]
	s.interaction.Caret = 0
	if sem := s.lowered.Semantics[targets[at]]; sem.Role == lowering.RoleTextField {
		s.interaction.Caret = len([]rune(sem.Input.Value))
	}
	return s.repaint()
}

func (s *Session[TState]) metricsOrFixed() textmetrics.Metrics {
	if s.metrics == nil {
		return textmetrics.Fixed{}
	}
	return s.metrics
}

// caretAt is the rune boundary in a text field nearest x.
func (s *Session[TState]) caretAt(id layout.NodeID, x float64) int {
	node := s.resolved.Nodes[id]
	m := s.metricsOrFixed()
	runes := []rune(s.lowered.Semantics[id].Input.Value)
	local := x - node.Rect.X - lowering.TextFieldPadX
	best, bestDist := 0, math.Inf(1)
	for i := 0; i <= len(runes); i++ {
		d := math.Abs(m.Width(string(runes[:i]), textmetrics.Body) - local)
		if d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

// sliderValueAt is the slider value under x: the thumb's center travels
// the track, clamped to the slider's range.
func (s *Session[TState]) sliderValueAt(id layout.NodeID, x float64) float64 {
	node := s.resolved.Nodes[id]
	in := s.lowered.Semantics[id].Input
	usable := node.Rect.Width - lowering.SliderThumb
	t := 0.0
	if usable > 0 {
		t = (x - node.Rect.X - lowering.SliderThumb/2) / usable
	}
	t = math.Min(math.Max(t, 0), 1)
	return in.Min + t*(in.Max-in.Min)
}
