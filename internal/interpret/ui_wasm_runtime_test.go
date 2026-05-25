package interpret

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

type testCounterState struct {
	Route string
	Count int
}

type testCounterWasmApp struct{}
type wasmRenderedDocument struct {
	ABI  string `json:"abi"`
	Root struct {
		Kind string `json:"kind"`
	} `json:"root"`
}

func (testCounterWasmApp) InitialState() any {
	return testCounterState{Route: "route.home", Count: 0}
}

func (testCounterWasmApp) Dispatch(state any, event uiirSerializedEvent) (any, error) {
	current := state.(testCounterState)
	switch event.Token {
	case "counter.increment":
		current.Count++
	case "counter.decrement":
		if current.Count > 0 {
			current.Count--
		}
	case "route.stats":
		current.Route = "route.stats"
	case "route.home":
		current.Route = "route.home"
	}
	return current, nil
}

func (testCounterWasmApp) Project(state any) (*uiirNode, error) {
	current := state.(testCounterState)
	if current.Route == "route.stats" {
		return &uiirNode{Kind: uiirNodeColumn, Children: []*uiirNode{
			{Kind: uiirNodeText, Text: "route=stats"},
			{Kind: uiirNodeText, Text: "count=" + strconv.Itoa(current.Count)},
			{Kind: uiirNodeButton, Label: "Home", Event: "route.home", Enabled: true},
		}}, nil
	}
	canDec := current.Count > 0
	return &uiirNode{Kind: uiirNodeColumn, Children: []*uiirNode{
		{Kind: uiirNodeText, Text: "route=home"},
		{Kind: uiirNodeText, Text: "count=" + strconv.Itoa(current.Count)},
		{Kind: uiirNodeRow, Children: []*uiirNode{
			{Kind: uiirNodeButton, Label: "Increment", Event: "counter.increment", Enabled: true},
			{Kind: uiirNodeButton, Label: "Decrement", Event: "counter.decrement", Enabled: canDec},
			{Kind: uiirNodeButton, Label: "Stats", Event: "route.stats", Enabled: true},
		}},
	}}, nil
}

func TestUIWasmRuntimeInitAndRenderBoundary(t *testing.T) {
	rt := mustNewUIWasmRuntime(t)
	if status := rt.ExportMachinaUIInit(); status != uiWasmStatusOK {
		t.Fatalf("init status=%d error=%q", status, rt.LastError())
	}
	if status := rt.ExportMachinaUIRender(); status != uiWasmStatusOK {
		t.Fatalf("render status=%d error=%q", status, rt.LastError())
	}
	if rt.ExportMachinaUIBufferPtr() != 0 {
		t.Fatalf("buffer ptr must be deterministic scratch base")
	}
	var doc wasmRenderedDocument
	if err := json.Unmarshal(rt.HostReadOutput(), &doc); err != nil {
		t.Fatalf("rendered output is not valid canonical uiir json: %v", err)
	}
	if doc.ABI != uiirJSONABI {
		t.Fatalf("expected abi %q, got %q", uiirJSONABI, doc.ABI)
	}
	if doc.Root.Kind != string(uiirNodeColumn) {
		t.Fatalf("expected root kind Column, got %q", doc.Root.Kind)
	}
}

func TestUIWasmRuntimeDispatchEventBoundaryUpdatesRender(t *testing.T) {
	rt := mustNewUIWasmRuntime(t)
	mustStatusOK(t, rt.ExportMachinaUIInit(), rt)
	mustStatusOK(t, rt.ExportMachinaUIRender(), rt)
	initial := string(rt.HostReadOutput())

	eventJSON, err := serializeUIIREventCanonicalJSON("counter.increment", nil)
	if err != nil {
		t.Fatalf("serializeUIIREventCanonicalJSON failed: %v", err)
	}
	ptr, length, err := rt.HostWriteInput([]byte(eventJSON))
	if err != nil {
		t.Fatalf("HostWriteInput failed: %v", err)
	}
	mustStatusOK(t, rt.ExportMachinaUIDispatchEventJSON(ptr, length), rt)
	mustStatusOK(t, rt.ExportMachinaUIRender(), rt)
	updated := string(rt.HostReadOutput())

	if initial == updated {
		t.Fatalf("dispatch must alter rendered json for increment event")
	}
	if !strings.Contains(updated, `"count=1"`) {
		t.Fatalf("updated render must include incremented count, got %q", updated)
	}
}

func TestUIWasmRuntimeDeterminismForStateAndSequence(t *testing.T) {
	left := mustNewUIWasmRuntime(t)
	right := mustNewUIWasmRuntime(t)
	mustStatusOK(t, left.ExportMachinaUIInit(), left)
	mustStatusOK(t, right.ExportMachinaUIInit(), right)
	mustStatusOK(t, left.ExportMachinaUIRender(), left)
	mustStatusOK(t, right.ExportMachinaUIRender(), right)
	if string(left.HostReadOutput()) != string(right.HostReadOutput()) {
		t.Fatalf("same initial state must render identical canonical json")
	}
	for _, token := range []string{"counter.increment", "counter.increment", "route.stats"} {
		eventJSON, err := serializeUIIREventCanonicalJSON(token, nil)
		if err != nil {
			t.Fatalf("serializeUIIREventCanonicalJSON failed: %v", err)
		}
		leftPtr, leftLen, err := left.HostWriteInput([]byte(eventJSON))
		if err != nil {
			t.Fatalf("left HostWriteInput failed: %v", err)
		}
		rightPtr, rightLen, err := right.HostWriteInput([]byte(eventJSON))
		if err != nil {
			t.Fatalf("right HostWriteInput failed: %v", err)
		}
		mustStatusOK(t, left.ExportMachinaUIDispatchEventJSON(leftPtr, leftLen), left)
		mustStatusOK(t, right.ExportMachinaUIDispatchEventJSON(rightPtr, rightLen), right)
	}
	mustStatusOK(t, left.ExportMachinaUIRender(), left)
	mustStatusOK(t, right.ExportMachinaUIRender(), right)
	if string(left.HostReadOutput()) != string(right.HostReadOutput()) {
		t.Fatalf("same event sequence must produce identical final canonical json")
	}
}

func TestUIWasmRuntimeRejectsMalformedAndInvalidShapeEvents(t *testing.T) {
	rt := mustNewUIWasmRuntime(t)
	mustStatusOK(t, rt.ExportMachinaUIInit(), rt)

	badJSONPtr, badJSONLen, err := rt.HostWriteInput([]byte(`{"token":`))
	if err != nil {
		t.Fatalf("HostWriteInput malformed json failed: %v", err)
	}
	if status := rt.ExportMachinaUIDispatchEventJSON(badJSONPtr, badJSONLen); status != uiWasmStatusInvalidInput {
		t.Fatalf("expected malformed json status=%d got status=%d error=%q", uiWasmStatusInvalidInput, status, rt.LastError())
	}

	badShapePtr, badShapeLen, err := rt.HostWriteInput([]byte(`{"payload":null}`))
	if err != nil {
		t.Fatalf("HostWriteInput invalid shape failed: %v", err)
	}
	if status := rt.ExportMachinaUIDispatchEventJSON(badShapePtr, badShapeLen); status != uiWasmStatusInvalidInput {
		t.Fatalf("expected invalid shape status=%d got status=%d error=%q", uiWasmStatusInvalidInput, status, rt.LastError())
	}
}

func TestUIWasmRuntimeRoutedModeCompatibility(t *testing.T) {
	rt := mustNewUIWasmRuntime(t)
	mustStatusOK(t, rt.ExportMachinaUIInit(), rt)

	routeStatsJSON, err := serializeUIIREventCanonicalJSON("route.stats", nil)
	if err != nil {
		t.Fatalf("serializeUIIREventCanonicalJSON failed: %v", err)
	}
	ptr, length, err := rt.HostWriteInput([]byte(routeStatsJSON))
	if err != nil {
		t.Fatalf("HostWriteInput failed: %v", err)
	}
	mustStatusOK(t, rt.ExportMachinaUIDispatchEventJSON(ptr, length), rt)
	mustStatusOK(t, rt.ExportMachinaUIRender(), rt)
	routed := string(rt.HostReadOutput())
	if !strings.Contains(routed, `"route=stats"`) {
		t.Fatalf("routed render must project stats mode, got %q", routed)
	}
	if !strings.Contains(routed, `"route.home"`) {
		t.Fatalf("routed render must preserve event token bindings, got %q", routed)
	}
}

func mustNewUIWasmRuntime(t *testing.T) *uiWasmRuntime {
	t.Helper()
	rt, err := newUIWasmRuntime(testCounterWasmApp{}, 0)
	if err != nil {
		t.Fatalf("newUIWasmRuntime failed: %v", err)
	}
	return rt
}

func mustStatusOK(t *testing.T, status int32, rt *uiWasmRuntime) {
	t.Helper()
	if status != uiWasmStatusOK {
		t.Fatalf("unexpected status=%d error=%q", status, rt.LastError())
	}
}
