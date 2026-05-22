# Octxiliary: Compiled Wrapper Sidecar Bridge

## Purpose

Compiled `.octbin` programs currently cannot execute wrapper-backed standard library builtins because wrapper dispatch/registry state exists only inside interpreter runtime memory.

M0 defines a process-boundary bridge proving one wrapper builtin call from compiled mode without broad wrapper lowering.

## Audit findings

1. **Wrapper registry location**
   - `wrapperBuiltinRegistry` lives in `internal/interpret/wrapper_bridge.go` and stores builtin-name -> handler maps.
   - Handler sets are composed from family-specific maps (`wrapper_file.go`, `wrapper_path.go`, `wrapper_directory.go`, etc.) in interpreter construction.

2. **Reuse constraints for sidecar**
   - Existing handlers are methods on `*interpreter` and require `environment`, `evalExpr`, and `ast.Expr` argument evaluation (`wrapperCall.evalArg`).
   - Therefore handlers are not directly reusable from a sidecar main without pulling interpreter execution engine objects.
   - Reuse is still possible at the operation level by extracting pure backend helpers (for M0 builtin) into shared package functions and calling them from both interpreter handler and sidecar dispatcher.

3. **Current wrapper value model**
   - Wrapper handlers currently receive unevaluated AST argument expressions and decode interpreter `Value` (`ValueString`, `ValueArray`, etc.).
   - Wrapper fallible failures are mapped via `wrapperErrorResult(callee, err)` into `evalResult{hasError:true,errorVal:ValueError(...)}`.

4. **Octagon codec feasibility**
   - Compiled runtime already ships textual Octagon encode/decode helpers for `WriteOctagon`/`LoadOctagon` generation (`__octToOctagonValue`, parser/decoder helpers in `internal/build/compiler.go`).
   - M0 can use this substrate for narrow request/response payloads; broad dynamic `any`-array payloads remain deferred.

5. **Compiled fallible representation**
   - Compiled output models fallible values as generated Go structs with `Value`, `Err string`, `IsErr bool` (see generated `goResultTypeName(...)` forms and existing `LoadOctagon`/Random lowering).

6. **Prometheus bridge pattern**
   - Prometheus runtime performs runtime discovery and validation of optional native bridge artifacts and resolves them dynamically at execution time.
   - Discovery/validation is explicit (no compile-time hard-linking), and fallback/error messaging distinguishes unavailable bridge vs runtime failure.
   - This pattern is architectural precedent for wrapper sidecar discovery/lazy init, but transport/ABI differs (process IPC vs native dynamic loader).

7. **M0 builtin candidate selection**
   - `FileReadText` is the smallest fallible, scalar-only, handle-free wrapper builtin with clear error propagation (`os.ReadFile` + mapped path error) and interpreter coverage.

## Why process boundary

Use standalone sidecar executables for Go wrapper families.

Reasons:
- Avoid second Go runtime hazards from `-buildmode=c-shared` loaded into `.octbin`.
- Avoid Go plugin portability/ABI constraints (especially Windows and strict module graph identity).
- Keep wrapper family code/versioning/deployment independent from each compiled program binary.

## Why framed Octagon (not JSON)

- Octagon is native Oct serialization.
- JSON is not type-faithful enough and JSON itself is part of wrapper surfaces.
- Protocol should exchange Oct values using Oct-native codec.

## M0 protocol

Transport: stream over stdin/stdout.

### Handshake

Client (`.octbin`) writes:
- magic: `OCTWRAP\x00`
- ABI major: `uint16 LE`
- ABI minor: `uint16 LE`

Sidecar validates and echoes same tuple on success; otherwise closes with framed error payload.

### Framing

Each message:
- `uint32 LE` payload byte length
- `N` bytes Octagon payload

### M0 request shape (narrow)

For M1 `FileReadText`/`FileWriteText` only:
- `{ id: Int, family: String, function: String, path: String }`

### M0 response shape

- success: `{ id: Int, ok: Bool, text: String }`
- failure: `{ id: Int, ok: Bool, error: String }`

General dynamic argument arrays are explicitly deferred.

## Sidecar discovery (compiled runtime)

Order:
1. sibling executable near `.octbin`
2. `OCT_WRAPPER_PATH` (directory or explicit executable path)
3. optional dev/test override (test-only)

Missing sidecar maps to fallible `Error` string (not panic) for wrapper calls.

## Handle ownership (future)

- Sidecar process owns wrapper-family handle stores.
- Compiled caller transports opaque handles only.
- M1 builtins (`FileReadText`, `FileWriteText`) do not use handles.

## Deferred beyond M0

- Multi-builtin dispatch table and family-wide lowering.
- Dynamic typed argument/result payload envelopes.
- Concurrency/multiplexing and persistent process pool management.
- Robust lifecycle (restart policy, health checks, TTL).
- Handle-backed wrappers (xlsx/pdf/image/plot/etc.).

## Known inconsistency surfaced

`Language/reference` defines wrapper standard-library availability at language level, but compiled support documentation still marks wrapper-backed calls as deferred except selected builtins. This is a real mode-coverage gap (not syntax/contract disagreement) and should continue to be tracked explicitly in `docs/COMPILED_SUPPORT.md`.


## M2 fallibility policy (wrapper calls)

- Octxiliary is for compiled wrapper operations that are operationally fallible at the process boundary (spawn, handshake, framing/protocol, sidecar runtime failures).
- Those operations should be language-fallible (`... ! Error`) so sidecar/runtime failures are representable to user code.
- Simple non-fallible host queries should compile directly unless an explicit compiled preflight policy is adopted.
- M2 decision: `FileExists(path: String) -> Bool` compiles directly (host `os.Stat` check) and does not require sidecar or `OCT_WRAPPER_PATH`.
- Missing-sidecar behavior must never be hidden behind default values for non-fallible APIs.
