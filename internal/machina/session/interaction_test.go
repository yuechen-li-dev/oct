package session

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// formState is a model owned entirely by the app: the inputs only propose.
type formState struct {
	Name   string
	Audio  bool
	Volume float64
	Saves  int
}

type formApp struct{}

func (formApp) Project(s formState) (*uiir.Node, error) {
	style := uiir.StyleSpec{Background: uiir.RGBA{R: 1, A: 255}}
	return &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{
		{Kind: uiir.NodeTextField, Event: "Rename", Callback: "rename", Input: &uiir.InputSpec{Value: s.Name}},
		{Kind: uiir.NodeCheckbox, Label: "Audio", Event: "SetAudio", Callback: "audio", Input: &uiir.InputSpec{Checked: s.Audio}},
		{Kind: uiir.NodeSlider, Event: "SetVolume", Callback: "volume", Input: &uiir.InputSpec{Number: s.Volume, Max: 1}},
		{Kind: uiir.NodeButton, Label: "Save", Event: "Save", Enabled: true, Style: &style,
			States: &uiir.StateStyles{Hover: uiir.StyleSpec{Background: uiir.RGBA{R: 2, A: 255}}, Pressed: uiir.StyleSpec{Background: uiir.RGBA{R: 3, A: 255}}, Focused: uiir.StyleSpec{Background: uiir.RGBA{R: 4, A: 255}}}},
	}}, nil
}

func (formApp) Dispatch(s formState, a lowering.Action) (formState, error) {
	switch a.Callback {
	case "rename":
		s.Name = a.Value.(string)
	case "audio":
		s.Audio = a.Value.(bool)
	case "volume":
		s.Volume = a.Value.(float64)
	default:
		if a.Name == "Save" {
			s.Saves++
		}
	}
	return s, nil
}

func newForm(t *testing.T) *Session[formState] {
	t.Helper()
	s, err := NewSession[formState](formApp{}, formState{Name: "ab", Volume: 0.5}, layout.Rect{Width: 400, Height: 300})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func center(t *testing.T, s *Session[formState], alias string) PointerEvent {
	t.Helper()
	r, ok := s.ActionRect(alias)
	if !ok {
		t.Fatalf("no control %s", alias)
	}
	return PointerEvent{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestTextFieldEditsRoundTripThroughTheModel(t *testing.T) {
	s := newForm(t)
	res, err := s.PointerDown(center(t, s, "Rename"))
	must(t, err)
	if res.Action != nil || s.Focused() == "" {
		t.Fatalf("pressing a text field focuses it without sending: %+v, focus %q", res.Action, s.Focused())
	}
	_, err = s.KeyPress(KeyEnd)
	must(t, err)
	_, err = s.TextInput("cd")
	must(t, err)
	_, err = s.KeyPress(KeyLeft)
	must(t, err)
	_, err = s.KeyPress(KeyBackspace)
	must(t, err)
	if got := s.State().Name; got != "abd" {
		t.Fatalf("name = %q, want abd", got)
	}
	if s.Caret() != 2 {
		t.Fatalf("caret = %d, want 2", s.Caret())
	}
	snap, _ := s.Snapshot()
	if !strings.Contains(snap, "abd") {
		t.Fatalf("the frame shows the model's value: %s", snap)
	}
}

func TestCheckboxAndSliderProposeValues(t *testing.T) {
	s := newForm(t)
	_, err := s.PointerDown(center(t, s, "SetAudio"))
	must(t, err)
	if !s.State().Audio {
		t.Fatal("pressing the checkbox proposes the opposite value")
	}
	_, err = s.KeyPress(KeySpace)
	must(t, err)
	if s.State().Audio {
		t.Fatal("Space toggles the focused checkbox")
	}
	r, _ := s.ActionRect("SetVolume")
	_, err = s.PointerDown(PointerEvent{X: r.X, Y: r.Y + r.Height/2})
	must(t, err)
	if s.State().Volume != 0 {
		t.Fatalf("a press at the left end proposes the minimum, got %g", s.State().Volume)
	}
	_, err = s.PointerMove(PointerEvent{X: r.X + r.Width, Y: r.Y})
	must(t, err)
	if s.State().Volume != 1 {
		t.Fatalf("dragging to the right end proposes the maximum, got %g", s.State().Volume)
	}
	_, err = s.PointerUp(PointerEvent{})
	must(t, err)
	_, err = s.KeyPress(KeyLeft)
	must(t, err)
	if v := s.State().Volume; v < 0.949 || v > 0.951 {
		t.Fatalf("Left steps the focused slider down by a twentieth, got %g", v)
	}
}

func TestTabWalksControlsAndStateStylesFollow(t *testing.T) {
	s := newForm(t)
	var walked []layout.NodeID
	for i := 0; i < 5; i++ {
		_, err := s.KeyPress(KeyTab)
		must(t, err)
		walked = append(walked, s.Focused())
	}
	if walked[0] != walked[4] || walked[0] == walked[1] {
		t.Fatalf("Tab walks the four controls and wraps: %v", walked)
	}
	_, err := s.KeyPress(KeyShiftTab)
	must(t, err)
	snap, _ := s.Snapshot()
	if !strings.Contains(snap, "bg=#040000ff") {
		t.Fatalf("the focused button paints its Focused style: %s", snap)
	}
	_, err = s.KeyPress(KeyEnter)
	must(t, err)
	if s.State().Saves != 1 {
		t.Fatal("Enter presses the focused button")
	}
	_, err = s.PointerMove(center(t, s, "Save"))
	must(t, err)
	_, err = s.KeyPress(KeyEscape)
	must(t, err)
	snap, _ = s.Snapshot()
	if !strings.Contains(snap, "bg=#020000ff") {
		t.Fatalf("without focus, the hovered button paints its Hover style: %s", snap)
	}
}
