# Mechanics / Continuum M3 report

## What interop mechanism was added

A single narrow runtime interop path was added: `Trace(...)` now accepts representational tensor-like field outputs (`DiffOp` / `FieldOp`) and returns a representational `Trace(...)` differential-expression value instead of requiring materialized matrix elements.

No evaluation, discretization, solver, or broad symbolic algebra machinery was introduced.

## Does `Trace(...)` now work over representational tensor-like outputs?

Yes.

`Trace(SymGrad(u))` is now accepted and remains representational (non-evaluative).

`Trace(...)` still computes numerically for materialized matrices and still enforces square/non-empty checks in that materialized path.

## Are direct constitutive chains now possible?

Partially.

This pass unblocks representational invariant use directly in constitutive pipelines:

- `eps = SymGrad(u)`
- `trEps = Trace(eps)`
- `sigma = (2 * mu) * eps`

This is now first-class and honest.

However, one seam remains: constitutive outputs assembled as generic representational field expressions (`FieldOp`) are not yet accepted by `Div(...)`. So `Div(sigma)` is still blocked when `sigma` is a `FieldOp` rather than a direct matrix/differential-op shape.

## What remained awkward

- There is still no dedicated built-in constitutive helper such as `LinearIsotropicStress2D(...)`; users write the formula inline.
- Full isotropic split in one expression (`2μ ε + λ tr(ε) I`) cannot yet feed directly into `Div(...)` without another narrow interop step.
- `Trace(...)` interop is intentionally narrow; this pass does not add broad invariant/symbolic algebra.

## Is the continuum library sufficiently complete for current stage?

Close, but not fully complete for the intended small-strain constitutive-to-balance flow.

M3 closes the explicit `Trace(...)` representational seam, but one final helper/interoperability seam remains at `Div(FieldOp)`.

## Recommendation for next step

Do one final narrow continuum/helper cleanup: allow `Div(...)` to accept representational tensor-like `FieldOp` outputs under the same strict non-evaluative model.

After that, move to richer continuum formulations rather than substrate rescue.
