# Rules

## Overview

These are global language rules that apply across all syntax categories. They define how Oct stays explicit and predictable. These rules are normative for reading and writing Oct programs.

## Rules

- No implicit conversions.
- No truthiness.
- Strict typing on every expression, assignment, call, and return.
- Dimensions are enforced as part of type checking.
- Fallibility must be handled explicitly.
- Control flow is explicit (`if`, `switch`, `for`, `while`, `match`).
- `switch` does not fall through.
- Evaluation order is deterministic within expression and statement semantics.
- Unsupported operator/type combinations are compile-time errors.
- Invalid programs belong in `.octfail`; valid behavior belongs in `.octest`.

## Examples

Valid:

```oct
fn Main() -> Int {
    if true {
        return 1
    }
    return 0
}
```

Invalid:

```oct
fn Main() -> Int {
    if 1 {
        return 1
    }
    return 0
}
```
