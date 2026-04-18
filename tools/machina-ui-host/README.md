# Machina UI Browser Host (M99)

This is the first real Machina UI host renderer.

It is intentionally small and explicit:

- loads an emitted Machina UI `.wasm` artifact
- calls the locked boundary exports (`ExportMachinaUIInit`, `ExportMachinaUIRender`, `ExportMachinaUIDispatchEventJSON`, buffer ptr/len)
- reads canonical M96 UIIR JSON (`machina.uiir.v1`)
- renders visible DOM nodes for current node families (`Text`, `Button`, `AbsoluteBox`, `AnchorBox`, `Row`, `Column`, `Grid`, `Spacer`)
- enforces explicit box units: `AbsoluteBox` as `px`, `AnchorBox` as normalized `ui`
- enforces bounded box z-order (`-5..5`, default `0`) with deterministic paint ordering (z ascending, then stable node order)
- dispatches canonical event JSON back into Wasm on click (`{"token":"...","payload":...}`)
- rerenders after each successful event transition

## Run locally (bounded dev flow)

1. Emit the current bounded Machina UI Wasm artifact:

```bash
go test ./internal/interpret -run TestEmitMachinaUIWasmArtifactAndExecuteBoundary
```

or via a tiny Go helper that calls `interpret.EmitMachinaUIWasmArtifact(...)`.

2. Place `machina-ui-m98-counter.wasm` next to `index.html` + `host.js`.

3. Serve the folder with any static HTTP server and open:

- `http://localhost:<port>/index.html`
- optional explicit wasm override: `?wasm=/path/to/machina-ui-m98-counter.wasm`

## Boundary reminder

The Wasm module remains the source of truth for control state, transitions, and UIIR generation.

This host is only:

- UIIR renderer
- event bridge

No second UI runtime is introduced here.

## Reuse in M100 desktop host

M100 desktop hosting reuses this exact host runtime/renderer through
`tools/machina-ui-desktop/desktop_host.js` rather than forking semantics.
