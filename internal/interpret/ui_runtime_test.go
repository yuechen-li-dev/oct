package interpret

import "testing"

func TestUISignatureIsDeterministic(t *testing.T) {
	tree := &uiNode{
		Kind: uiNodeColumn,
		Children: []*uiNode{
			{Kind: uiNodeText, Text: "count:0"},
			{Kind: uiNodeRow, Children: []*uiNode{{Kind: uiNodeButton, Label: "+", Event: "inc"}, {Kind: uiNodeButton, Label: "-", Event: "dec"}}},
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
			{Kind: uiNodeButton, Label: "go", Event: "go"},
		},
	}

	if !uiTreeContainsEvent(tree, "go") {
		t.Fatal("expected event token to be discoverable")
	}
	if uiTreeContainsEvent(tree, "stop") {
		t.Fatal("unexpected missing token discovered")
	}
}
