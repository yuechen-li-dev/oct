//go:build machina_gio

package cli

import (
	"github.com/yuechen-li-dev/oct/internal/interpret"
	backendgio "github.com/yuechen-li-dev/oct/internal/machina/backend/gio"
	"github.com/yuechen-li-dev/oct/internal/machina/session"
)

// Builds tagged machina_gio can open native windows for `oct ui run`.
func init() {
	uiWindowRunner = func(opts uiOptions, s *session.Session[interpret.Value]) error {
		return backendgio.Run(backendgio.WindowOptions{Title: opts.title, Width: opts.width, Height: opts.height}, s)
	}
}
