# Mechanics / Continuum M4 report

## What `Div(FieldOp)` interoperability was added

A single narrow runtime interop adjustment was added to `Div(...)`: it now accepts representational field-form operands that are already representational field values (`FieldOp` and `DiffOp`), not only materialized vectors/matrices.

This remains strictly representational and non-evaluative: `Div(...)` still returns a representational differential-expression value and does not perform numerical derivative evaluation.

No discretization, solver, weak-form, or broad symbolic-operator rewrite machinery was introduced.

## What direct field-form expressions are now possible

The previously awkward inline constitutive/strong-form chain can now be written directly and honestly, e.g.:

- `eps = SymGrad(u)`
- `sigma = (2 * mu) * eps` (representational `FieldOp`)
- `r = Div(sigma) + b`

and inline:

- `r = Div((2 * mu) * SymGrad(u)) + b`

These remain typed and representational.

## What invalid cases remain correctly rejected

M4 keeps strict rejection for invalid shapes/compositions, including:

- `Div(...)` on scalar constitutive expressions (`Float`) such as `Div(2 * Trace(eps))`
- balance/residual composition with incompatible term shapes (e.g. vector `Div(...)` result plus matrix body-force term)

The interop remains narrow: only already-representational field-form values are admitted, not arbitrary non-field operands.

## Required M4 questions answered

1. Was the `Div(FieldOp)` seam closed? **Yes**.
2. Can previously awkward inline constitutive/strong-form paths now be direct? **Yes** (`Div((2*mu)*SymGrad(u)) + b`).
3. Did the solution remain narrow and representational? **Yes**.
4. What invalid cases still reject? **Scalar `Div` operands and incompatible balance compositions remain rejected**.
5. Is the current small-strain continuum field-form substrate complete enough to move on? **Yes, for the current intended small-strain strong-form representational layer**.

## Recommendation for next step

Move on to richer continuum mechanics formulations (still representational/typed), rather than additional foundational `Div/Trace/helper` plumbing.
