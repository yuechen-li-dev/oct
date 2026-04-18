# Machina UI First Real Host Renderer (M99)

M99 is the first milestone where Machina UI becomes visibly rendered UI.

This milestone does **not** change M94/M95/M96/M97/M98 contracts. It uses them as-is:

- M96 canonical JSON ABI (`machina.uiir.v1`)
- M97 runtime boundary exports
- M98 emitted real Machina UI `.wasm` artifact

## What lands in M99

A bounded browser host renderer at `tools/machina-ui-host/`:

- `host.js` – minimal host runtime + renderer + event bridge
- `index.html` – minimal browser shell
- `styles.css` – minimal rendering support styles

The host behavior is intentionally explicit:

1. load real emitted `.wasm`
2. call `ExportMachinaUIInit()`
3. call `ExportMachinaUIRender()`
4. read canonical UIIR JSON bytes using `ExportMachinaUIBufferPtr()` + `ExportMachinaUIBufferLen()`
5. parse document with `abi == "machina.uiir.v1"`
6. materialize visible UI nodes
7. on button click, serialize canonical event JSON `{ "token": "...", "payload": ... }`
8. call `ExportMachinaUIDispatchEventJSON(ptr, len)`
9. rerender by calling `ExportMachinaUIRender()` again

## Renderer coverage (bounded)

Current bounded host renderer supports these UIIR families:

- `Text`
- `Button`
- `AbsoluteBox`
- `AnchorBox`
- `Row`
- `Column`
- `Grid`
- `Spacer`

`AbsoluteBox` and `AnchorBox` remain explicit primitives (not erased into host-local semantics).

## Source-of-truth boundary remains unchanged

The Wasm module remains the source of truth for:

- state
- transitions
- UIIR generation

The host remains only:

- renderer
- event bridge

No second UI runtime/state machine is introduced in the host.

## Build/artifact and launch shape (bounded)

- Wasm artifact name remains `machina-ui-m98-counter.wasm`.
- Host accepts a wasm URL and defaults to `./machina-ui-m98-counter.wasm` in `index.html`.
- Optional query override: `index.html?wasm=/path/to/machina-ui-m98-counter.wasm`.

This milestone does not broaden into packaging/pipeline polish.

## Testing in M99

`internal/interpret/ui_wasm_host_renderer_test.go` adds end-to-end host tests over real emitted Wasm:

1. real Wasm load + first render
2. click/event roundtrip (`counter.increment`)
3. route switch roundtrip (`route.stats`, `route.home`)
4. deterministic repeated interaction sequence checks
5. bounded renderer node-family coverage checks for all currently locked UIIR node kinds

## Screenshot proof

Screenshot proof is optional and environment-dependent in this milestone.

When screenshot tooling is unavailable or unreliable, M99 relies on deterministic DOM/tree assertions in the host harness tests.
