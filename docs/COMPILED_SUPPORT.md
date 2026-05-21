# Compiled Support Tracker

_Last updated: 2026-05-21._

This file is the **source of truth** for compiled support posture.

## Current green surfaces

- Core compiled harness: `oct test --execution compiled|auto|interpreted` control-plane is available.
- Explicit selected-file compiled test targeting is working for focused fixtures.
- String compiled surface used by `Libraries/String` is green (no fallback in compiled runs on the current verification set).
- Core assertion fixture lane used by compiled test harness is green.
- Pure-builtin compiled sweep fixture (`Language/Testing/CompiledBuiltinSweep/valid`) is green in current verification.

## Current deferred / partial categories

- Markdown wrapper-heavy paths are still largely interpreted/fallback territory.
- Artifact-lane compiled support remains partial (artifact workflows should assume interpreted execution unless explicitly verified).
- IO/Csv/Json wrapper breadth remains mixed and should be verified per-target.
- Some experiment packages still hit compiled-only blocker combinations (wrapper reachability, generated-Go mismatch, or timeout fallback pressure in auto).

## Recent sweep summary

- Recent pure-builtin compiled sweep confirms core obvious pure builtins in the sweep fixture are lowering successfully.
- String-focused compiled sweep and library lane remain green on current repo measurements.

## Selected-file harness status

- Selected-file compiled mode is repaired and currently usable for isolating a single `.octest` file in package context.
- Sibling files/imports still load/typecheck per normal package rules.

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
