package render

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// textFrame renders one Text node with spec into a w x h box.
func textFrame(t *testing.T, label string, spec uiir.TextSpec, w, h float64) ([]Command, error) {
	t.Helper()
	root := layout.NodeID("0")
	resolved := &layout.ResolvedLayoutDocument{RootID: root,
		Nodes:    map[layout.NodeID]layout.ResolvedLayoutNode{root: {ID: root, Rect: layout.Rect{Width: w, Height: h}}},
		Children: map[layout.NodeID][]layout.NodeID{}}
	sem := map[layout.NodeID]lowering.Semantics{root: {Role: lowering.RoleText, Label: label, Text: &spec}}
	return BuildCommands(resolved, sem, nil, Options{})
}

func texts(cmds []Command) []string {
	var out []string
	for _, c := range cmds {
		if d, ok := c.(DrawTextCommand); ok {
			out = append(out, d.Text)
		}
	}
	return out
}

func hasClip(cmds []Command) bool {
	for _, c := range cmds {
		if _, ok := c.(PushClipCommand); ok {
			return true
		}
	}
	return false
}

var body = uiir.TextSpec{Size: 16, Weight: 400}

func TestOverflowEllipsisShortensALineThatIsTooWide(t *testing.T) {
	spec := body
	spec.Overflow = uiir.OverflowEllipsis
	cmds, err := textFrame(t, "a long label", spec, 64, 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(cmds); len(got) != 1 || got[0] != "a long…" {
		t.Fatalf("texts = %q", got)
	}
}

func TestOverflowClipDrawsTheWholeLineInsideAClip(t *testing.T) {
	spec := body
	spec.Overflow = uiir.OverflowClip
	cmds, err := textFrame(t, "a long label", spec, 64, 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(cmds); got[0] != "a long label" || !hasClip(cmds) {
		t.Fatalf("texts = %q, clip = %v", got, hasClip(cmds))
	}
	if snap, err := RecordSnapshot(cmds); err != nil || !strings.Contains(snap, "PushClip") {
		t.Fatalf("snapshot = %q, %v", snap, err)
	}
}

func TestOverflowErrorFailsTheFrame(t *testing.T) {
	spec := body
	spec.Overflow = uiir.OverflowError
	_, err := textFrame(t, "a long label", spec, 64, 20)
	if err == nil || !strings.Contains(err.Error(), `node "0" text "a long label" needs 96 x 20 but its box is 64 x 20`) {
		t.Fatalf("error = %v", err)
	}
	spec.Wrap = true
	_, err = textFrame(t, "one two three four five six", spec, 80, 40)
	if err == nil || !strings.Contains(err.Error(), "wraps to 3 lines but its 80x40 box has room for 2") {
		t.Fatalf("wrapped error = %v", err)
	}
}

func TestWrappedTextDrawsTheLinesThatFitAndMarksTheCut(t *testing.T) {
	spec := body
	spec.Wrap = true
	spec.Overflow = uiir.OverflowEllipsis
	cmds, err := textFrame(t, "one two three four five six", spec, 80, 40)
	if err != nil {
		t.Fatal(err)
	}
	got := texts(cmds)
	if strings.Join(got, "|") != "one two|three fou…" {
		t.Fatalf("lines = %q", got)
	}
	if d := cmds[2].(DrawTextCommand); d.Rect != (layout.Rect{Y: 20, Width: 80, Height: 20}) {
		t.Fatalf("second line rect = %+v", d.Rect)
	}
}

func TestTextSpecFontReachesTheCommand(t *testing.T) {
	cmds, err := textFrame(t, "Title", uiir.TextSpec{Size: 22, Weight: 700, Mono: true, Color: &uiir.RGBA{R: 1, G: 2, B: 3, A: 255}}, 200, 40)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := RecordSnapshot(cmds)
	if !strings.Contains(snap, `text="Title" color=#010203ff size=22 weight=700 mono`) {
		t.Fatalf("snapshot = %s", snap)
	}
}
