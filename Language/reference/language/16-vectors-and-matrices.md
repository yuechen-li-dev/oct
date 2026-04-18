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
- `+`, `-`, `*`, `/` on vectors and matrices are element-wise operations.
- Element-wise vector operations require equal runtime lengths.
- Element-wise matrix operations require equal runtime shapes.
- `@` supports `Matrix<T> @ Vector<U>` and `Matrix<T> @ Matrix<U>`.
- `@` requires dimension compatibility (`left.cols == right.rows` for matrix-matrix).
- For dimension-qualified elements, `@` propagates dimensions by scalar multiplication and addition across contractions:
  - `Matrix<Float<D1>> @ Vector<Float<D2>> -> Vector<Float<D1*D2>>`
  - `Matrix<Float<D1>> @ Matrix<Float<D2>> -> Matrix<Float<D1*D2>>`
- Vector/matrix element mutation through index assignment is not supported.

## Examples

Valid:

```oct
package Main

fn Main() -> Vector<Int> {
    let m = matrix[[1, 2] [3, 4]]
    let v = vector[10, 20]
    return m @ v
}
```

```oct
package Main

fn Main() -> Vector<Float<kg*m/s^2>> {
    let stiffness = matrix[[2.0kg/s^2, 0.0kg/s^2] [0.0kg/s^2, 3.0kg/s^2]]
    let displacement = vector[4.0m, 5.0m]
    return stiffness @ displacement
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
