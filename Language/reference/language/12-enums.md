# Enums

## Overview

Enums are nominal sum types with named variants.
Variant references are qualified.
Enum switching is explicit and checked for coverage.

## Rules

- Enum declaration form is `enum Name { Variant ... }`.
- Variant reference form is `Name.Variant`.
- Enum values are compared and switched by qualified variants.
- `switch` over an enum is exhaustive when all variants are listed.
- Non-exhaustive enum `switch` requires an `else` arm.
- Duplicate enum case labels are rejected.
- Enum identity is nominal by enum name. See [02 Types](./02-types.md).

## Examples

Valid:

```oct
enum Mode {
    Fast
    Safe
}

fn Weight(m: Mode) -> Int {
    return switch m {
        case Mode.Fast => 2
        case Mode.Safe => 1
    }
}
```

Invalid:

```oct
enum Mode {
    Fast
    Safe
}

fn Weight(m: Mode) -> Int {
    return switch m {
        case Mode.Fast => 2
    }
}
```
