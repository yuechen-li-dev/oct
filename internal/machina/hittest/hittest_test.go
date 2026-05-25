package hittest

import (
	"math"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func TestNoActionsNoHit(t *testing.T) {
	resolved := mustResolve(t, []layout.LayoutRow{{ID: "root", Frame: layout.FrameSpec{Kind: layout.RootFrame}}}, layout.Rect{X: 0, Y: 0, Width: 100, Height: 100})
	idx, err := BuildIndex(resolved, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err := idx.HitTest(Point{X: 1, Y: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no hit")
	}
}

func TestSingleActionHitAndOutside(t *testing.T) {
	rows := []layout.LayoutRow{{ID: "root", Frame: layout.FrameSpec{Kind: layout.RootFrame}}, {ID: "btn", Parent: ptr("root"), Frame: layout.FrameSpec{Kind: layout.AbsoluteFrame, X: 10, Y: 10, Width: 40, Height: 20}}}
	resolved := mustResolve(t, rows, layout.Rect{X: 0, Y: 0, Width: 100, Height: 100})
	idx, err := BuildIndex(resolved, map[layout.NodeID]lowering.Action{"btn": {Name: "go"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, ok, err := idx.HitTest(Point{X: 12, Y: 12}, nil)
	if err != nil || !ok || r.Action.Name != "go" {
		t.Fatalf("bad hit: %#v ok=%v err=%v", r, ok, err)
	}
	_, ok, err = idx.HitTest(Point{X: 99, Y: 99}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected outside miss")
	}
}

func TestHalfOpenBounds(t *testing.T) {
	rows := []layout.LayoutRow{{ID: "root", Frame: layout.FrameSpec{Kind: layout.RootFrame}}, {ID: "btn", Parent: ptr("root"), Frame: layout.FrameSpec{Kind: layout.AbsoluteFrame, X: 10, Y: 10, Width: 5, Height: 5}}}
	resolved := mustResolve(t, rows, layout.Rect{X: 0, Y: 0, Width: 100, Height: 100})
	idx, _ := BuildIndex(resolved, map[layout.NodeID]lowering.Action{"btn": {Name: "go"}}, nil)
	if _, ok, _ := idx.HitTest(Point{X: 10, Y: 10}, nil); !ok {
		t.Fatal("left/top should hit")
	}
	if _, ok, _ := idx.HitTest(Point{X: 15, Y: 10}, nil); ok {
		t.Fatal("right edge excluded")
	}
	if _, ok, _ := idx.HitTest(Point{X: 10, Y: 15}, nil); ok {
		t.Fatal("bottom edge excluded")
	}
}

func TestZeroSizeExcluded(t *testing.T) {
	rows := []layout.LayoutRow{{ID: "root", Frame: layout.FrameSpec{Kind: layout.RootFrame}}, {ID: "z", Parent: ptr("root"), Frame: layout.FrameSpec{Kind: layout.AbsoluteFrame, X: 0, Y: 0, Width: 0, Height: 10}}}
	resolved := mustResolve(t, rows, layout.Rect{Width: 100, Height: 100})
	idx, _ := BuildIndex(resolved, map[layout.NodeID]lowering.Action{"z": {Name: "z"}}, nil)
	if _, ok, _ := idx.HitTest(Point{X: 0, Y: 0}, nil); ok {
		t.Fatal("zero width should exclude candidate")
	}
}

func TestOverlapAndNestedPriority(t *testing.T) {
	rows := []layout.LayoutRow{{ID: "root", Frame: layout.FrameSpec{Kind: layout.RootFrame}},
		{ID: "parent", Parent: ptr("root"), Order: 0, Frame: layout.FrameSpec{Kind: layout.AbsoluteFrame, X: 0, Y: 0, Width: 40, Height: 40}},
		{ID: "child", Parent: ptr("parent"), Order: 0, Frame: layout.FrameSpec{Kind: layout.AbsoluteFrame, X: 10, Y: 10, Width: 20, Height: 20}},
		{ID: "sib", Parent: ptr("root"), Order: 1, Frame: layout.FrameSpec{Kind: layout.AbsoluteFrame, X: 0, Y: 0, Width: 40, Height: 40}},
	}
	resolved := mustResolve(t, rows, layout.Rect{Width: 100, Height: 100})
	actions := map[layout.NodeID]lowering.Action{"parent": {Name: "parent"}, "child": {Name: "child"}, "sib": {Name: "sib"}}
	idx, _ := BuildIndex(resolved, actions, nil)
	if r, ok, _ := idx.HitTest(Point{X: 15, Y: 15}, nil); !ok || r.Action.Name != "sib" {
		t.Fatalf("later pre-order candidate should win overlap: %#v", r)
	}
}

func TestErrorsAndSemanticsAndIntegration(t *testing.T) {
	if _, err := BuildIndex(nil, nil, nil); err == nil {
		t.Fatal("expected nil resolved error")
	}
	resolved := mustResolve(t, []layout.LayoutRow{{ID: "root", Frame: layout.FrameSpec{Kind: layout.RootFrame}}}, layout.Rect{Width: 10, Height: 10})
	if _, err := BuildIndex(resolved, map[layout.NodeID]lowering.Action{"missing": {Name: "x"}}, nil); err == nil {
		t.Fatal("expected missing action node error")
	}
	idx, _ := BuildIndex(resolved, nil, nil)
	if _, _, err := idx.HitTest(Point{X: math.NaN(), Y: 0}, nil); err == nil {
		t.Fatal("expected invalid point error")
	}

	root := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{{Kind: uiir.NodeButton, Label: "One", Enabled: true, Event: "one"}, {Kind: uiir.NodeButton, Label: "Two", Enabled: true, Event: "two"}, {Kind: uiir.NodeButton, Label: "Off", Enabled: false, Event: "off"}}}
	low, err := lowering.Lower(root)
	if err != nil {
		t.Fatal(err)
	}
	resolved2, err := layout.ResolveRows(low.Rows, layout.Rect{X: 0, Y: 0, Width: 200, Height: 120})
	if err != nil {
		t.Fatal(err)
	}
	idx2, err := BuildIndex(resolved2, low.Actions, low.Semantics)
	if err != nil {
		t.Fatal(err)
	}
	p1 := center(resolved2.Nodes["0.0"].Rect)
	p2 := center(resolved2.Nodes["0.1"].Rect)
	p3 := center(resolved2.Nodes["0.2"].Rect)
	r1, ok, err := idx2.HitTest(p1, low.Semantics)
	if err != nil || !ok || r1.Action.Name != "one" {
		t.Fatalf("button one miss: %#v ok=%v err=%v", r1, ok, err)
	}
	if r1.Semantics == nil || r1.Semantics.Role != lowering.RoleButton {
		t.Fatal("missing semantics on hit")
	}
	r2, ok, err := idx2.HitTest(p2, nil)
	if err != nil || !ok || r2.Action.Name != "two" || r2.Semantics != nil {
		t.Fatalf("button two mismatch: %#v", r2)
	}
	if _, ok, err := idx2.HitTest(p3, low.Semantics); err != nil || ok {
		t.Fatalf("disabled button should not be actionable: ok=%v err=%v", ok, err)
	}
}

func ptr(s layout.NodeID) *layout.NodeID { return &s }
func mustResolve(t *testing.T, rows []layout.LayoutRow, root layout.Rect) *layout.ResolvedLayoutDocument {
	t.Helper()
	r, err := layout.ResolveRows(rows, root)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func center(r layout.Rect) Point { return Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2} }
