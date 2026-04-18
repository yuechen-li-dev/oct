# Machina UI Native Host (M100 + M100b)

M100 established the first native-host trajectory for Machina UI with a **webview-backed host layer**.

M100b completes the first real native shell slice by adding a **real OS webview binding** around the existing shared host runtime.

These milestones keep the architectural boundary locked:

- same emitted Wasm artifact (`.ui.wasm`) as browser host
- same M96 JSON ABI (`machina.uiir.v1`)
- same canonical event JSON dispatch contract
- same layout semantics (`AbsoluteBox`=`px`, `AnchorBox`=`ui`, bounded deterministic z-order)

## What landed in M100

- `tools/machina-ui-desktop/desktop_host.js`
  - desktop/webview adapter that asks a native bridge for real Wasm bytes
  - reuses `tools/machina-ui-host/host.js` (`MachinaUIWasmHost`) directly
  - optionally forwards canonical dispatch JSON to host diagnostics hook
- `tools/machina-ui-desktop/index.html`
  - minimal desktop webview page wiring
- `internal/interpret/ui_wasm_desktop_host_test.go`
  - end-to-end harness proving desktop-host path consumes real emitted Wasm and M96 ABI

## What landed in M100b

- `internal/machina/desktophost/`
  - bounded native shell launcher that wraps and feeds the shared host runtime
  - native bridge injection for `.ui.wasm` bytes (`window.MachinaDesktopBridge.getWasmArtifactBase64`)
  - inlined host-page generation that embeds shared `tools/machina-ui-host/host.js` + desktop adapter `tools/machina-ui-desktop/desktop_host.js`
- `cmd/machina-ui-desktop-host`
  - launchable native desktop host entrypoint
  - CLI flags for artifact path, optional asset root, and window sizing/title
- `internal/machina/desktophost/driver_webview.go`
  - real OS webview binding via `github.com/webview/webview_go`
  - intentionally bounded to `linux`/`darwin`, `cgo`, and explicit build tag `machina_desktop_webview`
- `internal/machina/desktophost/driver_stub.go`
  - explicit unsupported-mode message in default builds without the tag/toolchain

## Source-of-truth boundary

Wasm remains source of truth for:

- state
- transitions
- UIIR generation

Desktop shell remains only:

- native window/container
- webview bootstrapping + asset loading
- Wasm bytes bridge

No second UI runtime or native-widget semantic fork is introduced.

## Build/launch scope (intentional)

M100b lands a coherent real-shell slice without cross-platform productization churn.

Real native shell path is enabled when built with:

- build tag: `machina_desktop_webview`
- platform: `linux` or `darwin`
- `cgo` enabled
- system webview dependencies installed for `webview_go`

Example launch:

```bash
go run -tags machina_desktop_webview ./cmd/machina-ui-desktop-host \
  -wasm /path/to/counter.ui.wasm
```

If the tag/toolchain/platform preconditions are not met, the command exits with an explicit bounded unsupported message instead of pretending full coverage.
