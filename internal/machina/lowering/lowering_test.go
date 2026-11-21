package lowering

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func TestLowerTextRoot(t *testing.T) {
	root := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{{Kind: uiir.NodeText, Text: "hello"}}}
	out, err := Lower(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Rows) != 2 {
		t.Fatalf("rows=%d", len(out.Rows))
	}
	if out.Rows[0].Frame.Kind != layout.RootFrame {
		t.Fatalf("root kind=%s", out.Rows[0].Frame.Kind)
	}
	if out.Semantics[layout.NodeID("0.0")].Role != RoleText {
		t.Fatal("missing text semantics")
	}
}

func TestButtonActionsAndDisabledSemantics(t *testing.T) {
	enabled := &uiir.Node{Kind: uiir.NodeButton, Label: "Go", Enabled: true, Event: "submit"}
	out, err := Lower(&uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{enabled}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Actions["0.0"].Name != "submit" {
		t.Fatal("expected action")
	}
	disabled := &uiir.Node{Kind: uiir.NodeButton, Label: "Stop", Enabled: false, Event: "halt"}
	out2, err := Lower(&uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{disabled}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out2.Actions["0.0"]; ok {
		t.Fatal("disabled button should omit action")
	}
	sem := out2.Semantics["0.0"]
	if !sem.Disabled || sem.Focusable {
		t.Fatalf("bad semantics: %#v", sem)
	}
}

func TestRowColumnAndOrder(t *testing.T) {
	root := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{{Kind: uiir.NodeRow, Children: []*uiir.Node{{Kind: uiir.NodeText, Text: "a"}, {Kind: uiir.NodeText, Text: "b"}}}, {Kind: uiir.NodeText, Text: "c"}}}
	out, err := Lower(root)
	if err != nil {
		t.Fatal(err)
	}
	if out.Rows[0].Arrange.Axis != layout.AxisVertical {
		t.Fatal("root should be vertical")
	}
	if out.Rows[1].Arrange.Axis != layout.AxisHorizontal {
		t.Fatal("row should be horizontal")
	}
	if out.Rows[2].Order != 0 || out.Rows[3].Order != 1 {
		t.Fatal("child order not preserved")
	}
}

func TestAbsoluteAndAnchorResolve(t *testing.T) {
	abs := &uiir.Node{Kind: uiir.NodeAbsoluteBox, Box: &uiir.BoxSpec{Kind: uiir.BoxAbsolute, X: 10, Y: 20, Width: 30, Height: 40}}
	anch := &uiir.Node{Kind: uiir.NodeAnchorBox, Box: &uiir.BoxSpec{Kind: uiir.BoxAnchored, Left: 0.1, Top: 0.2, Right: 0.6, Bottom: 0.8}}
	out, err := Lower(&uiir.Node{Kind: uiir.NodeAbsoluteBox, Box: &uiir.BoxSpec{Kind: uiir.BoxAbsolute, X: 0, Y: 0, Width: 200, Height: 100}, Children: []*uiir.Node{abs, anch}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Rows[1].Frame.Kind != layout.AbsoluteFrame || out.Rows[2].Frame.Kind != layout.AnchorFrame {
		t.Fatal("placed frame kinds mismatch")
	}
	if _, err := layout.ResolveRows(out.Rows, layout.Rect{Width: 200, Height: 100}); err != nil {
		t.Fatal(err)
	}
}

func TestDeterministicIDsAndPreservedIDs(t *testing.T) {
	a := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{{Kind: uiir.NodeText, Text: "x"}}}
	b := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{{Kind: uiir.NodeText, Text: "x"}}}
	a1, _ := Lower(a)
	b1, _ := Lower(b)
	if a1.Rows[1].ID != b1.Rows[1].ID {
		t.Fatal("ids should be deterministic")
	}
	preserve, _ := Lower(&uiir.Node{NodeID: "root", Kind: uiir.NodeColumn, Children: []*uiir.Node{{NodeID: "child", Kind: uiir.NodeText, Text: "x"}}})
	if preserve.Rows[0].ID != "0" || preserve.Rows[1].ID != "0.0" {
		t.Fatal("lowering normalizes to canonical IDs")
	}
}

func TestUnsupportedAndNilRootErrors(t *testing.T) {
	if _, err := Lower(nil); err == nil {
		t.Fatal("expected nil root error")
	}
	if _, err := Lower(&uiir.Node{Kind: uiir.NodeKind("unknown")}); err == nil {
		t.Fatal("expected unsupported kind")
	}
}

func TestLoweringResultResolves(t *testing.T) {
	root := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{{Kind: uiir.NodeButton, Label: "ok", Enabled: true, Event: "go"}, {Kind: uiir.NodeSpacer}}}
	out, err := Lower(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := layout.ResolveRows(out.Rows, layout.Rect{X: 0, Y: 0, Width: 300, Height: 120}); err != nil {
		t.Fatal(err)
	}
}

func spanBox(child *uiir.Node, region string, layer int64) *uiir.Node {
	return &uiir.Node{Kind: uiir.NodeAnchorBox, Box: &uiir.BoxSpec{Kind: uiir.BoxSpan, Region: region, Layer: layer,
		XSpan: uiir.SpanSpec{Kind: uiir.SpanStart, A: 10, B: 100},
		YSpan: uiir.SpanSpec{Kind: uiir.SpanStart, A: 20, B: 30}}, Children: []*uiir.Node{child}}
}

func TestCanvasPlacesBoxesAndBoxContentFillsTheBox(t *testing.T) {
	canvas := &uiir.Node{Kind: uiir.NodeAbsoluteBox, Children: []*uiir.Node{spanBox(&uiir.Node{Kind: uiir.NodeText, Text: "hi"}, "Shell.Header", 1)}}
	out, err := Lower(canvas)
	if err != nil {
		t.Fatal(err)
	}
	box, content := out.Rows[1], out.Rows[2]
	if box.Frame.Kind != layout.SpanFrame || box.Layer != 1 || box.DebugLabel != "Shell.Header" {
		t.Fatalf("box row = %+v", box)
	}
	doc, err := layout.ResolveRows(out.Rows, layout.Rect{Width: 640, Height: 480})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := doc.Nodes[content.ID].Rect, (layout.Rect{X: 10, Y: 20, Width: 100, Height: 30}); got != want {
		t.Fatalf("content = %+v, want the box %+v", got, want)
	}
	if out.Kinds[box.ID] != "Region" || out.Kinds[out.Rows[0].ID] != "Canvas" {
		t.Fatalf("kinds = %v", out.Kinds)
	}
}

func TestCanvasRejectsUnplacedChildren(t *testing.T) {
	canvas := &uiir.Node{Kind: uiir.NodeAbsoluteBox, Children: []*uiir.Node{{Kind: uiir.NodeText, Text: "loose"}}}
	_, err := Lower(canvas)
	if err == nil || !strings.Contains(err.Error(), "is a child of a canvas but is not placed") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlacedBoxesBelongInACanvas(t *testing.T) {
	col := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{spanBox(&uiir.Node{Kind: uiir.NodeText, Text: "x"}, "", 0)}}
	_, err := Lower(col)
	if err == nil || !strings.Contains(err.Error(), "must be a child of UI.Canvas") {
		t.Fatalf("err = %v", err)
	}
}
