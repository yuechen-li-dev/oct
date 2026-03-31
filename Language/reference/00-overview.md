# Oct Reference Overview

## Overview

Oct is a statically typed language implemented in Go.
Language behavior is defined by `.octest` and `.octfail` suites in `Language/`.
This reference restates current behavior in human-readable form.
If this reference conflicts with implementation or tests, implementation and tests are authoritative.

## Start here: practical reading paths

Choose one path based on what you are trying to do:

- First Oct program (preferred when learning core syntax):
  - [02 Types](./language/02-types.md) -> [05 Functions](./language/05-functions.md) -> [04 Control Flow](./language/04-control-flow.md) -> [09 Builtins](./language/09-builtins.md)
- Control/behavior authoring (use this when building modes and transitions):
  - [04 Control Flow](./language/04-control-flow.md) -> [21 Octomata](./runtime/21-octomata.md) -> [11 Records](./language/11-records.md)
- Tooling/testing (preferred when validating contracts):
  - [31 octest](./tooling/31-octest.md) -> [32 ocfmt](./tooling/32-ocfmt.md) -> [35 CLI](./tooling/35-cli.md)
- UI authoring (use this when building Machina UI values):
  - [02 Types](./language/02-types.md) -> [08 Units](./language/08-units.md) -> [09 Builtins](./language/09-builtins.md)

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
