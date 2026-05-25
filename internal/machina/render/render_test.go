package render

import (
	"math"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func TestSnapshotRecorderLifecycleAndFormatting(t *testing.T) {
	snap, err := RecordSnapshot([]Command{BeginFrameCommand{Root: layout.Rect{Width: 800, Height: 600}}, EndFrameCommand{}})
	if err != nil || snap != "BeginFrame rect=(0.00,0.00,800.00,600.00)\nEndFrame" {
		t.Fatalf("snap=%q err=%v", snap, err)
	}
	if _, err := RecordSnapshot([]Command{EndFrameCommand{}}); err == nil {
		t.Fatal("expected end without begin error")
	}
	if _, err := RecordSnapshot([]Command{BeginFrameCommand{}, BeginFrameCommand{}}); err == nil {
		t.Fatal("expected nested begin error")
	}
	if _, err := RecordSnapshot([]Command{BeginFrameCommand{}, EndFrameCommand{}, DrawTextCommand{Text: "x"}}); err == nil {
		t.Fatal("expected command after end error")
	}
	if _, err := RecordSnapshot([]Command{BeginFrameCommand{}, PushClipCommand{NodeID: "n", Rect: layout.Rect{Width: 1, Height: 1}}, PopClipCommand{NodeID: "n"}, EndFrameCommand{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordSnapshot([]Command{BeginFrameCommand{}, PushClipCommand{NodeID: "n", Rect: layout.Rect{Width: 1, Height: 1}}, EndFrameCommand{}}); err == nil {
		t.Fatal("expected unbalanced clip error")
	}
	snap, err = RecordSnapshot([]Command{BeginFrameCommand{}, DrawTextCommand{NodeID: "n", Rect: layout.Rect{X: 1.23456, Y: 2, Width: 3, Height: 4.567}, Text: "t"}, EndFrameCommand{}})
	if err != nil || !strings.Contains(snap, "rect=(1.23,2.00,3.00,4.57)") {
		t.Fatalf("expected deterministic float format, got %q err=%v", snap, err)
	}
}

func TestBuildCommandsEmitsTextAndButtonAndIgnoresActions(t *testing.T) {
	root := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{{Kind: uiir.NodeText, Text: "Hello"}, {Kind: uiir.NodeButton, Label: "Go", Enabled: true, Event: "submit"}, {Kind: uiir.NodeButton, Label: "Stop", Enabled: false, Event: "halt"}}}
	low, _ := lowering.Lower(root)
	resolved, _ := layout.ResolveRows(low.Rows, layout.Rect{X: 0, Y: 0, Width: 300, Height: 200})
	commands, err := BuildCommands(resolved, low.Semantics, low.Styles, Options{})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := RecordSnapshot(commands)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snap, `DrawText node=0.0`) || !strings.Contains(snap, `text="Hello"`) {
		t.Fatalf("missing text draw: %s", snap)
	}
	if !strings.Contains(snap, `DrawText node=0.1`) || !strings.Contains(snap, `text="Go"`) {
		t.Fatalf("missing enabled button draw: %s", snap)
	}
	if !strings.Contains(snap, `DrawText node=0.2`) || !strings.Contains(snap, `text="Stop"`) {
		t.Fatalf("missing disabled button draw: %s", snap)
	}
	if strings.Contains(snap, "submit") || strings.Contains(snap, "halt") {
		t.Fatalf("actions leaked into render commands: %s", snap)
	}
}

func TestBuildCommandsDeterministicOrderAndNilCases(t *testing.T) {
	if _, err := BuildCommands(nil, nil, nil, Options{}); err == nil {
		t.Fatal("expected nil resolved error")
	}
	r, err := layout.ResolveRows([]layout.LayoutRow{{ID: "root", Frame: layout.FrameSpec{Kind: layout.RootFrame}}, {ID: "b", Parent: ptr("root"), Order: 1, Z: 0, Frame: layout.FrameSpec{Kind: layout.AbsoluteFrame, X: 0, Y: 0, Width: 10, Height: 10}}, {ID: "a", Parent: ptr("root"), Order: 0, Z: 0, Frame: layout.FrameSpec{Kind: layout.AbsoluteFrame, X: 0, Y: 0, Width: 10, Height: 10}}}, layout.Rect{Width: 20, Height: 20})
	if err != nil {
		t.Fatal(err)
	}
	sem := map[layout.NodeID]lowering.Semantics{"a": {Role: lowering.RoleText, Label: "A"}, "b": {Role: lowering.RoleText, Label: "B"}}
	commands, err := BuildCommands(r, sem, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := RecordSnapshot(commands)
	if strings.Index(snap, `node=a`) > strings.Index(snap, `node=b`) {
		t.Fatalf("expected a before b:\n%s", snap)
	}
	empty, _ := BuildCommands(r, nil, nil, Options{})
	emptySnap, _ := RecordSnapshot(empty)
	if emptySnap != "BeginFrame rect=(0.00,0.00,20.00,20.00)\nEndFrame" {
		t.Fatalf("unexpected empty snapshot %q", emptySnap)
	}

	r.Nodes["a"] = layout.ResolvedLayoutNode{ID: "a", Rect: layout.Rect{X: math.NaN(), Y: 0, Width: 1, Height: 1}}
	if _, err := BuildCommands(r, sem, nil, Options{}); err == nil {
		t.Fatal("expected non-finite rect error")
	}
}

func ptr(id layout.NodeID) *layout.NodeID { return &id }
