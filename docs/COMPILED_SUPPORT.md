# Compiled Support Tracker

_Last updated: 2026-05-22._

This file is the **source of truth** for compiled support posture.

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

The M4 hardcoded IO file/directory compiled helpers remain supported and coexist with the generic path. M7 migrates the first real standard-library generic wrapper package: `Libraries/Hash` now compiles `Sha256Text`, `Sha256Bytes`, and `Sha256File` through manifest metadata and `cmd/octxiliary-hash`. M8 migrates `Libraries/Compression` so `CompressBytes`, `DecompressBytes`, `CompressFile`, and `DecompressFile` compile through manifest metadata and `cmd/octxiliary-compression`; this is the proof for `Bytes -> Bytes` wrapper transforms. M9 migrates `Libraries/Time` so `NowIso8601`, `ParseIso8601`, `FormatIso8601`, `UnixSecondsNow`, and `FormatUnixSeconds` compile through manifest metadata and `cmd/octxiliary-time`; this proves host/time helper calls through the same generic path. The required sidecar executable (`octxiliary-hash`, `octxiliary-compression`, or `octxiliary-time`) must be available beside the compiled `.octbin` or discoverable through `OCT_WRAPPER_PATH`.

The remaining standard-library wrapper backlog identified in M5g is still future work: Archive, Plot, Pdf, Text/Regex, Image, CSV, JSON, XLSX, Markdown helpers, handle transports, record transports, and generated-Go numeric hardening are not completed by M9.

Current proof fixture:

- `Language/Testing/CompiledOctxiliary/valid/generic_wrapper_m6.octest`
- `cmd/octxiliary-test-wrapper`

The fixture covers generic `String`, `String[]`, `Bytes`, `Int`, `Float`, `Bool`, `Void` return, missing sidecar diagnostics, and fallible sidecar-error propagation without changing real standard-library wrapper surfaces.
