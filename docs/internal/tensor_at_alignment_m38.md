# M38 — align `@` with vector/matrix Einstein contractions

M38 aligns the ordinary `@` operator with the vector rank-1 and matrix rank-2 Einstein contractions that now have interpreted and compiled parity.

## Baseline before implementation

The pre-change `@` surface supported:

- `Matrix<T> @ Matrix<U> -> Matrix<V>` as matrix-matrix contraction.
- `Matrix<T> @ Vector<U> -> Vector<V>` as matrix-vector contraction.

The existing typechecker rejected `Vector @ Matrix`, `Vector @ Vector`, and arrays with diagnostics of the form:

```text
operator '@' not defined for <left> and <right>
```

That staged gap was intentional in M36/M37 while vector indexed terms reached interpreted and compiled parity.

## M38 support

The implemented `@` surface is now:

- `A @ B`, equivalent to `A[i, k] * B[k, j]`.
- `A @ x`, equivalent to `A[i, j] * x[j]`.
- `x @ A`, equivalent to `x[i] * A[i, j]`.
- `x @ y`, equivalent to `x[i] * y[i]`.

The typechecker reuses existing scalar `*` result rules for the contracted element type. Dimension-qualified values therefore propagate dimensions through the contraction result in the same way as indexed multiplication.

## Runtime and compiled lowering

Interpreted mode now evaluates:

- vector-matrix contraction with `out[j] = sum_i x[i] * A[i, j]` and vector length equal to matrix rows;
- vector-vector dot product with `sum_i x[i] * y[i]` and matching vector lengths.

Compiled mode lowers the new cases to generated helpers:

- `MatMulVM` for `Vector @ Matrix`;
- `VecDot` for `Vector @ Vector`.

Existing `MatMulMM` and `MatMulMV` lowering remains unchanged. The generated helpers preserve the current style of explicit runtime shape validation and no broadcasting.

## Tests

M38 adds `Language/Expressions/TensorEinsteinM6` contracts:

- valid equivalence tests comparing `@` against indexed notation for matrix-matrix, matrix-vector, vector-matrix, and vector-vector dot product;
- rectangular matrix/vector coverage for `A @ x` and `x @ A`;
- invalid compile-time coverage confirming arrays and scalar/vector operands remain rejected. Runtime vector-matrix and vector-vector shape diagnostics are implemented in the interpreter and generated compiled helpers; static shape-mismatch `.octfail` contracts are not expressible because vector/matrix lengths are not part of the current static type.

## Still deferred

M38 does not change element-wise `*` and does not add arrays, broadcasting, rank-N tensors, matrix/matrix scalar double contractions, trace-style `A[i, i]`, covariant/contravariant variance, raising/lowering indices, or Prometheus/reactor/GPU tensor kernels.
