// Package octapp runs an Oct program as a Machina UI application.
//
// An Oct UI app is a package that declares the Elm-shaped trio already used by
// the Machina storefront fixtures:
//
//	fn Init() -> S
//	fn Update(model: S, msg: Msg) -> S     // Msg: the app's message enum
//	fn View(model: S) -> UI<Msg>         // the same Msg
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

	"github.com/yuechen-li-dev/oct/internal/ast"
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
	// msgEnum is the enum Update's second parameter takes.
	msgEnum string
}

// convention lists the required functions and their parameter counts.
var convention = []struct {
	name   string
	params int
	sig    string
}{
	{"Init", 0, "fn Init() -> Model"},
	{"Update", 2, "fn Update(model: Model, msg: Msg) -> Model"},
	{"View", 1, "fn View(model: Model) -> UI<Msg>"},
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
				if want.name == "Update" {
					t := fn.Parameters[1].Type
					if !declaresEnum(pkg.Enums, t.Name) || t.Package != "" || t.IsArray {
						return fmt.Errorf("%s: Update's second parameter must be the app's message enum, got %s", path, typeRefLabel(t))
					}
					a.msgEnum = t.Name
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
	for _, fn := range pkg.Functions {
		if fn.Name != "View" {
			continue
		}
		r := fn.ReturnType
		typed := r.Name == "UI" && r.Package == "" && !r.IsArray && len(r.TypeArguments) == 1
		if !typed || r.TypeArguments[0].Package != "" || r.TypeArguments[0].Name != a.msgEnum || len(r.TypeArguments[0].TypeArguments) > 0 || r.TypeArguments[0].IsArray {
			return fmt.Errorf("%s: View must return UI<%s>, the message type Update takes; got %s", path, a.msgEnum, typeRefLabel(r))
		}
	}
	return nil
}

// typeRefLabel spells a written type for diagnostics.
func typeRefLabel(r ast.TypeRef) string {
	name := r.Name
	if r.Package != "" {
		name = r.Package + "." + name
	}
	if len(r.TypeArguments) > 0 {
		args := make([]string, len(r.TypeArguments))
		for i, arg := range r.TypeArguments {
			args[i] = typeRefLabel(arg)
		}
		name += "<" + strings.Join(args, ", ") + ">"
	}
	return name + strings.Repeat("[]", r.ArrayDepth)
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

func declaresEnum(enums []ast.EnumDecl, name string) bool {
	for _, e := range enums {
		if e.Name == name {
			return true
		}
	}
	return false
}

// Dispatch delivers an action's message value to Update. The typechecker
// has already matched every button's message to View's UI<Msg>; a node
// built from the raw string-event builtin has none and is an error.
func (a *App) Dispatch(state interpret.Value, action lowering.Action) (interpret.Value, error) {
	if action.Callback != nil {
		built, err := a.buildMessage(action)
		if err != nil {
			return state, err
		}
		action.Message = built
	}
	msg, typed := action.Message.(interpret.Value)
	if !typed {
		return state, fmt.Errorf("button sends the event string %q, but Update takes %s; give the button a %s value", action.Name, a.msgEnum, a.msgEnum)
	}
	if localName(msg.Enum.TypeName) != a.msgEnum {
		return state, fmt.Errorf("button sends %s, but Update takes %s", action.Name, a.msgEnum)
	}
	next, err := a.call("Update", state, msg)
	if err != nil {
		return state, fmt.Errorf("Update(%s): %w", action.Name, err)
	}
	return next, nil
}

// buildMessage calls an input's callback with the value the session
// proposes, which makes the message Update receives.
func (a *App) buildMessage(action lowering.Action) (interpret.Value, error) {
	callback, ok := action.Callback.(interpret.Value)
	if !ok {
		return interpret.Value{}, fmt.Errorf("input %s has no callback value", action.Name)
	}
	var arg interpret.Value
	switch v := action.Value.(type) {
	case bool:
		arg = interpret.Value{Kind: interpret.ValueBool, Bool: v}
	case string:
		arg = interpret.Value{Kind: interpret.ValueString, Text: v}
	case float64:
		arg = interpret.Value{Kind: interpret.ValueFloat, Float: v}
	default:
		return interpret.Value{}, fmt.Errorf("input %s proposed a %T, which no input takes", action.Name, action.Value)
	}
	msg, err := interpret.CallFunctionValue(a.program, a.pkg, callback, []interpret.Value{arg}, a.out)
	if err != nil {
		return interpret.Value{}, fmt.Errorf("%s(%s): %w", action.Name, arg.String(), err)
	}
	return msg, nil
}

func localName(typeName string) string {
	return strings.TrimPrefix(typeName, strings.SplitN(typeName, ".", 2)[0]+".")
}

// NewSession loads the app at path and builds a session for the viewport.
func NewSession(path string, viewport layout.Rect, out io.Writer) (*session.Session[interpret.Value], error) {
	app, state, err := Load(path, out)
	if err != nil {
		return nil, err
	}
	return session.NewSession[interpret.Value](app, state, viewport)
}
