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


## Proposed: judgment enum utility selection

**Status: proposed/design-only.** This syntax is not implemented in J1.
See [J1 judgment enum utility selection design](../../../docs/internal/judgment_enums_j1.md).

Enums can serve as closed judgment spaces for one-shot utility-scored selection.
The enum declaration remains ordinary; judgment behavior is introduced at an expression site by the proposed enum-targeted utility form:

```oct
when utility TreatmentDecision {
    case TreatmentDecision.Observe when risk < 0.3 score 40
    case TreatmentDecision.Treat when risk >= 0.6 score 80
    else TreatmentDecision.Observe
}
```

`match` and judgment utility have opposite roles:

- `match` analyzes an enum value that has already been selected and must be exhaustive.
- Proposed `when utility EnumName` scores candidate variants and produces a selected enum value, with an explicit `else` fallback.

The proposed M0 design keeps payload candidates deferred: tag-only candidate variants are the intended first implementation target.

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
