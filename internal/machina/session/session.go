package session

import (
	"fmt"

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

	root     *uiir.Node
	lowered  *lowering.Result
	resolved *layout.ResolvedLayoutDocument
	index    *hittest.Index
	commands []render.Command
	snapshot string
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
	low, err := lowering.Lower(root)
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
	commands, err := render.BuildCommands(resolved, low.Semantics, low.Styles, render.Options{})
	if err != nil {
		return fmt.Errorf("session.err.build_commands: %w", err)
	}
	snapshot, err := render.RecordSnapshot(commands)
	if err != nil {
		return fmt.Errorf("session.err.record_snapshot: %w", err)
	}
	s.root = root
	s.lowered = low
	s.resolved = resolved
	s.index = index
	s.commands = commands
	s.snapshot = snapshot
	return nil
}

func (s *Session[TState]) SetViewport(viewport layout.Rect) error {
	if s == nil {
		return fmt.Errorf("session.err.nil_session: session is required")
	}
	if err := validateViewport(viewport); err != nil {
		return err
	}
	s.viewport = viewport
	return s.Rebuild()
}

func (s *Session[TState]) Snapshot() (string, error) {
	if s == nil {
		return "", fmt.Errorf("session.err.nil_session: session is required")
	}
	return s.snapshot, nil
}

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
		return &DispatchResult{Hit: false, SnapshotBefore: before, SnapshotAfter: before}, nil
	}
	nextState, err := s.app.Dispatch(s.state, hit.Action)
	if err != nil {
		return nil, fmt.Errorf("session.err.dispatch: %w", err)
	}
	s.state = nextState
	if err := s.Rebuild(); err != nil {
		return nil, err
	}
	action := hit.Action
	return &DispatchResult{Hit: true, Action: &action, SnapshotBefore: before, SnapshotAfter: s.snapshot}, nil
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
