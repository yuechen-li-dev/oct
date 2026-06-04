# Vectors, Matrices, and Tensors

## Overview

Vectors and matrices are fixed-shape mathematical structures with dedicated literal, indexing, and arithmetic forms.
Vectors are rank-1 mathematical tensor values, and matrices are rank-2 mathematical tensor values.
Tensor notation is an index-aware expression surface over vectors, matrices, and tensor-like values; the current implemented source-level indexed surface is matrix-backed and rank-2.

Arrays and tensors are related but separate concepts.
Arrays are general ordered collection/storage values, while vectors and matrices are mathematical value categories.
`@` is Einstein contraction shorthand for the currently supported matrix contractions, distinct from element-wise operators and from arbitrary array multiplication.

## Arrays vs vectors, matrices, and tensors

- Arrays are general ordered collection/storage values.
- `Float[]` is a sequence of `Float` values.
- Arrays support collection-style operations and concrete integer indexing.
- Vectors and matrices are mathematical value categories.
- `Vector<T>` is a rank-1 mathematical tensor value.
- `Matrix<T>` is a rank-2 mathematical tensor value.
- Vectors and matrices may use storage representations internally, but their language semantics are linear-algebra/tensor semantics.
- `Float[]` is not automatically the same thing as `Vector<Float>`.
- `Float[][]` is not automatically the same thing as `Matrix<Float>`.
- `[...]` is always an array literal.
- Arrays (`T[]`, `T[][]`, ...) are generic containers; vectors/matrices are mathematical values.
- Arrays are not implicitly reinterpreted as vectors or matrices by expected type.
- `[[...], [...]]` is an array-of-arrays literal, not a matrix literal.

## Vector and matrix literals and constructors

- Vector literal form is `vector[a, b, c]`.
- Matrix literal form is `matrix[[r1c1, r1c2] [r2c1, r2c2]]`.
- Vector literals require homogeneous element type.
- Matrix rows must all have equal length.
- Vectors and matrices may use dimension-qualified numeric elements.

Use constructors when matrix values are generated, repetitive, or large:

- `Matrix.tabulate(rows, cols, Fn)` where `Fn` is `fn(r: Int, c: Int) -> T`.
- `Matrix.fill(rows, cols, value)` for constant matrices.
- `Matrix.zeros<T>(rows, cols)` for typed zero matrices.
- `Matrix.identity<T>(n)` for identity matrices.

Use literals (`matrix[[...]]`) for small hand-authored constants where the literal is clearer.
For benchmark/corpus-style setup, prefer constructors over giant literals.

## Concrete indexing

Concrete indexing uses `Int` indices.

- Vector element access is `v[0]`.
- Matrix element access is `A[0, 1]`.
- Vector indexing form is `v[i]` (exactly one `Int` index).
- Matrix indexing form is `m[r, c]` (exactly two `Int` indices).
- Matrix element assignment form is `m[r, c] = value` (exactly two `Int` indices, mutable bindings only).
- Matrix shape accessors are `m.rows` and `m.cols` (both `Int`, read-only).
- Vector element mutation through index assignment is not supported.

## Element-wise vector and matrix arithmetic

- `+`, `-`, `*`, `/` on vectors and matrices are element-wise operations.
- Element-wise vector operations require equal runtime lengths.
- Element-wise matrix operations require equal runtime shapes.

## Symbolic index values and matrix indexed terms

Oct has a narrow but real interpreted tensor-indexing surface.
Users create symbolic labels with `Idx("name")`, which returns `Index`.

- `A[i, j]` where `i, j: Index` creates a rank-2 indexed tensor term in interpreted mode.
- Mixed index forms such as `A[i, 0]` are invalid; matrix indexing expects either `[Int, Int]` concrete element access or `[Index, Index]` Einstein term access.
- `v[i]` where `i: Index` is future/unsupported in M32; vector rank-1 indexed terms are not implemented yet.
- `A[i, i]` trace-style sugar is intentionally unsupported; use `Trace(A)`.

Indexed tensor expressions preserve label structure while they are being composed.
A materialized indexed expression result is a `Matrix<T>` that can be re-indexed in a later expression.

## Einstein multiplication

`A[i, k] * B[k, j]` performs Einstein multiplication over rank-2 indexed matrix terms.
Repeated labels are contracted, and labels appearing once are free indices.

```oct
let i = Idx("i")
let j = Idx("j")
let k = Idx("k")

let a = matrix[[1, 2] [3, 4]]
let b = matrix[[5, 6] [7, 8]]
let c = a[i, k] * b[k, j]
```

Current constraints:

- Both operands must be indexed terms.
- A label appearing more than twice is invalid.
- The current rank-2 implementation requires exactly two free indices for multiplication results.
- Scalar, dot-product, trace-style, and double-contraction tensor results are not implemented by indexed source syntax in M32.

## Einstein addition

`A[i, j] + B[i, j]` performs elementwise indexed addition when free-index order matches exactly.

```oct
let i = Idx("i")
let j = Idx("j")

let a = matrix[[1, 2] [3, 4]]
let b = matrix[[10, 20] [30, 40]]
let c = a[i, j] + b[i, j]
```

Validation rules:

- Both operands must be indexed rank-2 matrix terms.
- Labels within each term must be distinct.
- Free-index order must match exactly.
- Shapes must match at runtime.
- There is no automatic transposition or reordering for `A[i, j] + B[j, i]`.

## Einstein subtraction

M32 supports interpreted rank-2 Einstein subtraction.
`A[i, j] - B[i, j]` performs elementwise indexed subtraction when free-index order matches exactly.

```oct
let i = Idx("i")
let j = Idx("j")

let a = matrix[[10, 20] [30, 40]]
let b = matrix[[1, 2] [3, 4]]
let c = a[i, j] - b[i, j]
```

Validation mirrors addition:

- Both operands must be indexed rank-2 matrix terms.
- Labels within each term must be distinct.
- Free-index order must match exactly.
- Shapes must match at runtime.
- There is no automatic transposition or reordering for `A[i, j] - B[j, i]`.

## `@` as Einstein contraction shorthand

`@` is not arbitrary array multiplication.
It is shorthand for currently supported linear-algebra contractions:

- `A @ B` is matrix-matrix contraction, conceptually `A[i, k] * B[k, j]`.
- `A @ x` is matrix-vector contraction, conceptually `A[i, j] * x[j]`.

Vector rank-1 indexed terms are not yet source-level syntax, so `A @ x` has this mathematical meaning even before `x[j]` syntax is implemented.
Current supported cases are `Matrix<T> @ Vector<U>` and `Matrix<T> @ Matrix<U>`.
`@` requires dimension compatibility (`left.cols == right.rows` for matrix-matrix).
For dimension-qualified elements, `@` propagates dimensions by scalar multiplication and addition across contractions:

- `Matrix<Float<D1>> @ Vector<Float<D2>> -> Vector<Float<D1*D2>>`
- `Matrix<Float<D1>> @ Matrix<Float<D2>> -> Matrix<Float<D1*D2>>`

Current unsupported `@` cases include vector-matrix, vector-vector, and arrays.

## Trace

Trace-style indexed sugar is intentionally unsupported:

```oct
let i = Idx("i")
let bad = A[i, i]
```

Use `Trace(A)` instead.
This keeps trace semantics explicit while the indexed surface remains rank-2 matrix-result only.

## Differential representational operators

Current tensor/differential surface includes:

- `Grad(x)`
- `Div(x)`
- `SymGrad(x)`
- `Trace(x)`

These are representational/tensor-aware operators used in continuum mechanics contracts.
Oct tensor work is representational and typed: tensor expressions can preserve index structure and composition, and differential operators compose as typed symbolic/representational terms rather than forcing numerical discretization.
Continuum mechanics contracts use this surface to express field-form equations such as strain/stress and balance skeletons directly in language-level tests.

## Interpreted vs compiled support

Indexed tensor notation is currently interpreted-supported.
Compiled support for `Idx` and indexed Einstein terms remains deferred to M33+.

- Compiled mode currently supports concrete vector/matrix indexing and `@` helper lowering.
- Compiled mode does not currently support `Idx` / indexed Einstein terms.
- Differential tensor operators are not documented as compiled-parity guarantees in the current corpus.
- Do not infer arbitrary rank-N tensors, vector rank-1 indexed terms, broadcasting, covariant/contravariant variance, raising/lowering indices, or compiled indexed tensor lowering from the current indexed matrix surface.

Derived from the compiled parity corpus (`internal/build/compiler_test.go`) and compiler lowering (`internal/build/compiler.go`):

- **Corpus-verified in compiled mode:** vector literals, matrix literals, `@` for matrix-vector and matrix-matrix, dimensioned matrix `@` vector, `Matrix.tabulate`, `Matrix.fill`, `m.rows`, `m.cols`, and compiled element indexing `m[r, c]`.
- **Code-implemented in compiler lowering (not explicitly parity-listed in the same corpus block):** `Matrix.zeros<T>` and `Matrix.identity<T>`.
- **Still constrained:** compiled general indexing remains strict; matrix indexing must use exactly two concrete indices for element access, and non-matrix indexing remains single-dimension.

## Examples

Valid:

```oct
package Main

fn Main() -> Vector<Int> {
    let m = Matrix.tabulate(2, 2, Weight)
    let v = vector[10, 20]
    return m @ v
}

fn Weight(r: Int, c: Int) -> Int {
    return r * 2 + c + 1
}
```

```oct
package Main

fn Main() -> Vector<Float<kg*m/s^2>> {
    let stiffness = Matrix.tabulate(2, 2, DiagonalStiffness)
    let displacement = vector[4.0m, 5.0m]
    return stiffness @ displacement
}

fn DiagonalStiffness(r: Int, c: Int) -> Float<kg/s^2> {
    if r != c { return 0.0kg/s^2 }
    if r == 0 { return 2.0kg/s^2 }
    return 3.0kg/s^2
}
```

```oct
package Main

fn Main() -> Int {
    let m = Matrix.fill(3, 4, 7)
    return m.rows + m.cols + m[2, 1]
}
```

Invalid:

```oct
package Main

fn Main() -> Int {
    let m = matrix[[1, 2] [3, 4]]
    return m[0]
}
```

```oct
package Main

fn Main() -> Vector<Int> {
    // Invalid: [1, 2, 3] is Int[] (array), not Vector<Int>.
    return [1, 2, 3]
}
```

```oct
package Main

fn Main() -> Matrix<Int> {
    // Invalid: [[1, 2], [3, 4]] is Int[][], not Matrix<Int>.
    return [[1, 2], [3, 4]]
}
```
