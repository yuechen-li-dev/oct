# M62 Report — Compiled `.octagon` Read/Write Support

## What M62 adds

Compiled mode now supports runtime-backed `.octagon` data operations for the current representable subset:

- `WriteOctagon(path, value)`
- `LoadOctagon<T>(path) -> T ! Error`

This is implemented in the compiled pipeline without introducing a separate semantic model.

## Lowering model

MIR lowering keeps both operations explicit as builtin runtime calls:

- `call WriteOctagon(path, value)`
- `call LoadOctagon(path)` (with explicit return type retained in MIR call metadata)

`LoadOctagon<T>` remains fallible in MIR and through Go emission.

## Runtime / backend model

The Go backend emits small runtime helpers into generated artifacts for:

- deterministic `.octagon` serialization of representable compiled values
- `.octagon` parsing for the supported value forms
- typed materialization into expected `T`
- explicit runtime error messages/propagation for path, parse, IO, and type mismatch failures

This keeps `.octagon` behavior runtime-backed and explicit, not compile-time embedded.

## Tests added

Focused compiled tests now cover:

- write success and loadable output
- typed load success
- write/load roundtrip
- typed load failure path (`match err`)
- fallible integration via `?` and `match ok/err`
- MIR inspection proving explicit `.octagon` runtime calls

Existing unsupported-feature honesty tests (e.g. `batch`) remain unchanged and continue to assert explicit build failures.

## Still intentionally unsupported

M62 does **not** add compiled support for:

- `batch`
- Octomata runtime features
- plotting
- benchmark/artifact special execution semantics
- generalized file IO surface
- schema/embedding systems

M62 is intentionally narrow: compiled `.octagon` typed data flow support only.
