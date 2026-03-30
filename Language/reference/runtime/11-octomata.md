# Octomata

## Overview

Octomata is Oct's explicit flow-state runtime model.
Flows run through declared states and explicit transitions.
Runtime observation uses flow builtins.
Execution is deterministic for identical inputs.

## Rules

- Flow declaration form is `flow Name(params) -> ReturnType { state ... }`.
- A flow must declare at least one `state`.
- State declaration form is `state Name { ... }`.
- `goto StateName` transitions to a declared state.
- `suspend` yields control without completing the flow.
- `return value` completes the flow with the declared return type.
- `when` inside a state uses ordered guards: first true `case` action wins.
- Flow `when` requires `else`.
- Flow `when` actions are only `goto`, `suspend`, or `return`.
- Utility form `when policy { hysteresis: Int min_commit: Int } { ... }` is valid only inside flow state bodies.
- `remember` stores the current state as a resume target.
- `resume` jumps to the remembered target.
- Resume storage is a single slot.
- A later `remember` overwrites the existing slot value.
- Successful `resume` clears the slot.
- `resume` with an empty slot is a runtime error.
- `Step(flow)` advances one scheduling step.
- `Active(flow)` returns the active state name or `""` when inactive/completed-before-step.
- `Complete(flow)` reports completion status.
- `Result(flow)` is available only after completion.
- `ResumeTarget(flow)` reports the current remembered target or `""` when slot is empty.
- `StateHistory(flow)` returns state-entry history as `String[]`.
- Builtins `Step`, `Active`, `Complete`, `Result`, `ResumeTarget`, and `StateHistory` require a flow instance argument.

See also [12 Batch](./12-batch.md) for array-parallel execution constructs.

## Examples

Valid:

```oct
flow Ordered(flagA: Bool, flagB: Bool) -> Int {
    state Start {
        when {
            case flagA -> goto A
            case flagB -> goto B
            else -> return 0
        }
    }

    state A { return 1 }
    state B { return 2 }
}

flow Policy(signal: Float) -> Int {
    state Start {
        when policy { hysteresis: 2 min_commit: 1 } {
            case signal > 0.5 -> goto High
            else -> suspend
        }
    }

    state High { return 1 }
}

fn Main() -> Int {
    let f = Ordered(true, true)
    Step(f)
    let history = StateHistory(f)
    if Len(history) > 0 {
        return 0
    }
    return 1
}
```

Invalid:

```oct
flow Broken() -> Int {
    state Start {
        resume
    }
}
```
