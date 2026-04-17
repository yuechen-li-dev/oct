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

Current production path (post-M98e completion slice):

- `internal/interpret/ui_wasm_lowering.go`
  - builds canonical M96 JSON render templates from real Machina UI UIIR projection nodes
  - emits the bounded Machina UI module sections directly (type/function/memory/export/code/data)
  - appends a direct-emission provenance custom section (`oct.m98e.lowering`) seeded from lowering templates
- `internal/interpret/wasm_module_builder.go`
  - provides explicit Wasm section framing + ULEB emission utilities used by production emission

Reference-only fixtures retained in-repo:

- `internal/interpret/testdata/machina_ui_m98_counter.c`
- `internal/interpret/ui_wasm_artifact_fixture.go`

Those fixture assets remain historical/reference artifacts and optional oracle material only. Production emission no longer reads historical runtime/template-body bytes.

## M98e status (current bounded slice)

M98e completes backend ownership for the bounded Machina UI module:

- bounded module sections are directly emitted in-process via the internal builder
- preserve M94–M97 semantics and the exact M96 ABI
- keep reference fixture artifacts as reference/oracle only (not production inputs)

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
8. production-path replacement guard: emitted bytes must differ from the legacy fixture byte blob
9. production emission works with `PATH` cleared (guards against accidental compiler toolchain reliance)
10. production emission ignores the legacy fixture decode hook (guards against hidden runtime/template-byte fallback)

## Explicitly preserved boundaries

M98 does **not** add:

- host renderer (native/web)
- patch/diff stream protocol
- effects runtime
- generalized Oct-to-Wasm backend for arbitrary language surfaces

This milestone is specifically the first real emitted Wasm artifact and real boundary execution path for Machina UI.
