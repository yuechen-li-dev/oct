package interpret

import (
	"encoding/json"
	"reflect"
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

func TestUIIRCanonicalJSONDeterministicAndOrderSensitive(t *testing.T) {
	ordered := &uiirNode{
		Kind: uiirNodeColumn,
		Children: []*uiirNode{
			{Kind: uiirNodeText, Text: "first"},
			{Kind: uiirNodeButton, Label: "go", Event: "evt.go", Enabled: true},
		},
	}
	sameShape := cloneUIIR(ordered)
	reversed := &uiirNode{
		Kind: uiirNodeColumn,
		Children: []*uiirNode{
			{Kind: uiirNodeButton, Label: "go", Event: "evt.go", Enabled: true},
			{Kind: uiirNodeText, Text: "first"},
		},
	}

	left, err := serializeUIIRCanonicalJSON(ordered)
	if err != nil {
		t.Fatalf("serializeUIIRCanonicalJSON returned unexpected error: %v", err)
	}
	right, err := serializeUIIRCanonicalJSON(sameShape)
	if err != nil {
		t.Fatalf("serializeUIIRCanonicalJSON returned unexpected error: %v", err)
	}
	reversedJSON, err := serializeUIIRCanonicalJSON(reversed)
	if err != nil {
		t.Fatalf("serializeUIIRCanonicalJSON returned unexpected error: %v", err)
	}
	if left != right {
		t.Fatalf("expected deterministic canonical json, got %q vs %q", left, right)
	}
	if left == reversedJSON {
		t.Fatalf("expected child order to affect canonical json; left=%q", left)
	}
}

func TestUIIRCanonicalJSONStructuralKindsAndLayoutStayExplicit(t *testing.T) {
	root := &uiirNode{
		Kind: uiirNodeColumn,
		Children: []*uiirNode{
			{Kind: uiirNodeText, Text: "header"},
			{Kind: uiirNodeRow, Children: []*uiirNode{{Kind: uiirNodeSpacer}}},
			{Kind: uiirNodeGrid, Children: []*uiirNode{{Kind: uiirNodeText, Text: "g0"}}},
			{Kind: uiirNodeAbsoluteBox, Box: &uiirBoxSpec{Kind: uiirBoxAbsolute, X: 10, Y: 20, Width: 30, Height: 40}, Children: []*uiirNode{{Kind: uiirNodeButton, Label: "A", Event: "evt.abs", Enabled: true}}},
			{Kind: uiirNodeAnchorBox, Box: &uiirBoxSpec{Kind: uiirBoxAnchored, Left: 0.1, Top: 0.2, Right: 0.8, Bottom: 0.9}, Children: []*uiirNode{{Kind: uiirNodeButton, Label: "B", Event: "evt.anchor", Enabled: false}}},
		},
	}
	resolved, err := resolveUIIRBoxes(withUIIRNodeIDs(root), &uiirResolvedBox{X: 0, Y: 0, Width: uiirRootLayoutExtent, Height: uiirRootLayoutExtent})
	if err != nil {
		t.Fatalf("resolveUIIRBoxes returned unexpected error: %v", err)
	}
	encoded, err := serializeUIIRCanonicalJSON(resolved)
	if err != nil {
		t.Fatalf("serializeUIIRCanonicalJSON returned unexpected error: %v", err)
	}
	for _, expected := range []string{`"kind":"Text"`, `"kind":"Button"`, `"kind":"Row"`, `"kind":"Column"`, `"kind":"Grid"`, `"kind":"Spacer"`, `"kind":"AbsoluteBox"`, `"kind":"AnchorBox"`, `"box":{"kind":"absolute"`, `"box":{"kind":"anchored"`, `"event":{"token":"evt.abs","payload":null}`, `"enabled":false`, `"layout":{"x":10`} {
		if !strings.Contains(encoded, expected) {
			t.Fatalf("expected canonical json to include %q, got %q", expected, encoded)
		}
	}
}

func TestUIIRCanonicalJSONRoundTrip(t *testing.T) {
	root := &uiirNode{
		Kind: uiirNodeColumn,
		Children: []*uiirNode{
			{Kind: uiirNodeText, Text: "hello"},
			{Kind: uiirNodeButton, Label: "Run", Event: "evt.run", Enabled: true},
			{Kind: uiirNodeAbsoluteBox, Box: &uiirBoxSpec{Kind: uiirBoxAbsolute, X: 5, Y: 6, Width: 70, Height: 22}, Children: []*uiirNode{{Kind: uiirNodeSpacer}}},
		},
	}
	serialized, err := serializeUIIRCanonicalJSON(root)
	if err != nil {
		t.Fatalf("serializeUIIRCanonicalJSON returned unexpected error: %v", err)
	}
	decoded, err := deserializeUIIRCanonicalJSON(serialized)
	if err != nil {
		t.Fatalf("deserializeUIIRCanonicalJSON returned unexpected error: %v", err)
	}
	if !reflect.DeepEqual(withUIIRNodeIDs(root), decoded) {
		t.Fatalf("uiir json roundtrip mismatch\nwant: %#v\ngot: %#v", withUIIRNodeIDs(root), decoded)
	}
}

func TestUIIREventCanonicalJSONTokenOnlyAndPayload(t *testing.T) {
	tokenOnly, err := serializeUIIREventCanonicalJSON("counter.increment", nil)
	if err != nil {
		t.Fatalf("serializeUIIREventCanonicalJSON returned unexpected error: %v", err)
	}
	if tokenOnly != `{"token":"counter.increment","payload":null}` {
		t.Fatalf("unexpected token-only serialization: %q", tokenOnly)
	}
	decoded, err := deserializeUIIREventCanonicalJSON(tokenOnly)
	if err != nil {
		t.Fatalf("deserializeUIIREventCanonicalJSON returned unexpected error: %v", err)
	}
	if decoded.Token != "counter.increment" || string(decoded.Payload) != "null" {
		t.Fatalf("unexpected decoded token-only event: %+v", decoded)
	}

	withPayload, err := serializeUIIREventCanonicalJSON("counter.set", json.RawMessage(`{"next":2}`))
	if err != nil {
		t.Fatalf("serializeUIIREventCanonicalJSON returned unexpected error: %v", err)
	}
	payloadDecoded, err := deserializeUIIREventCanonicalJSON(withPayload)
	if err != nil {
		t.Fatalf("deserializeUIIREventCanonicalJSON returned unexpected error: %v", err)
	}
	if payloadDecoded.Token != "counter.set" || string(payloadDecoded.Payload) != `{"next":2}` {
		t.Fatalf("unexpected payload event roundtrip: %+v", payloadDecoded)
	}
}

func TestUIIRCanonicalJSONModeSwitchCompatibility(t *testing.T) {
	home := &uiirNode{
		Kind: uiirNodeColumn,
		Children: []*uiirNode{
			{Kind: uiirNodeText, Text: "Home"},
			{Kind: uiirNodeButton, Label: "Go Stats", Event: "counter.route.stats", Enabled: true},
		},
	}
	stats := &uiirNode{
		Kind: uiirNodeColumn,
		Children: []*uiirNode{
			{Kind: uiirNodeText, Text: "Stats"},
			{Kind: uiirNodeButton, Label: "Back", Event: "counter.route.home", Enabled: true},
		},
	}
	homeJSON, err := serializeUIIRCanonicalJSON(home)
	if err != nil {
		t.Fatalf("serializeUIIRCanonicalJSON returned unexpected error: %v", err)
	}
	statsJSON, err := serializeUIIRCanonicalJSON(stats)
	if err != nil {
		t.Fatalf("serializeUIIRCanonicalJSON returned unexpected error: %v", err)
	}
	homeJSONAgain, err := serializeUIIRCanonicalJSON(home)
	if err != nil {
		t.Fatalf("serializeUIIRCanonicalJSON returned unexpected error: %v", err)
	}
	if homeJSON != homeJSONAgain {
		t.Fatalf("mode projection must serialize deterministically: %q vs %q", homeJSON, homeJSONAgain)
	}
	if homeJSON == statsJSON {
		t.Fatalf("distinct routed modes must serialize differently: %q", homeJSON)
	}
	if !strings.Contains(homeJSON, `"counter.route.stats"`) || !strings.Contains(statsJSON, `"counter.route.home"`) {
		t.Fatalf("mode serialization must preserve routed event bindings")
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
