# Oct Reference Overview

## Overview

Oct is a statically typed language implemented in Go. The language contract is expressed by `.octest` and `.octfail` suites in `Language/`. This reference restates current behavior in a human-readable form. If this text conflicts with implementation or tests, implementation and tests win.

## Rules

- `Language/` test suites are the executable contract for language behavior.
- Go implementation under `cmd/` and `internal/` is the runtime and checker authority.
- This reference does not add features.
- This reference does not relax type rules.
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
