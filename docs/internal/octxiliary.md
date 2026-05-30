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


## M3 narrow additions

- Added sidecar dispatch for `FileDelete`, `DirectoryMake`, and `DirectoryMakeAll` with existing narrow request shape (`path` string only).
- Path helpers (`PathJoin`, `PathBaseName`, `PathExtension`, `PathStem`, `PathParent`, `PathClean`) remain direct compiled lowerings and do not use Octxiliary.
- Protocol remains framed Octagon transport (no JSON protocol).

## M4 IO-family completion

- Added byte payload support to the narrow protocol for `FileReadBytes` and `FileWriteBytes`; byte lists are deterministic textual integer lists and each value must be in `[0, 255]`.
- Added sidecar dispatch for `DirectoryList` and `DirectoryRemoveAll`; directory listing returns sorted entry names only.
- Current compiled split:
  - Direct/no-sidecar: `FileExists`, `PathJoin`, `PathBaseName`, `PathExtension`, `PathStem`, `PathParent`, `PathClean`.
  - Sidecar-backed/fallible: `FileReadText`, `FileWriteText`, `FileReadLines`, `FileWriteLines`, `FileReadBytes`, `FileWriteBytes`, `FileDelete`, `DirectoryList`, `DirectoryMake`, `DirectoryMakeAll`, `DirectoryRemoveAll`.
  - Deferred in this family: none for the IO file/directory wrapper set listed above.

## M6 generic scalar/list/bytes wrapper lowering

M6 adds the first generic compiled call path for manifest-declared wrapper functions. The older M4 IO file/directory helpers still exist unchanged for the established `FileReadText`, `FileWriteText`, `FileReadLines`, `FileWriteLines`, `FileReadBytes`, `FileWriteBytes`, `FileDelete`, `DirectoryList`, `DirectoryMake`, `DirectoryMakeAll`, and `DirectoryRemoveAll` fast path; M6 layers generic metadata-driven calls beside that path rather than replacing it.

Generic requests and responses use typed `OctxiliaryValue` envelopes inside the existing `OCTWRAP` handshake and framed textual protocol:

- requests carry ordered `args: [OctxiliaryValue ...]` payloads,
- responses carry one typed `value: OctxiliaryValue ...` payload,
- sidecar errors still use the existing `ok: false error: ...` response shape.

The supported M6 transport set is intentionally limited to the M5c wrapper manifest types:

- `Void`
- `Int`
- `Float`
- `Bool`
- `String`
- `String[]`
- `Bytes`

`String[]` is encoded as a deterministic quoted string list. `Bytes` is encoded as a deterministic integer list. Floats use stable `strconv.FormatFloat` rendering for protocol round-trip. Unknown kinds and malformed typed payloads are rejected during parse.

Compiled lowering now recognizes wrapper functions from package manifest metadata loaded with the project. When a call targets a manifest-declared wrapper function, the compiler validates the argument and return transport types, emits typed `octxiliary.Value` arguments, calls the sidecar using the wrapper family, wire function name, and sidecar command, then converts the typed result back into the compiled fallible result representation. Sidecar failures propagate through `... ! Error` results.

Generic sidecar discovery uses the sidecar command from wrapper metadata:

1. executable beside the compiled `.octbin`, named by `SidecarCommand`,
2. `OCT_WRAPPER_PATH` as a directory containing `SidecarCommand`,
3. `OCT_WRAPPER_PATH` as an explicit executable only when its basename matches `SidecarCommand`.

A missing generic sidecar reports a message in the form `Octxiliary sidecar "<name>" not found; set OCT_WRAPPER_PATH or place it beside .octbin`.

M6 is infrastructure only. It proves the path with the isolated `octxiliary-test-wrapper` fixture and does not migrate Archive, Compression, Hash, Plot, Pdf, Text/Regex, Time, Image, CSV, JSON, XLSX, or Markdown wrappers. Handles, records, maps, nested arrays beyond `String[]`, dynamic `any`, sidecar builds, lockfiles, native permission prompts, and broad standard-library migration remain future work.

## M7 Hash standard-library generic wrapper migration

M7 migrates `Libraries/Hash` onto the M6 generic wrapper path as the first real non-IO standard-library wrapper package. The package manifest declares `Kind: "wrapper"` and a `Hash` wrapper family using protocol `octxiliary.v0`, sidecar command `octxiliary-hash`, and package-local Go module directory `octxiliary`.

The production `cmd/octxiliary-hash` sidecar uses the existing `OCTWRAP` handshake/framing and the generic typed-value request shape. It dispatches `Family: "Hash"` for these manifest-declared wire functions:

- `HashSha256Text(String) -> String ! Error`
- `HashSha256Bytes(Bytes) -> String ! Error`
- `HashSha256File(String) -> String ! Error`

All three return lowercase hexadecimal SHA-256 strings. `Sha256Text` hashes the UTF-8 bytes of the input string, `Sha256Bytes` hashes the supplied raw byte payload, and `Sha256File` reads and hashes the file contents. File read failures are returned as sidecar errors (`ok: false`) instead of panics.

This proves that a standard-library package can compile through manifest metadata and a sidecar command without adding a bespoke compiler builtin case for each Hash operation. The M4 IO file/directory sidecar path remains in place and coexists with generic wrappers. Package-manager wrapper planning remains inspection-only: M7 does not add sidecar builds, downloads, lockfiles, permission prompts, or runtime registry consumption.
