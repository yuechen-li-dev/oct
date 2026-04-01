# Mathematics / Tensor Utilities M4 Report

## Decision

`Trace(...)` was added as an explicit, narrow mathematics-layer tensor utility.

## Exact semantics (implemented)

- Surface: `Trace(A)`
- Operand scope: exactly one argument, and it must be a matrix value (`Matrix<T>`).
- Result type: scalar `T` (same base numeric type and same dimension/unit as matrix elements).
- Runtime shape rule: matrix must be square (`rows == cols`), otherwise fail.
- Behavior: sum of diagonal entries `A[0,0] + A[1,1] + ... + A[n-1,n-1]`.

## What was deliberately excluded

- No inline trace contraction sugar (`A[i, i]`) was added; it remains rejected.
- No differential operators (`Grad`, `Div`, `Curl`, `SymGrad`).
- No rank-N tensor helpers.
- No mechanics-specific formulas or domain types.
- No utility-zoo expansion beyond `Trace(...)`.

## Additional helpers

No additional helper was added in M4.

Reason: for immediate early continuum-mechanics library authoring, explicit `Trace(...)` is the only clearly justified missing mathematics-layer utility exposed by M0–M3 constraints.

## Failure modes enforced

- Typecheck rejection when operand is not a matrix.
- Runtime rejection for non-square matrices.
- Existing rejection of inline trace-style indexed contraction (`A[i, i]`) preserved.

## Readiness finding

With explicit `Trace(...)` in place, the matrix-backed indexed tensor substrate is minimally sufficient for early continuum mechanics library work while preserving strict M3 boundaries.

## Recommendation

Proceed to early Mechanics/Continuum library work.

Do not broaden tensor utilities until concrete mechanics pressure demonstrates a specific missing primitive.
