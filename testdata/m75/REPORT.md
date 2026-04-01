# Tensor / Einstein Notation M3 Report

## Decision: trace-style contraction

`A[i, i]` is **rejected in M3**.

Reasoning:
- The current matrix-backed Einstein surface models rank-2 indexed terms that compose via `+` and `*`.
- Treating `A[i, i]` as scalar trace would introduce a rank-lowering behavior not otherwise present in this indexed layer.
- Keeping it out preserves strict free/bound index expectations and avoids silently changing operand categories.
- Users can continue to express trace via explicit library helpers (`Trace(...)`) while the indexed layer remains narrow.

## Deeper composition validated

Validated with contracts in `Language/Expressions/TensorEinsteinM3/valid`:
- Nested contraction chains with mixed product/sum subexpressions.
- Intermediate indexed-expression workflows that re-index matrix-backed intermediate values.
- Mechanics-adjacent skeleton flow (stiffness/strain-like naming) without differential operators.

## Diagnostics improved in this pass

- Added explicit rejection message for repeated index inside a single indexed matrix term:
  - `trace-style contraction '[i,i]' is not supported in M3; use Trace(...) for now`
- Existing strict diagnostics for multiplicity and free-index count were pressure-tested under deeper nesting and retained.

## Readiness verdict

- **Strictness under deeper nesting:** yes, retained.
- **Diagnostics near real usage:** yes, sufficient for this stage.
- **Matrix-backed model sufficiency:** yes, still sufficient for early mechanics library authoring.
- **Tensor layer readiness for early mechanics libraries:** **go**.

## Recommendation for next layer

Proceed to **early mechanics library work first** (constitutive/operator-free formulations).
After that, add differential operators (gradient/divergence/curl/sym-grad) as a separate pass.
