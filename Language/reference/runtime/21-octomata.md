# Octomata

## Overview

Octomata is Oct's explicit behavioral/control runtime model.
Use it when behavior progression matters (modes, transitions, interruption, arbitration), not as a universal replacement for records.

Octomata and records are complementary:

- Octomata: behavior progression and control memory.
- Records: numeric/data lanes and durable payload state.

## Rules

## Contextual keywords

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
- Flow `when` actions are only `goto`, `suspend`, or `return`.
- `board { Field: Type ... }` declares flow-local board memory with a fixed field shape.
- Board fields must be declared up front and are not dynamically extensible.
- Board fields are default-initialized from their declared scalar type (for example: `Bool` -> `false`, `Int`/`Int<D>` -> `0`, `Float`/`Float<D>` -> `0.0`, `String` -> `""`).
- Board writes are valid only inside flow state bodies (including nested `if`/`when` inside a state body).
- Controller utility form `when policy { hysteresis: Int min_commit: Int } { case value when condition score Int ... else value }` is valid only inside flow state bodies.
- Standalone utility form `when utility { case value when condition score Int ... else value }` is an expression form valid wherever expressions are allowed.
- Standalone `when utility` also accepts optional policy fields via `when utility { hysteresis: Int min_commit: Int } { ... }`; omitted fields default to `0`.
- `remember` stores the current state as a resume target.
- `resume` jumps to the remembered target.
- Resume storage is a single slot.
- A later `remember` overwrites the existing slot value.
- Successful `resume` clears the slot.
- `resume` with an empty slot is a runtime error.
- `Step(flow)` advances one scheduling step.
- Input-bearing flows require `Step(flow, input)` with the declared input type; non-input flows reject a second argument.
- One turn executes from the current continuation through ordinary `goto` transitions until `yield`, `suspend`, or final `return`.
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
- `BoardSnapshot(flow)` returns a fallible, read-only typed record snapshot of current scalar board fields when the flow declares a board. Supported board field types are `Bool`, `String`, scalar `Int`/`Int<D>`, and scalar `Float`/`Float<D>`; arrays, vectors, matrices, records, enums, and other non-scalar runtime types are unsupported.

## Turn and yield model (FLOW-TURN-M0)

```oct
flow Counter(start: Int) accepts amount: Int yields Int -> String {
    board { Count: Int }

    state Active {
        board.Count = board.Count + amount
        yield board.Count
        goto Active
    }
}

fn UseCounter() -> Void {
    let counter = Counter(0)
    Step(counter, 2)
    let first = Yielded(counter)!
    Step(counter, 5)
    let second = Yielded(counter)!
}
```

`yield` is a state-machine boundary, not a lazy collection operation. A yielding flow can model a generator, reactive controller, workflow, or decision machine without introducing a second coroutine/generator runtime. `suspend` remains a legal value-less scheduling boundary. `Result(flow)` remains unavailable until a final `return` completes the flow.

An iterator/generator is a yielding flow consumed repeatedly. This model has no
hidden lazy-list semantics and does not require a parallel generator runtime.

### Logical checkpoints and generated Go host facade (FLOW-TURN-M1)

Embeddable compiled-Go emission exposes an experimental generated facade for
each flow: `NewFlow`, a typed `Step` method, a typed turn with `Yielded`, and,
for yielding flows, `Checkpoint` plus `RestoreFlow`. Record-shaped Concept
inputs use their generated concrete Go struct; refinement Concepts use their
lowered static representation; nominal enum inputs use the generated enum type
and variant constructors. External Go does not need compiler-private
`fn_...`/`__octStep` symbols.

A checkpoint is legal after a turn ended at `yield`. It represents the machine
after the yield: version and flow fingerprints, named state and opaque
continuation position, construction values, private board, resume slot,
feature-observed history, typed utility commitment state, and the last yielded
value needed to preserve `DidYield`/`Yielded` observability. The completed turn
input and state locals are absent. Restore rejects incompatible version, flow,
fingerprint, state/continuation, board, construction, utility-site, and yield
schemas with machine-readable reasons.

The interpreter and compiled materializer use this same logical ownership
model. They do not serialize ASTs, environments, Go pointers, or arbitrary
object graphs. Because locals may not survive `yield`, durable continuation
must be represented in construction state, the private board, resume slot, or
utility policy state. Generated checkpoint bytes are deterministic typed JSON
for the experimental host boundary; that byte encoding is not yet a permanent
Oct 1.0 ABI.


### Dimensioned scalar board fields

Board fields may use dimensioned scalar numeric types. `BoardSnapshot` preserves the exact unit-qualified field type.

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

test "result after completion" {
    let machine = DoneFlow()
    Step(machine)
    let value = Result(machine)!
    Assert.Equal(7, value)
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

## Utility `when`

Use utility `when` for ranked arbitration and prioritization.

- `when policy`: controller-bound Octomata form (inside `flow/state` only) with required `hysteresis` + `min_commit`.
- `when utility`: standalone expression form for one-shot deterministic ranked choice.

Use utility `when` for:

- display/status arbitration
- alert prioritization
- policy choice under competing conditions

Avoid utility `when` when one guard transition is enough.
Guard `when` is simpler and should be preferred for single-condition transitions.

```oct
package Main

flow AlertChannel(fault: Bool, mode: Int) -> Int {
    state Decide {
        let channel = when policy {
            hysteresis: 2
            min_commit: 3
        } {
            case 3 when fault score 120
            case 2 when mode == 2 score 85
            case 1 when mode == 1 score 70
            else 0
        }
        return channel
    }
}
```

Standalone one-shot form:

```oct
package Main

fn LocalOwner(a: Int, b: Int) -> Int {
    return when utility {
        case 100 when a > 0 score a
        case 200 when b > 0 score b
        else -1
    }
}
```

In utility `when`, the `else` arm is the default selected value when no case qualifies as the winner.
It is not a statement-style `return`; it is the fallback candidate in the selection set.

Standalone utility selection evaluates policy expressions first, then visits
cases in source order. Each condition is evaluated once. A false condition
skips its score and value. For each true condition, its dimensionless `Int`
score and value are evaluated once. The greatest score wins; equal scores keep
the earliest source case. If no condition is true, only the required `else`
value is evaluated. All case values and `else` must have one result type, and
the expression returns that type directly. Because scores are `Int`, NaN is
not representable in the established utility surface. `Float` scores and a
separate decision-evidence result are not part of this form.

Use this when multiple valid choices compete and you need explicit arbitration.
Avoid this when a single guard decides the branch; guard `when` is the simpler form.

Contrast: if you only need `case tempHigh -> goto Alarm else -> goto Normal`, a guard `when` is enough.


### Judgment enum utility

Standalone `when utility` may name an explicit enum target to make a closed judgment space visible at the expression site:

```oct
when utility PumpJudgment {
    case PumpJudgment.Fault when fault score 100
    case PumpJudgment.Run when pressure > 20.0 score 60
    else PumpJudgment.Hold
}
```

This enum-targeted form is still one-shot utility selection. It supports tag-only variants and explicit single-payload variant construction such as `LabDecision.Retest(3)`, `LabDecision.Treat(2.5)`, and `LabDecision.Escalate("critical")`. Payload expressions are evaluated only for the selected candidate or selected `else` fallback; losing candidate payloads are not evaluated. Utility cases do not bind payloads, and selected payloads are analyzed later with ordinary `match`.

It does not add hidden state, controller commitment memory, hysteresis, `min_commit`, or enum-attached policy. Octomata remains responsible for behavioral progression through states, boards, guard `when`, and controller-bound `when policy`.

A flow can compute an enum judgment with one-shot utility and then use ordinary enum control flow:

```oct
enum PumpJudgment { Hold Run Fault }

flow PumpController(pressure: Float, fault: Bool) -> Int {
    state Decide {
        let judgment = when utility PumpJudgment {
            case PumpJudgment.Fault when fault score 100
            case PumpJudgment.Run when pressure > 20.0 score 60
            else PumpJudgment.Hold
        }
        return switch judgment {
            case PumpJudgment.Hold => 0
            case PumpJudgment.Run => 1
            case PumpJudgment.Fault => 2
        }
    }
}
```

## `hysteresis` and `min_commit` in practice

`hysteresis` and `min_commit` exist to prevent unstable arbitration behavior.

- `hysteresis`: requires a meaningful score gap before switching away from current choice.
  - Practical effect: reduces chatter near threshold ties.
- `min_commit`: forces a chosen policy to stick for a minimum number of ticks/steps.
  - Practical effect: prevents immediate flip-flop from transient noise.

Without them (unstable near threshold):

```oct
package Main

// Tick 1 picks "cool"; Tick 2 tiny score wobble picks "heat"; Tick 3 flips back.
flow UnstableChoice(nearHot: Bool, nearCold: Bool) -> Int {
    state Decide {
        return when policy {
            hysteresis: 0
            min_commit: 1
        } {
            case 1 when nearHot score 50
            case 2 when nearCold score 50
            else 0
        }
    }
}
```

With them (stable arbitration):

```oct
package Main

flow StableChoice(nearHot: Bool, nearCold: Bool) -> Int {
    state Decide {
        return when policy {
            hysteresis: 3
            min_commit: 4
        } {
            case 1 when nearHot score 70
            case 2 when nearCold score 75
            else 0
        }
    }
}
```

This keeps control behavior meaningful instead of reacting to every tiny oscillation.

## Record updates in Octomata-based systems

Octomata examples may use ordinary record `with` updates for immutable data-lane state.
`with` is a general record feature, not an Octomata-specific feature; see [11 Records](../language/11-records.md) for the full syntax and semantics.

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

See also [22 Batch](./22-batch.md) for array-parallel execution constructs.
