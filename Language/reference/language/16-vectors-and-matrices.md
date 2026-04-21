# Vectors and Matrices

## Overview

Vectors and matrices are fixed-shape numeric structures.
They have dedicated literal, indexing, and arithmetic forms.
`@` is matrix multiplication, distinct from element-wise operators.
They are mathematical value categories, not general-purpose storage containers.

## Rules

- `[...]` is always an array literal.
- Vector literal form is `vector[a, b, c]`.
- Matrix literal form is `matrix[[r1c1, r1c2] [r2c1, r2c2]]`.
- Arrays (`T[]`, `T[][]`, ...) are generic containers; vectors/matrices are mathematical values.
- Vectors and matrices may use dimension-qualified numeric elements (for example `Vector<Float<m>>`, `Matrix<Float<kg/s^2>>`).
- Arrays are not implicitly reinterpreted as vectors or matrices by expected type.
- `[[...], [...]]` is an array-of-arrays literal, not a matrix literal.
- Vector literals require homogeneous element type.
- Matrix rows must all have equal length.
- Vector indexing form is `v[i]` (exactly one index).
- Matrix indexing form is `m[r, c]` (exactly two indices).
- Matrix shape accessors are `m.rows` and `m.cols` (both `Int`, read-only).
- `+`, `-`, `*`, `/` on vectors and matrices are element-wise operations.
- Element-wise vector operations require equal runtime lengths.
- Element-wise matrix operations require equal runtime shapes.
- `@` supports `Matrix<T> @ Vector<U>` and `Matrix<T> @ Matrix<U>`.
- `@` requires dimension compatibility (`left.cols == right.rows` for matrix-matrix).
- For dimension-qualified elements, `@` propagates dimensions by scalar multiplication and addition across contractions:
  - `Matrix<Float<D1>> @ Vector<Float<D2>> -> Vector<Float<D1*D2>>`
  - `Matrix<Float<D1>> @ Matrix<Float<D2>> -> Matrix<Float<D1*D2>>`
- Vector/matrix element mutation through index assignment is not supported.

## Matrix construction surface (Mx100a+)

Use constructors when matrix values are generated, repetitive, or large:

- `Matrix.tabulate(rows, cols, Fn)` where `Fn` has shape `fn(r: Int, c: Int) -> T`.
- `Matrix.fill(rows, cols, value)` for constant matrices.
- `Matrix.zeros<T>(rows, cols)` for typed zero matrices.
- `Matrix.identity<T>(n)` for identity matrices.

Use literals (`matrix[[...]]`) for small hand-authored constants where the literal is clearer.
For benchmark/corpus-style setup, prefer constructors over giant literals.

Compiled mode supports the same matrix element indexing form `m[r, c]` as interpreted mode.

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
    // Invalid: [1, 2, 3] is an Int[] array literal, not a Vector<Int>.
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
