# Errors

## Overview

Error handling is explicit and typed.
Function fallibility is declared in function signatures.
Every fallible expression must be handled.
Handling forms are `?`, `!`, and `match`.

## Rules

- A function is fallible only when its return type includes `! Error`.
- Fallible expressions cannot be ignored.
- `?` propagates `err` to the current fallible function.
- `?` is invalid in an infallible function.
- `?` requires a fallible expression.
- `match expr { ok(v) => ... err(e) => ... }` requires a fallible expression.
- Fallible `match` must include both `ok` and `err` arms.
- `!` unwrap is explicit handling for a fallible expression.
- Returning a fallible value from an infallible function is invalid.

## Examples

Valid:

```oct
fn Read() -> Int ! Error {
    return 7
}

fn Main() -> Int ! Error {
    let x = Read()?
    return x
}
```

Invalid:

```oct
fn Fail() -> Int ! Error {
    return error("bad")
}

fn Main() -> Int {
    let x = Fail()?
    return x
}
```
