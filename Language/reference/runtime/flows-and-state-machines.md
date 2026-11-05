# Flows and state machines (Octomata)

[Reference home](../README.md) · [Coverage](../coverage.md)

## Overview

Octomata is Oct's explicit behavioral/control runtime model.
Use it when behavior progression matters (modes, transitions, interruption, arbitration), not as a universal replacement for records.

Octomata and records are complementary:

- Octomata: behavior progression and control memory.
- Records: numeric/data lanes and durable payload state.

## Rules and contextual keywords

`flow`, `state`, and `step` are contextual keywords.

- `flow` starts flow declarations.
- `state` starts state declarations.
- `step` marks range step clauses.
- In non-ambiguous identifier/binding positions, `flow`/`state`/`step` can still be used as ordinary names.

- Flow declaration form is `flow Name(params) -> ReturnType { state ... }` (source also accepts `=>` for the arrow).
- A reactive flow may separately declare one named turn input and one yielded type: `flow Name(params) accepts input: InputType yields YieldType -> FinalType { state ... }`.
- Construction parameters are retained for the flow lifetime. The `accepts` binding exists only during one `Step(flow, input)` turn and is cleared at the boundary.
- `yield value` publishes one value of the declared yield type, preserves the continuation immediately after the yield, and returns control without completing the flow.
- Yield and final return types are distinct. Heterogeneous yielded values require a common nominal enum/record type.
- State locals do not survive a yield. A reference after a yield must use construction state, the current turn input, or explicit private `board` state; a pre-yield local is out of scope.
- A flow must declare at least one `state`.
- State declaration form is `state Name { ... }`.
- `goto StateName` transitions to a declared state.
- `suspend` yields control without completing the flow.
- `return value` completes the flow with the declared return type.
- Guard form `when { case ... else ... }` inside a state uses ordered guards: first true `case` action wins.
- Flow `when` requires `else`.
- Flow guard `when` actions are `goto`, `suspend`, or `return`, or the bounded action blocks described under [Guard `when`](#guard-when).
- `board { Field: Type ... }` declares flow-local board memory with a fixed field shape.
- Board fields must be declared up front and are not dynamically extensible.
- Board fields may be deterministic persistent values composed from `Bool`, `String`, `Int`/`Int<D>`, `Float`/`Float<D>`, arrays, vectors, matrices, records, and enums. Function values remain excluded because capture-environment identity, checkpoint representation, and history semantics require a separate persistence contract.
- Board fields are default-initialized recursively from their declared type (scalars use their zero value, collections are empty, records default each field, and enums use their first declared variant with a defaulted payload when present).
- Board writes are valid only inside flow state bodies (including nested `if`/`when` inside a state body).
- Controller utility form `when policy { hysteresis: Int min_commit: Int } { case value when condition score Int ... else value }` is valid only inside flow state bodies.
- Standalone utility form `when utility { case value when condition score Int ... else value }` is an expression form valid wherever expressions are allowed.
- Standalone `when utility` takes no policy fields. It keeps no commitment between evaluations, so `hysteresis` and `min_commit` would have nothing to act on; writing them is an error.
- `remember` stores the current state as a resume target.
- `resume` jumps to the remembered target.
- Resume storage is a single slot.
- A later `remember` overwrites the existing slot value.
- Successful `resume` clears the slot.
- `resume` with an empty slot is a runtime error.
- `Step(flow)` advances one scheduling step.
- Input-bearing flows require `Step(flow, input)` with the declared input type; non-input flows reject a second argument.
- One turn executes from the current continuation through ordinary `goto` and `resume` transitions until `yield`, `suspend`, or final `return`. A state entered by `goto` or `resume` runs from its first statement within the same turn.
- `DidYield(flow)` reports whether the most recent turn ended at `yield`.
- `Yielded(flow)` is fallible and extracts the most recent turn's typed yielded value.
- Starting another `Step` clears prior-turn yield observability before execution;
  afterward `DidYield`/`Yielded` describe only the newly completed turn.
- `Active(flow)` returns the active state name or `""` when inactive/completed-before-step.
- `Complete(flow)` reports completion status.
- `Result(flow)` is fallible because the flow may not have completed yet.
- `ResumeTarget(flow)` reports the current remembered target or `""` when slot is empty.
- `StateHistory(flow)` returns state-entry history as `String[]`.
- Builtins `Step`, `DidYield`, `Yielded`, `Active`, `Complete`, `Result`, `ResumeTarget`, `StateHistory`, and `BoardSnapshot` require a flow instance argument.
- `BoardSnapshot(flow)` returns a fallible, read-only typed record snapshot of current board fields when the flow declares a board. One recursive persistent-value clone semantic detaches arrays, nested arrays, vectors, matrices, records, and enum payloads in interpreted and compiled execution. Later indexed board mutation cannot change an earlier snapshot.

## Ordinary computation inside a state

A state activation uses ordinary Oct expression and type rules. Parameters,
the current turn input, state locals, board fields, globals, and imports form
the FLOW expression context; they do not define a secondary expression
language. Ordinary synchronous user functions and function values may be
called. `!` has its ordinary fatal-unwrap meaning, and a fallible call may be
handled locally with statement `match`.

The compiled backend routes ordinary state expressions through the ordinary
compiled MIR lowering context. FLOW adds typed bindings for construction
parameters, turn input, board values, and activation locals; it does not add a
reduced expression language. Controller-bound `when policy` remains a
FLOW-specific persistent decision operation, and machine transfers remain
FLOW-specific control MIR.

`?` is deliberately rejected until FLOW declares an explicit fallible terminal
contract. The diagnostic directs authors to local `match` or `!`; M0 does not
invent an implicit failed-machine state.

State locals, including captured anonymous functions, exist only for the
current activation. They do not cross `goto`, `suspend`, `yield`, or turn
boundaries unless their value is written into an explicitly legal persistent
construct. FLOW accepts exactly one typed turn input value; use a record or enum
when one turn carries several related facts.

## Turn and yield model (FLOW-TURN-M0)

See [Turns and checkpoints](turns-and-checkpoints.md) for reactive inputs,
yields, continuation rules, and the generated Go host boundary.

### Dimensioned scalar and array board fields

Board fields may use dimensioned scalar numeric types and arrays of them. `BoardSnapshot` preserves the exact unit-qualified field and element types.

```oct
package Main

flow TemperatureProbe(temp: Float<K>) -> Float<K> {
    board {
        LastTemp: Float<K>
    }

    state Start {
        board.LastTemp = temp
        return board.LastTemp
    }
}

[Fact]
fn SnapshotPreservesTemperatureUnits() -> Void {
    let machine = TemperatureProbe(294.0K)
    Step(machine)
    let snapshot = BoardSnapshot(machine)!
    let typed: Float<K> = snapshot.LastTemp
    Assert.Equal(294.0K, typed, "temperature snapshot")
}
```

## Result handling

`Result(machine)` extracts a flow's completed return value.
It is a fallible operation because a flow may not have completed yet (for example, it may still be active, suspended, or not stepped to completion).

Use the handling form that matches your intent:

- Tests or assertions after guaranteed completion: `Result(machine)!`
- Fallible function boundaries: `Result(machine)?`
- Robust branch handling: `match Result(machine) { ok(v) => ... err(e) => ... }`

For status/inspection, do not use `Result` as a status accessor.
Prefer:

- `Active(machine)` for current active state visibility
- `Complete(machine)` for completion status
- `StateHistory(machine)` for transition/history visibility
- `ResumeTarget(machine)` for remembered resume-slot state

```oct
package Main

import Assert

flow DoneFlow() -> Int {
    state Start {
        return 7
    }
}

[Fact]
fn ResultAfterCompletion() -> Void {
    let machine = DoneFlow()
    Step(machine)
    let value = Result(machine)!
    Assert.Equal(7, value, "completed flow result")
}
```

## When to use Octomata

Use Octomata when behavior progression is a first-class concern:

- modes with explicit allowed transitions
- interruption/resume semantics
- fault/acknowledge flows
- arbitration among competing behaviors or policies
- control loops with meaningful behavioral state

Prefer ordinary functions + records when behavior progression is not the core problem:

- numeric pipelines
- deterministic record transforms
- one-off local branching with no behavioral memory

### Use Octomata here (behavior progression)

```oct
package Main

flow MotorControl(overheat: Bool, ack: Bool) -> String {
    state Run {
        when {
            case overheat -> goto Fault
            else -> return "running"
        }
    }

    state Fault {
        when {
            case ack -> goto Recover
            else -> return "fault"
        }
    }

    state Recover { return "recovering" }
}
```

### Records/ordinary control are enough here (data pipeline)

```oct
package Main

record Sample {
    Value: Float
    Bias: Float
}

fn Corrected(s: Sample) -> Float {
    let corrected = s.Value - s.Bias
    if corrected < 0.0 {
        return 0.0
    }
    return corrected
}
```

## Records with Octomata: behavior/data split

Records remain preferred for data-heavy lanes, even in Octomata-driven systems.

Use records for:

- sampled numeric values
- history buffers
- durable payload state
- deterministic transforms
- values that are data (not behavioral control memory)

Use Octomata states + transitions for behavior progression only.

```oct
package Main

record Telemetry {
    Temperature: Float<K>
    Pressure: Float
    WindowAvgTemperature: Float<K>
}

flow CoolingController(t: Telemetry, trip: Float<K>) -> String {
    state Evaluate {
        when {
            case t.WindowAvgTemperature >= trip -> goto Cooling
            else -> return "hold"
        }
    }

    state Cooling { return "cooling" }
}
```

## Blackboards for control loops

A blackboard is behavior-local working memory owned by control flow.

### Board declaration and write scope

Declare board shape inside the flow with `board { ... }` before states.
Board shape is fixed for the flow instance and fields are declared up front.
Board fields are default initialized by type:

- `Bool -> false`
- `Int -> 0`
- `Float -> 0.0`
- `String -> ""`

```oct
package Main

flow PumpLoop() -> Int {
    board {
        FaultLatched: Bool
        ResumeTarget: Int
    }

    state Tick {
        board.ResumeTarget = 2
        return board.ResumeTarget
    }
}
```

Use board memory for behavior-local latches, resume targets, cooldown/commit memory, and policy edge memory.
Avoid using boards as generic mutable application state or as a dumping ground for numeric data lanes.


Use blackboards when control logic needs local memory such as:

- interruption/resume targets
- fault latches
- alert/control status memory
- policy edge memory, cooldown memory, commitment memory

Avoid blackboards for:

- numeric data lanes
- broad heterogeneous dumping
- general mutable application state

Preferred blackboard shape is typed, fixed-shape, constrained, and explicit.

```oct
package Main

record Telemetry {
    Temperature: Float<K>
    Pressure: Float
}

flow PumpLoop(t: Telemetry) -> String {
    board {
        FaultLatched: Bool
        CooldownTicks: Int
    }

    state Tick {
        if t.Temperature > 95C {
            board.FaultLatched = true
            board.CooldownTicks = 5
            goto Fault
        }

        if board.CooldownTicks > 0 {
            board.CooldownTicks = board.CooldownTicks - 1
            return "cooldown"
        }

        return "normal"
    }

    state Fault {
        if board.CooldownTicks > 0 {
            board.CooldownTicks = board.CooldownTicks - 1
            suspend
        }
        return "fault-latched"
    }
}
```

In this pattern, telemetry stays in records while the board stores control-owned memory.

## Guard `when`

Use guard `when` when transitions/actions depend on explicit conditions and you want allowed transitions visible and local.

Prefer guard `when` over scattering equivalent branching across helpers when the logic is truly behavioral.

```oct
package Main

flow DoorControl(openCmd: Bool, closeCmd: Bool, blocked: Bool) -> String {
    state Closed {
        when {
            case openCmd and blocked == false -> goto Opening
            else -> return "closed"
        }
    }

    state Opening {
        when {
            case blocked -> goto Closed
            case closeCmd -> goto Closing
            else -> return "opening"
        }
    }

    state Closing { return "closing" }
}
```

Why this is clearer than ad hoc `if` branching: transition options are listed in one place, in execution order.

For interrupt-style control, guard branches also support bounded action blocks:

```oct
when {
    case input.HazardActive -> {
        remember
        goto HazardHold
    }
    else -> {
        suspend
    }
}
```

Action blocks are intentionally bounded to flow-control statements (`remember`, `resume`, `goto`, `suspend`, `return`) plus board field assignment.

A guard action block must finish with a control transfer: `goto`, `suspend`,
`resume`, or `return`. Compute ordinary local values before the guard; arbitrary
computation and an unterminated fallthrough block are rejected here.

## Utility `when`

Use [standalone utility selection](../language/utility-selection.md) for a
fresh ranked choice. Use [controller policy](utility-policy.md) when the
FLOW must retain commitment between evaluations.

### Judgment enum utility

The [enum-targeted utility form](../language/utility-selection.md#enum-targeted-selection)
returns an ordinary enum; subsequent `match` analyzes its payload.

## `hysteresis` and `min_commit` in practice

The [controller-policy reference](utility-policy.md) defines both settings
and their effect on a persistent selection site.

## Record updates in Octomata-based systems

Octomata examples may use ordinary record `with` updates for immutable data-lane state.
`with` is a general record feature, not an Octomata-specific feature; see [Records](../language/records-and-tables.md) for the full syntax and semantics.

Keep the split clear:

- Use record `with` updates for data-lane or application-state values that should remain immutable.
- Use board field assignment for behavior-local control memory owned by the flow.

Board field update is correct for control memory:

```oct
package Main

flow AckLoop(ack: Bool) -> String {
    board {
        FaultLatched: Bool
    }

    state Fault {
        if ack {
            board.FaultLatched = false
            return "cleared"
        }
        board.FaultLatched = true
        return "latched"
    }
}
```

See also [Batch](batch.md) for array-parallel execution constructs.

## Async sugar over FLOW

[Async functions](async.md) lower to the same FLOW model. The async chapter
explains the admitted await positions and generated continuation.
