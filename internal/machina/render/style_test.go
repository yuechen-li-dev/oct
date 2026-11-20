package render

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

var panel = &uiir.StyleSpec{
	Background:  uiir.RGBA{R: 32, G: 64, B: 128, A: 255},
	Foreground:  uiir.RGBA{R: 255, G: 255, B: 255, A: 255},
	BorderColor: uiir.RGBA{R: 16, G: 32, B: 64, A: 255},
	BorderWidth: 2,
	Radius:      6,
}

func snapshotOf(t *testing.T, root *uiir.Node) string {
	t.Helper()
	low, err := lowering.Lower(root)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := layout.ResolveRows(low.Rows, layout.Rect{Width: 300, Height: 200})
	if err != nil {
		t.Fatal(err)
	}
	commands, err := BuildCommands(resolved, low.Semantics, low.Styles, Options{})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := RecordSnapshot(commands)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestStyledTextGetsPaintThenColoredText(t *testing.T) {
	snap := snapshotOf(t, &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{
		{Kind: uiir.NodeText, Text: "Setpoint", Style: panel},
	}})
	fill := strings.Index(snap, "FillRect node=0.0")
	text := strings.Index(snap, `DrawText node=0.0`)
	if fill < 0 || text < 0 || fill > text {
		t.Fatalf("styled text must paint before its text:\n%s", snap)
	}
	for _, want := range []string{"fill=styled", "bg=#204080ff", "border=#102040ff/2.00", "radius=6.00", "color=#ffffffff"} {
		if !strings.Contains(snap, want) {
			t.Errorf("snapshot lacks %q:\n%s", want, snap)
		}
	}
}

func TestStyledButtonReplacesDefaultChrome(t *testing.T) {
	snap := snapshotOf(t, &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{
		{Kind: uiir.NodeButton, Label: "Raise", Event: "raise", Enabled: true, Style: panel},
	}})
	if strings.Contains(snap, "fill=button") {
		t.Fatalf("styled button still emits default chrome:\n%s", snap)
	}
	if !strings.Contains(snap, "fill=styled") || !strings.Contains(snap, `text="Raise" align=center color=#ffffffff`) {
		t.Fatalf("styled button paint or label missing:\n%s", snap)
	}
}

func TestUnstyledSnapshotsAreUnchanged(t *testing.T) {
	snap := snapshotOf(t, &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{{Kind: uiir.NodeText, Text: "Hello"}}})
	for _, absent := range []string{"fill=", "color=", "bg="} {
		if strings.Contains(snap, absent) {
			t.Fatalf("unstyled text gained %q:\n%s", absent, snap)
		}
	}
}
