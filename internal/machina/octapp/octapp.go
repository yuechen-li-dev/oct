// Package octapp runs an Oct program as a Machina UI application.
//
// An Oct UI app is a package that declares the Elm-shaped trio already used by
// the Machina storefront fixtures:
//
//	fn Init() -> S
//	fn Update(model: S, event: String) -> S
//	fn View(model: S) -> UI
//
// App adapts that trio to session.App, so the existing deterministic spine
// (UIIR -> lowering -> layout -> hit test -> render) drives any backend. The
// program runs in the interpreter: compiled mode does not yet support the UI
// builtins (see `oct test` fallback notes), so this is the honest lane today.
package octapp

import (
	"fmt"
	"io"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/lowering"
	"github.com/yuechen-li-dev/oct/internal/machina/session"
	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

// App implements session.App[interpret.Value] over an Oct program.
type App struct {
	program project.Program
	pkg     string
	out     io.Writer
}

// convention lists the required functions and their parameter counts.
var convention = []struct {
	name   string
	params int
	sig    string
}{
	{"Init", 0, "fn Init() -> Model"},
	{"Update", 2, "fn Update(model: Model, event: String) -> Model"},
	{"View", 1, "fn View(model: Model) -> UI"},
}

// Load loads and type-checks the package containing path, checks the app
// convention, and returns the app with its initial state from Init().
// Program output (Print) goes to out.
func Load(path string, out io.Writer) (*App, interpret.Value, error) {
	if out == nil {
		out = io.Discard
	}
	program, err := project.Load(path)
	if err != nil {
		return nil, interpret.Value{}, err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return nil, interpret.Value{}, err
	}
	app := &App{program: program, pkg: program.Entry, out: out}
	if err := app.checkConvention(path); err != nil {
		return nil, interpret.Value{}, err
	}
	state, err := app.call("Init")
	if err != nil {
		return nil, interpret.Value{}, fmt.Errorf("Init: %w", err)
	}
	return app, state, nil
}

func (a *App) checkConvention(path string) error {
	pkg, ok := a.program.Packages[a.pkg]
	if !ok {
		return fmt.Errorf("%s: entry package %q not found", path, a.pkg)
	}
	var missing []string
	for _, want := range convention {
		found := false
		for _, fn := range pkg.Functions {
			if fn.Name == want.name {
				found = true
				if len(fn.Parameters) != want.params {
					return fmt.Errorf("%s: %s takes %d parameter(s); a Machina UI app declares %s",
						path, want.name, len(fn.Parameters), want.sig)
				}
			}
		}
		if !found {
			missing = append(missing, want.sig)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: not a Machina UI app; missing %s (an app declares Init, Update and View, and imports UI)",
			path, strings.Join(missing, ", "))
	}
	return nil
}

func (a *App) call(name string, args ...interpret.Value) (interpret.Value, error) {
	return interpret.CallFunctionWithArgsAndOptions(a.program, a.pkg, name, args, a.out, interpret.ExecuteOptions{})
}

// Project renders the model through View.
func (a *App) Project(state interpret.Value) (*uiir.Node, error) {
	v, err := a.call("View", state)
	if err != nil {
		return nil, fmt.Errorf("View: %w", err)
	}
	if v.UI == nil {
		return nil, fmt.Errorf("View returned %s, want UI", v.Kind)
	}
	return v.UI, nil
}

// Dispatch delivers a symbolic event to Update.
func (a *App) Dispatch(state interpret.Value, action lowering.Action) (interpret.Value, error) {
	next, err := a.call("Update", state, interpret.Value{Kind: interpret.ValueString, Text: action.Name})
	if err != nil {
		return state, fmt.Errorf("Update(%q): %w", action.Name, err)
	}
	return next, nil
}

// NewSession loads the app at path and builds a session for the viewport.
func NewSession(path string, viewport layout.Rect, out io.Writer) (*session.Session[interpret.Value], error) {
	app, state, err := Load(path, out)
	if err != nil {
		return nil, err
	}
	return session.NewSession[interpret.Value](app, state, viewport)
}
