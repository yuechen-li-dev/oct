package gio

import (
	"github.com/yuechen-li-dev/oct/internal/machina/render"
	"github.com/yuechen-li-dev/oct/internal/machina/session"
)

type SessionAdapter interface {
	Commands() ([]render.Command, error)
	PointerDown(event session.PointerEvent) (*session.DispatchResult, error)
}
