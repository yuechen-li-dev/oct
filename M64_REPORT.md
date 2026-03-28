# M64b Follow-up — Remove Shim Path and Keep Compiled Pipeline Honest

## Summary

This follow-up removes the previously introduced shim/special-case flow build route so compiled mode no longer switches to a separate execution pipeline when flows are present.

Compiled builds now use a single compiler path only. Flow programs are rejected by normal MIR lowering until true MIRFlow integration lands.

## What changed in this follow-up

- Removed special-case flow detection/build redirection logic from `build.Compile`.
- Restored flow handling to normal MIR lowering diagnostics:
  - `compiled mode does not yet support Octomata flow/state runtime in compiled mode (M64)`
- Added regression coverage asserting flow+decision inputs fail in the normal compiled path (no shim/fallback route).

## Why this was added

The shim path violated architecture by creating an alternate compiled execution route. Removing it restores a single honest compiler pipeline and prevents interpreter-like fallback behavior under `oct build`.

## Deferred (still required for full M64)

- MIRFlow representation for flow machine lowering
- compiled flow instance construction + `Step(...)` execution path
- state dispatch and `goto`/`suspend` lowering
- ordered `when` lowering in flow states
- utility `when` lowering/state tracking (`hysteresis`, `min_commit`, per-site memory)
- resume slot runtime (`remember` / `resume`) and `ResumeTarget(...)` (still intentionally deferred)

## Notes

This follow-up is an architecture correction only: shim path removed, single pipeline restored. Decision support must be reintroduced by extending MIRFlow/backend directly (not via fallback execution).
