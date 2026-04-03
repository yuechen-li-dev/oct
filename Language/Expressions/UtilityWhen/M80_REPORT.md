# Language Milestone M80 — standalone `when utility`

## Why this pass exists

M9 exposed a language-shape gap: one-shot local ranked-choice decisions were forced to use `flow/state` plus `when policy` policy fields, even when there was no controller lifecycle and no temporal memory requirement.

## What changed

- Added standalone `when utility` as an expression form.
- `when utility` is valid anywhere expressions are valid.
- Standalone policy fields are optional:
  - omitted `hysteresis` defaults to `0`
  - omitted `min_commit` defaults to `0`
- Explicit policy fields remain accepted on `when utility` for readability and future-proofing.

## What did not change

`when policy` remains exactly the Octomata/HSFM control form:

- still only valid inside `flow/state`
- still requires explicit `hysteresis`
- still requires explicit `min_commit`
- still provides controller-bound hysteresis/min-commit memory behavior

No control-language redesign was introduced.

## Determinism

Standalone `when utility` keeps deterministic ranked-choice behavior:

- highest score wins among valid cases
- tie break remains source-order stable
- else arm is used when no candidate is valid

## M9-style motivation outcome

The local junction ownership shape is materially cleaner now.

Before: one-shot ownership probes had to create synthetic `flow/state` ceremony just to use ranked choice.

Now: one-shot ownership can be written directly as an expression with `when utility`, while true controller arbitration remains on `when policy`.
