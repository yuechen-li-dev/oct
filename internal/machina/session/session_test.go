package session

import (
	"errors"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

type counterState struct {
	Count    int
	Enabled  bool
	FailProj bool
	FailDisp bool
}

type counterApp struct{}

func (counterApp) Project(state counterState) (*uiir.Node, error) {
	if state.FailProj {
		return nil, errors.New("project failed")
	}
	return &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{
		{Kind: uiir.NodeText, Text: "count=" + itoa(state.Count)},
		{Kind: uiir.NodeButton, Label: "increment", Event: "counter.increment", Enabled: state.Enabled},
	}}, nil
}

func (counterApp) Dispatch(state counterState, action lowering.Action) (counterState, error) {
	if state.FailDisp {
		return state, errors.New("dispatch failed")
	}
	if action.Name == "counter.increment" {
		state.Count++
	}
	return state, nil
}

func TestNewSessionBuildsSnapshotAndRendersInitialCount(t *testing.T) {
	s, err := NewSession[counterState](counterApp{}, counterState{Enabled: true}, layout.Rect{Width: 300, Height: 200})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !strings.Contains(snap, "DrawText") || !strings.Contains(snap, "count=0") {
		t.Fatalf("expected initial DrawText count=0 in snapshot, got %q", snap)
	}
}

func TestPointerDownHitDispatchesAndRebuilds(t *testing.T) {
	s, _ := NewSession[counterState](counterApp{}, counterState{Enabled: true}, layout.Rect{Width: 300, Height: 200})
	before, _ := s.Snapshot()
	res, err := s.PointerDown(PointerEvent{X: 10, Y: 30})
	if err != nil {
		t.Fatalf("pointer down: %v", err)
	}
	if !res.Hit || res.Action == nil || res.Action.Name != "counter.increment" {
		t.Fatalf("expected hit on counter.increment, got %+v", res)
	}
	after, _ := s.Snapshot()
	if before == after {
		t.Fatalf("expected snapshot to change after dispatch")
	}
	if !strings.Contains(after, "count=1") {
		t.Fatalf("expected count=1 in snapshot, got %q", after)
	}
}

func TestPointerDownMissDoesNotDispatch(t *testing.T) {
	s, _ := NewSession[counterState](counterApp{}, counterState{Enabled: true}, layout.Rect{Width: 300, Height: 200})
	before, _ := s.Snapshot()
	res, err := s.PointerDown(PointerEvent{X: 250, Y: 190})
	if err != nil {
		t.Fatalf("pointer down miss: %v", err)
	}
	if res.Hit {
		t.Fatalf("expected miss")
	}
	after, _ := s.Snapshot()
	if before != after {
		t.Fatalf("expected snapshot unchanged on miss")
	}
}

func TestDisabledButtonNotActionable(t *testing.T) {
	s, _ := NewSession[counterState](counterApp{}, counterState{Enabled: false}, layout.Rect{Width: 300, Height: 200})
	res, err := s.PointerDown(PointerEvent{X: 10, Y: 30})
	if err != nil {
		t.Fatalf("pointer down disabled: %v", err)
	}
	if res.Hit {
		t.Fatalf("expected disabled button to miss")
	}
}

func TestRebuildDeterministicWithoutStateChange(t *testing.T) {
	s, _ := NewSession[counterState](counterApp{}, counterState{Enabled: true}, layout.Rect{Width: 300, Height: 200})
	a, _ := s.Snapshot()
	if err := s.Rebuild(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	b, _ := s.Snapshot()
	if a != b {
		t.Fatalf("expected stable snapshot across rebuilds")
	}
}

func TestErrorsPropagateAndInvalidRejected(t *testing.T) {
	if _, err := NewSession[counterState](nil, counterState{}, layout.Rect{Width: 1, Height: 1}); err == nil {
		t.Fatalf("expected nil app error")
	}
	if _, err := NewSession[counterState](counterApp{}, counterState{}, layout.Rect{Width: 0, Height: 1}); err == nil {
		t.Fatalf("expected invalid viewport error")
	}
	if _, err := NewSession[counterState](counterApp{}, counterState{FailProj: true}, layout.Rect{Width: 1, Height: 1}); err == nil || !strings.Contains(err.Error(), "session.err.project") {
		t.Fatalf("expected project error, got %v", err)
	}
	s, _ := NewSession[counterState](counterApp{}, counterState{Enabled: true}, layout.Rect{Width: 300, Height: 200})
	s.state.FailDisp = true
	if _, err := s.PointerDown(PointerEvent{X: 10, Y: 30}); err == nil || !strings.Contains(err.Error(), "session.err.dispatch") {
		t.Fatalf("expected dispatch error, got %v", err)
	}
}

func TestSetViewportRebuilds(t *testing.T) {
	s, _ := NewSession[counterState](counterApp{}, counterState{Enabled: true}, layout.Rect{Width: 300, Height: 200})
	a, _ := s.Snapshot()
	if err := s.SetViewport(layout.Rect{Width: 120, Height: 80}); err != nil {
		t.Fatalf("set viewport: %v", err)
	}
	b, _ := s.Snapshot()
	if a == b {
		t.Fatalf("expected snapshot to change after viewport update")
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	buf := make([]byte, 0, 12)
	for v > 0 {
		buf = append([]byte{byte('0' + (v % 10))}, buf...)
		v /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

// wideMetrics measures every rune as 100 dp, so nothing fits a small window.
type wideMetrics struct{}

func (wideMetrics) Width(text string, spec uiir.TextSpec) float64 { return float64(len(text)) * 100 }
func (wideMetrics) LineHeight(spec uiir.TextSpec) float64         { return 400 }

func TestSetTextMetricsKeepsTheLastFrameWhenItCannotFit(t *testing.T) {
	s, err := NewSession[counterState](counterApp{}, counterState{Enabled: true}, layout.Rect{Width: 300, Height: 200})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := s.Snapshot()
	err = s.SetTextMetrics(wideMetrics{})
	if err == nil || !strings.Contains(err.Error(), "stack_overflow") {
		t.Fatalf("error = %v", err)
	}
	if after, _ := s.Snapshot(); after != before {
		t.Fatalf("frame changed after metrics that do not fit:\n%s", after)
	}
	// Later rebuilds still use the metrics that fit.
	if _, err := s.PointerDown(PointerEvent{X: 10, Y: 30}); err != nil {
		t.Fatalf("click after a failed metrics switch: %v", err)
	}
}
