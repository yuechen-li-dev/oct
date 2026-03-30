# LinearAlgebra M0b

## Representation

LinearAlgebra M0b uses native Oct arrays only:

- Vector: `Float[]`
- Matrix: flattened row-major `Float[]` with explicit `(rows, cols)` parameters

Index convention: `A[row * cols + col]`.

> Note: nested array types (`Float[][]`) are currently not supported by the Oct parser, so M0 uses flat row-major arrays while keeping matrix data transparent and first-principles.

## Surface

- `Dot(a: Float[], b: Float[]) -> Float ! Error`
- `Norm(a: Float[]) -> Float ! Error`
- `MatMul(A: Float[], aRows: Int, aCols: Int, B: Float[], bRows: Int, bCols: Int) -> Float[] ! Error`
- `MatVecMul(A: Float[], rows: Int, cols: Int, x: Float[]) -> Float[] ! Error`
- `VecMatMul(x: Float[], A: Float[], rows: Int, cols: Int) -> Float[] ! Error`
- `Transpose(A: Float[], rows: Int, cols: Int) -> Float[] ! Error`
- `Trace(A: Float[], rows: Int, cols: Int) -> Float ! Error`
- `Diagonal(A: Float[], rows: Int, cols: Int) -> Float[] ! Error`
- `Identity(n: Int) -> Float[] ! Error`
- `Zeros(m: Int, n: Int) -> Float[] ! Error`
- `Ones(m: Int, n: Int) -> Float[] ! Error`
- `Add(A: Float[], B: Float[], rows: Int, cols: Int) -> Float[] ! Error`
- `Sub(A: Float[], B: Float[], rows: Int, cols: Int) -> Float[] ! Error`
- `Scale(A: Float[], rows: Int, cols: Int, k: Float) -> Float[] ! Error`
- `MatScalarMul(A: Float[], rows: Int, cols: Int, s: Float) -> Float[] ! Error`
- `Determinant(a: Float[], rows: Int, cols: Int) -> Float ! Error`
- `SolveLinearSystem(a: Float[], rows: Int, cols: Int, b: Float[]) -> Float[] ! Error`
- `Inverse(a: Float[], rows: Int, cols: Int) -> Float[] ! Error`

### Usage pattern reminder

All matrix operations require explicit shape passing (`rows`, `cols`) and use flat row-major storage.
No function reshapes data, infers dimensions, or silently truncates mismatched inputs.

## Shape rules

- Vectors must be non-empty.
- Matrices require `rows > 0`, `cols > 0`, and `Len(data) == rows * cols`.
- `Dot` requires equal vector lengths.
- `MatMul` requires `aCols == bRows`.
- `MatVecMul` requires `Len(x) == cols`.
- `VecMatMul` requires `Len(x) == rows`.
- `Trace` requires `rows == cols`.
- `Diagonal` returns `min(rows, cols)` elements.
- `Add` and `Sub` require both inputs to satisfy the same `(rows, cols)`.
- `Identity` requires `n > 0`.
- `Zeros` and `Ones` require `m > 0` and `n > 0`.
- `MatScalarMul` validates `(rows, cols)` against data length before scaling.
- `Determinant`, `SolveLinearSystem`, and `Inverse` require square matrices.
- `SolveLinearSystem` requires `Len(b) == rows`.

## Edge-case policy

- Empty vectors are rejected.
- Empty matrix data is rejected.
- Zero dimensions are rejected.
- `1×1` vectors/matrices are valid.
- Singular or effectively singular square systems are rejected for solving/inversion.

### Singularity threshold (M0b)

`Determinant`, `SolveLinearSystem`, and `Inverse` use deterministic partial pivoting with a fixed pivot threshold:

- `epsilon = 1e-12`
- if the selected pivot magnitude is `<= epsilon`, the system is treated as singular

No pseudoinverse fallback or partial solution is attempted.

All functions are deterministic and implemented with explicit loops and validation.

## Non-goals (still out of scope)

M0b intentionally keeps scope dense/square/core and does **not** include decomposition APIs or advanced numerical methods:

- LU / QR / SVD / eigensolvers
- least squares / pseudoinverse
- sparse formats or alternate storage layouts
- condition estimation or advanced stability tooling
