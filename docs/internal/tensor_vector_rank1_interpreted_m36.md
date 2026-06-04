# M36 — interpreted vector rank-1 indexed terms

M36 implements interpreted/typechecked support for rank-1 vector indexed tensor terms and mixed vector/matrix Einstein operations. The work follows the M35 design audit and keeps compiled parity deferred.

## Baseline reproduced

Before the M36 implementation, `v[i]` where `v: Vector<T>` and `i: Index` was rejected as concrete vector indexing with the diagnostic:

```text
vector indexing index must be Int, got Index
```

The pre-change rank-2 tensor suites were reproduced in interpreted and compiled mode:

```sh
go run ./cmd/oct test Language/Expressions/TensorEinsteinM0/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/TensorEinsteinM1/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/TensorEinsteinM3/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/TensorEinsteinM4/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/TensorEinsteinM0/valid --execution compiled
go run ./cmd/oct test Language/Expressions/TensorEinsteinM1/valid --execution compiled
go run ./cmd/oct test Language/Expressions/TensorEinsteinM3/valid --execution compiled
go run ./cmd/oct test Language/Expressions/TensorEinsteinM4/valid --execution compiled
```

All listed rank-2 valid suites passed.

## Supported after M36

Interpreted mode supports:

- `v[n]` where `n: Int` as ordinary concrete vector element access.
- `v[i]` where `i: Index` as a rank-1 indexed vector term.
- `a[i] + b[i] -> Vector<T>`.
- `a[i] - b[i] -> Vector<T>`.
- `a[i] * b[i] -> T` dot product.
- `a[i] * b[j] -> Matrix<T>` outer product.
- `A[i, j] * x[j] -> Vector<T>` matrix-vector indexed contraction.
- `x[i] * A[i, j] -> Vector<T>` vector-matrix indexed contraction.
- Existing rank-2 matrix indexed `*`, `+`, and `-` behavior.

Typechecking now tracks indexed term metadata as element scalar type, rank, ordered labels, and label availability. Matrix indexed terms carry rank 2 and vector indexed terms carry rank 1.

Interpreter indexed terms now carry the underlying value, rank, and ordered labels. Assigned/materialized results are ordinary scalar/vector/matrix values; explicit reindexing reintroduces labels.

## Still deferred

M36 intentionally does not add compiled lowering for vector rank-1 indexed terms. Compiled vector-rank support is M37 scope.

Also deferred/unchanged:

- `@` behavior is unchanged.
- Arrays are not tensor-indexable.
- `Float[]` does not implicitly act like `Vector<Float>`.
- Arbitrary rank-N tensors and rank-3/rank-4 storage.
- Trace-style `A[i, i]` indexed sugar.
- Rank-2 matrix scalar double contractions such as `A[i, j] * B[j, i]` and `A[i, j] * B[i, j]`.
- Covariant/contravariant variance, raising/lowering, broadcasting, Prometheus/reactor/GPU tensor kernels.

## Tests

M36 adds `Language/Expressions/TensorEinsteinM5` interpreted contracts for valid vector-rank behavior and invalid/deferred cases. The valid suite covers concrete vector indexing, vector addition/subtraction, dot product, outer product, matrix-vector contraction, vector-matrix contraction, reindexed vector intermediates, and a matrix-matrix regression. The invalid suite covers arrays, wrong vector-index arity/type, mismatched free-index order, rank mismatch, rank-N output rejection, trace rejection, and deferred matrix double contractions.
