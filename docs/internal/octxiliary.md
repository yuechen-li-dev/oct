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

M6 is infrastructure only. It proves the path with the isolated `octxiliary-test-wrapper` fixture and does not migrate Archive, Compression, Hash, Plot, Pdf, Text/Regex, Time, Image, CSV, JSON, XLSX, or Markdown wrappers. Later milestones migrate individual standard-library packages onto this path. Handles, records, maps, nested arrays beyond `String[]`, dynamic `any`, sidecar builds, lockfiles, native permission prompts, and broad standard-library migration remain future work.

## M7 Hash standard-library generic wrapper migration

M7 migrates `Libraries/Hash` onto the M6 generic wrapper path as the first real non-IO standard-library wrapper package. The package manifest declares `Kind: "wrapper"` and a `Hash` wrapper family using protocol `octxiliary.v0`, sidecar command `octxiliary-hash`, and package-local Go module directory `octxiliary`.

The production `cmd/octxiliary-hash` sidecar uses the existing `OCTWRAP` handshake/framing and the generic typed-value request shape. It dispatches `Family: "Hash"` for these manifest-declared wire functions:

- `HashSha256Text(String) -> String ! Error`
- `HashSha256Bytes(Bytes) -> String ! Error`
- `HashSha256File(String) -> String ! Error`

All three return lowercase hexadecimal SHA-256 strings. `Sha256Text` hashes the UTF-8 bytes of the input string, `Sha256Bytes` hashes the supplied raw byte payload, and `Sha256File` reads and hashes the file contents. File read failures are returned as sidecar errors (`ok: false`) instead of panics.

This proves that a standard-library package can compile through manifest metadata and a sidecar command without adding a bespoke compiler builtin case for each Hash operation. The M4 IO file/directory sidecar path remains in place and coexists with generic wrappers. Package-manager wrapper planning remains inspection-only: M7 does not add sidecar builds, downloads, lockfiles, permission prompts, or runtime registry consumption.


## M8 Compression standard-library generic wrapper migration

M8 migrates `Libraries/Compression` onto the same generic wrapper path to prove byte-transform workflows, especially `Bytes -> Bytes` gzip round-trips. The package manifest declares `Kind: "wrapper"` and a `Compression` wrapper family using protocol `octxiliary.v0`, sidecar command `octxiliary-compression`, and package-local Go module directory `octxiliary`.

The production `cmd/octxiliary-compression` sidecar uses the existing `OCTWRAP` handshake/framing and generic typed-value request shape. It dispatches `Family: "Compression"` for these manifest-declared wire functions:

- `GzipCompressBytes(Bytes) -> Bytes ! Error`
- `GzipDecompressBytes(Bytes) -> Bytes ! Error`
- `GzipCompressFile(String, String) -> Int ! Error`
- `GzipDecompressFile(String, String) -> Int ! Error`

The public Oct APIs remain `CompressBytes`, `DecompressBytes`, `CompressFile`, and `DecompressFile`; compiled lowering intercepts those manifest-declared public functions and invokes the corresponding gzip wire functions instead of lowering the interpreted wrapper bodies. Invalid gzip payloads and file errors are returned as sidecar errors (`ok: false`) instead of panics.

This extends the M7 proof from string-return hashing to `Bytes -> Bytes` transforms and file-producing gzip helpers without changing the M6 transport set. The M4 IO file/directory sidecar path remains in place and coexists with generic wrappers. Package-manager wrapper planning remains inspection-only: M8 does not add sidecar builds, downloads, lockfiles, permission prompts, or runtime registry consumption.

## M9 Time standard-library generic wrapper migration

M9 migrates `Libraries/Time` onto the generic wrapper path to prove host/time helpers through manifest-declared Octxiliary calls. The package manifest declares `Kind: "wrapper"` and a `Time` wrapper family using protocol `octxiliary.v0`, sidecar command `octxiliary-time`, and package-local Go module directory `octxiliary`.

The production `cmd/octxiliary-time` sidecar uses the existing `OCTWRAP` handshake/framing and generic typed-value request shape. It dispatches `Family: "Time"` for these manifest-declared wire functions:

- `TimeNowIso8601() -> String`
- `TimeParseIso8601(String) -> String ! Error`
- `TimeFormatIso8601(String) -> String ! Error`
- `TimeUnixSecondsNow() -> Int`
- `TimeFormatUnixSecond(Int) -> String ! Error`

The public Oct APIs remain `NowIso8601`, `ParseIso8601`, `FormatIso8601`, `UnixSecondsNow`, and `FormatUnixSeconds`. `ParseIso8601` and `FormatIso8601` preserve the existing Time API by returning normalized RFC3339/ISO-8601 text, not Unix seconds. Invalid time strings return sidecar errors (`ok: false`) instead of panics. `UnixSecondsNow` returns host Unix seconds as `Int`.

This extends the M7/M8 proof from hash and compression wrappers to time-dependent host helpers without changing the M6 transport set. The M4 IO file/directory sidecar path remains in place and coexists with generic wrappers. Package-manager wrapper planning remains inspection-only: M9 does not add sidecar builds, downloads, lockfiles, permission prompts, or runtime registry consumption.

## M10 Text/Regex standard-library generic wrapper migration

M10 migrates `Libraries/Text` onto the generic wrapper path to prove compact host-backed regex/text operations through manifest-declared Octxiliary calls. The package manifest declares `Kind: "wrapper"` and a `Text` wrapper family using protocol `octxiliary.v0`, sidecar command `octxiliary-text`, and package-local Go module directory `octxiliary`.

The production `cmd/octxiliary-text` sidecar uses the existing `OCTWRAP` handshake/framing and generic typed-value request shape. It dispatches `Family: "Text"` for these manifest-declared wire functions:

- `RegexIsMatch(String, String) -> Bool ! Error`
- `RegexFindAll(String, String) -> String[] ! Error`
- `RegexReplaceAll(String, String, String) -> String ! Error`
- `RegexSplit(String, String) -> String[] ! Error`

The public Oct APIs remain `IsMatch(pattern, text)`, `FindAll(pattern, text)`, `ReplaceAll(pattern, text, replacement)`, and `Split(pattern, text)`. Compiled lowering intercepts those public functions through manifest metadata and invokes the corresponding regex wire functions instead of lowering the interpreted wrapper bodies. Invalid regex patterns return sidecar errors (`ok: false`) instead of panics.

The Text sidecar follows Go standard-library `regexp` syntax and behavior, matching the existing interpreter-backed Text implementation. This extends the M7/M8/M9 proof to `String, String -> Bool`, `String, String -> String[]`, and compact text transforms without changing the M6 transport set. The M4 IO file/directory sidecar path remains in place and coexists with generic wrappers. Package-manager wrapper planning remains inspection-only: M10 does not add sidecar builds, downloads, lockfiles, permission prompts, or runtime registry consumption.

## M11 wrapper sweep

M11 migrated the remaining standard-library wrappers whose public APIs fit the M6 generic typed transport set without adding new compiler concepts:

- `Libraries/Archive` now declares wrapper metadata for `ListEntries`, `ExtractAll`, and `CreateFromFiles`, served by `octxiliary-archive`.
- `Libraries/Json` now declares wrapper metadata for `Save` and `Load`, served by `octxiliary-json`; `Object` remains a direct pure Oct string identity helper.

The sweep intentionally deferred candidates that require transports outside M6: CSV row matrices (`String[][]`), Markdown record/nested block helpers, PDF/Image handles and records, and Plot `Float[]`/record arguments. See `docs/internal/octxiliary_m11_wrapper_sweep.md` for the full blocker table.

## M13 String[][] transport and CSV row-major wrappers

M13 adds exactly one generic Octxiliary transport kind: `String[][]` (Go `[][]string`, wire kind `"String[][]"`). This is a deliberately narrow row-major string-table transport, not a general nested-array mechanism.

Protocol payloads use deterministic nested string lists:

```text
OctxiliaryValue { kind: "String[][]" strings2: [ [ "a" "b" ] [ "c" ] ] }
```

The codec preserves outer row order, cell order, empty outer arrays, empty rows, empty string cells, escaped strings, and ragged rows. It does not pad, truncate, transpose, infer numbers, or rectangularize row data.

`Libraries/Csv` now declares wrapper metadata for row-major `Read(path: String) -> String[][] ! Error` and `Write(path: String, rows: String[][]) -> Int ! Error`, served by `cmd/octxiliary-csv`. Raw CSV reads use ragged-row policy (`encoding/csv.Reader.FieldsPerRecord = -1`) so workflows that need lossless parsed rows can compile through the generic sidecar path. `Libraries/IO` row-major `Read`/`Write` aliases also declare the same Csv sidecar metadata for focused compiled use.

The `octxiliary-csv` executable must be available beside the compiled `.octbin` or through `OCT_WRAPPER_PATH`. Missing sidecars report a clear fallible error such as `Octxiliary sidecar "octxiliary-csv" not found`.

Still deferred after M13: records, handles, dynamic `Any`, `Float[]`, `Float[][]`, `Int[]`, `Bytes[]`, Markdown table records/nested blocks, Plot/Pdf/Image/XLSX wrappers, structured JSON graph helpers, compiled Complex, compiled Einstein notation, package-manager sidecar builds, and broad generated-Go numeric/type hardening.

## M16 Plot transport expansion

M16 extends generic Octxiliary with two deliberately narrow transports needed by `Libraries/Plot`:

- `Float[]` encodes a Go `[]float64` as `OctxiliaryValue { kind: "Float[]" floats: [ ... ] }`. Encoding uses `strconv.FormatFloat(v, 'g', -1, 64)` and parsing uses `strconv.ParseFloat(..., 64)`. Empty arrays round-trip. M16 rejects malformed float tokens and non-finite values (`NaN`, `+Inf`, `-Inf`) at parse/sidecar validation boundaries.
- Declared, non-recursive record **arguments** encode as `OctxiliaryValue { kind: "Record" recordType: "..." fields: [ ... ] }`. Record schemas are declared in wrapper manifest `TransportTypes` and the compiler packs fields in manifest order. `Int<...>` fields such as `Int<px>` are transported as `Int` values while retaining their declared field type in metadata.

Record returns are still unsupported. Nested records, recursive records, maps, handles, dynamic `Any`, `Float[][]`, broad `Int[]`, and package-manager sidecar builds remain out of scope.

`Libraries/Plot` is now a wrapper package served by `cmd/octxiliary-plot`. Compiled `Line`, `Scatter`, and `Histogram` calls use `Float[]` plot data plus declared `Plot.Size` and `Plot.Labels` record arguments. `DefaultSize` and `DefaultLabels` remain pure/local Oct helpers. The `octxiliary-plot` sidecar must be available beside the compiled `.octbin` or discoverable through `OCT_WRAPPER_PATH`; missing sidecars report `Octxiliary sidecar "octxiliary-plot" not found`.
