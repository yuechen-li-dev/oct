# Compiled Support Tracker

_Last updated: 2026-06-04._

This file is the **source of truth** for compiled support posture.


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

## Current deferred / partial categories

- Markdown wrapper-heavy paths are still largely interpreted/fallback territory.
- Artifact-lane compiled support remains partial (artifact workflows should assume interpreted execution unless explicitly verified).
- Flow expression calls still reject side-effectful wrappers and fallible calls in compiled mode.
- **Direct compiled (no sidecar):** `FileExists`, `PathJoin`, `PathBaseName`, `PathExtension`, `PathStem`, `PathParent`, `PathClean`.
- **Octxiliary sidecar-backed compiled (`... ! Error`):** `FileReadText`, `FileWriteText`, `FileReadLines`, `FileWriteLines`, `FileReadBytes`, `FileWriteBytes`, `FileDelete`, `DirectoryList`, `DirectoryMake`, `DirectoryMakeAll`, `DirectoryRemoveAll`.
- **Deferred wrapper builtins in this family:** none for the IO file/directory wrapper set listed above.
- Sidecar-backed compiled calls require Octxiliary availability (the required sidecar beside `.octbin` or discoverable through `OCT_WRAPPER_PATH`; Time wrapper calls require `octxiliary-time`).
- IO/Csv/Json wrapper breadth remains mixed and should be verified per-target.
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

| BoardSnapshot(machine) | Supported | Supported (M0 scalar board fields) | `OctomataBoardSnapshot`; `Experiments.FmBrownNoiseKalman.M3.FlowSmoke` | Compiled support is currently limited to detached/read-only snapshots of scalar board fields (`Bool`, `Int`, `Float`, `String`). Board arrays remain unsupported in compiled M0. |

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
- `Json`: `Save` and `Load` through `octxiliary-json` using `String` and `Int` transports; `Object` remains direct pure Oct.

Deferred after the M11 sweep:

- `Csv` remains blocked by `String[][]` row data (`needs_nested_array_transport`).
- `Markdown` remains blocked by record-of-`String[]` tables and nested block arrays (`needs_record_transport`, `needs_nested_array_transport`).
- `Pdf` and `Image` remain blocked by opaque handles and record arguments (`needs_handle_transport`, `needs_record_transport`).
- `Plot` remains blocked by `Float[]` plot data and record arguments (`needs_float_array_transport`, `needs_record_transport`).

M11 did not add new transport kinds, compiled Complex support, Einstein notation, record transport, handle transport, package-manager sidecar builds, lockfiles, or public API redesigns.

## M13 Octxiliary Csv row-major status

M13 extends generic Octxiliary lowering with exactly `String[][]` transport (`[][]string` in Go; wire kind `"String[][]"`). This unlocks compiled row-major CSV wrappers without adding general nested-array, record, handle, dynamic, numeric-array, Complex, or Einstein support.

Compiled support now includes:

- `Libraries/Csv.Read(path: String) -> String[][] ! Error`
- `Libraries/Csv.Write(path: String, rows: String[][]) -> Int ! Error`
- focused `Libraries/IO` row-major `Read`/`Write` aliases when `octxiliary-csv` is available

Raw CSV row reads preserve ragged rows and exact parsed string cells. CSV writes emit exactly the supplied row-major string data through Go's standard `encoding/csv` writer. The `octxiliary-csv` sidecar must be discoverable beside the `.octbin` or through `OCT_WRAPPER_PATH`.

Still unsupported/deferred: Markdown structured table transports, Plot numeric arrays, Pdf/Image/XLSX handle-backed workflows, structured JSON graph helpers, record transport, handle transport, dynamic `Any`, and broad nested-array generalization.

## M16 Octxiliary Plot status

M16 adds generic `Float[]` transport and manifest-declared, non-recursive record argument transport. This migrates `Libraries/Plot.Line`, `Libraries/Plot.Scatter`, and `Libraries/Plot.Histogram` to compiled wrapper lowering through `cmd/octxiliary-plot` while preserving the public Oct signatures.

`Plot.Size` and `Plot.Labels` are declared as wrapper `TransportTypes`; compiled code packs those generated Go record structs into ordered Octxiliary record arguments. Dimensioned `Int<px>` size fields are dimension-erased over the wire as `Int` payloads. `DefaultSize` and `DefaultLabels` remain pure/local and do not call the sidecar.

Still unsupported/deferred after M19: general record returns, nested or recursive records, cross-family handles, handle close/destructor semantics, handle serialization across runs, dynamic `Any`, maps, `Float[][]`, broad `Int[]`, Pdf migration, structured JSON graph helpers, Markdown-as-Octxiliary, package-manager sidecar builds, native permission prompts, and lockfiles. The `octxiliary-plot`, `octxiliary-xlsx`, and `octxiliary-image` executables must be beside the compiled `.octbin` or on `OCT_WRAPPER_PATH`. M18 adds handle transport M0 and migrates `IO.Xlsx`; M19 reuses that handle path for Image; handles are sidecar-owned capabilities, not plain `Int` values.

## M19 Octxiliary Image status

M19 migrates `Libraries/Image` to compiled generic wrapper lowering through `cmd/octxiliary-image` and handle transport. The public API remains `Load(path: String) -> ImageHandle ! Error`, `Save(image: ImageHandle, path: String) -> Int ! Error`, `Width(image: ImageHandle) -> Int<px>`, `Height(image: ImageHandle) -> Int<px>`, and `Format(image: ImageHandle) -> String`.

`Image.ImageHandle` is a sidecar-owned, process-lifetime handle capability. The public record still has a single `Handle: Int` field, but compiled transport carries family `Image`, handle type `Image.ImageHandle`, and a sidecar-local positive ID. Handles are not serializable across program runs and are not shared with `Pdf` or any other family. There is still no `Close`/destructor API.

`Width`, `Height`, and `Format` remain non-fallible. If an invalid/stale Image handle reaches one of those operations in compiled mode, the sidecar error becomes a runtime failure through the generic non-fallible wrapper path. `Load` and `Save` remain fallible and report missing files, corrupt images, unsupported save extensions, and invalid handles as `Error` values. The `octxiliary-image` executable must be beside the compiled `.octbin` or available through `OCT_WRAPPER_PATH`.

## M21 Pdf compiled subset

`Libraries/Pdf` now has focused compiled support for the text/page/save subset through `octxiliary-pdf`:

- `Pdf.NewPage(width: Int<px>, height: Int<px>) -> Pdf.PdfPage ! Error`
- `Pdf.DrawText(page: Pdf.PdfPage, x: Int<px>, y: Int<px>, text: String) -> Int ! Error`
- `Pdf.DrawTextStyled(page: Pdf.PdfPage, x: Int<px>, y: Int<px>, text: String, style: Pdf.TextStyle) -> Int ! Error`
- `Pdf.Save(page: Pdf.PdfPage, path: String) -> Int ! Error`

`Pdf.PdfPage` is a sidecar-owned handle and `Pdf.TextStyle` is a record argument transport. `Pdf.DefaultTextStyle()` remains pure/local Oct.

Deferred in compiled mode: `Pdf.DrawImage`, `Pdf.DrawImageSized`, and compiled `Pdf.ImageHandle` support. `octxiliary-pdf` does not consume `Image.ImageHandle` values from `octxiliary-image`; cross-family handle sharing remains unsupported. Put `octxiliary-pdf` beside the compiled `.octbin` or set `OCT_WRAPPER_PATH` to a directory containing it.
