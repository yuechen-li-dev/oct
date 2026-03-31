# Arrays

## Overview

Arrays are one-dimensional homogeneous values.
Array literals, indexing, assignment, and element-wise arithmetic are supported.
Type matching is exact, including dimensions and nominal names.

## Rules

- Array literal form is `[a, b, c]`.
- All array literal elements must have one exact type.
- Array type form is `T[]`.
- Indexing form is `xs[i]`.
- Index expressions must have type `Int`.
- Indexed assignment requires a mutable array binding (`var`).
- Indexed assignment values must match the element type exactly.
- `Append(xs, value)` requires `xs` to be an array.
- `Append` values must match the array element type exactly.
- Element-wise arithmetic requires arrays with the same element type.
- Element-wise arithmetic requires equal runtime lengths.

## Examples

Valid:

```oct
package Main

fn Main() -> Int {
    var xs = [1, 2]
    xs = Append(xs, 3)
    return xs[2]
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
