# Errors

## Overview

Oct error handling is explicit and typed. Fallibility is declared in function signatures. Fallible results must be handled at use sites. Handling is done with `?`, `!`, or `match`.

## Rules

- Only `! Error` marks a function as fallible.
- A fallible expression cannot be ignored.
- `?` propagates `err` to the current fallible function.
- `?` is invalid in infallible functions.
- `?` requires a fallible expression.
- `match expr { ok(v) => ... err(e) => ... }` requires a fallible expression.
- `match` must provide both `ok` and `err` arms.
- `!` unwrap handles a fallible expression explicitly and is permitted where explicit handling is required.
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
