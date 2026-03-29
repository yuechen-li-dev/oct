# Control Flow

## Overview

Oct control flow is explicit and type-checked. `if`, `switch`, `for`, and `while` cover core branching and looping. `switch` is expression-oriented. Branch conditions never use implicit coercion.

## Rules

- `if` condition must be `Bool`.
- `if` expression requires `else` and matching branch result types.
- Subject `switch` matches strict case types against one subject value.
- Condition `switch` (`switch { ... }`) requires Bool `case` conditions.
- Condition `switch` requires an `else` arm.
- `switch` arms must produce one result type.
- Enum `switch` has no fallthrough.
- `for i in start..end` uses inclusive start and exclusive end.
- `for` bounds must be `Int`.
- `step` is optional, must be `Int`, and must be positive.
- `while` condition must be `Bool`.
- No implicit conditions are allowed in any control-flow form.

## Examples

Valid:

```oct
fn Main() -> Int {
    var sum = 0
    for i in 0..10 step 2 {
        sum = sum + i
    }
    return sum
}
```

Invalid:

```oct
fn Main() -> Int {
    return switch {
        case true => 1
    }
}
```
