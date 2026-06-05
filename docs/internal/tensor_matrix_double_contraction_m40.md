# M40 — matrix/matrix scalar double contractions

M40 closes the smallest remaining high-value rank-1/rank-2 tensor gap after the M39 tensor ecosystem audit: rank-2 matrix/matrix indexed multiplication expressions that contract all labels and return a scalar.

## Baseline before M40

The pre-M40 checker and runtime rejected matrix/matrix scalar double contractions with the deferred diagnostic:

```text
matrix/matrix scalar double contractions are deferred in M36
```

The representative deferred forms were:

```oct
let s1 = A[i, j] * B[i, j]
let s2 = A[i, j] * B[j, i]
```

Existing interpreted and compiled tensor suites for M0, M1, M3, M4, M5, and M6 were used as the baseline corpus before enabling the new scalar result path.

## Implemented scope

M40 supports these scalar-producing rank-2 matrix/matrix contractions in interpreted and compiled execution:

```oct
A[i, j] * B[i, j]
A[i, j] * B[j, i]
```

The typechecker now accepts result rank 0 when both operands are rank-2 indexed matrix terms, every label appears exactly twice across the two operands, there are zero free labels, and the scalar element multiplication/addition type is valid. Scalar results do not carry Einstein metadata after materialization.

## Runtime extent rules

Label extents are derived from matrix slots:

- slot 0 is rows;
- slot 1 is columns.

Therefore:

- `A[i, j] * B[i, j]` requires `A.rows == B.rows` for `i` and `A.cols == B.cols` for `j`;
- `A[i, j] * B[j, i]` requires `A.rows == B.cols` for `i` and `A.cols == B.rows` for `j`.

The generalized label extent map enforces these rules and supports rectangular compatible matrices.

## Compiled parity

Compiled lowering routes rank-2/rank-2 indexed multiplication with zero free labels to a generated label-aware helper. The helper validates non-empty labels, rectangular matrices, label extent consistency, label counts, and zero free labels before accumulating the scalar result.

Existing compiled vector dot product, vector outer product, matrix-vector/vector-matrix contractions, rank-2 matrix multiplication, indexed add/sub, and `@` behavior are unchanged.

## Still deferred

M40 intentionally does not add:

- trace-style indexed sugar `A[i, i]` (use `Trace(A)`);
- rank-N tensor values, storage, constructors, literals, or shape APIs;
- arrays as tensor-indexable values;
- implicit `Float[]` to `Vector<Float>` tensor behavior;
- broadcasting;
- covariant/contravariant variance;
- raising/lowering indices;
- Prometheus/reactor/GPU tensor kernels;
- package-manager federation or P2P implementation;
- any `@` behavior changes.
