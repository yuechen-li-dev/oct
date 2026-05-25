package layout

import (
	"strings"
	"testing"
)

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

func TestGridBasicFillGapPaddingSpanDeterminism(t *testing.T) {
	rows := []LayoutRow{
		{ID: "root", Frame: FrameSpec{Kind: RootFrame}, Arrange: &ArrangeSpec{Kind: GridArrange, Padding: EdgeInsets{Top: 10, Left: 10, Right: 10, Bottom: 10}, ColumnGap: 10, RowGap: 10, Columns: []GridTrack{{Kind: GridTrackFixed, Size: 50}, {Kind: GridTrackFill, Weight: 2}, {Kind: GridTrackFill, Weight: 1}}, Rows: []GridTrack{{Kind: GridTrackFixed, Size: 20}, {Kind: GridTrackFill, Weight: 1}}}},
		{ID: "a", Parent: ptr("root"), Order: 2, Frame: FrameSpec{Kind: CellFrame, Column: 0, Row: 0, ColumnSpan: 1, RowSpan: 1}},
		{ID: "b", Parent: ptr("root"), Order: 0, Frame: FrameSpec{Kind: CellFrame, Column: 1, Row: 0, ColumnSpan: 1, RowSpan: 1}},
		{ID: "c", Parent: ptr("root"), Order: 1, Frame: FrameSpec{Kind: CellFrame, Column: 2, Row: 0, ColumnSpan: 1, RowSpan: 2}},
		{ID: "d", Parent: ptr("root"), Order: 3, Frame: FrameSpec{Kind: CellFrame, Column: 0, Row: 1, ColumnSpan: 2, RowSpan: 1}},
	}
	out, err := ResolveRows(rows, Rect{Width: 310, Height: 130})
	if err != nil {
		t.Fatal(err)
	}
	if out.Children["root"][0] != "b" || out.Children["root"][1] != "c" || out.Children["root"][2] != "a" || out.Children["root"][3] != "d" {
		t.Fatalf("deterministic order mismatch: %#v", out.Children["root"])
	}
	if out.Nodes["a"].Rect != (Rect{X: 10, Y: 10, Width: 50, Height: 20}) {
		t.Fatalf("a rect %#v", out.Nodes["a"].Rect)
	}
	if out.Nodes["b"].Rect != (Rect{X: 70, Y: 10, Width: 146.66666666666666, Height: 20}) {
		t.Fatalf("b rect %#v", out.Nodes["b"].Rect)
	}
	if out.Nodes["c"].Rect != (Rect{X: 226.66666666666666, Y: 10, Width: 73.33333333333333, Height: 110}) {
		t.Fatalf("c rect %#v", out.Nodes["c"].Rect)
	}
	if out.Nodes["d"].Rect != (Rect{X: 10, Y: 40, Width: 206.66666666666666, Height: 80}) {
		t.Fatalf("d rect %#v", out.Nodes["d"].Rect)
	}
}

func TestGridValidationErrors(t *testing.T) {
	base := func(arr *ArrangeSpec, children ...LayoutRow) []LayoutRow {
		rows := []LayoutRow{{ID: "root", Frame: FrameSpec{Kind: RootFrame}, Arrange: arr}}
		return append(rows, children...)
	}
	_, err := CompileRows(base(&ArrangeSpec{Kind: GridArrange, Rows: []GridTrack{{Kind: GridTrackFixed, Size: 1}}}))
	if err == nil || !strings.Contains(err.Error(), "invalid_grid_columns") {
		t.Fatalf("expected invalid columns: %v", err)
	}
	_, err = CompileRows(base(&ArrangeSpec{Kind: GridArrange, Columns: []GridTrack{{Kind: GridTrackFixed, Size: 1}}}))
	if err == nil || !strings.Contains(err.Error(), "invalid_grid_rows") {
		t.Fatalf("expected invalid rows: %v", err)
	}
	_, err = CompileRows(base(&ArrangeSpec{Kind: GridArrange, Columns: []GridTrack{{Kind: GridTrackFixed, Size: -1}}, Rows: []GridTrack{{Kind: GridTrackFixed, Size: 1}}}))
	if err == nil || !strings.Contains(err.Error(), "invalid_grid_track_size") {
		t.Fatalf("expected invalid size: %v", err)
	}
	_, err = CompileRows(base(&ArrangeSpec{Kind: GridArrange, Columns: []GridTrack{{Kind: GridTrackFill, Weight: 0}}, Rows: []GridTrack{{Kind: GridTrackFixed, Size: 1}}}))
	if err == nil || !strings.Contains(err.Error(), "invalid_grid_track_weight") {
		t.Fatalf("expected invalid weight: %v", err)
	}
	_, err = ResolveRows(base(&ArrangeSpec{Kind: GridArrange, Columns: []GridTrack{{Kind: GridTrackFixed, Size: 100}}, Rows: []GridTrack{{Kind: GridTrackFixed, Size: 1}}, Padding: EdgeInsets{Left: 60, Right: 60}}, LayoutRow{ID: "x", Parent: ptr("root"), Frame: FrameSpec{Kind: CellFrame, Column: 0, Row: 0, ColumnSpan: 1, RowSpan: 1}}), Rect{Width: 100, Height: 10})
	if err == nil || !strings.Contains(err.Error(), "grid_negative_content_size") {
		t.Fatalf("expected negative content: %v", err)
	}
	_, err = ResolveRows(base(&ArrangeSpec{Kind: GridArrange, ColumnGap: 20, Columns: []GridTrack{{Kind: GridTrackFixed, Size: 60}, {Kind: GridTrackFixed, Size: 60}}, Rows: []GridTrack{{Kind: GridTrackFixed, Size: 1}}}, LayoutRow{ID: "x0", Parent: ptr("root"), Frame: FrameSpec{Kind: CellFrame, Column: 0, Row: 0, ColumnSpan: 1, RowSpan: 1}}), Rect{Width: 100, Height: 100})
	if err == nil || !strings.Contains(err.Error(), "grid_negative_remaining_space") {
		t.Fatalf("expected negative remaining: %v", err)
	}
	_, err = ResolveRows(base(&ArrangeSpec{Kind: GridArrange, Columns: []GridTrack{{Kind: GridTrackFixed, Size: 10}}, Rows: []GridTrack{{Kind: GridTrackFixed, Size: 10}}}, LayoutRow{ID: "x", Parent: ptr("root"), Frame: FrameSpec{Kind: FixedFrame, Width: 1, Height: 1}}), Rect{Width: 100, Height: 100})
	if err == nil || !strings.Contains(err.Error(), "invalid_grid_child_frame_kind") {
		t.Fatalf("expected child kind: %v", err)
	}
	_, err = CompileRows(base(&ArrangeSpec{Kind: GridArrange, Columns: []GridTrack{{Kind: GridTrackFixed, Size: 10}}, Rows: []GridTrack{{Kind: GridTrackFixed, Size: 10}}}, LayoutRow{ID: "x", Parent: ptr("root"), Frame: FrameSpec{Kind: CellFrame, Column: -1, Row: 0, ColumnSpan: 1, RowSpan: 1}}))
	if err == nil || !strings.Contains(err.Error(), "invalid_cell_frame") {
		t.Fatalf("expected invalid cell: %v", err)
	}
	_, err = ResolveRows(base(&ArrangeSpec{Kind: GridArrange, Columns: []GridTrack{{Kind: GridTrackFixed, Size: 10}}, Rows: []GridTrack{{Kind: GridTrackFixed, Size: 10}}}, LayoutRow{ID: "x", Parent: ptr("root"), Frame: FrameSpec{Kind: CellFrame, Column: 1, Row: 0, ColumnSpan: 1, RowSpan: 1}}), Rect{Width: 100, Height: 100})
	if err == nil || !strings.Contains(err.Error(), "grid_cell_out_of_range") {
		t.Fatalf("expected out of range: %v", err)
	}
}
