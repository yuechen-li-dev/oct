# Rules

## Overview

These rules apply across all language categories.
They define explicit, predictable program behavior.
They are normative for reading and writing Oct code.

## Rules

- Implicit conversion is not allowed.
- Truthiness is not allowed.
- Type checking is strict for expressions, assignments, calls, and returns.
- Dimensions are enforced by type checking.
- Fallibility must be handled explicitly.
- Control flow is explicit through `if`, `switch`, `for`, `while`, and `match`.
- `switch` does not fall through.
- Expression and statement evaluation order is deterministic.
- Unsupported operator and type combinations are compile-time errors.
- Valid behavior belongs in `.octest`.
- Invalid behavior belongs in `.octfail`.

## Examples

Valid:

```oct
package Main

fn Main() -> Int {
    if true {
        return 1
    }
    return 0
}
```

Invalid:

```oct
package Main

fn Main() -> Int {
    if 1 {
        return 1
    }
    return 0
}
```
