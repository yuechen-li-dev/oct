# Variables

## Overview

Oct bindings are explicit: immutable `let`, mutable `var`.
Assignment is type-checked and exact.
Mutation for structured values is whole-value oriented.

## Rules

- `let` creates an immutable binding.
- `var` creates a mutable binding.
- Assignment (`name = expr`) requires a mutable binding.
- Reassignment type must match the binding type exactly, including dimensions and nominal type identity.
- Record, enum, vector, and matrix updates use whole-value reassignment.
- Array element assignment (`xs[i] = value`) requires `xs` bound with `var`.
- Array element assignment value must match the array element type exactly.
- Assignment never changes a binding's declared/inferred type.

## Examples

Valid:

```oct
record Point { X: Int Y: Int }

fn Main() -> Int {
    var d = 1m
    d = 2m

    var p = Point { X: 1 Y: 2 }
    p = Point { X: 3 Y: p.Y }

    var xs = [1, 2]
    xs[0] = 5
    return xs[0] + p.X
}
```

Invalid:

```oct
fn Main() -> Int {
    let x = 1
    x = 2
    return x
}
```
