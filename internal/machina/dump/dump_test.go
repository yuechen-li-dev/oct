package dump

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func region(name string, layer int64, x, y uiir.SpanSpec, child *uiir.Node) *uiir.Node {
	return &uiir.Node{Kind: uiir.NodeAnchorBox, Box: &uiir.BoxSpec{Kind: uiir.BoxSpan, Region: name, Layer: layer, XSpan: x, YSpan: y}, Children: []*uiir.Node{child}}
}

func TestDumpWritesRegionsAndSugarInSpanVocabulary(t *testing.T) {
	root := &uiir.Node{Kind: uiir.NodeAbsoluteBox, Children: []*uiir.Node{
		region("Shell.Header", 0, uiir.SpanSpec{Kind: uiir.SpanStretch, A: 16, B: 16}, uiir.SpanSpec{Kind: uiir.SpanStart, A: 16, B: 56},
			&uiir.Node{Kind: uiir.NodeRow, Children: []*uiir.Node{{Kind: uiir.NodeButton, Label: "Shop", Event: "nav.shop", Enabled: true}}}),
		region("Shell.Cart", 1, uiir.SpanSpec{Kind: uiir.SpanEnd, A: 24, B: 160}, uiir.SpanSpec{Kind: uiir.SpanEnd, A: 24, B: 40},
			&uiir.Node{Kind: uiir.NodeText, Text: "Cart"}),
	}}
	low, err := lowering.Lower(root)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := layout.ResolveRows(low.Rows, layout.Rect{Width: 1024, Height: 768})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Format(resolved, low)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"0                        Canvas     (0,0) 1024x768",
		"  Shell.Header           Region     StretchX(16px, 16px)   Top(16px, 56px)        (16,16) 992x56",
		"    .0                   Stack(h)   StretchX(0px, 0px)     StretchY(0px, 0px)     (16,16) 992x56",
		`      .0.0               Button     Left(0px, 80px)        Top(0px, 32px)         (16,16) 80x32 "Shop" -> nav.shop`,
		"  Shell.Cart [Chrome]    Region     Right(24px, 160px)     Bottom(24px, 40px)     (840,704) 160x40",
		`    .0                   Text       StretchX(0px, 0px)     StretchY(0px, 0px)     (840,704) 160x40 "Cart"`,
		"",
	}, "\n")
	if got != want {
		t.Fatalf("dump mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}
