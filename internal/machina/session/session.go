package session

import (
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/machina/dump"
	"github.com/yuechen-li-dev/oct/internal/machina/textmetrics"

	"github.com/yuechen-li-dev/oct/internal/machina/hittest"
	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/render"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

type Viewport struct {
	Rect layout.Rect
}

type PointerEvent struct {
	X float64
	Y float64
}

type App[TState any] interface {
	Project(state TState) (*uiir.Node, error)
	Dispatch(state TState, action lowering.Action) (TState, error)
}

type Session[TState any] struct {
	app      App[TState]
	state    TState
	viewport layout.Rect
	// metrics measures text for layout and render; nil is the fixed
	// headless metric.
	metrics textmetrics.Metrics

	root     *uiir.Node
	lowered  *lowering.Result
	resolved *layout.ResolvedLayoutDocument
	index    *hittest.Index
	commands []render.Command
	snapshot string

	// interaction is the transient state the session owns between frames
	// (hover, press, focus, caret), keyed by node id; the model owns every
	// value (DESIGN.md §5.9). dragging is a slider following the pointer.
	interaction render.Interaction
	dragging    layout.NodeID
}

type DispatchResult struct {
	Hit            bool
	Action         *lowering.Action
	SnapshotBefore string
	SnapshotAfter  string
}

func NewSession[TState any](app App[TState], initialState TState, viewport layout.Rect) (*Session[TState], error) {
	if app == nil {
		return nil, fmt.Errorf("session.err.nil_app: app is required")
	}
	if err := validateViewport(viewport); err != nil {
		return nil, err
	}
	s := &Session[TState]{app: app, state: initialState, viewport: viewport}
	if err := s.Rebuild(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session[TState]) Rebuild() error {
	if s == nil {
		return fmt.Errorf("session.err.nil_session: session is required")
	}
	root, err := s.app.Project(s.state)
	if err != nil {
		return fmt.Errorf("session.err.project: %w", err)
	}
	low, err := lowering.LowerWith(root, s.metrics)
	if err != nil {
		return fmt.Errorf("session.err.lower: %w", err)
	}
	resolved, err := layout.ResolveRows(low.Rows, s.viewport)
	if err != nil {
		return fmt.Errorf("session.err.resolve_layout: %w", err)
	}
	index, err := hittest.BuildIndex(resolved, low.Actions, low.Semantics)
	if err != nil {
		return fmt.Errorf("session.err.build_hittest: %w", err)
	}
	previous := [3]any{s.lowered, s.resolved, s.index}
	s.lowered, s.resolved, s.index = low, resolved, index
	s.settleInteraction()
	if err := s.repaint(); err != nil {
		s.lowered, s.resolved, s.index = previous[0].(*lowering.Result), previous[1].(*layout.ResolvedLayoutDocument), previous[2].(*hittest.Index)
		return err
	}
	s.root = root
	return nil
}

// repaint rebuilds the frame's commands from the current layout and
// interaction, without projecting the model again.
func (s *Session[TState]) repaint() error {
	commands, err := render.BuildCommands(s.resolved, s.lowered.Semantics, s.lowered.Styles, render.Options{Metrics: s.metrics, Interaction: s.interaction, States: s.lowered.States})
	if err != nil {
		return fmt.Errorf("session.err.build_commands: %w", err)
	}
	snapshot, err := render.RecordSnapshot(commands)
	if err != nil {
		return fmt.Errorf("session.err.record_snapshot: %w", err)
	}
	s.commands = commands
	s.snapshot = snapshot
	return nil
}

// SetTextMetrics switches the session to a backend's text metrics and
// rebuilds the frame. If the frame does not fit with them, the session
// keeps its previous metrics and frame and returns why.
func (s *Session[TState]) SetTextMetrics(m textmetrics.Metrics) error {
	if s == nil {
		return fmt.Errorf("session.err.nil_session: session is required")
	}
	previous := s.metrics
	s.metrics = m
	if err := s.Rebuild(); err != nil {
		s.metrics = previous
		return err
	}
	return nil
}

func (s *Session[TState]) SetViewport(viewport layout.Rect) error {
	if s == nil {
		return fmt.Errorf("session.err.nil_session: session is required")
	}
	if err := validateViewport(viewport); err != nil {
		return err
	}
	previous := s.viewport
	s.viewport = viewport
	if err := s.Rebuild(); err != nil {
		// Keep the last frame that fit; the caller reports why this one
		// did not.
		s.viewport = previous
		return err
	}
	return nil
}

// ActionRect finds the enabled control whose action alias is alias (for
// example `Msg.Inspect("harbor-board")`) in the current frame.
func (s *Session[TState]) ActionRect(alias string) (layout.Rect, bool) {
	if s == nil || s.lowered == nil || s.resolved == nil {
		return layout.Rect{}, false
	}
	for id, action := range s.lowered.Actions {
		if action.Name == alias {
			if node, ok := s.resolved.Nodes[id]; ok {
				return node.Rect, true
			}
		}
	}
	return layout.Rect{}, false
}

// Dump prints the current frame as a layout dump (internal/machina/dump).
func (s *Session[TState]) Dump() (string, error) {
	if s == nil {
		return "", fmt.Errorf("session.err.nil_session: session is required")
	}
	return dump.Format(s.resolved, s.lowered)
}

func (s *Session[TState]) Snapshot() (string, error) {
	if s == nil {
		return "", fmt.Errorf("session.err.nil_session: session is required")
	}
	return s.snapshot, nil
}

// PointerDown presses the control under the pointer and focuses it. A
// button sends its message and a checkbox proposes the opposite value; a
// text field places its caret; a slider proposes the value under the
// pointer and follows it until PointerUp. A press on nothing clears focus.
// Action is the action sent, or nil when the press sent nothing.
func (s *Session[TState]) PointerDown(event PointerEvent) (*DispatchResult, error) {
	if s == nil {
		return nil, fmt.Errorf("session.err.nil_session: session is required")
	}
	before := s.snapshot
	hit, ok, err := s.index.HitTest(hittest.Point{X: event.X, Y: event.Y}, s.lowered.Semantics)
	if err != nil {
		return nil, fmt.Errorf("session.err.hit_test: %w", err)
	}
	if !ok {
		s.interaction.Focus, s.interaction.Pressed = "", ""
		if err := s.repaint(); err != nil {
			return nil, err
		}
		return &DispatchResult{Hit: false, SnapshotBefore: before, SnapshotAfter: s.snapshot}, nil
	}
	id := hit.NodeID
	if s.interaction.Focus != id {
		s.interaction.Caret = 0
	}
	s.interaction.Focus, s.interaction.Pressed = id, id
	action := hit.Action
	sem := s.lowered.Semantics[id]
	var sent *lowering.Action
	switch sem.Role {
	case lowering.RoleCheckbox:
		action.Value = !sem.Input.Checked
		sent = &action
	case lowering.RoleTextField:
		s.interaction.Caret = s.caretAt(id, event.X)
	case lowering.RoleSlider:
		s.dragging = id
		if v := s.sliderValueAt(id, event.X); v != sem.Input.Number {
			action.Value = v
			sent = &action
		}
	default:
		sent = &action
	}
	if sent != nil {
		if err := s.send(*sent); err != nil {
			return nil, err
		}
	} else if err := s.repaint(); err != nil {
		return nil, err
	}
	return &DispatchResult{Hit: true, Action: sent, SnapshotBefore: before, SnapshotAfter: s.snapshot}, nil
}

// send delivers an action to the app and rebuilds the frame from the new
// model.
func (s *Session[TState]) send(action lowering.Action) error {
	nextState, err := s.app.Dispatch(s.state, action)
	if err != nil {
		return fmt.Errorf("session.err.dispatch: %w", err)
	}
	s.state = nextState
	return s.Rebuild()
}

func (s *Session[TState]) State() TState {
	return s.state
}

func (s *Session[TState]) Commands() ([]render.Command, error) {
	if s == nil {
		return nil, fmt.Errorf("session.err.nil_session: session is required")
	}
	out := make([]render.Command, len(s.commands))
	copy(out, s.commands)
	return out, nil
}

func validateViewport(viewport layout.Rect) error {
	if viewport.Width <= 0 || viewport.Height <= 0 {
		return fmt.Errorf("session.err.invalid_viewport: viewport width/height must be > 0")
	}
	return nil
}
