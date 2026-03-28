# M64a — Compiled Octomata Core

## Summary

Compiled mode now supports the Octomata core machine path:

- `flow` declarations and flow instantiation
- `state` execution with `goto`, `suspend`, and `return`
- runtime builtins: `Step`, `Active`, `Result`, `Complete`, `StateHistory`

This implementation is intentionally runtime-backed and explicit, preserving the interpreted core stepping model.

## Lowering/runtime shape

- MIR now carries explicit flow declarations (`MIRFlow`) so MIR dumps expose flow machine shape and state ordering.
- Go backend emits one runtime struct per flow with explicit machine fields:
  - started/completed flags
  - current state id
  - instruction index
  - result storage
  - deterministic state history
- Backend emits per-flow step dispatch (`__octStep`) that executes until:
  - `suspend`
  - `return`
  - invariant panic on malformed machine state

## Semantics now preserved in compiled mode

- flow call returns a flow instance, not final result
- `Step` resumes machine execution
- `goto` transitions immediately
- `suspend` pauses and preserves state/instruction position
- `return` completes flow and stores result
- `Step` on completed flow is a no-op
- `Active` is `""` before first step and after completion
- `Result` returns error before completion
- `StateHistory` records entry state on first step and each `goto` target

## Still intentionally deferred

- ordered `when`
- utility `when`
- `remember`
- `resume`
- `ResumeTarget(...)`

These remain explicit compiled-mode errors in this milestone.
