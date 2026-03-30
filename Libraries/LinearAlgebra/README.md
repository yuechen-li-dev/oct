# LinearAlgebra M0

## Representation

LinearAlgebra M0 uses native Oct arrays only:

- Vector: `Float[]`
- Matrix: flattened row-major `Float[]` with explicit `(rows, cols)` parameters

Index convention: `A[row * cols + col]`.

> Note: nested array types (`Float[][]`) are currently not supported by the Oct parser, so M0 uses flat row-major arrays while keeping matrix data transparent and first-principles.

## Surface

- `Dot(a: Float[], b: Float[]) -> Float ! Error`
- `Norm(a: Float[]) -> Float ! Error`
- `MatMul(A: Float[], aRows: Int, aCols: Int, B: Float[], bRows: Int, bCols: Int) -> Float[] ! Error`
- `Transpose(A: Float[], rows: Int, cols: Int) -> Float[] ! Error`
- `Identity(n: Int) -> Float[] ! Error`
- `Zeros(m: Int, n: Int) -> Float[] ! Error`
- `Ones(m: Int, n: Int) -> Float[] ! Error`
- `Add(A: Float[], B: Float[], rows: Int, cols: Int) -> Float[] ! Error`
- `Sub(A: Float[], B: Float[], rows: Int, cols: Int) -> Float[] ! Error`
- `Scale(A: Float[], rows: Int, cols: Int, k: Float) -> Float[] ! Error`

## Shape rules

- Vectors must be non-empty.
- Matrices require `rows > 0`, `cols > 0`, and `Len(data) == rows * cols`.
- `Dot` requires equal vector lengths.
- `MatMul` requires `aCols == bRows`.
- `Add` and `Sub` require both inputs to satisfy the same `(rows, cols)`.
- `Identity` requires `n > 0`.
- `Zeros` and `Ones` require `m > 0` and `n > 0`.

## Edge-case policy

- Empty vectors are rejected.
- Empty matrix data is rejected.
- Zero dimensions are rejected.
- `1×1` vectors/matrices are valid.

All functions are deterministic and implemented with explicit loops and validation.
