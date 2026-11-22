# UI-M0 — Running Oct programs as native Machina UI apps

UI-M0 reaches **Success**: an Oct program opens in a native window from one
command, and clicks round-trip through Oct `Update`.

```bash
go build -tags machina_gio -o oct ./cmd/oct    # native backend build
./oct ui run Examples/MachinaThermostat/main.oct
./oct ui check Examples/MachinaThermostat/main.oct --click Raise   # headless, any build
```

## App convention

> Since UI-M3, `Update` takes the app's message enum (`fn Update(model:
> Model, msg: Msg) -> Model`) and buttons carry message values; since UI-M5
> `View` returns `UI<Msg>`. See `Libraries/UI/README.md`. The string form
> below still works.

An app package declares the trio the Machina storefront fixtures already used:

```oct
fn Init() -> Model
fn Update(model: Model, event: String) -> Model
fn View(model: Model) -> UI
```

`internal/machina/octapp` adapts it to `session.App`, so the existing
deterministic spine (UIIR → lowering → layout → hit test → render → session)
drives the window unchanged. Programs run in the interpreter: compiled mode
does not yet support the UI builtins (`compiled mode does not yet support
builtin UIButton`). That gap is reported, not hidden.

## What M0 found and fixed

1. **The Gio backend did not compile.** `window_gio.go` targeted the pre-0.5
   Gio API (`w.Events()`, `layout.NewContext`, `pointer.InputOp`) while
   `go.mod` pins Gio v0.9.0. The `machina_gio` tag kept it out of every
   default build, so nothing caught it. Ported to v0.9.
2. **The window loop spun a CPU core.** It called `w.Invalidate()` after every
   frame. It now redraws only after a click changes state.
3. **Resizes never reached the session**, and logical and physical pixels were
   mixed (wrong size on scaled displays). Layout now runs in dp; drawing
   multiplies by `PxPerDp`, pointer input divides by it, and frame size
   changes call `SetViewport`.
4. **Buttons were indistinguishable from text.** `render.emitNode` emitted the
   same `DrawText` for both roles and no `FillRect` at all. Render now emits
   button chrome (`FillRect` with `Fill: button | button-disabled`) and a
   centered, muted-when-disabled label. Snapshot lines gain ` fill=`,
   ` align=` and ` muted` only when set, so text-only snapshots are unchanged.
5. **Labels wrapped out of sight.** Layout sizes text at a fixed 8 dp per
   character; the real font is slightly wider, so `"Status: heating"` wrapped
   and its second line was clipped. Labels are now single-line and may
   overflow their rect slightly instead of disappearing.

Verified on Linux under Xvfb with synthetic clicks (20.0 °C → 21.0 °C after
two Raise presses; a disabled button renders muted and ignores clicks).
Windows builds of both `oct` variants cross-compile. macOS is untested: Gio
needs cgo there.

## Known gaps (not addressed in M0)

- **The WebView/wasm path is a harness, not an app path.**
  `EmitMachinaUIWasmArtifact` writes a hand-assembled module that *is* the
  M98 counter; only tests call it. Showing a user program there needs either
  an Oct → wasm compiler backend or a Go-session → WebView bridge (a second
  native backend duplicating Gio). Neither is a hookup; both were declined in
  M0 in favour of Gio. Its docs now say so.
- **Styles** were wired end to end after M0 (M117: `UI.Styled`). Padding
  became layout in UI-M3; text size and weight are `UI.TextSpec` (UI-M4).
- **Text metrics** come from the backend's font since UI-M4; headless tools
  keep a fixed metric. See `Libraries/UI/README.md`.
- **No keyboard input, focus traversal or text entry** in the Gio backend.
- **UI builtins are interpreter-only.**
