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

## Current deferred / partial categories

- Markdown wrapper-heavy paths are still largely interpreted/fallback territory.
- Artifact-lane compiled support remains partial (artifact workflows should assume interpreted execution unless explicitly verified).
- Flow expression calls still reject side-effectful wrappers and fallible calls in compiled mode.
- `DirectoryMakeAll` remains wrapper/deferred for compiled mode, including flow-expression contexts.
- OctErgonomicsLab M1 now has an explicit suite split: `Experiments.OctErgonomicsLab.M1.FlowSmoke` is compiled-green, while `Experiments.OctErgonomicsLab.M1.Artifacts` remains interpreted fallback because `DirectoryMakeAll` is still deferred in compiled mode.
- IO/Csv/Json wrapper breadth remains mixed and should be verified per-target.
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

| BoardSnapshot(machine) | Supported | Not yet supported | `OctomataBoardSnapshot` (runtime only) | Interpreted support is available; compiled lowering not yet implemented in M0. |
