# Oct Reference Overview

## Overview

Oct is a statically typed language implemented in Go.
Language behavior is defined by the `Language/` contract suites and this reference.
This reference is the human-readable source of truth for syntax, style conventions, and supported user-facing language features.
If this reference, implementation, and tests disagree, treat the disagreement as a documentation or implementation gap to surface explicitly rather than silently cargo-culting older code.

## Start here: practical reading paths

Choose one path based on what you are trying to do:

- First Oct program (preferred when learning core syntax):
  - [02 Types](./language/02-types.md) -> [05 Functions](./language/05-functions.md) -> [04 Control Flow](./language/04-control-flow.md) -> [09 Builtins](./language/09-builtins.md) -> [17 Standard Libraries](./language/17-standard-libraries.md)
- Control/behavior authoring (use this when building modes and transitions):
  - [04 Control Flow](./language/04-control-flow.md) -> [21 Octomata](./runtime/21-octomata.md) -> [11 Records](./language/11-records.md)
- Read-only iteration/query authoring:
  - [07 Arrays](./language/07-arrays.md) -> [21 Octomata](./runtime/21-octomata.md) -> [24 FLOW-backed queries](./runtime/24-query.md)
- Tooling/testing (preferred when validating contracts):
  - [31 octest](./tooling/31-octest.md) -> [32 ocfmt](./tooling/32-ocfmt.md) -> [35 CLI](./tooling/35-cli.md)
- Prometheus experiments (explicitly non-core):
  - [23 Prometheus](./runtime/23-prometheus.md) -> [35 CLI](./tooling/35-cli.md)
- UI authoring (use this when building Machina UI values):
  - [02 Types](./language/02-types.md) -> [08 Units](./language/08-units.md) -> [09 Builtins](./language/09-builtins.md)

## Rules

- `Language/` suites are the executable language contract.
- `Language/reference` is the human-readable reference for syntax, style conventions, and supported features.
- Go code in `cmd/` and `internal/` implements the language; it should not define a second user-facing semantic contract.
- This reference does not add features.
- This reference does not relax typing rules.
- This reference does not define future behavior.

## Examples

Valid:

```oct
package Main

fn Main() -> Int {
    return 42
}
```

Invalid:

```oct
package Main

fn Main() -> Int {
    return true
}
```
