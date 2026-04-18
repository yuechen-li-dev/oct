# Machina UI Desktop Host (M100 sub-slice)

This directory contains the first desktop-host webview layer for Machina UI.

It is intentionally minimal and keeps the architecture boundary explicit:

- reuses `tools/machina-ui-host/host.js` renderer/runtime bridge directly
- consumes the same emitted `.ui.wasm` bytes
- consumes the same locked M96 UIIR JSON ABI (`machina.uiir.v1`)
- dispatches the same canonical event JSON (`{"token":"...","payload":...}`)

## Bounded shape

`desktop_host.js` adds only host-shell concerns:

- asks a native bridge for wasm bytes (`getWasmBytes` or `getWasmArtifactBase64`)
- boots `MachinaUIWasmHost`
- optionally forwards dispatch JSON to native diagnostics hook (`onEventJSON`)

No second UI runtime or semantic model is introduced.

## Current blocker for full native shell productization

The repository currently avoids binding this to a platform-specific webview dependency in default builds, because that introduces broad OS/packaging churn. M100 in this slice focuses on proving that the desktop webview layer can consume the exact same runtime artifact/boundary as the browser host.
