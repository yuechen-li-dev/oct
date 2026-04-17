# Machina UI Wasm Runtime Skeleton (M97)

M97 introduces the first executable **Wasm-boundary runtime skeleton** for Machina UI.

This is intentionally narrow and host-agnostic.

## Role

The M97 runtime owns:

- app instance state
- event application (`token` + `payload`) using M96 event JSON
- projection to UIIR
- canonical JSON serialization (`machina.uiir.v1`)

The M97 runtime does **not** own:

- native/web rendering widgets
- window lifecycle
- effects system
- patch/diff transport

## Boundary functions

The runtime currently exposes an explicit export-style boundary surface:

- `ExportMachinaUIInit()`
  - initialize/reset runtime state from app initial state
- `ExportMachinaUIRender()`
  - project current state to UIIR
  - resolve layout boxes
  - serialize canonical M96 JSON document into the runtime output buffer
- `ExportMachinaUIDispatchEventJSON(ptr, len)`
  - read serialized JSON event from linear memory
  - decode/validate M96 event shape
  - apply deterministic state transition
- `ExportMachinaUIBufferPtr()` / `ExportMachinaUIBufferLen()`
  - expose output buffer location and current byte length

## Memory and ownership discipline

M97 uses a single bounded linear memory region with a deterministic scratch discipline:

- output JSON bytes are written at buffer base (`ptr=0`)
- `BufferLen` indicates valid output byte count
- host input event JSON is copied into memory and passed by `ptr/len`
- next render overwrites the previous output bytes

There is no heap choreography or dynamic free API in M97.

This keeps ownership deterministic and adequate for first host integration slices.

## Error model

Boundary calls return explicit status codes:

- `OK`
- `NotInitialized`
- `InvalidInput`
- `RuntimeError`
- `BufferTooSmall`

Detailed failure text remains available to runtime tests via `LastError()`.

## Non-goals (still out of scope)

- real `.wasm` artifact integration tests
- browser/native host embedding
- patch-stream protocols
- effects or async host service bridges
- alternate binary transport formats

M97 is solely the first concrete runtime boundary layer built on top of the M96 JSON ABI.
