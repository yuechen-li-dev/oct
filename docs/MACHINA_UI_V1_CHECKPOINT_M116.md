# M116 — Machina UI v1 checkpoint

M116 is a consolidation checkpoint for Machina UI (Machine Native UI) after M101 through M115.

The production spine is now coherent and native-first:

`UI.* authoring -> semantic UIIR -> lowering.Result -> layout resolved geometry -> hittest -> render commands/snapshots -> headless session`.

Renderer backends (Gio, WebView) are consumers of this spine and do not own UI semantics.

## 1) Current architecture (v1 checkpoint)

### Authoring and semantic layer

- Oct user code authors through `Libraries/UI` (`UI.*`).
- UI authoring remains explicit and deterministic (symbolic events, record state, pure `View(state) -> UI`).
- Semantic model remains in Machina UIIR (`machina.uiir.v1`).

### Core deterministic execution path

- `internal/machina/lowering` produces canonical `lowering.Result` from UIIR.
- `internal/machina/layout` resolves deterministic geometry from lowered layout rows/grid data.
- `internal/machina/hittest` maps pointer coordinates to symbolic actions.
- `internal/machina/render` emits renderer-independent commands and supports deterministic snapshots.
- `internal/machina/session` composes projection, lowering, layout, hittest, render, dispatch, and rebuild in a backend-neutral loop.

### Backend layer

- `internal/machina/backend/gio` is an optional native backend lane (tagged/manual).
- `internal/machina/desktophost` is a compatibility/demo host path around WebView shell integration.

## 2) Public Oct `UI.*` surface (M0 + M111 + M112 + M114)

Current public surface includes:

- Structural wrappers (`UI.Text`, `UI.Button`, `UI.Row`, `UI.Column`, `UI.Canvas`, `UI.Grid`, `UI.Spacer`).
- Direct placement constructors (`UI.AbsoluteBox`, `UI.AnchorBox`, plus `...Z`) are canonical with unit-aware coordinates; `UIBox + Place` is compatibility-only.
- Deterministic grid/cell authoring via nested arrays: `UI.GridRows(UI[][])`.
- Lifecycle wrappers (`UI.Mount`, `UI.Patch`, `UI.Unmount`, `UI.Emit`, `UI.DrainEvents`, `UI.Signature`) and `UI.MountRef`.
- Dispatch helpers (`UI.EventValueDispatch`, `UI.ResolveEventValue`, `UI.MatchEventPrefix`).
- Immutable style records (`UI.Color`, `UI.Insets`, `UI.TextStyle`, `UI.Style`) and helper constructors.

## 3) Internal package boundaries (semantic ownership)

Semantic ownership remains core-first and backend-neutral:

- `internal/machina/uiir`: semantic document/event model and ABI contract.
- `internal/machina/lowering`: canonical lowering result.
- `internal/machina/layout`: deterministic geometry resolution.
- `internal/machina/hittest`: pure hit-test selection.
- `internal/machina/render`: renderer-independent command stream and snapshots.
- `internal/machina/session`: backend-neutral runtime/session loop.
- `internal/machina/backend/gio`: optional backend-only native renderer/input loop.
- `internal/machina/desktophost`: compatibility/demo shell host path.

Backends are not semantic authorities.

## 4) Testing posture after M116

### Deterministic core lanes (primary CI lanes)

Primary quality signal is deterministic and headless:

- `Libraries/UI` contract tests via `go run ./cmd/oct test Libraries/UI`.
- Storefront M7 contract tests via `go run ./cmd/oct test Experiments/Storefront/M7`.
- Core package tests (`layout`, `lowering`, `hittest`, `render`, `session`, broader `internal/...`, `cmd/oct`).
- WASM/host parity tests that are fake-DOM/headless and avoid native WebView/window dependencies.

### Optional/manual lanes

- Gio native lane remains explicitly tagged/manual (`machina_gio`) with platform dependency gating.
- WebView native smoke is demoted from normal CI and retained as compatibility/demo validation.

Manual WebView smoke command:

```bash
CGO_ENABLED=1 MACHINA_WEBVIEW_SMOKE=1 go test -tags machina_desktop_webview ./internal/machina/desktophost -run TestWebviewDriverFactoryConstructsAndInitializesNativeBinding -count=1
```

## 5) Explicit constraints (still in force)

- No CSS/class/cascade system in Oct `UI.*`.
- Styles are immutable records; style wiring to renderer remains separate work.
- No Dominatus-style implicit stack assumption in Octomata.
- Gio is backend-only and optional.
- WebView is compatibility/demo infrastructure, not primary semantic path.
- WASM future direction remains MIR-backend-first, browser target second.

## 6) Deferred work beyond v1 checkpoint

- Style wiring into lowering/render/backends.
- Richer component set and higher-level composition primitives.
- Public grid API expansion (gaps/spans/specs) beyond current deterministic baseline.
- WebView compatibility path refactor vs. retirement decision.
- MIR -> WASM backend planning and implementation path.
- Optional Storefront snapshot artifact lane (render/session snapshots) for release evidence.

## 7) CI demotion note (M116)

As of M116, normal CI no longer runs native WebView shell smoke by default.

Reason:

- deterministic Machina core correctness is validated in renderer-independent lanes;
- native WebView/windowing is a compatibility/demo lane with environment-heavy dependencies and expected future refactor pressure.


- Authoring boundary note: UI events/actions are authored as nominal `UI.EventToken` values in `Libraries/UI`, while runtime/UIIR token transport remains string-backed for compatibility in current lanes.
