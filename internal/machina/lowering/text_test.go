package lowering

import (
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func paragraph(s string) *uiir.Node {
	return &uiir.Node{Kind: uiir.NodeText, Text: s, TextSpec: &uiir.TextSpec{Size: 16, Weight: 400, Wrap: true, Overflow: uiir.OverflowEllipsis}}
}

// 26 runes = 208 dp on one line; at 120 dp (15 runes) it wraps to two
// 20 dp lines: "alpha bravo" and "charlie delta.".
const twoLines = "alpha bravo charlie delta."

func TestParagraphInAVerticalStackIsAsTallAsItsLines(t *testing.T) {
	col := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{paragraph(twoLines), text("after")}}
	_, doc := resolve(t, col, 120, 200)
	if got := doc.Nodes["0.0"].Rect; got != (layout.Rect{Width: 120, Height: 40}) {
		t.Fatalf("paragraph = %+v", got)
	}
	if got := doc.Nodes["0.1"].Rect.Y; got != 40 {
		t.Fatalf("the next child starts at %g, want 40", got)
	}
	_, wide := resolve(t, col, 400, 200)
	if got := wide.Nodes["0.1"].Rect.Y; got != 20 {
		t.Fatalf("wider stack: next child at %g, want 20", got)
	}
}

func TestNestedStacksMeasureParagraphsAtTheirWidth(t *testing.T) {
	inner := &uiir.Node{Kind: uiir.NodeColumn, Stack: &uiir.StackSpec{Padding: uiir.EdgeInsets{Left: 8, Right: 8}}, Children: []*uiir.Node{paragraph(twoLines)}}
	row := &uiir.Node{Kind: uiir.NodeRow, Children: []*uiir.Node{sized(text("ab"), uiir.SizingSpec{Kind: uiir.SizingFixed, Main: 16}), paragraph(twoLines)}}
	col := &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{inner, row, text("end")}}
	_, doc := resolve(t, col, 136, 400)
	if got := doc.Nodes["0.0"].Rect.Height; got != 40 {
		t.Fatalf("padded column = %g tall, want 40 (two lines at 120 dp)", got)
	}
	if got := doc.Nodes["0.1"].Rect.Height; got != 40 {
		t.Fatalf("row = %g tall, want 40 (paragraph fills 120 dp beside a 16 dp label)", got)
	}
	if got := doc.Nodes["0.1.1"].Rect.Width; got != 120 {
		t.Fatalf("paragraph in a row fills: width %g", got)
	}
	if got := doc.Nodes["0.2"].Rect.Y; got != 80 {
		t.Fatalf("end label at %g, want 80", got)
	}
}

func TestTextSpecSetsTheMeasuredLineSize(t *testing.T) {
	title := &uiir.Node{Kind: uiir.NodeText, Text: "Title", TextSpec: &uiir.TextSpec{Size: 24, Weight: 700}}
	low, err := Lower(&uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{title}})
	if err != nil {
		t.Fatal(err)
	}
	if f := low.Rows[1].Frame; f.Width != 60 || f.Height != 30 {
		t.Fatalf("24 dp title frame = %+v, want 60x30", f)
	}
	if sem := low.Semantics["0.0"]; sem.Text == nil || sem.Text.Size != 24 || sem.Text.Overflow != uiir.OverflowEllipsis {
		t.Fatalf("semantics carry the spec with defaults filled: %+v", sem.Text)
	}
}
