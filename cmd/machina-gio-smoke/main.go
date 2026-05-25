//go:build machina_gio

package main

import (
	"fmt"

	backendgio "github.com/yuechen-li-dev/oct/internal/machina/backend/gio"
	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/session"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

type counterState struct{ Count int }

type counterApp struct{}

func (counterApp) Project(state counterState) (*uiir.Node, error) {
	return &uiir.Node{Kind: uiir.NodeColumn, Children: []*uiir.Node{
		{Kind: uiir.NodeText, Text: fmt.Sprintf("count=%d", state.Count)},
		{Kind: uiir.NodeButton, Label: "Increment", Event: "counter.increment", Enabled: true},
	}}, nil
}

func (counterApp) Dispatch(state counterState, action lowering.Action) (counterState, error) {
	if action.Name == "counter.increment" {
		state.Count++
	}
	return state, nil
}

func main() {
	s, err := session.NewSession[counterState](counterApp{}, counterState{}, layout.Rect{Width: 640, Height: 480})
	if err != nil {
		panic(err)
	}
	if err := backendgio.RunWindow("Machina Gio Smoke", s); err != nil {
		panic(err)
	}
}
