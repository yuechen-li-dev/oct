# Compiled Support Tracker

_Last updated: 2026-06-12._

This file is the **source of truth** for compiled support posture.


## F4 scalar `String.From<T>` compiled parity

Compiled mode now lowers the documented M0 scalar `String.From<T>` conversions for `Int`, `Float`, `Bool`, and `String`. The generated Go uses `strconv.Itoa`, `strconv.FormatFloat(value, 'g', -1, 64)`, `strconv.FormatBool`, and identity for `String`, matching interpreted output for the supported scalar values.

Unsupported type arguments remain rejected before lowering: enums, records, arrays, and dimensioned numeric values such as `Float<K>` are not part of `String.From<T>`. Use `FormatFloat(value, precision)` when dimensionless `Float` display precision matters; unit-aware formatting remains a future/separate API.


## M28a RF compiled cleanup status

M28a makes the full `Libraries/RF --execution compiled` package run compiled-green: the focused run reports 57 passed / 0 failed, while `Libraries/RF/RF.SParameters.octest --execution compiled` remains 7 passed / 0 failed and interpreted RF remains 57 passed / 0 failed. The fixes are generated-Go cleanup only: discard `for _ in ...` loops now use real compiler temporaries instead of reading Go's blank identifier, expected parameter/return/local/record contexts now coerce integer array literals toward `Float[]` where Oct typing permits it, and unit-bearing `Int<unit>[]` values are converted elementwise when passed to `Float<unit>[]` parameters.

Focused `internal/build` regressions cover `Int[]` to `Float[]` argument, return, record-field, and typed-local contexts plus generated-Go inspection that rejects `_` as a loop value. See `docs/internal/stdlib_compiled_coverage_m28a.md` for the M28a delta.

Still deferred after M28a: Einstein/tensor notation, new Complex features, broad callback/function-value support, wrapper migrations, Octxiliary protocol or transport changes, Pdf image interop, UI live/native bridge work, package-manager sidecar lifecycle changes, and RF public API redesign.

## M27b DifferentialEquations step coercion status

M27b confirms `Libraries/DifferentialEquations --execution compiled` is now compiled-green: the focused run reports 6 passed / 0 failed, matching the interpreted run. The remaining M27a failures were generated-Go `Int`/`Float` coercion leaks around `steps: Int`; the fixed lowering keeps `steps` as an `Int` binding and emits `float64(steps)` only at Float expression sites, not when passing the same binding to `Int`-typed helpers such as solve/validation calls.

A focused internal/build regression now covers the solver-like shape directly, including a callback-shaped helper path: a `steps: Int` parameter is used in Float arithmetic and later passed to `Int` parameters, and the MIR dump is checked to reject `float64(steps)` at those Int call sites. See `docs/internal/stdlib_compiled_coverage_m27b.md` for the M27b delta and verification results.

Still deferred after M27b: broad callback/function-value support beyond named top-level functions and local callback parameters, anonymous lambdas/closures, returned or aggregate-stored function values, fallible callbacks, Einstein/tensor notation, additional Complex work, wrapper migrations, Octxiliary protocol or transport changes, PDF image interop, live UI bridge support, package-manager sidecar lifecycle changes, and DifferentialEquations public API redesign.

## M27a named callback/function-value lowering status

Compiled mode now supports the narrow standard-library callback pattern of passing named top-level functions to monomorphic function-typed parameters and invoking callback parameters inside ordinary compiled functions. The supported current shapes are `fn(Float) -> Float` and `fn(Float, Float) -> Float`; these lower to Go `func(float64) float64` and `func(float64, float64) float64`. This is not a broad first-class function system: lambdas, closures, partial application, aggregate-stored function values, returned function values, fallible callbacks, and Octxiliary callback transport remain deferred.

Focused package results after M27a: `Libraries/Mathematics --execution compiled` is 21 passed / 0 failed, `Libraries/Numerics --execution compiled` is 6 passed / 0 failed, and `Libraries/Optimization --execution compiled` is 7 passed / 0 failed. `Libraries/DifferentialEquations --execution compiled` improves to 2 passed / 4 failed; the remaining solve failures are generated-Go Int/Float coercions for `steps: Int`, not callback identifier lowering. See `docs/internal/stdlib_compiled_coverage_m27a.md` for the detailed M27a delta.

## M26 Complex compiled support status

M26 adds compiled scalar `Complex` support on top of Go `complex128`. The compiled backend now lowers the current interpreted/typechecked Complex surface used by the standard libraries: `I`, `Complex`, `ComplexPolar`, `Real`, `Imag`, `Arg`, `Conj`, `Abs(Complex)`, Complex `Exp`/`Ln`, and Complex arithmetic mixed with dimensionless `Int`/`Float` where the typechecker already permits it. `Real` and `Imag` use generated helpers so Oct locals named `real` or `imag` do not shadow Go's predeclared functions.

Focused package results after M26: `Libraries/Complex --execution compiled` is 9 passed / 0 failed, `Libraries/Signal --execution compiled` is 37 passed / 0 failed, and the Complex-dependent `Libraries/RF/RF.SParameters.octest --execution compiled` is 7 passed / 0 failed. `Libraries/Mathematics --execution compiled` improves to 14 passed / 7 failed; the remaining failures are callback/function-value calculus cases (`unknown identifier 'f'`) rather than Complex support. Full `Libraries/RF` still has non-Complex generated-Go placeholder and `Int[]`/`Float[]` coercion failures. See `docs/internal/stdlib_compiled_coverage_m26.md` for the detailed M26 delta.

Still deferred after M26: Einstein/tensor notation, broad callback/function-value lowering, new wrapper migrations, new Octxiliary transports or protocol changes, PDF image interop, live UI bridge support, package-manager sidecar build lifecycle, public API redesign, and arbitrary numeric tower/generic algebra beyond the Complex coercions selected by existing typechecking.

## M25 runtime/index cleanup status

M25 fixes the remaining M24 `Libraries/Statistics` compiled runtime failures and the `Libraries/Analysis` zero-assertion test shape. Statistics and Analysis are now compiled-green in focused package checks: `Libraries/Statistics --execution compiled` reports 36 passed / 0 failed, and `Libraries/Analysis --execution compiled` reports 36 passed / 0 failed.

The Statistics issue was not a library sorting bug: generated-Go lowering for logical `and` / `or` evaluated both operands into temporaries before emitting `&&` / `||`, so `SortedCopy` evaluated `out[j - 1]` after `j` reached zero. Logical binary lowering now emits explicit short-circuit control flow. Analysis was fixed by adding the intended assertion to `LocalMaximaDoesNotIncludeEndpoints`; no runner workaround was added.

Still deferred after M25: Complex support, Einstein/tensor notation, broad callback/function-value lowering, new wrapper migrations, new Octxiliary transports or protocol changes, PDF image interop, live UI bridge support, package-manager sidecar build lifecycle, and public API redesign. The language reference still has a documentation gap: it lists logical operators and deterministic evaluation order but does not explicitly state short-circuit behavior for `and` and `or`.

## M24 numeric/shape lowering status

M24 extends the M23 generated-Go hardening work without changing Octxiliary transports or public APIs. Interpolation, LinearAlgebra, and Random now compile green in focused `Libraries/* --execution compiled` checks, and Statistics improves from generated-Go numeric type failures to 32 passed / 3 runtime failures. Analysis still has one local-maxima compiled test that reaches runtime but exits with zero assertions.

Still deferred after M24: Complex support, Einstein/tensor notation, broad callback/function-value lowering, PDF image interop, live UI bridge support, legacy structured IO cleanup, and package manifest/dependency cleanup. Remaining generated numeric/shape work is narrower: Statistics `SortedCopy` runtime/index behavior, Analysis compiled zero-assertion runner/test shape, and any Mechanics/RF array cases that do not require Complex or Einstein/tensor support.

## M14 Markdown compiled support

- `Libraries/Markdown` is compiled as ordinary deterministic report construction. It is **not** an Octxiliary wrapper package, and no `octxiliary-markdown` sidecar is required.
- Scalar/list/report helpers now lower directly in generated Go: headings, paragraphs, blank lines, horizontal rules, bullets, numbered lists, code blocks, callouts, images, figures, key-value tables, sections, subsections, report flattening, and escape helpers.
- `Markdown.Report(blocks: String[][]) -> String[]` uses the existing compiled `String[][]` value path for block composition.
- Columnar record table helpers (`Markdown.Table` and `Markdown.TableWithColumns`) compile as direct in-process helpers over generated record structs; this does not add record transport or an Octxiliary boundary.
- Package-level compiled verification: `go run ./cmd/oct test Libraries/Markdown --execution compiled`.

## Current green surfaces

- Core compiled harness: `oct test --execution compiled|auto|interpreted` control-plane is available.
- Explicit selected-file compiled test targeting is working for focused fixtures.
- String compiled surface used by `Libraries/String` is green (no fallback in compiled runs on the current verification set).
- Core assertion fixture lane used by compiled test harness is green.
- Pure-builtin compiled sweep fixture (`Language/Testing/CompiledBuiltinSweep/valid`) is green in current verification.

## Octomata feature-granular support snapshot

Measured using `go run ./cmd/oct test ... --execution compiled|auto` on May 21, 2026.

| Feature | Interpreted support | Compiled support | Fixture evidence | Limitations |
| --- | --- | --- | --- | --- |
| flow declaration lowering | Supported | Supported | `OctomataCompiledBoundary/valid/compiled_boundary_core.octest` | No contradiction found between MIR flow lowering and Go emission paths. |
| state declaration lowering | Supported | Supported | `OctomataCompiledBoundary/valid/compiled_boundary_core.octest` | State machine still requires explicit control-transfer (`suspend`, `goto`, `return`). |
| suspend | Supported | Supported (inside `flow state`) | `OctomataCompiledBoundary/valid/compiled_boundary_core.octest` | Outside flow state remains rejected with explicit diagnostic. |
| goto | Supported | Supported (inside `flow state`) | Existing runtime fixtures in `OctomataCoreA/runtime/valid` and `OctomataCoreB/runtime/valid` | Outside flow state remains rejected. |
| resume | Supported | Supported | `OctomataResumeM57/runtime/valid/single_slot_resume_behaviors.octest` | Empty-slot resume remains runtime error path by design. |
| remember | Supported | Supported | `OctomataResumeM57/runtime/valid/single_slot_resume_behaviors.octest` | Single-slot semantics only; latest remember overwrites prior slot. |
| when policy / utility when | Supported | Supported | `OctomataCoreB` + `OctomataUtilityWhen` suites | Must satisfy existing utility-when shape/type constraints. |
| board declarations | Supported | Supported | `OctomataBlackboardM6/valid/flow_declared_board_surface.octest` | Placement/type rules still enforced by invalid fixtures. |
| scalar board fields Bool/Int/Float/String | Supported | Supported | `OctomataCompiledBoundary/valid/compiled_boundary_core.octest` | Scalar-only field contract. |
| flow-state local `let` temporaries | Supported | Supported (state/block scoped, immutable, non-persistent) | `OctomataFlowLocals/valid/flow_state_let_surface.octest` | Local bindings are scoped to the current state/block only; they do not persist across states and do not appear in board/history snapshots. Fallible flow-let RHS remains unsupported in compiled mode. |
| flow expression calls (pure builtins) | Supported | Supported (builtin-only in flow state expressions) | `OctomataFlowCallExpr/valid/flow_call_builtin_surface.octest` | Calls inside flow expressions are currently limited to non-fallible pure builtins (`Len`, `Abs`, `Sqrt`, trig/log family, `FloorToInt`/`CeilToInt`/`RoundToInt`, `FormatFloat`). Fallible or side-effectful wrappers remain deferred. |
| flow expression indexing | Supported (array indexing) | Supported (array indexing `T[]` with `Int` index in flow expressions) | `OctomataFlowIndexExpr/valid/flow_index_expr_surface.octest` | Current compiled flow support is scoped to single-dimension array indexing. String/matrix/vector flow-expression indexing remains deferred until explicitly verified. |
| board array fields | Unsupported | Unsupported | `OctomataCompiledBoundary/invalid/board_array_unsupported.octfail` | Diagnostic: board fields must be Bool/Int/Float/String. |
| Step(machine) | Supported | Supported | `OctomataCompiledBoundary/valid/compiled_boundary_core.octest` | Mutates flow in place; returns `Int` sentinel in lowered MIR. |
| Active(machine) | Supported | Supported | `OctomataCompiledBoundary/valid/compiled_boundary_core.octest` | Returns empty string before first step and after completion. |
| Complete(machine) | Supported | Supported | `OctomataCompiledBoundary/valid/compiled_boundary_core.octest` | `false` until completed terminal path executes. |
| Result(machine) | Supported | Supported | `OctomataCompiledBoundary/valid/compiled_boundary_core.octest` | Calling before completion returns error result contract. |
| StateHistory(machine) | Supported | Supported | `OctomataCompiledBoundary/valid/compiled_boundary_core.octest`; `OctomataResumeM57/runtime/valid/...` | History records entry + transitions only. |
| ResumeTarget(machine) | Supported | Supported | `OctomataCompiledBoundary/valid/compiled_boundary_core.octest`; `OctomataResumeM57/runtime/valid/...` | Empty string when slot unused/cleared. |

Notes:
- Compiler audit found active support in `lowerFlow`, `emitGoFlow`, and MIR builtin emission for Step/Active/Result/Complete/StateHistory/ResumeTarget.
- No stale “unsupported Step/Result/etc” gate was found ahead of those paths; stale wording was only in generic non-flow statement diagnostics and was narrowed.



## Flow expression parity matrix (compiled)

| AST expression kind | Normal compiled | Flow compiled | Safe for flow M0 | Decision | Notes |
| --- | --- | --- | --- | --- | --- |
| `ast.IntegerLiteral` / `ast.FloatLiteral` / `ast.BoolLiteral` / `ast.StringLiteralExpr` | Yes | Yes | Yes | Keep | Baseline literal support in both paths. |
| `ast.IdentifierExpr` | Yes | Yes | Yes | Keep | Flow identifiers include params + flow locals. |
| `ast.BinaryExpr` / `ast.UnaryExpr` | Yes | Yes | Yes | Keep | Arithmetic/logical/comparison operators are already available. |
| `ast.ParenExpr` | Yes | **Yes (added in this sweep)** | Yes | Implement now | Lower/type-infer now recurse into inner expression. |
| `ast.CallExpr` | Yes | Yes (builtin-only) | Yes (pure only) | Keep allowlist | Flow keeps non-fallible pure builtin-only boundary. |
| `ast.IndexExpr` | Yes | Yes (single-dimension array only) | Yes | Keep scoped | Matrix/vector/string indexing still deferred in flow path. |
| `ast.FieldAccessExpr` | Yes | Partial (`board.<field>` + enum variant values/constructors) | Yes (M0 board + enum constructors) | Scoped support | Compiled flow now supports enum `Type.Variant` values and `Type.Variant(payload)` constructors in expression positions; arbitrary object/member field access remains unsupported. |
| `ast.ArrayLiteralExpr` | Yes | No | Maybe | Defer | Not required for current scalar-board M0 parity and would expand flow type surface. |
| `ast.RecordLiteralExpr` | Yes | Yes (record construction for flow return/value expressions) | Yes (named-field record construction) | Implemented | Compiled flow lowering now supports named-field record literals for direct value construction (including return paths). Imported/advanced record forms remain bounded by existing flow type resolution. |
| `ast.RecordUpdateExpr` (`with`) | Yes | No | Maybe | Defer | Wider record semantics not needed for current M0 flow support target. |
| `ast.IfExpr` / `ast.SwitchExpr` | Yes | `ast.IfExpr`: **Yes (M0)**, `ast.SwitchExpr`: Yes | Yes | Implemented for `IfExpr` | Compiled flow now supports expression `if { } else { }` with Bool condition and same-typed branches; `else if` syntax remains unsupported by language policy. |
| `ast.UtilityWhenExpr` | Yes | Yes | Yes | Keep | Compiled flow has dedicated utility-when MIR node. |
| `ast.PropagateExpr` / `ast.UnwrapExpr` | Yes | No | No | Defer/reject | Flow expressions deliberately reject fallible expression handling in compiled mode. |
| `ast.MatchExpr` | Partial | **Yes (enum-tag/payload M0)** | Yes (enum-only) | Implemented | Compiled flow supports enum `match` with tag dispatch and optional single payload binding in arm scope. |
| `ast.BatchExpr` / runtime-heavy expression forms | Partial | No | No | Defer | Outside current pure, deterministic local flow-expression scope. |

M3 status after this sweep: the prior compiled blocker for `ast.ParenExpr` in flow expressions is removed, and compiled `BoardSnapshot` support is now green for M0 scalar board fields (`Bool`/`Int`/`Float`/`String`) with board arrays intentionally unsupported.

## Array cross-section

| Surface | Interpreted | Compiled | Coverage | Notes |
| --- | --- | --- | --- | --- |
| `Array.CrossSection(values: T[], range: Range) -> T[]` | Supported | Supported | `Language/Types/Arrays/valid/array_cross_section.octest` | 1D arrays only; returns a new copy, not a Go slice view; preserves exact element type including SI dimensions and record/enum element types. Colon slicing (`xs[1:3]`) and bracket range extraction (`xs[1..3]`) remain invalid. Negative indices, reverse ranges, views, `Array.TryCrossSection`, and `Array.Copy`/`Take`/`Drop`/`Window` aliases are deferred/not M0. |

## Current deferred / partial categories

- Markdown wrapper-heavy paths are still largely interpreted/fallback territory.
- Artifact-lane compiled support remains partial (artifact workflows should assume interpreted execution unless explicitly verified).
- Flow expression calls still reject side-effectful wrappers and fallible calls in compiled mode.
- **Direct compiled (no sidecar):** `FileExists`, `PathJoin`, `PathBaseName`, `PathExtension`, `PathStem`, `PathParent`, `PathClean`.
- **Octxiliary sidecar-backed compiled (`... ! Error`):** `FileReadText`, `FileWriteText`, `FileReadLines`, `FileWriteLines`, `FileReadBytes`, `FileWriteBytes`, `FileDelete`, `DirectoryList`, `DirectoryMake`, `DirectoryMakeAll`, `DirectoryRemoveAll`, `CsvRead`, `CsvReadRows`, `CsvReadTable`, `CsvReadMatrix`, `CsvWrite`, `CsvWriteRows`, `JsonNormalize`, `JsonParse`, `JsonStringify`, `JsonLoad`, `JsonSave`.
- **Deferred wrapper builtins in this family:** none for the IO file/directory wrapper set listed above.
- Sidecar-backed compiled calls require Octxiliary availability (the required sidecar beside `.octbin` or discoverable through `OCT_WRAPPER_PATH`; CSV calls require `octxiliary-csv`, JSON calls require `octxiliary-json`, and Time wrapper calls require `octxiliary-time`).
- IO/Csv/Json wrapper breadth is still mixed for structured JSON graph helpers (`JsonLower`, `JsonLoadStructured`) and broader record/handle transports; the scalar JSON and CSV helpers listed above are compiled-supported when their sidecars are discoverable.
- Octxiliary policy: fallible wrapper operations listed above route through the Octxiliary sidecar and propagate sidecar/process/protocol failures as `Error`; simple non-fallible host queries such as `FileExists(path: String) -> Bool` compile directly to host runtime checks so compiled execution does not require `OCT_WRAPPER_PATH` for that query. All other wrappers remain deferred unless explicitly listed as compiled-supported.
- Some experiment packages still hit compiled-only blocker combinations (wrapper reachability, generated-Go mismatch, or timeout fallback pressure in auto).

## Recent sweep summary

- Recent pure-builtin compiled sweep confirms core obvious pure builtins in the sweep fixture are lowering successfully.
- String-focused compiled sweep and library lane remain green on current repo measurements.

## Selected-file harness status

- Selected-file compiled mode is repaired and currently usable for isolating a single `.octest` file in package context.
- Sibling files/imports still load/typecheck per normal package rules.

## Selected-directory fixture convention

- `Language/Testing/SelectedDirectoryCompiled` is a fixture container, not a package root.
- The actual package root target is `Language/Testing/SelectedDirectoryCompiled/valid`.
- Running compiled test directly on the parent container currently surfaces `unknown package 'Main'`; treat this as expected shape mismatch for that container target, not as a compiled-lowering regression.

## M2 / M2b smoke status (current)

- M2 and M2b suites are **not** fully compiled-green yet.
- Auto mode remains useful for probing support (`compiled` then interpreted fallback), but fallback timeouts/blockers still appear in this area.
- Treat this area as ongoing compiled convergence work, not a completed surface.

## Artifact lane limitation

- `[Artifact]` workflows are not yet a broadly green compiled lane.
- Prefer interpreted artifact execution unless a specific compiled artifact target has been explicitly validated.

## Next priorities

1. Continue reducing wrapper-bridge gaps that force fallback in Markdown/IO-adjacent paths.
2. Keep selected-target compiled fixture coverage tight and diagnostic-friendly.
3. Isolate and clear remaining M2/M2b compiled blockers with focused fixtures.
4. Expand compiled sweep fixtures only after each newly-green surface is measured.

| BoardSnapshot(machine) | Supported | Supported (scalar board fields, including `Int<D>`/`Float<D>`) | `OctomataBoardSnapshot`; `DimensionedScalarBoardProbe`; `Experiments.FmBrownNoiseKalman.M3.FlowSmoke` | Compiled support returns detached/read-only snapshots of scalar board fields (`Bool`, `String`, `Int`/`Int<D>`, `Float`/`Float<D>`). Board arrays, vectors, matrices, records, and enums remain unsupported. |

## 2026-06-12 F6 SI board and BaseUnit release contract

- Compiled mode supports `BaseUnit(Float<D>) -> Float` and dimensionless `BaseUnit(Float) -> Float`; the lowering erases only the static dimension and leaves the numeric value unchanged. `BaseValue` remains accepted as the older spelling.
- Compiled Octomata board fields support scalar `Int<D>` and `Float<D>` values in addition to `Bool`, `Int`, `Float`, and `String`.
- Compiled `BoardSnapshot(machine)!` preserves exact dimensioned scalar field types in the generated snapshot record.
- Board arrays, vectors, matrices, records, enums, and non-scalar runtime values remain outside the compiled board snapshot contract.

## 2026-05-23 SmartGreenhouse compiled convergence pass 2

- `ast.MatchExpr` now has compiled M0 support for enum-tag dispatch with optional single payload binding and expression-valued arms.
- Added focused fixture: `Language/ControlFlow/EnumPayloadMatchCompiled/valid/payload_match.octest` (compiled/auto/default green).
- `examples/SmartGreenhouseController --execution compiled` now passes `EnumPayloadMatchWorks`; the prior blocker `unsupported expression ast.MatchExpr` is resolved.
- Remaining SmartGreenhouse compiled blockers (unchanged semantics):
  - Matrix/vector codegen mismatch in generated Go (`[]float64` emitted where scalar/vector ops are expected) in `MatrixVectorDistinctFromArray`.
  - Runtime assertion failure `battery consumed` in `FlowCompletesAndHistoryPresent`.

## 2026-05-23 SmartGreenhouse compiled convergence pass 3

- Fixed compiled vector codegen mismatch by lowering vector arithmetic operators to dedicated compiled helpers instead of raw Go `+/-/*//` on slices, preserving vector semantics distinct from arrays.
- Fixed compiled vector indexing type propagation so indexing `Vector<T>` now lowers to scalar `T` (instead of incorrectly carrying `Vector<T>`).
- Fixed compiled flow `if` block emission so sequential field updates inside a single conditional no longer short-circuit after the first assignment.
- `examples/SmartGreenhouseController --execution compiled` is now fully green, including:
  - `MatrixVectorDistinctFromArray`
  - `FlowCompletesAndHistoryPresent`


- Compiled mode supports `Float(Int) -> Float` in flow expressions (including `let`, assignment RHS, return expressions, and nested arithmetic), `Clamp01(Float) -> Float`, and switch expressions in flow blocks (expression form only; no `else if` syntax).

## M6 Octxiliary generic wrapper lowering status

M6 adds metadata-driven compiled lowering for manifest-declared Octxiliary wrapper functions whose argument and return transport types are limited to `Void`, `Int`, `Float`, `Bool`, `String`, `String[]`, and `Bytes`.

This is a generic sidecar call path: compiled code uses wrapper manifest metadata (family, wire name, sidecar command, argument types, return type, and fallibility) to emit a typed Octxiliary request instead of teaching the compiler one bespoke builtin per wrapper function. Fallible wrapper calls return compiled fallible result structs and propagate sidecar/protocol errors as `Error` values.

The M4 hardcoded IO file/directory compiled helpers remain supported and coexist with the generic path. M7 migrates the first real standard-library generic wrapper package: `Libraries/Hash` now compiles `Sha256Text`, `Sha256Bytes`, and `Sha256File` through manifest metadata and `cmd/octxiliary-hash`. M8 migrates `Libraries/Compression` so `CompressBytes`, `DecompressBytes`, `CompressFile`, and `DecompressFile` compile through manifest metadata and `cmd/octxiliary-compression`; this is the proof for `Bytes -> Bytes` wrapper transforms. M9 migrates `Libraries/Time` so `NowIso8601`, `ParseIso8601`, `FormatIso8601`, `UnixSecondsNow`, and `FormatUnixSeconds` compile through manifest metadata and `cmd/octxiliary-time`; this proves host/time helper calls through the same generic path. M10 migrates `Libraries/Text` so `IsMatch`, `FindAll`, `ReplaceAll`, and `Split` compile through manifest metadata and `cmd/octxiliary-text`; this proves compact regex/text helpers, including `String, String -> Bool`, on the same generic path. The required sidecar executable (`octxiliary-hash`, `octxiliary-compression`, `octxiliary-time`, or `octxiliary-text`) must be available beside the compiled `.octbin` or discoverable through `OCT_WRAPPER_PATH`.

`Libraries/Text` compiled regex support follows the existing public API order (`pattern`, then `text`) and the Go standard-library `regexp` syntax used by the interpreter-backed implementation. Invalid regex patterns are fallible sidecar errors rather than panics.

The remaining standard-library wrapper backlog identified in M5g is still future work: Archive, Plot, Pdf, Image, CSV, JSON, XLSX, Markdown helpers, handle transports, record transports, and generated-Go numeric hardening are not completed by M10.

Current proof fixture:

- `Language/Testing/CompiledOctxiliary/valid/generic_wrapper_m6.octest`
- `cmd/octxiliary-test-wrapper`

The fixture covers generic `String`, `String[]`, `Bytes`, `Int`, `Float`, `Bool`, `Void` return, missing sidecar diagnostics, and fallible sidecar-error propagation without changing real standard-library wrapper surfaces.

## M11 generic wrapper sweep

Compiled generic Octxiliary wrapper lowering now covers these additional standard-library packages:

- `Archive`: `ListEntries`, `ExtractAll`, and `CreateFromFiles` through `octxiliary-archive` using `String`, `String[]`, and `Int` transports.
- `Json`: `Save` and `Load` through `octxiliary-json` using `String` and `Int` transports; direct JSON builtins `JsonNormalize`, `JsonParse`, `JsonStringify`, `JsonLoad`, and `JsonSave` also lower to `octxiliary-json`; `Object` remains direct pure Oct.

Deferred after the M11 sweep:

- Historical M11 state: `Csv` was blocked by `String[][]` row data (`needs_nested_array_transport`); M13 and H3 now cover row-major CSV plus the narrow direct `CsvReadRows`, `CsvReadTable`, and `CsvReadMatrix` builtins.
- `Markdown` remains blocked by record-of-`String[]` tables and nested block arrays (`needs_record_transport`, `needs_nested_array_transport`).
- Historical M11 state: `Pdf` and `Image` were still blocked by opaque handles and record arguments (`needs_handle_transport`, `needs_record_transport`); later M19/M21/M30 sections below update that status.
- `Plot` remains blocked by `Float[]` plot data and record arguments (`needs_float_array_transport`, `needs_record_transport`).

M11 did not add new transport kinds, compiled Complex support, Einstein notation, record transport, handle transport, package-manager sidecar builds, lockfiles, or public API redesigns.

## M13 Octxiliary Csv row-major status

M13 extends generic Octxiliary lowering with exactly `String[][]` transport (`[][]string` in Go; wire kind `"String[][]"`). This unlocks compiled row-major CSV wrappers without adding general nested-array, record, handle, dynamic, numeric-array, Complex, or Einstein support.

Compiled support now includes:

- `Libraries/Csv.Read(path: String) -> String[][] ! Error`
- `Libraries/Csv.Write(path: String, rows: String[][]) -> Int ! Error`
- focused `Libraries/IO` row-major `Read`/`Write` aliases when `octxiliary-csv` is available
- direct `CsvReadRows(path: String) -> String[][] ! Error` via `octxiliary-csv` `CsvReadRows`
- direct `CsvReadTable(path: String) -> Csv.Table ! Error` via `octxiliary-csv` `CsvReadRows` plus compiled header/ragged validation
- direct `CsvReadMatrix(path: String) -> Float[][] ! Error` via `octxiliary-csv` `CsvReadRows` plus compiled numeric conversion

Raw CSV row reads preserve ragged rows and exact parsed string cells. CSV writes emit exactly the supplied row-major string data through Go's standard `encoding/csv` writer. Table and matrix convenience builtins intentionally reuse the row-major sidecar function and keep their validation/conversion in compiled runtime glue, matching the interpreted wrappers without changing the sidecar protocol. The `octxiliary-csv` sidecar must be discoverable beside the `.octbin` or through `OCT_WRAPPER_PATH`.

Still unsupported/deferred: Markdown structured table transports, Plot numeric arrays, Pdf/Image/XLSX handle-backed workflows beyond their specifically listed support, structured JSON graph helpers (`JsonLower`, `JsonLoadStructured`), record transport beyond narrow handles/pseudo-table validation, dynamic `Any`, and broad nested-array generalization.

## M16 Octxiliary Plot status

M16 adds generic `Float[]` transport and manifest-declared, non-recursive record argument transport. This migrates `Libraries/Plot.Line`, `Libraries/Plot.Scatter`, and `Libraries/Plot.Histogram` to compiled wrapper lowering through `cmd/octxiliary-plot` while preserving the public Oct signatures.

`Plot.Size` and `Plot.Labels` are declared as wrapper `TransportTypes`; compiled code packs those generated Go record structs into ordered Octxiliary record arguments. Dimensioned `Int<px>` size fields are dimension-erased over the wire as `Int` payloads. `DefaultSize` and `DefaultLabels` remain pure/local and do not call the sidecar.

Still unsupported/deferred after M19: general record returns, nested or recursive records, cross-family handles, handle close/destructor semantics, handle serialization across runs, dynamic `Any`, maps, `Float[][]`, broad `Int[]`, legacy Pdf image-handle bridge, structured JSON graph helpers, Markdown-as-Octxiliary, package-manager sidecar builds, native permission prompts, and lockfiles. The `octxiliary-plot`, `octxiliary-xlsx`, and `octxiliary-image` executables must be beside the compiled `.octbin` or on `OCT_WRAPPER_PATH`. M18 adds handle transport M0 and migrates `IO.Xlsx`; M19 reuses that handle path for Image; handles are sidecar-owned capabilities, not plain `Int` values.

## M19 Octxiliary Image status

M19 migrates `Libraries/Image` to compiled generic wrapper lowering through `cmd/octxiliary-image` and handle transport. M30 adds `EncodePng(image: ImageHandle) -> Bytes ! Error` for explicit serialized transfer. The public API includes `Load(path: String) -> ImageHandle ! Error`, `Save(image: ImageHandle, path: String) -> Int ! Error`, `EncodePng(image: ImageHandle) -> Bytes ! Error`, `Width(image: ImageHandle) -> Int<px>`, `Height(image: ImageHandle) -> Int<px>`, and `Format(image: ImageHandle) -> String`.

`Image.ImageHandle` is a sidecar-owned, process-lifetime handle capability. The public record still has a single `Handle: Int` field, but compiled transport carries family `Image`, handle type `Image.ImageHandle`, and a sidecar-local positive ID. Handles are not serializable across program runs and are not shared with `Pdf` or any other family. There is still no `Close`/destructor API.

`Width`, `Height`, and `Format` remain non-fallible. If an invalid/stale Image handle reaches one of those operations in compiled mode, the sidecar error becomes a runtime failure through the generic non-fallible wrapper path. `Load`, `Save`, and `EncodePng` remain fallible and report missing files, corrupt images, unsupported save extensions, and invalid handles as `Error` values. The `octxiliary-image` executable must be beside the compiled `.octbin` or available through `OCT_WRAPPER_PATH`.

## M21 Pdf compiled subset

`Libraries/Pdf` now has focused compiled support for the text/page/save subset through `octxiliary-pdf`:

- `Pdf.NewPage(width: Int<px>, height: Int<px>) -> Pdf.PdfPage ! Error`
- `Pdf.DrawText(page: Pdf.PdfPage, x: Int<px>, y: Int<px>, text: String) -> Int ! Error`
- `Pdf.DrawTextStyled(page: Pdf.PdfPage, x: Int<px>, y: Int<px>, text: String, style: Pdf.TextStyle) -> Int ! Error`
- `Pdf.Save(page: Pdf.PdfPage, path: String) -> Int ! Error`

`Pdf.PdfPage` is a sidecar-owned handle and `Pdf.TextStyle` is a record argument transport. `Pdf.DefaultTextStyle()` remains pure/local Oct.

Deferred in compiled mode: `Pdf.DrawImage`, `Pdf.DrawImageSized`, and compiled `Pdf.ImageHandle` support. `octxiliary-pdf` does not consume `Image.ImageHandle` values from `octxiliary-image`; cross-family handle sharing remains unsupported. Put `octxiliary-pdf` beside the compiled `.octbin` or set `OCT_WRAPPER_PATH` to a directory containing it.


## M30 Pdf/Image bytes interop

`Image.EncodePng` is compiled-supported and exports an Image-owned handle as PNG `Bytes`. `Pdf.DrawImageBytes` and `Pdf.DrawImageBytesSized` are compiled-supported and consume PNG bytes in the Pdf sidecar. Legacy `Pdf.DrawImage` / `Pdf.DrawImageSized` remain interpreted legacy bridge APIs and compiled-deferred; M30 adds no cross-family handles, sidecar-to-sidecar calls, broker, protocol change, path/file drawing API, or Pdf-owned image handle table.


## M33 rank-2 matrix Einstein compiled support

Compiled mode now supports the existing rank-2 matrix indexed Einstein surface:

- `Idx("name") -> Index` lowers to a validated string label.
- Explicit public `EinMul(A, i, k, B, k, j)` and `EinAdd(A, i, j, B, i, j)` lower to shared generated matrix helpers.
- Infix `A[i, k] * B[k, j]`, `A[i, j] + B[i, j]`, and `A[i, j] - B[i, j]` are compiled-supported for rank-2 matrices.
- Nested expression trees preserve labels during lowering, while assigned intermediates remain ordinary matrices and can be reindexed explicitly.

M37 extends compiled indexed notation with vector rank-1 terms, dot/outer products from vector indexed notation, and mixed matrix/vector indexed contractions. M38 aligns compiled `@` with the supported indexed contractions: matrix-matrix (`A @ B`), matrix-vector (`A @ x`), vector-matrix (`x @ A`), and vector-vector dot product (`x @ y`). M40 adds compiled matrix/matrix scalar double contractions (`A[i, j] * B[i, j]` and `A[i, j] * B[j, i]`). Still deferred in compiled mode: trace-style `A[i, i]`, arbitrary rank-N tensors, broadcasting, index variance, raising/lowering, and Prometheus/reactor/GPU tensor kernels.


## M34 Mechanics matrix/scalar compiled cleanup

Compiled mode now supports the ordinary element-wise matrix arithmetic already accepted by interpreted mode and needed by `Libraries/Mechanics`: matrix-matrix `+`, `-`, `*`, `/`, matrix-scalar `+`, `-`, `*`, `/`, and scalar-matrix `+`, `-`, `*`, `/`. Generated helpers validate rectangular matrices and matrix-matrix shape compatibility, return new matrices, and preserve mixed `Int`/`Float` result typing through explicit helper type arguments.

`Libraries/Mechanics` is compiled-supported after M34: the Mechanics suite runs with compiled execution only and no interpreted fallbacks. Mechanics/Continuum stress composition such as `(lambda * vol) * identity + (2.0 * mu) * strain` now lowers through matrix/scalar helpers instead of invalid raw Go operators.

M34 did not expand Einstein notation beyond the rank-2 matrix support described in M33. Vector symbolic indexing, rank-N tensors, scalar/double contractions, trace sugar, broadcasting, variance, raising/lowering, and Prometheus/reactor/GPU tensor kernels remained deferred. The `@` operator behavior was unchanged.


## M36 interpreted vector rank-1 Einstein support

Interpreted mode now supports `Vector[Index]` as a rank-1 indexed tensor term while preserving `Vector[Int]` as concrete element access. Arrays remain storage values only and are not tensor-indexable.

The interpreted M36 surface supports vector indexed addition/subtraction, vector dot product (`a[i] * b[i]`), vector outer product (`a[i] * b[j]`), matrix-vector indexed contraction (`A[i, j] * x[j]`), and vector-matrix indexed contraction (`x[i] * A[i, j]`). Existing rank-2 matrix indexed `*`, `+`, and `-` support is unchanged.

Compiled support was not expanded in M36: vector rank-1 indexed terms and mixed vector/matrix indexed contractions were deferred to M37. `@` behavior was unchanged. Rank-N tensors, trace-style `A[i, i]`, broadcasting, variance, raising/lowering, Prometheus/reactor/GPU tensor kernels, and rank-2 matrix scalar double contractions remained deferred.

## M37 compiled vector rank-1 Einstein support

Compiled mode now supports `Vector[Index]` as a rank-1 indexed tensor term while preserving `Vector[Int]` as concrete element access. Arrays remain storage values only and are not tensor-indexable.

The interpreted and compiled M37 surface now has parity for vector indexed addition/subtraction (`a[i] + b[i]`, `a[i] - b[i]`), vector dot product (`a[i] * b[i]`), vector outer product (`a[i] * b[j]`), matrix-vector indexed contraction (`A[i, j] * x[j]`), and vector-matrix indexed contraction (`x[i] * A[i, j]`). Existing M33 rank-2 matrix indexed `*`, `+`, and `-` support is unchanged.

M37 does not change `@` behavior and does not add `x @ A` or `x @ y`. Rank-N tensors, trace-style `A[i, i]`, broadcasting, variance, raising/lowering, Prometheus/reactor/GPU tensor kernels, and rank-2 matrix scalar double contractions remain deferred.


## M38 `@` tensor contraction alignment

Compiled and interpreted `@` now cover the same common rank-1/rank-2 contraction shorthand described in the vector/matrix reference:

- `A @ B` remains matrix-matrix contraction, equivalent to `A[i, k] * B[k, j]`.
- `A @ x` remains matrix-vector contraction, equivalent to `A[i, j] * x[j]`.
- `x @ A` is now vector-matrix contraction, equivalent to `x[i] * A[i, j]`.
- `x @ y` is now vector-vector dot product, equivalent to `x[i] * y[i]`.

Generated compiled helpers validate vector lengths, matrix row/column compatibility, and rectangular matrix shape in the same non-broadcasting style as existing matrix multiplication helpers. Arrays remain unsupported for `@`, and `*` remains element-wise outside indexed tensor notation. Rank-N tensors, matrix/matrix scalar double contractions, trace sugar, covariant/contravariant variance, raising/lowering indices, broadcasting, and Prometheus/reactor/GPU tensor kernels remained deferred after M38.

## M40 matrix/matrix scalar double contractions

Interpreted and compiled indexed tensor notation now support rank-2 matrix/matrix multiplication expressions whose labels are all contracted and whose result is scalar:

- `A[i, j] * B[i, j]` is the Frobenius-style matrix inner product.
- `A[i, j] * B[j, i]` is a label-aware matrix/matrix scalar double contraction.

The compiled helper validates non-empty labels, rectangular matrix values, repeated label counts, zero free labels, and runtime label extent consistency using matrix slot extents (slot 0 rows, slot 1 columns). Trace-style indexed sugar `A[i, i]` remains unsupported; use `Trace(A)`. Rank-N tensors, arrays as tensors, broadcasting, covariant/contravariant variance, raising/lowering, `@` behavior changes, Prometheus/reactor/GPU tensor kernels, and package-manager federation remain deferred.

## Interpreted generic wrapper dispatch status (W7b)

Interpreted execution now has an M0 generic Octxiliary dispatch path for third-party-style wrapper packages. A public Oct function can call a package-local manifest raw function declared in `WrapperFunction.OctName` even when that raw function has no source-level `fn` declaration. Source functions and existing interpreted wrapper builtins still take precedence, so this is a fallback for manifest-only raw names rather than a stdlib wrapper migration.

W7b interpreted discovery matches the focused test scope: sidecars are found beside the current `oct` executable or via `OCT_WRAPPER_PATH` as either a directory containing the sidecar command or an explicit executable path whose basename equals the sidecar command. It does not add PATH lookup, package-cache lookup, sidecar build/install lifecycle, lockfiles, native permission prompts, registry/federation/P2P, `@extern`, or `EXTERNAL` syntax. `oct pkg wrappers` remains planning-only and does not execute sidecars.
