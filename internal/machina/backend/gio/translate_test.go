package gio

import (
	"testing"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/render"
)

func TestTranslateRoundTripsCommandKinds(t *testing.T) {
	ops, err := Translate([]render.Command{
		render.BeginFrameCommand{Root: layout.Rect{Width: 100, Height: 100}},
		render.PushClipCommand{Rect: layout.Rect{X: 5, Y: 5, Width: 20, Height: 20}},
		render.FillRectCommand{Rect: layout.Rect{X: 6, Y: 6, Width: 10, Height: 10}},
		render.DrawTextCommand{Rect: layout.Rect{X: 8, Y: 8, Width: 10, Height: 10}, Text: "hello"},
		render.PopClipCommand{},
		render.EndFrameCommand{},
	})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(ops) != 6 {
		t.Fatalf("unexpected op count: %d", len(ops))
	}
	if ops[3].Kind != DrawOpDrawText || ops[3].Text != "hello" {
		t.Fatalf("unexpected draw text op: %+v", ops[3])
	}
}

func TestTranslateRejectsUnbalancedPop(t *testing.T) {
	_, err := Translate([]render.Command{render.PopClipCommand{}})
	if err == nil {
		t.Fatalf("expected unbalanced clip error")
	}
}
