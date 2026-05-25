package interpret

import (
	"testing"

	machinalayout "github.com/yuechen-li-dev/oct/internal/machina/layout"
	machinauiir "github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func TestMachinaUIIRBridgeParity(t *testing.T) {
	cases := []struct {
		name string
		root *uiirNode
	}{
		{"basic", &uiirNode{Kind: uiirNodeColumn, Children: []*uiirNode{{Kind: uiirNodeText, Text: "hello"}, {Kind: uiirNodeButton, Label: "Go", Event: "go.event", Enabled: true}}}},
		{"absolute", &uiirNode{Kind: uiirNodeColumn, Children: []*uiirNode{{Kind: uiirNodeAbsoluteBox, Box: &uiirBoxSpec{Kind: uiirBoxAbsolute, X: 10, Y: 20, Width: 300, Height: 40, ZOrder: 1}, Children: []*uiirNode{{Kind: uiirNodeText, Text: "abs"}}}}}},
		{"anchored", &uiirNode{Kind: uiirNodeColumn, Children: []*uiirNode{{Kind: uiirNodeAnchorBox, Box: &uiirBoxSpec{Kind: uiirBoxAnchored, Left: 0.1, Top: 0.2, Right: 0.9, Bottom: 0.4, ZOrder: 0}, Children: []*uiirNode{{Kind: uiirNodeButton, Label: "anchored", Event: "anchor.click", Enabled: true}}}}}},
		{"nested", &uiirNode{Kind: uiirNodeColumn, Children: []*uiirNode{{Kind: uiirNodeRow, Children: []*uiirNode{{Kind: uiirNodeSpacer}, {Kind: uiirNodeGrid, Children: []*uiirNode{{Kind: uiirNodeText, Text: "g0"}, {Kind: uiirNodeText, Text: "g1"}}}}}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			machina, err := toMachinaUIIRNode(tc.root)
			if err != nil {
				t.Fatalf("toMachinaUIIRNode: %v", err)
			}

			oldJSON, _ := serializeUIIRCanonicalJSON(tc.root)
			newJSON, _ := machinauiir.SerializeCanonicalJSON(machina)
			newDecoded, err := machinauiir.DeserializeCanonicalJSON(newJSON)
			if err != nil {
				t.Fatalf("deserialize new json: %v", err)
			}
			newAsLocal, err := fromMachinaUIIRNode(newDecoded)
			if err != nil {
				t.Fatalf("convert new json to local: %v", err)
			}
			if uiirSignature(withUIIRNodeIDs(tc.root)) != uiirSignature(withUIIRNodeIDs(newAsLocal)) {
				t.Fatalf("json semantic mismatch old=%s new=%s", oldJSON, newJSON)
			}

			if uiirSignature(withUIIRNodeIDs(tc.root)) != machinauiir.Signature(machinauiir.WithNodeIDs(machina)) {
				t.Fatalf("signature mismatch")
			}

			oldResolved, err := resolveUIIRBoxes(withUIIRNodeIDs(tc.root), &uiirResolvedBox{X: 0, Y: 0, Width: uiirRootLayoutExtent, Height: uiirRootLayoutExtent})
			if err != nil {
				t.Fatalf("old resolve: %v", err)
			}
			newResolved, err := machinalayout.ResolveBoxes(machinauiir.WithNodeIDs(machina), &machinauiir.ResolvedRect{X: 0, Y: 0, Width: machinalayout.RootLayoutExtent, Height: machinalayout.RootLayoutExtent})
			if err != nil {
				t.Fatalf("new resolve: %v", err)
			}
			newResolvedLocal, err := fromMachinaUIIRNode(newResolved)
			if err != nil {
				t.Fatalf("convert resolved to local: %v", err)
			}
			normalizeLocalLayoutForParity(oldResolved)
			normalizeLocalLayoutForParity(newResolvedLocal)
			if uiirSignature(oldResolved) != uiirSignature(newResolvedLocal) {
				oldResolvedJSON, _ := serializeUIIRCanonicalJSON(oldResolved)
				newResolvedJSON, _ := machinauiir.SerializeCanonicalJSON(newResolved)
				t.Fatalf("resolved semantic mismatch\nold=%s\nnew=%s", oldResolvedJSON, newResolvedJSON)
			}
		})
	}
}

func TestMachinaUIIRBridgeKindCoverage(t *testing.T) {
	kinds := []uiirNodeKind{uiirNodeText, uiirNodeButton, uiirNodeColumn, uiirNodeRow, uiirNodeGrid, uiirNodeSpacer, uiirNodeAbsoluteBox, uiirNodeAnchorBox}
	for _, k := range kinds {
		if _, err := toMachinaNodeKind(k); err != nil {
			t.Fatalf("kind %s missing mapping: %v", k, err)
		}
	}
}

func normalizeLocalLayoutForParity(node *uiirNode) {
	if node == nil {
		return
	}
	if node.Box == nil {
		node.Layout = nil
	}
	for _, c := range node.Children {
		normalizeLocalLayoutForParity(c)
	}
}
