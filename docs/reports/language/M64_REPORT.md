# M64b — Compiled Ordered and Utility `when` in MIRFlow

## Summary

Compiled Octomata now supports:

- ordered `when` inside flow state bodies
- utility `when policy { hysteresis, min_commit }` inside flow state bodies

Both forms are integrated into the normal compiled pipeline:

- MIRFlow lowering
- backend emission
- generated compiled flow runtime

No shim/special-case flow compilation path is used.

## MIRFlow integration

- `MIRFlow` now contains lowered state statements (not raw AST-only state bodies).
- Ordered `when` is represented as explicit MIR flow decision statements/actions.
- Utility `when` is represented as explicit MIR flow expression nodes, including:
  - utility site identity (`site`)
  - candidate conditions/scores/values
  - policy expressions (`hysteresis`, `min_commit`)
- MIR dumps now show lowered ordered and utility decision structure directly.

## Runtime/backend shape

- Generated flow structs continue to model explicit machine state (`started`, `completed`, `currentState`, `instruction`, `history`, result).
- Utility support is implemented with narrow runtime helpers (`__octUtilSelect`) and per-flow-instance site state:
  - `HasCurrent`
  - `Current`
  - `Score`
  - `CommitAge`
- Utility site state is stored per flow instance and keyed per utility site id.

## Semantics preserved in compiled mode

- Ordered `when`:
  - source-order evaluation
  - first-true-wins
  - mandatory `else` behavior
- Utility `when`:
  - validity filtering
  - highest-score selection
  - deterministic source-order tie behavior
  - hysteresis and min-commit stability
  - invalid-current discard
  - per-site memory per flow instance

## Still intentionally deferred

- `remember`
- `resume`
- `ResumeTarget(...)`

These continue to fail clearly in compiled mode.
