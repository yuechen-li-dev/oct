# Machina UI Real Wasm Emission (M98)

M98 is the first milestone where Machina UI lowers to a **real emitted `.wasm` module**.

This milestone builds directly on locked contracts:

- M94 control semantics
- M95 UIIR projection model
- M96 canonical JSON ABI (`machina.uiir.v1`)
- M97 boundary/export contract (`init`/`render`/`dispatch` + output buffer ptr/len)

## Scope of this slice

M98 intentionally lands a bounded, real-execution Wasm slice for the canonical counter+routing Machina UI app contract used by the M97 boundary harness.

It is not yet a full general-purpose Oct-to-Wasm backend.

## Emitted artifact

The emitted module is produced via runtime emission API:

- `interpret.EmitMachinaUIWasmArtifact(outputPath string)`

This writes a real Wasm binary artifact to the requested path.

Current embedded artifact source-of-truth in-repo:

- `internal/interpret/testdata/machina_ui_m98_counter.c` (declared source of truth)
- `internal/interpret/ui_wasm_artifact_fixture.go` (text-only exact-byte fixture representation used by emission/tests)

## Exported boundary surface

The emitted module exports:

- `ExportMachinaUIInit()`
- `ExportMachinaUIRender()`
- `ExportMachinaUIDispatchEventJSON(ptr, len)`
- `ExportMachinaUIBufferPtr()`
- `ExportMachinaUIBufferLen()`
- `memory` (linear memory)

Status codes remain aligned with M97 runtime contract values:

- `0`: OK
- `1`: not initialized
- `2`: invalid input

## Memory/buffer discipline

The Wasm artifact keeps the M97-style simple contract:

- host writes event JSON input bytes into linear memory
- host passes `ptr/len` to `ExportMachinaUIDispatchEventJSON`
- `ExportMachinaUIRender` writes canonical UIIR JSON bytes to a deterministic output buffer location
- host discovers output via `ExportMachinaUIBufferPtr` + `ExportMachinaUIBufferLen`
- next render overwrites output bytes (scratch semantics)

No allocator API or advanced memory ownership protocol is introduced in M98.

## Tested behavior through real Wasm execution

`internal/interpret/ui_wasm_artifact_test.go` validates the emitted `.wasm` via Node's WebAssembly runtime:

1. init + render from real Wasm exports
2. UIIR JSON read back via Wasm linear memory
3. event JSON dispatch through ptr/len
4. rerender after dispatch and state update verification
5. determinism across repeated module executions with identical sequences
6. malformed/invalid-shape event JSON rejection (`invalid input` status)
7. routed mode switch behavior (`route.stats` -> UIIR update)

## Explicitly preserved boundaries

M98 does **not** add:

- host renderer (native/web)
- patch/diff stream protocol
- effects runtime
- generalized Oct-to-Wasm backend for arbitrary language surfaces

This milestone is specifically the first real emitted Wasm artifact and real boundary execution path for Machina UI.
