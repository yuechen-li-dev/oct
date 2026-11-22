package gio

import (
	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/render"
	"github.com/yuechen-li-dev/oct/internal/machina/session"
	"github.com/yuechen-li-dev/oct/internal/machina/textmetrics"
)

// SessionAdapter is the backend-neutral session surface a window drives.
// session.Session satisfies it.
type SessionAdapter interface {
	Commands() ([]render.Command, error)
	PointerDown(event session.PointerEvent) (*session.DispatchResult, error)
	PointerMove(event session.PointerEvent) (bool, error)
	PointerUp(event session.PointerEvent) (bool, error)
	PointerLeave() (bool, error)
	KeyPress(key session.Key) (bool, error)
	TextInput(text string) (bool, error)
	SetViewport(viewport layout.Rect) error
	SetTextMetrics(m textmetrics.Metrics) error
}
