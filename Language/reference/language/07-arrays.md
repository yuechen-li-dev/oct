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
- `Array.CrossSection(xs, range)` requires a 1D array and a `Range`, and returns a new `T[]` copy.
- `Array.CrossSection` preserves the exact array element type, including SI dimensions and nominal record/enum types.
- `Array.CrossSection` resolves omitted range start to `0`, omitted range end to `Len(xs)`, and omitted step to `1`.
- `Array.CrossSection` checks that step is positive, bounds are non-negative and within `Len(xs)`, and start is not after end.
- Element-wise arithmetic requires arrays with the same element type.
- Element-wise arithmetic requires equal runtime lengths.
- Nested arrays are still arrays (containers), not matrix values.
- Nested arrays may be ragged/jagged (`[[1], [2, 3]]`) because they are container-of-container values.
- `[]` never means null/nil; it is a zero-length array with a known element type.
- Oct does not have Python colon slice syntax: `xs[1:3]` is invalid.
- Oct does not have bracket range extraction in M0: `xs[1..3]` is invalid. Use `Array.CrossSection(xs, 1..3)`.
- `Array.CrossSection` is not a view; mutating the result array storage does not mutate the source array storage.
- `Array.CrossSection` is for 1D arrays only. Vectors, matrices, and tensors have separate rank-aware APIs and are not accepted.
- Negative indices, reverse ranges, lazy views, `Array.TryCrossSection`, `Array.Copy`, `Array.Take`, `Array.Drop`, and `Array.Window` are deferred/not part of M0.

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

```oct
package Main

fn Main() -> Int[] {
    let samples = [10, 20, 30, 40, 50]
    return Array.CrossSection(samples, 1..5 step 2)
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
