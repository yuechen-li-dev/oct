package layout

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func TestResolveSpanShapes(t *testing.T) {
	cases := []struct {
		name      string
		span      uiir.SpanSpec
		pos, size float64
	}{
		{"start", uiir.SpanSpec{Kind: uiir.SpanStart, A: 16, B: 240}, 116, 240},
		{"end", uiir.SpanSpec{Kind: uiir.SpanEnd, A: 24, B: 160}, 100 + 1000 - 24 - 160, 160},
		{"stretch", uiir.SpanSpec{Kind: uiir.SpanStretch, A: 16, B: 16}, 116, 968},
		{"center", uiir.SpanSpec{Kind: uiir.SpanCenter, B: 480}, 100 + 260, 480},
		{"fraction", uiir.SpanSpec{Kind: uiir.SpanFraction, A: 0.25, B: 0.75}, 350, 500},
	}
	for _, c := range cases {
		pos, size, err := ResolveSpan(c.span, 100, 1000, "x")
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if pos != c.pos || size != c.size {
			t.Errorf("%s: got (%g, %g), want (%g, %g)", c.name, pos, size, c.pos, c.size)
		}
	}
}

func TestResolveSpanRejectsInsteadOfClamping(t *testing.T) {
	cases := []struct {
		span uiir.SpanSpec
		want string
	}{
		{uiir.SpanSpec{Kind: uiir.SpanStretch, A: 600, B: 600}, "exceed the parent"},
		{uiir.SpanSpec{Kind: uiir.SpanStart, A: 0, B: -1}, "size must be non-negative"},
		{uiir.SpanSpec{Kind: uiir.SpanFraction, A: 0.8, B: 0.2}, "0 <= from <= to <= 1"},
		{uiir.SpanSpec{Kind: "sideways"}, "unsupported x span kind"},
	}
	for _, c := range cases {
		_, _, err := ResolveSpan(c.span, 0, 1000, "x")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: error %v, want %q", c.span, err, c.want)
		}
	}
}

func TestSpanFrameResolvesAgainstItsParent(t *testing.T) {
	root := NodeID("root")
	rows := []LayoutRow{
		{ID: root, Frame: FrameSpec{Kind: RootFrame}},
		{ID: "header", Parent: &root, Frame: FrameSpec{Kind: SpanFrame,
			XSpan: uiir.SpanSpec{Kind: uiir.SpanStretch, A: 16, B: 16},
			YSpan: uiir.SpanSpec{Kind: uiir.SpanStart, A: 16, B: 56}}},
	}
	doc, err := ResolveRows(rows, Rect{Width: 1024, Height: 768})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := doc.Nodes["header"].Rect, (Rect{X: 16, Y: 16, Width: 992, Height: 56}); got != want {
		t.Fatalf("header = %+v, want %+v", got, want)
	}
}

func TestPaintOrderIsLayerThenZThenAuthoredOrder(t *testing.T) {
	root := NodeID("root")
	rows := []LayoutRow{
		{ID: root, Frame: FrameSpec{Kind: RootFrame}},
		{ID: "chrome", Parent: &root, Order: 0, Layer: 1, Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 1}},
		{ID: "raised", Parent: &root, Order: 1, Z: 2, Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 1}},
		{ID: "first", Parent: &root, Order: 2, Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 1}},
		{ID: "second", Parent: &root, Order: 3, Frame: FrameSpec{Kind: AbsoluteFrame, Width: 1, Height: 1}},
	}
	doc, err := ResolveRows(rows, Rect{Width: 10, Height: 10})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, id := range doc.PaintOrder(root) {
		got = append(got, string(id))
	}
	if strings.Join(got, ",") != "first,second,raised,chrome" {
		t.Fatalf("paint order = %v", got)
	}
}
