# Arrays

## Overview

Arrays are homogeneous container values.
Array literals, nested array literals, indexing, assignment, and element-wise arithmetic are supported.
Type matching is exact, including dimensions and nominal names.

## Rules

- Array literal form is `[a, b, c]`.
- Empty array literal form is `[]`, but only in explicit array-typed context.
- All array literal elements must have one exact type.
- Array type forms are `T[]`, `T[][]`, and deeper nested container forms.
- Indexing form is `xs[i]`.
- Index expressions must have type `Int`.
- Indexed assignment requires a mutable array binding (`var`).
- Indexed assignment values must match the element type exactly.
- `Append(xs, value)` requires `xs` to be an array.
- `Append` values must match the array element type exactly.
- Element-wise arithmetic requires arrays with the same element type.
- Element-wise arithmetic requires equal runtime lengths.
- Nested arrays are still arrays (containers), not matrix values.
- Nested arrays may be ragged/jagged (`[[1], [2, 3]]`) because they are container-of-container values.
- `[]` never means null/nil; it is a zero-length array with a known element type.

## Examples

Valid:

```oct
package Main

fn Main() -> Int {
    let grid = [[1, 2], [3, 4]]
    return grid[1][0]
}
```

```oct
package Main

fn Main() -> Int {
    var xs: Int[] = []
    var ys: Float<m>[] = []
    return Len(xs) + Len(ys)
}
```

Invalid:

```oct
package Main

fn Main() -> Int<m>[] {
    var xs = [1m, 2m]
    return Append(xs, 3s)
}
```
