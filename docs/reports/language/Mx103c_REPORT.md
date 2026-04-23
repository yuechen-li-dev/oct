# Mx103c REPORT — Narrow Bytes Support for Wrapper Boundaries

## 1) Mx103b inconsistency addressed

Mx103b introduced wrapper-backed file byte APIs (`FileReadBytes`/`FileWriteBytes`, surfaced via `IO.ReadBytes`/`IO.WriteBytes`) but represented byte payloads as `Int[]`.

That caused an avoidable mismatch:
- binary boundary payloads were modeled as general numeric arrays,
- wrapper decoding had ad hoc per-call `Int[]` validation/range logic,
- language docs did not have a dedicated binary boundary type.

## 2) Why `Bytes` is justified

`Bytes` is introduced as a narrow transport/storage boundary type so wrapper APIs with genuinely binary payloads can expose a faithful contract (`Bytes`) rather than synthetic `Int[]`.

This improves shape clarity at boundaries without broadening Oct into byte-centric general programming.

## 3) Why `Dynamic` remains out of scope

This milestone does not add `Dynamic` and does not broaden JSON semantics.

`.octagon` remains the native structured format; JSON and bytes wrapper surfaces remain compatibility/boundary oriented.

## 4) Wrapper/library surfaces improved now

Implemented now:
- `FileReadBytes(path) -> Bytes ! Error`
- `FileWriteBytes(path, data: Bytes) -> Int ! Error`
- `IO.ReadBytes(path) -> Bytes ! Error`
- `IO.WriteBytes(path, data: Bytes) -> Int ! Error`
- wrapper substrate helpers: `wrapperBytesResult(...)` and `call.bytesArg(index)`

## 5) How the design keeps `Bytes` narrow

- No byte literals were added.
- No numeric/operator semantics were added for `Bytes`.
- `Bytes` is intentionally only usable through boundary-producing APIs (for now), indexing, and length queries.
- Typechecking rejects treating `Bytes` as numeric or array-substitute in arithmetic.

## 6) Audit summary: fixed now vs deferred

### Sites where lack of `Bytes` was causing awkward API shape

Fixed now:
- `internal/interpret/wrapper_file.go` byte read/write builtins previously exposed/validated `Int[]`.
- `Libraries/IO/IO.File.oct` and `Libraries/IO/README.md` previously documented `Int[]` byte payloads.
- wrapper substrate had no shared bytes lift/decode helper, forcing per-wrapper shape handling.

Deferred:
- no hash/compression/archive/http wrapper surfaces exist in this pass; these remain future consumers of the shared `Bytes` substrate.
- JSON wrappers remain `String`-based compatibility wrappers by design in this milestone.
