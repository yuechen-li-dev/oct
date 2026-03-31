# Octomata

## Overview

Octomata is Oct's explicit behavioral/control runtime model.
Use it when behavior progression matters (modes, transitions, interruption, arbitration), not as a universal replacement for records.

Octomata and records are complementary:

- Octomata: behavior progression and control memory.
- Records: numeric/data lanes and durable payload state.

## Rules

- Flow declaration form is `flow Name(params) -> ReturnType { state ... }`.
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
- Board writes are valid only inside flow state bodies (including nested `if`/`when` inside a state body).
- Utility form `when policy { hysteresis: Int min_commit: Int } { case value when condition score Int ... else value }` is valid only inside flow state bodies.
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
record Telemetry {
    TempC: Float
    Pressure: Float
    WindowAvgTempC: Float
}

flow CoolingController(t: Telemetry, trip: Float) -> String {
    state Evaluate {
        when {
            case t.WindowAvgTempC >= trip -> goto Cooling
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

```oct
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
record Telemetry {
    TempC: Float
    Pressure: Float
}

flow PumpLoop(t: Telemetry) -> String {
    board {
        FaultLatched: Bool
        CooldownTicks: Int
    }

    state Tick {
        if t.TempC > 95.0 {
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
flow DoorControl(openCmd: Bool, closeCmd: Bool, blocked: Bool) -> String {
    state Closed {
        when {
            case openCmd && !blocked -> goto Opening
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

## Utility `when`

Use utility `when` for arbitration and prioritization among competing behavioral policies.
It is not generic branching.

Use utility `when` for:

- display/status arbitration
- alert prioritization
- policy choice under competing conditions

Avoid utility `when` when one guard transition is enough.
Guard `when` is simpler and should be preferred for single-condition transitions.

```oct
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

Use this when multiple valid choices compete and you need explicit arbitration.
Avoid this when a single guard decides the branch; guard `when` is the simpler form.

Contrast: if you only need `case tempHigh -> goto Alarm else -> goto Normal`, a guard `when` is enough.

## `hysteresis` and `min_commit` in practice

`hysteresis` and `min_commit` exist to prevent unstable arbitration behavior.

- `hysteresis`: requires a meaningful score gap before switching away from current choice.
  - Practical effect: reduces chatter near threshold ties.
- `min_commit`: forces a chosen policy to stick for a minimum number of ticks/steps.
  - Practical effect: prevents immediate flip-flop from transient noise.

Without them (unstable near threshold):

```oct
// Tick 1 picks "cool"; Tick 2 tiny score wobble picks "heat"; Tick 3 flips back.
when policy {
    hysteresis: 0
    min_commit: 1
} {
    case 1 when nearHot score 50
    case 2 when nearCold score 50
    else 0
}
```

With them (stable arbitration):

```oct
when policy {
    hysteresis: 3
    min_commit: 4
} {
    case 1 when nearHot score 70
    case 2 when nearCold score 75
    else 0
}
```

This keeps control behavior meaningful instead of reacting to every tiny oscillation.

## `with` in Octomata-based systems

Use `with` for ordinary immutable record updates.
Do not use `with` to replace blackboard-owned behavior memory.

- `with` is for data-lane/state-structure updates.
- Board mutation is for behavior-local control memory.

`with` is correct for record updates:

```oct
record LoopData {
    Samples: Int
    AvgTempC: Float
}

fn PushSample(d: LoopData, sampleAvg: Float) -> LoopData {
    return d with {
        Samples: d.Samples + 1
        AvgTempC: sampleAvg
    }
}
```

Board field update is correct for control memory:

```oct
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

Use `with` and blackboards together, but for different jobs.

See also [22 Batch](./22-batch.md) for array-parallel execution constructs.
