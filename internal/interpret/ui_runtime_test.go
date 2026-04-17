package interpret

import (
	"strings"
	"testing"
)

func TestUIIRSignatureIsDeterministicForSameState(t *testing.T) {
	tree := &uiirNode{
		Kind: uiirNodeColumn,
		Children: []*uiirNode{
			{Kind: uiirNodeText, Text: "count:0"},
			{Kind: uiirNodeRow, Children: []*uiirNode{{Kind: uiirNodeButton, Label: "+", Event: "inc", Enabled: true}, {Kind: uiirNodeButton, Label: "-", Event: "dec", Enabled: false}}},
		},
	}

	left := uiirSignature(withUIIRNodeIDs(tree))
	right := uiirSignature(withUIIRNodeIDs(cloneUIIR(tree)))
	if left != right {
		t.Fatalf("expected deterministic signature, got %q vs %q", left, right)
	}
}

func TestUIIRTreeContainsOnlyEnabledEvents(t *testing.T) {
	tree := &uiirNode{
		Kind: uiirNodeColumn,
		Children: []*uiirNode{
			{Kind: uiirNodeText, Text: "hello"},
			{Kind: uiirNodeButton, Label: "go", Event: "go", Enabled: true},
			{Kind: uiirNodeButton, Label: "stop", Event: "stop", Enabled: false},
		},
	}

	if !uiirTreeContainsEvent(tree, "go") {
		t.Fatal("expected event token to be discoverable")
	}
	if uiirTreeContainsEvent(tree, "stop") {
		t.Fatal("disabled event token should not be discoverable")
	}
}

func TestUIIRResolveAbsoluteBoxPlacement(t *testing.T) {
	parent := &uiirResolvedBox{X: 0, Y: 0, Width: 1000, Height: 1000}
	resolved, err := resolveUIIRBox(&uiirBoxSpec{Kind: uiirBoxAbsolute, X: 15, Y: 25, Width: 200, Height: 80}, parent)
	if err != nil {
		t.Fatalf("resolveUIIRBox returned unexpected error: %v", err)
	}
	if resolved.X != 15 || resolved.Y != 25 || resolved.Width != 200 || resolved.Height != 80 {
		t.Fatalf("unexpected absolute placement resolution: %+v", resolved)
	}
}

func TestUIIRResolveAnchoredBoxPlacement(t *testing.T) {
	parent := &uiirResolvedBox{X: 10, Y: 20, Width: 400, Height: 200}
	resolved, err := resolveUIIRBox(&uiirBoxSpec{Kind: uiirBoxAnchored, Left: 0.25, Top: 0.10, Right: 0.75, Bottom: 0.60}, parent)
	if err != nil {
		t.Fatalf("resolveUIIRBox returned unexpected error: %v", err)
	}
	if resolved.X != 110 || resolved.Y != 40 || resolved.Width != 200 || resolved.Height != 100 {
		t.Fatalf("unexpected anchored placement resolution: %+v", resolved)
	}
}

func TestUIIRStructuralKindsRemainExplicit(t *testing.T) {
	root := &uiirNode{
		Kind: uiirNodeColumn,
		Children: []*uiirNode{
			{Kind: uiirNodeText, Text: "hello"},
			{Kind: uiirNodeButton, Label: "go", Event: "evt.go", Enabled: true},
			{Kind: uiirNodeAbsoluteBox, Box: &uiirBoxSpec{Kind: uiirBoxAbsolute, X: 20, Y: 30, Width: 120, Height: 40}, Children: []*uiirNode{{Kind: uiirNodeSpacer}}},
			{Kind: uiirNodeAnchorBox, Box: &uiirBoxSpec{Kind: uiirBoxAnchored, Left: 0.1, Top: 0.2, Right: 0.9, Bottom: 0.8}, Children: []*uiirNode{{Kind: uiirNodeGrid, Children: []*uiirNode{{Kind: uiirNodeText, Text: "g0"}, {Kind: uiirNodeText, Text: "g1"}}}}},
		},
	}
	sig := uiirSignature(withUIIRNodeIDs(root))
	if !containsAll(sig, []string{"Text#", "Button#", "AbsoluteBox#", "AnchorBox#", "Grid#", "Spacer#"}) {
		t.Fatalf("expected explicit UIIR node kinds in signature, got %q", sig)
	}
}

func TestUIIRNodeIDOrderingTracksChildOrder(t *testing.T) {
	ordered := &uiirNode{Kind: uiirNodeColumn, Children: []*uiirNode{{Kind: uiirNodeText, Text: "first"}, {Kind: uiirNodeText, Text: "second"}}}
	reversed := &uiirNode{Kind: uiirNodeColumn, Children: []*uiirNode{{Kind: uiirNodeText, Text: "second"}, {Kind: uiirNodeText, Text: "first"}}}
	orderedSig := uiirSignature(withUIIRNodeIDs(ordered))
	reversedSig := uiirSignature(withUIIRNodeIDs(reversed))
	if orderedSig == reversedSig {
		t.Fatalf("child ordering must affect deterministic UIIR signatures: %q", orderedSig)
	}
}

func containsAll(haystack string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(haystack, part) {
			return false
		}
	}
	return true
}
