# Arrays

## Overview

Arrays are homogeneous, one-dimensional values. Array literals, indexing, element assignment, and array arithmetic are supported. Types are exact, including dimensions and named types. Array values are used directly in loops and batch expressions.

## Rules

- Array literal form: `[a, b, c]`.
- All array literal elements must have one exact type.
- Array type form: `T[]`.
- Indexing form: `xs[i]`, where `i` is `Int`.
- Indexed assignment requires mutable array binding (`var`).
- Indexed assignment value must match element type exactly.
- `Append(xs, value)` requires `xs` to be an array.
- `Append` value type must match array element type exactly.
- Element-wise arithmetic requires arrays of same element type.
- Element-wise arithmetic requires equal runtime length.

## Examples

Valid:

```oct
fn Main() -> Int {
    var xs = [1, 2]
    xs = Append(xs, 3)
    return xs[2]
}
```

Invalid:

```oct
fn Main() -> Int<m>[] {
    var xs = [1m, 2m]
    return Append(xs, 3s)
}
```
