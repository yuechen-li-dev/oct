package lowering

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func resolve(t *testing.T, root *uiir.Node, w, h float64) (*Result, *layout.ResolvedLayoutDocument) {
	t.Helper()
	low, err := Lower(root)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := layout.ResolveRows(low.Rows, layout.Rect{Width: w, Height: h})
	if err != nil {
		t.Fatal(err)
	}
	return low, doc
}

func text(s string) *uiir.Node { return &uiir.Node{Kind: uiir.NodeText, Text: s} }

func sized(n *uiir.Node, z uiir.SizingSpec) *uiir.Node { n.Sizing = &z; return n }

func TestStackSpecAndSizingResolve(t *testing.T) {
	col := &uiir.Node{Kind: uiir.NodeColumn, Stack: &uiir.StackSpec{Gap: 10, Padding: uiir.EdgeInsets{Top: 16, Right: 16, Bottom: 16, Left: 16}},
		Children: []*uiir.Node{
			sized(text("header"), uiir.SizingSpec{Kind: uiir.SizingFixed, Main: 56}),
			sized(&uiir.Node{Kind: uiir.NodeRow, Children: []*uiir.Node{text("a")}}, uiir.SizingSpec{Kind: uiir.SizingFill, Weight: 1}),
			sized(text("footer"), uiir.SizingSpec{Kind: uiir.SizingAuto}),
		}}
	_, doc := resolve(t, col, 400, 300)
	cases := map[layout.NodeID]layout.Rect{
		"0.0": {X: 16, Y: 16, Width: 368, Height: 56},  // fixed main, cross stretches
		"0.1": {X: 16, Y: 82, Width: 368, Height: 172}, // 268 inner - 56 - 20 - 2*10
		"0.2": {X: 16, Y: 264, Width: 48, Height: 20},  // measured: 6 runes x 8
	}
	for id, want := range cases {
		if got := doc.Nodes[id].Rect; got != want {
			t.Errorf("%s = %+v, want %+v", id, got, want)
		}
	}
}

func TestAlignAndJustifyPlaceLeftoverSpace(t *testing.T) {
	row := &uiir.Node{Kind: uiir.NodeRow, Stack: &uiir.StackSpec{Align: "center", Justify: "space-between"},
		Children: []*uiir.Node{text("ab"), text("cd")}}
	_, doc := resolve(t, row, 200, 100)
	if got := doc.Nodes["0.0"].Rect; got != (layout.Rect{X: 0, Y: 40, Width: 16, Height: 20}) {
		t.Errorf("first = %+v", got)
	}
	if got := doc.Nodes["0.1"].Rect; got != (layout.Rect{X: 184, Y: 40, Width: 16, Height: 20}) {
		t.Errorf("second = %+v", got)
	}
}

func TestNudgeMovesOnlyTheNudgedNode(t *testing.T) {
	a := text("a")
	a.Offset = &uiir.Offset{X: 3, Y: -2}
	col := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{a, text("b")}}
	_, doc := resolve(t, col, 100, 100)
	if got := doc.Nodes["0.0"].Rect; got.X != 3 || got.Y != -2 {
		t.Errorf("nudged = %+v", got)
	}
	if got := doc.Nodes["0.1"].Rect; got.Y != 20 {
		t.Errorf("sibling moved: %+v", got)
	}
}

func TestKeyedChildrenKeepTheirIDs(t *testing.T) {
	card := func(key string) *uiir.Node { n := text(key); n.Key = key; return n }
	all, _ := Lower(&uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{card("harbor-board"), card("cable.dock")}})
	filtered, _ := Lower(&uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{card("cable.dock")}})
	if all.Rows[2].ID != "0.cable%2Edock" || filtered.Rows[1].ID != all.Rows[2].ID {
		t.Fatalf("ids %q and %q; a keyed card must keep its id when earlier cards are filtered out", all.Rows[2].ID, filtered.Rows[1].ID)
	}
	_, err := Lower(&uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{card("x"), card("x")}})
	if err == nil || !strings.Contains(err.Error(), `use UI.Keyed key "x"`) {
		t.Fatalf("duplicate key error = %v", err)
	}
}

func TestAutoGridTracksMeasureTheirCells(t *testing.T) {
	cell := func(s string, c, r int64) *uiir.Node {
		n := text(s)
		n.Cell = &uiir.CellSpec{Column: c, Row: r, ColumnSpan: 1, RowSpan: 1}
		return n
	}
	grid := &uiir.Node{Kind: uiir.NodeGrid, Grid: &uiir.GridSpec{
		Columns: []uiir.GridTrack{{Kind: uiir.GridTrackAuto}, {Kind: uiir.GridTrackFill, Weight: 1}},
		Rows:    []uiir.GridTrack{{Kind: uiir.GridTrackAuto}},
	}, Children: []*uiir.Node{cell("label", 0, 0), cell("value", 1, 0)}}
	_, doc := resolve(t, grid, 300, 100)
	if got := doc.Nodes["0.0"].Rect; got != (layout.Rect{Width: 40, Height: 20}) {
		t.Errorf("auto cell = %+v", got)
	}
	if got := doc.Nodes["0.1"].Rect; got != (layout.Rect{X: 40, Width: 260, Height: 20}) {
		t.Errorf("fill cell = %+v", got)
	}
}

func TestSugarMisuseIsReported(t *testing.T) {
	cases := []struct {
		root *uiir.Node
		want string
	}{
		{&uiir.Node{Kind: uiir.NodeGrid, Grid: &uiir.GridSpec{Columns: []uiir.GridTrack{{Kind: uiir.GridTrackFill, Weight: 1}}}, Children: []*uiir.Node{sized(text("x"), uiir.SizingSpec{Kind: uiir.SizingFixed, Main: 4})}}, "applies only to a child of a stack"},
		{&uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{{Kind: uiir.NodeSkip}}}, "only marks a cell inside UI.Grid rows"},
		{&uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{{Kind: uiir.NodeText, Text: "x", Cell: &uiir.CellSpec{ColumnSpan: 2, RowSpan: 1}}}}, "belongs in UI.Grid rows"},
		{&uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{sized(&uiir.Node{Kind: uiir.NodeAbsoluteBox}, uiir.SizingSpec{Kind: uiir.SizingAuto})}}, "has no measured size"},
	}
	for _, c := range cases {
		_, err := Lower(c.root)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("error %v, want %q", err, c.want)
		}
	}
}

func TestStackOverflowNamesTheStack(t *testing.T) {
	row := &uiir.Node{Kind: uiir.NodeRow, Stack: &uiir.StackSpec{Gap: 8}, Children: []*uiir.Node{text("abcdefghij"), text("abcdefghij")}}
	low, err := Lower(row)
	if err != nil {
		t.Fatal(err)
	}
	_, err = layout.ResolveRows(low.Rows, layout.Rect{Width: 150, Height: 40})
	if err == nil || !strings.Contains(err.Error(), `stack "0" needs 168 along its horizontal axis (children 160 + gaps 8) but its inner size is 150`) {
		t.Fatalf("error = %v", err)
	}
}
