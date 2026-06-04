# M34 — Mechanics compiled matrix/scalar cleanup

## Baseline after M33

M33 made the existing rank-2 matrix Einstein surface compile: `Idx`, explicit `EinMul`/`EinAdd`, and rank-2 indexed infix `*`, `+`, and `-` lower to generated Go helpers. That moved `Libraries/Mechanics --execution compiled` past the previous unsupported-`Idx` blocker.

The remaining Mechanics compiled failures were not Einstein failures. The representative generated-Go issue was ordinary element-wise linear algebra lowering: Mechanics continuum stress helpers use expressions such as `(lambda * vol) * identity + (2.0 * mu) * strain`, and compiled mode was emitting raw Go operators over `[][]float64`, for example scalar `*` matrix and matrix `+` matrix, which Go rejects.

## Root causes found in M34

- Interpreted and typechecked Oct already accept element-wise vector/matrix arithmetic for matching containers and numeric scalar expansion for `+`, `-`, `*`, and `/`.
- Compiled lowering already had generated helpers for vector/vector, vector/scalar, scalar/vector, `@` matrix multiplication, and M33 Einstein matrix helpers.
- Compiled lowering did not route ordinary matrix/matrix or matrix/scalar element-wise operators through helpers, so it fell back to raw Go binary expressions.
- A second Mechanics case used a dimensioned integer matrix literal in a record field with expected type `Matrix<Float<kg/s^2>>`; compiled matrix literal lowering was not applying expected matrix element context, so generated Go attempted to assign `[][]int` to a `[][]float64` field.

## Implemented compiled support

M34 adds ordinary generated-Go helpers for matrix element-wise arithmetic accepted by the existing typechecker/interpreter:

- `Matrix<T> + Matrix<U>`
- `Matrix<T> - Matrix<U>`
- `Matrix<T> * Matrix<U>`
- `Matrix<T> / Matrix<U>`
- `Matrix<T> +|-|*|/ scalar`
- `scalar +|-|*|/ Matrix<T>`

The helpers validate rectangular inputs and matrix/matrix shape compatibility, do not mutate operands, and return new matrices. Mixed `Int`/`Float` cases use the same result element type selected by compiled scalar lowering, with generated helper type arguments converting elements/scalars only for the returned matrix.

M34 also applies expected matrix element type while lowering matrix literals, preserving the M28a expected-context behavior for record fields and other contexts that expect `Matrix<Float...>` from integer-valued literals.

## Mechanics results after M34

After M34, `go run ./cmd/oct test Libraries/Mechanics --execution compiled` passes with 65 compiled tests, 0 interpreted fallbacks, and 0 failures. `go run ./cmd/oct test Libraries/Mechanics --execution interpreted` still passes with the same 65-test suite.

The previously failing `Mechanics.InternalForceMatchesKnownMatrixCase` is reclassified as an expected-context matrix literal lowering issue rather than an Einstein issue. It is fixed by applying the expected matrix field element type while lowering the literal.

## Deferred tensor features

M34 does not expand Einstein notation beyond the M33 rank-2 matrix support. The following remain deferred:

- vector rank-1 symbolic indexed terms;
- dot products or outer products from indexed vector terms;
- arbitrary rank-N tensors;
- scalar/double-contraction tensor results;
- trace-style `A[i, i]` sugar;
- covariant/contravariant variance;
- raising/lowering indices;
- broadcasting;
- Prometheus/reactor/GPU tensor kernels;
- any `@` semantics changes.

## Recommended next milestone

The next milestone should stay outside Einstein expansion unless a new motivating library requires it. Recommended focus: a compiled numeric/container hardening pass that audits expected-context coercion for vector literals, matrix literals, records, and function returns, using focused generated-Go regression tests similar to M34.
