# Variables

## Overview

Oct bindings are explicit: immutable `let`, mutable `var`.
Assignment is type-checked and exact.
Mutation for structured values is whole-value oriented.

`let` is the default.
Use `var` only when reassignment is required.

## Rules

- `let` creates an immutable binding.
- `var` creates a mutable binding.
- Assignment (`name = expr`) requires a mutable binding.
- Bindings may include explicit type annotations: `let name: Type = expr` and `var name: Type = expr`.
- Reassignment type must match the binding type exactly, including dimensions and nominal type identity.
- Record, enum, vector, and matrix updates use whole-value reassignment.
- Array element assignment (`xs[i] = value`) requires `xs` bound with `var`.
- Array element assignment value must match the array element type exactly.
- Assignment never changes a binding's declared/inferred type.

## Examples

Preferred (`let` default):

```oct
package Main

fn Main() -> Int {
    let base = 40
    let offset = 2
    return base + offset
}
```

Use `var` when reassignment is needed:

```oct
package Main

fn NextPowerOfTwo(n: Int) -> Int {
    var v = 1
    while v < n {
        v = v * 2
    }
    return v
}
```

Side-by-side contrast:

Typed mutable binding (including typed empty arrays):

```oct
package Main

fn Main() -> Int {
    var samples: Float<Hz>[] = []
    samples = [60.0Hz, 120.0Hz]
    return Len(samples)
}
```

```oct
package Main

fn LetVsVar(limit: Int) -> Int {
    let start = 1

    var running = start
    while running < limit {
        running = running + 1
    }

    return running
}
```

Valid (whole-value and array mutation):

```oct
package Main

record Point { X: Int Y: Int }

fn Main() -> Int {
    var p = Point { X: 1 Y: 2 }
    p = Point { X: 3 Y: p.Y }

    var xs = [1, 2]
    xs[0] = 5
    return xs[0] + p.X
}
```

Invalid (`let` reassignment):

```oct
package Main

fn Main() -> Int {
    let x = 1
    x = 2
    return x
}
```
