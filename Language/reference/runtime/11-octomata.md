# Octomata

## Overview

Octomata is Oct's explicit flow-state runtime model. A flow advances by explicit stepping and state transitions. Runtime observability is exposed through builtins such as `Step`, `Active`, `Complete`, and `Result`. Behavior is deterministic for identical inputs.

## Rules

- Flows execute through explicit runtime steps.
- State transitions are explicit (`goto`, `suspend`, `resume`, `return`).
- No hidden transition fallthrough exists.
- `Result(flow)` is only valid for flow instances.
- `Complete(flow)` reports completion status explicitly.
- Scheduler decisions are deterministic for a fixed program and input.
- Utility `when` and tie handling preserve deterministic selection.

## Examples

Valid:

```oct
flow Counter() -> Int {
    state Start {
        return 1
    }
}

fn Main() -> Int {
    let f = Counter()
    Step(f)
    match Result(f) {
        ok(v) => { return v }
        err(e) => { return 0 }
    }
}
```

Invalid:

```oct
fn Main() -> Int {
    match Result("x") {
        ok(v) => { return v }
        err(e) => { return 0 }
    }
}
```
