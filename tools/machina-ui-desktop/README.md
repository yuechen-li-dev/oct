# Machina UI Desktop Host (M100 + M100b)

This directory contains the desktop-host webview layer for Machina UI.

It keeps the architecture boundary explicit:

- reuses `tools/machina-ui-host/host.js` renderer/runtime bridge directly
- consumes the same emitted `.ui.wasm` bytes
- consumes the same locked M96 UIIR JSON ABI (`machina.uiir.v1`)
- dispatches the same canonical event JSON (`{"token":"...","payload":...}`)

## Shared host adapter shape

`desktop_host.js` adds only host-shell concerns:

- asks a native bridge for Wasm bytes (`getWasmBytes` or `getWasmArtifactBase64`)
- boots `MachinaUIWasmHost`
- optionally forwards dispatch JSON to native diagnostics hook (`onEventJSON`)

No second UI runtime or semantic model is introduced.

## M100b real native shell path

M100b adds a bounded launchable native shell command that wraps this shared runtime:

- command: `cmd/machina-ui-desktop-host`
- launcher/runtime glue: `internal/machina/desktophost`
- real webview binding: `github.com/webview/webview_go`

Build/launch is intentionally explicit and bounded:

```bash
go run -tags machina_desktop_webview ./cmd/machina-ui-desktop-host \
  -wasm /path/to/app.ui.wasm
```

Current real-shell support is bounded to `linux`/`darwin` with `cgo` and system webview prerequisites. Default builds without the tag keep returning an explicit unsupported message rather than claiming cross-platform completeness.
