# Enums

## Overview

Enums are nominal sum types with named variants.
Variants are either tag-only or single-payload.
Variant references are qualified.
`match` is the payload-binding analysis form for enums.

## Rules

- Enum declaration form is `enum Name { Variant ... }`.
- Supported variant forms are:
  - `Variant` (tag-only)
  - `Variant(Type)` (single payload)
- Variant construction forms are:
  - `Name.Variant`
  - `Name.Variant(value)` for payload variants
- `match` over enum variants is exhaustive and binds payload names per case:
  - `case Name.Variant(v) => ...` for payload variants
  - `case Name.Tag => ...` for tag-only variants
- Enum values are compared and switched by qualified variants.
- `switch` over an enum is exhaustive when all variants are listed.
- Non-exhaustive enum `switch` requires an `else` arm.
- Duplicate enum case labels are rejected.
- Enum identity is nominal by enum name. See [02 Types](./02-types.md).
- Intentionally out of scope in this milestone:
  - multi-field payloads
  - tuple or record destructuring patterns
  - nested pattern matching and guards

## Examples

Valid:

```oct
package Main

enum ParseResult {
    Ok(Int)
    Err(String)
}

fn Score(result: ParseResult) -> Int {
    return match result {
        case ParseResult.Ok(v) => v * 2
        case ParseResult.Err(msg) => -1
    }
}
```

Invalid:

```oct
package Main

enum ParseResult {
    Ok(Int)
    Err(String)
}

fn Broken(result: ParseResult) -> Int {
    return match result {
        case ParseResult.Ok => 1
        case ParseResult.Err(msg) => 0
    }
}
```
