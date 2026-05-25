package layout

import "testing"

func ptr(id NodeID) *NodeID { return &id }

func TestResolveRootOnly(t *testing.T) {
	rows := []LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}}}
	out, err := ResolveRows(rows, Rect{X: 0, Y: 0, Width: 100, Height: 80})
	if err != nil {
		t.Fatal(err)
	}
	if out.Nodes["root"].Rect.Width != 100 || out.Nodes["root"].Rect.Height != 80 {
		t.Fatalf("unexpected root rect: %#v", out.Nodes["root"].Rect)
	}
}

func TestAbsoluteAndNestedAndAnchor(t *testing.T) {
	rows := []LayoutRow{
		{ID: "root", Frame: FrameSpec{Kind: RootFrame}},
		{ID: "abs", Parent: ptr("root"), Frame: FrameSpec{Kind: AbsoluteFrame, X: 10, Y: 15, Width: 40, Height: 20}},
		{ID: "nested", Parent: ptr("abs"), Frame: FrameSpec{Kind: AbsoluteFrame, X: 2, Y: 3, Width: 5, Height: 6}},
		{ID: "anchor", Parent: ptr("root"), Frame: FrameSpec{Kind: AnchorFrame, Left: 0.25, Top: 0.25, Right: 0.75, Bottom: 0.75}},
	}
	out, err := ResolveRows(rows, Rect{X: 0, Y: 0, Width: 200, Height: 100})
	if err != nil {
		t.Fatal(err)
	}
	abs := out.Nodes["abs"].Rect
	if abs.X != 10 || abs.Y != 15 {
		t.Fatalf("abs wrong: %#v", abs)
	}
	nested := out.Nodes["nested"].Rect
	if nested.X != 12 || nested.Y != 18 {
		t.Fatalf("nested wrong: %#v", nested)
	}
	anchor := out.Nodes["anchor"].Rect
	if anchor.X != 50 || anchor.Y != 25 || anchor.Width != 100 || anchor.Height != 50 {
		t.Fatalf("anchor wrong: %#v", anchor)
	}
}

func TestCompileValidationAndOrder(t *testing.T) {
	_, err := CompileRows([]LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}}, {ID: "root", Frame: FrameSpec{Kind: RootFrame}}})
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
	_, err = CompileRows([]LayoutRow{{ID: "child", Parent: ptr("root"), Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 1}}})
	if err == nil {
		t.Fatal("expected missing root")
	}
	_, err = CompileRows([]LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}}, {ID: "child", Parent: ptr("missing"), Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 1}}})
	if err == nil {
		t.Fatal("expected unknown parent")
	}

	doc, err := CompileRows([]LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}}, {ID: "a", Parent: ptr("root"), Order: 1, Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 1}}, {ID: "b", Parent: ptr("root"), Order: 0, Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Children["root"][0] != "b" {
		t.Fatalf("order mismatch: %#v", doc.Children["root"])
	}
}

func TestStackLayoutsAndNegativeSpace(t *testing.T) {
	stack := &ArrangeSpec{Kind: StackArrange, Axis: AxisVertical, Gap: 10, Padding: EdgeInsets{Top: 5, Left: 2, Right: 2, Bottom: 5}}
	rows := []LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}, Arrange: stack}, {ID: "a", Parent: ptr("root"), Frame: FrameSpec{Kind: FixedFrame, Height: 20}}, {ID: "b", Parent: ptr("root"), Frame: FrameSpec{Kind: FillFrame, Weight: 1}}}
	out, err := ResolveRows(rows, Rect{X: 0, Y: 0, Width: 100, Height: 100})
	if err != nil {
		t.Fatal(err)
	}
	if out.Nodes["a"].Rect.Y != 5 || out.Nodes["a"].Rect.Height != 20 {
		t.Fatalf("a wrong: %#v", out.Nodes["a"].Rect)
	}
	if out.Nodes["b"].Rect.Y != 35 || out.Nodes["b"].Rect.Height != 60 {
		t.Fatalf("b wrong: %#v", out.Nodes["b"].Rect)
	}

	hRows := []LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}, Arrange: &ArrangeSpec{Kind: StackArrange, Axis: AxisHorizontal}}, {ID: "x", Parent: ptr("root"), Frame: FrameSpec{Kind: FixedFrame, Width: 30}}, {ID: "y", Parent: ptr("root"), Frame: FrameSpec{Kind: FixedFrame, Width: 20}}}
	hOut, err := ResolveRows(hRows, Rect{X: 0, Y: 0, Width: 100, Height: 50})
	if err != nil {
		t.Fatal(err)
	}
	if hOut.Nodes["x"].Rect.Width != 30 || hOut.Nodes["y"].Rect.X != 30 {
		t.Fatalf("horizontal wrong: %#v %#v", hOut.Nodes["x"].Rect, hOut.Nodes["y"].Rect)
	}

	_, err = ResolveRows([]LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}, Arrange: &ArrangeSpec{Kind: StackArrange, Axis: AxisHorizontal}}, {ID: "x", Parent: ptr("root"), Frame: FrameSpec{Kind: FixedFrame, Width: 80}}, {ID: "y", Parent: ptr("root"), Frame: FrameSpec{Kind: FixedFrame, Width: 80}}}, Rect{Width: 100, Height: 10})
	if err == nil {
		t.Fatal("expected negative space error")
	}
}

func TestInvalidRootAndCycleAndConveniencePath(t *testing.T) {
	_, err := ResolveRows([]LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}}}, Rect{Width: -1, Height: 5})
	if err == nil {
		t.Fatal("expected invalid root")
	}
	_, err = CompileRows([]LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}}, {ID: "a", Parent: ptr("b"), Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 1}}, {ID: "b", Parent: ptr("a"), Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 1}}})
	if err == nil {
		t.Fatal("expected cycle/unreachable")
	}

	rows := []LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}}, {ID: "child", Parent: ptr("root"), Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 2}}}
	a, err := ResolveRows(rows, Rect{Width: 10, Height: 10})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := CompileRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ResolveDocument(doc, Rect{Width: 10, Height: 10})
	if err != nil {
		t.Fatal(err)
	}
	if a.Nodes["child"].Rect != b.Nodes["child"].Rect {
		t.Fatalf("mismatch %#v %#v", a.Nodes["child"].Rect, b.Nodes["child"].Rect)
	}
}
