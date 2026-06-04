# M33 — compiled parity for rank-2 matrix Einstein expressions

## Baseline from M31/M32

M31 identified a real interpreted tensor surface and a compiled parity gap. Interpreted mode supported:

- `Idx("name") -> Index` with non-empty/whitespace validation.
- Explicit `EinMul(A, i, k, B, k, j)` and `EinAdd(A, i, j, B, i, j)`.
- Rank-2 matrix indexed terms `A[i, j]` where both labels are `Index` values.
- Infix indexed matrix `*` and `+`, including nested expression trees and reindexed intermediate matrices.

M32 merged that contract into the vector/matrix/tensor reference and added interpreted rank-2 indexed subtraction (`A[i, j] - B[i, j]`). The pre-M33 compiled baseline still failed before reaching indexed-expression lowering because the compiler rejected `Idx` with `compiled mode does not yet support builtin Idx`; explicit `EinMul`/`EinAdd` and infix `[Index, Index]` lowering were consequently unreachable in compiled tensor suites.

The valid interpreted tensor suites contained these green baseline counts:

- `Language/Expressions/TensorEinsteinM0/valid`: 3 tests.
- `Language/Expressions/TensorEinsteinM1/valid`: 5 tests.
- `Language/Expressions/TensorEinsteinM3/valid`: 2 tests.
- `Language/Expressions/TensorEinsteinM4/valid`: 4 tests.

## Compiled support added in M33

M33 adds the narrow compiled representation and lowering needed for existing rank-2 matrix Einstein notation:

- Oct `Index` lowers to Go `string`.
- `Idx(label)` lowers to `__octIdx(label)`, preserving interpreted validation that trimmed-empty labels are invalid.
- Concrete matrix indexing remains `Matrix<Int, Int>` element access.
- Matrix indexing with two `Index` operands now produces compiler-side indexed-term metadata instead of concrete element access.
- Infix indexed matrix `*`, `+`, and `-` lower to shared generated Go helpers:
  - `__octEinMulMM`
  - `__octEinAddMM`
  - `__octEinSubMM`
- Nested expression labels are preserved inside one expression tree, so expressions such as `(A[i, k] * B[k, m]) * (C[m, j] + D[m, j])` compile without materializing label metadata into user-visible values.
- Assigned intermediates remain ordinary matrices; reindexing them (`partial[i, j]`) reintroduces labels, matching interpreted behavior.
- Explicit public `EinMul` and `EinAdd` compile through the same helpers. The compiler also contains an `EinSub` helper path, but `EinSub` is not part of the public typechecked builtin surface unless the language layer exposes it later.

The generated helpers intentionally mirror interpreted M33 scope rather than specializing for performance. They validate non-empty labels, rectangular matrices, consistent label extents, matching shape/order for addition/subtraction, and exactly two free labels for multiplication.

## Supported compiled tensor scope after M33

Compiled mode supports the current rank-2 matrix indexed Einstein surface:

- `Idx("name") -> Index`.
- `A[i, j]` as an indexed matrix term when `i, j: Index`.
- `A[0, 1]` as concrete matrix element access when both indices are `Int`.
- `A[i, k] * B[k, j]` as rank-2 Einstein contraction.
- `A[i, j] + B[i, j]` as elementwise indexed matrix addition.
- `A[i, j] - B[i, j]` as elementwise indexed matrix subtraction.
- Nested rank-2 compositions and reindexed intermediate matrix workflows covered by `TensorEinsteinM1`, `TensorEinsteinM3`, and `TensorEinsteinM4`.

## Remaining deferred tensor features

M33 does not add any of the following:

- Vector rank-1 symbolic indexed terms (`v[i]` where `i: Index`).
- Dot products from `a[i] * b[i]` or outer products from `a[i] * b[j]`.
- Matrix-vector or vector-matrix indexed notation.
- Scalar/double-contraction results such as `A[i, j] * B[j, i]`.
- Trace-style indexed sugar `A[i, i]`; use `Trace(A)`.
- Arbitrary rank-N tensors.
- Broadcasting.
- Covariant/contravariant variance, raising, or lowering.
- New Prometheus/reactor/GPU tensor kernels.
- Any change to `@` behavior.

## Mechanics status after M33

The dedicated tensor suites now exercise compiled `Idx`, explicit Einstein builtins, infix rank-2 multiplication, addition, subtraction, nested expression metadata preservation, and reindexed intermediate workflows directly.

`Libraries/Mechanics` remains a broader compiled-convergence lane. After M33, interpreted Mechanics is green (65 passed). Compiled Mechanics improves past the old `Idx` blocker but still reports 58 passed and 7 failed. The remaining failures are not Einstein-label lowering failures: most are scalar-matrix generated-Go mismatches such as `invalid operation: _t4 * identity (mismatched types float64 and [][]float64)`, and one is an `[][]int` to `[][]float64` struct-literal mismatch. These should be handled as a separate scalar/matrix compiled-parity milestone rather than expanding M33 into full Mechanics parity.

## Recommended next milestone

The next tensor milestone should remain narrow. Recommended options:

1. Add compiled invalid-diagnostic harness coverage for shared tensor rejection cases if the existing `.octfail` runner does not distinguish execution lanes.
2. Audit Mechanics compiled blockers after M33 and isolate non-Einstein failures into a separate milestone.
3. Consider vector rank-1 indexed terms only after the vector/matrix/tensor reference defines their complete valid and invalid contract.
