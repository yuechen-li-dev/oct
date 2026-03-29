# Octomata

## Overview

Octomata is Oct's explicit flow-state runtime model.
Flows advance through explicit steps and explicit state transitions.
Runtime observation is exposed by builtins such as `Step`, `Active`, `Complete`, and `Result`.
Behavior is deterministic for identical inputs.

## Rules

- Flows execute through explicit runtime steps.
- State transitions are explicit (`goto`, `suspend`, `resume`, `return`).
- Hidden transition fallthrough is not allowed.
- `Result(flow)` is valid only for flow instances.
- `Complete(flow)` reports completion status.
- Scheduler decisions are deterministic for a fixed program and input.
- `when` clauses and tie handling preserve deterministic selection.

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
