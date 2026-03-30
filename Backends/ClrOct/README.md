# ClrOct (Scaffold)

ClrOct is an **escape hatch backend path** for Oct targeting CLR-oriented implementation exploration.

## Status

Scaffold only. No backend implementation logic is introduced here yet.

## Why This Exists

- Provide a clearly named location for CLR backend exploration.
- Make backend experimentation explicit without overloading GoOct internals.
- Preserve architectural clarity: GoOct remains the reference backend.

## What ClrOct Is Meant to Prove

- Feasibility of a CLR backend path for Oct.
- Practical backend portability while preserving one language contract corpus.
- Incremental backend bring-up strategy against existing Oct language tests.

## What ClrOct Is Not

- Not a replacement for GoOct in current milestones.
- Not a compiler architecture rewrite.
- Not a place to redefine language semantics.

## Bootstrap Strategy: Octest-First

ClrOct bootstrap is expected to be contract-driven:

1. Reuse `Language/` `.octest` and `.octfail` as canonical semantics.
2. Implement backend stages incrementally.
3. Validate backend progress by increasing language corpus pass coverage.

## Directory Intent

- `docs/` — ClrOct-specific planning and design notes.
- `src/` — future backend implementation source.
- `tests/` — backend-specific non-semantic integration checks.

Language semantics remain owned by `Language/`.
