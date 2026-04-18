# Machina UI Native Host (M100)

M100 establishes the first native-host trajectory for Machina UI with a **webview-backed host layer**.

This milestone keeps the architectural boundary locked:

- same emitted Wasm artifact (`.ui.wasm`) as browser host
- same M96 JSON ABI (`machina.uiir.v1`)
- same canonical event JSON dispatch contract
- same layout semantics (`AbsoluteBox`=`px`, `AnchorBox`=`ui`, bounded deterministic z-order)

## What lands

- `tools/machina-ui-desktop/desktop_host.js`
  - desktop/webview adapter that asks a native bridge for real Wasm bytes
  - reuses `tools/machina-ui-host/host.js` (`MachinaUIWasmHost`) directly
  - optionally forwards canonical dispatch JSON to host diagnostics hook
- `tools/machina-ui-desktop/index.html`
  - minimal desktop webview page wiring
- `internal/interpret/ui_wasm_desktop_host_test.go`
  - end-to-end harness proving desktop-host path consumes real emitted Wasm and M96 ABI

## Source-of-truth boundary

Wasm remains source of truth for:

- state
- transitions
- UIIR generation

Desktop host remains only:

- renderer container
- event bridge

No second UI runtime or native-widget semantic fork is introduced.

## Current bounded limitation (intentional)

This slice does **not** bind to a specific OS webview toolkit in default builds, because that would trigger broad cross-platform packaging churn.

The landed proof is that a desktop webview boundary can run the exact same runtime contract as the browser host with deterministic parity tests over the bounded UI slice.
