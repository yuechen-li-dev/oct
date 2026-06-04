# M37 — compiled parity for vector rank-1 Einstein terms

M37 adds compiled lowering for the vector rank-1 Einstein surface that M36 introduced in interpreted mode. The real compiled path now treats `Vector[Index]` as an indexed tensor term rather than concrete element access, while preserving `Vector[Int]` as ordinary vector element indexing.

## Baseline before M37

The interpreted M36 baseline passed:

```sh
go run ./cmd/oct test Language/Expressions/TensorEinsteinM5/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/TensorEinsteinM5/invalid --execution interpreted
```

Before this change, the compiled M5 valid suite intentionally failed with the M36 deferral diagnostic:

```text
FAIL MainValid.MixedMatrixVectorIndexedContractionsReturnVectors (vector_rank1_surface_m5.octest): compiled execution required: function MainValid.MixedMatrixVectorIndexedContractionsReturnVectors: compiled vector rank-1 indexed terms are deferred to M37
FAIL MainValid.ReindexedVectorIntermediateAndMatrixRegressionWork (vector_rank1_surface_m5.octest): compiled execution required: function MainValid.ReindexedVectorIntermediateAndMatrixRegressionWork: compiled vector rank-1 indexed terms are deferred to M37
FAIL MainValid.VectorIndexedAdditionAndSubtractionReturnVectors (vector_rank1_surface_m5.octest): compiled execution required: function MainValid.VectorIndexedAdditionAndSubtractionReturnVectors: compiled vector rank-1 indexed terms are deferred to M37
FAIL MainValid.VectorIndexedDotAndOuterProduct (vector_rank1_surface_m5.octest): compiled execution required: function MainValid.VectorIndexedDotAndOuterProduct: compiled vector rank-1 indexed terms are deferred to M37
```

## Compiled support added

Compiled mode now supports:

- `v[i]` where `v: Vector<T>` and `i: Index` as a rank-1 indexed tensor term.
- `v[n]` where `n: Int` as concrete vector element access, unchanged.
- `a[i] + b[i] -> Vector<T>`.
- `a[i] - b[i] -> Vector<T>`.
- `a[i] * b[i] -> T` dot product.
- `a[i] * b[j] -> Matrix<T>` outer product, with rows corresponding to the left free label and columns corresponding to the right free label.
- `A[i, j] * x[j] -> Vector<T>` matrix-vector indexed contraction.
- `x[i] * A[i, j] -> Vector<T>` vector-matrix indexed contraction.
- Nested expression metadata preservation for rank-1 and rank-2 intermediates within one compiled expression tree, such as `(A[i, j] * x[j]) + b[i]`.

M33 rank-2 matrix Einstein compiled behavior is unchanged for `A[i, k] * B[k, j]`, `A[i, j] + B[i, j]`, `A[i, j] - B[i, j]`, and explicit `EinMul` / `EinAdd` / `EinSub` rank-2 helpers.

## Lowering model

Compiled Einstein metadata is rank-aware:

```go
type einsteinTermMeta struct {
    Labels []string
    Rank   int
    Type   string
}
```

`Matrix[Index, Index]` produces rank-2 metadata and `Vector[Index]` produces rank-1 metadata. `Vector[Int]` remains a scalar element expression and carries no Einstein metadata. Indexed tensor terms are metadata-only until consumed by an indexed binary expression; lowering never attempts to use an index label string as a concrete Go slice index.

Generated helpers validate non-empty labels, shape compatibility, and rectangular matrices for mixed matrix/vector contraction helpers. Helpers allocate fresh results and do not mutate inputs.

## Still deferred or unchanged

M37 intentionally does not change `@` behavior. There is still no `x @ A` or `x @ y` support from this milestone.

The following remain deferred or rejected:

- arrays as tensor-indexable values (`Array[Index]` remains invalid);
- `Float[]` or other arrays implicitly acting as `Vector<Float>`;
- rank-N tensor outputs such as `A[i, j] * x[k]`;
- rank-3/rank-4 storage;
- trace-style indexed sugar such as `A[i, i]`;
- matrix/matrix scalar double contractions such as `A[i, j] * B[j, i]` and `A[i, j] * B[i, j]`;
- covariant/contravariant variance;
- raising/lowering indices;
- broadcasting;
- Prometheus/reactor/GPU tensor kernels.

## Verification

The M5 valid and invalid suites now pass in both interpreted and compiled mode. M0/M1/M3/M4 tensor suites remain passing in both modes, preserving the earlier rank-2 matrix behavior while adding vector rank-1 compiled parity.
