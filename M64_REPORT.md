# M64 — Compiled Octomata Bring-up Status

## Summary

M64 is the milestone for compiled-mode Octomata support (`flow`, `state`, `goto`, `suspend`, ordered `when`, utility `when`, `remember`/`resume`, and flow observability/runtime operations).

This repository change does **not** land full M64 semantics yet. It adds explicit compiled-mode diagnostics so unsupported Octomata paths fail clearly and deterministically.

## What changed in this patch

- Compiled lowering now reports an explicit M64-oriented diagnostic when encountering flow declarations:
  - `compiled mode does not yet support Octomata flow/state runtime in compiled mode (M64)`
- Compiled call resolution now reports explicit diagnostics for Octomata runtime builtins:
  - `Step`
  - `Active`
  - `Result`
  - `Complete`
  - `StateHistory`
  - `ResumeTarget`

## Why this was added

Until full runtime-backed lowering is implemented, explicit diagnostics keep compiled-mode behavior honest and deterministic instead of surfacing generic “unknown function” failures.

## Deferred (still required for full M64)

- MIR representation for flow machine lowering
- compiled flow instance construction + `Step(...)` execution path
- state dispatch and `goto`/`suspend` lowering
- ordered `when` lowering in flow states
- resume slot runtime (`remember` / `resume`)
- compiled observability parity (`Active`, `Complete`, `Result`, `StateHistory`, `ResumeTarget`)
- utility `when` site memory + hysteresis/min-commit behavior in compiled mode

## Notes

This patch is an honesty/diagnostic tightening step only; it should be treated as preparatory work toward full M64 implementation.
