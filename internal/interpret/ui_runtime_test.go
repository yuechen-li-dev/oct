package interpret

import "testing"

func TestUISignatureIsDeterministic(t *testing.T) {
	tree := &uiNode{
		Kind: uiNodeColumn,
		Children: []*uiNode{
			{Kind: uiNodeText, Text: "count:0"},
			{Kind: uiNodeRow, Children: []*uiNode{{Kind: uiNodeButton, Label: "+", Event: "inc", Enabled: true}, {Kind: uiNodeButton, Label: "-", Event: "dec", Enabled: false}}},
		},
	}

	left := uiSignature(tree)
	right := uiSignature(cloneUI(tree))
	if left != right {
		t.Fatalf("expected deterministic signature, got %q vs %q", left, right)
	}
}

func TestUIEventDiscovery(t *testing.T) {
	tree := &uiNode{
		Kind: uiNodeColumn,
		Children: []*uiNode{
			{Kind: uiNodeText, Text: "hello"},
			{Kind: uiNodeButton, Label: "go", Event: "go", Enabled: true},
			{Kind: uiNodeButton, Label: "stop", Event: "stop", Enabled: false},
		},
	}

	if !uiTreeContainsEvent(tree, "go") {
		t.Fatal("expected event token to be discoverable")
	}
	if uiTreeContainsEvent(tree, "stop") {
		t.Fatal("disabled event token should not be discoverable")
	}
}

func TestResolveAbsoluteBoxPlacement(t *testing.T) {
	parent := &uiResolvedBox{X: 0, Y: 0, Width: 1000, Height: 1000}
	resolved, err := resolveBox(&uiBoxSpec{Kind: uiBoxAbsolute, X: 15, Y: 25, Width: 200, Height: 80}, parent)
	if err != nil {
		t.Fatalf("resolveBox returned unexpected error: %v", err)
	}
	if resolved.X != 15 || resolved.Y != 25 || resolved.Width != 200 || resolved.Height != 80 {
		t.Fatalf("unexpected absolute placement resolution: %+v", resolved)
	}
}

func TestResolveAnchoredBoxPlacement(t *testing.T) {
	parent := &uiResolvedBox{X: 10, Y: 20, Width: 400, Height: 200}
	resolved, err := resolveBox(&uiBoxSpec{Kind: uiBoxAnchored, Left: 0.25, Top: 0.10, Right: 0.75, Bottom: 0.60}, parent)
	if err != nil {
		t.Fatalf("resolveBox returned unexpected error: %v", err)
	}
	if resolved.X != 110 || resolved.Y != 40 || resolved.Width != 200 || resolved.Height != 100 {
		t.Fatalf("unexpected anchored placement resolution: %+v", resolved)
	}
}

func TestResolveMixedChildrenWithoutBreakingLegacyNodes(t *testing.T) {
	root := &uiNode{
		Kind: uiNodeCanvas,
		Children: []*uiNode{
			{
				Kind: uiNodePlaced,
				Box:  &uiBoxSpec{Kind: uiBoxAbsolute, X: 20, Y: 30, Width: 120, Height: 40},
				Children: []*uiNode{
					{Kind: uiNodeButton, Label: "go", Event: "go", Enabled: true},
				},
			},
			{Kind: uiNodeText, Text: "legacy"},
		},
	}
	resolved, err := resolveUIBoxes(root, &uiResolvedBox{X: 0, Y: 0, Width: 1000, Height: 1000})
	if err != nil {
		t.Fatalf("resolveUIBoxes returned unexpected error: %v", err)
	}
	placedChild := resolved.Children[0].Children[0]
	if placedChild.Layout == nil {
		t.Fatal("expected placed child to receive resolved layout")
	}
	if resolved.Children[1].Layout != nil {
		t.Fatal("expected non-placed legacy child to keep nil layout")
	}
}

func TestPatchedBoxChangeUpdatesResolvedLayout(t *testing.T) {
	initial := &uiNode{
		Kind: uiNodeCanvas,
		Children: []*uiNode{
			{Kind: uiNodePlaced, Box: &uiBoxSpec{Kind: uiBoxAbsolute, X: 10, Y: 10, Width: 50, Height: 20}, Children: []*uiNode{{Kind: uiNodeText, Text: "a"}}},
		},
	}
	next := &uiNode{
		Kind: uiNodeCanvas,
		Children: []*uiNode{
			{Kind: uiNodePlaced, Box: &uiBoxSpec{Kind: uiBoxAbsolute, X: 5, Y: 10, Width: 50, Height: 20}, Children: []*uiNode{{Kind: uiNodeText, Text: "a"}}},
		},
	}
	initialResolved, err := resolveUIBoxes(initial, &uiResolvedBox{X: 0, Y: 0, Width: 1000, Height: 1000})
	if err != nil {
		t.Fatalf("resolveUIBoxes initial returned unexpected error: %v", err)
	}
	nextResolved, err := resolveUIBoxes(next, &uiResolvedBox{X: 0, Y: 0, Width: 1000, Height: 1000})
	if err != nil {
		t.Fatalf("resolveUIBoxes next returned unexpected error: %v", err)
	}
	beforeX := initialResolved.Children[0].Children[0].Layout.X
	afterX := nextResolved.Children[0].Children[0].Layout.X
	if beforeX == afterX {
		t.Fatalf("expected box patch to update resolved layout x, both were %v", beforeX)
	}
}
