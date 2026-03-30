# Oct Reference Overview

## Overview

Oct is a statically typed language implemented in Go.
Language behavior is defined by `.octest` and `.octfail` suites in `Language/`.
This reference restates current behavior in human-readable form.
If this reference conflicts with implementation or tests, implementation and tests are authoritative.

## Where to start

Recommended reading order for first-pass orientation:
- [13 Packages](./language/13-packages.md)
- [02 Types](./language/02-types.md)
- [05 Functions](./language/05-functions.md)
- [06 Errors](./language/06-errors.md)
- then the remaining sections as needed

## Rules

- `Language/` suites are the executable language contract.
- Go code in `cmd/` and `internal/` is the implementation authority.
- This reference does not add features.
- This reference does not relax typing rules.
- This reference does not define future behavior.

## Examples

Valid:

```oct
fn Main() -> Int {
    return 42
}
```

Invalid:

```oct
fn Main() -> Int {
    return true
}
```
