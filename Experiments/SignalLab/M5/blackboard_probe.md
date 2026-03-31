# SignalLab M5 — Blackboard Thought Experiment Probe

This file is a design sketch only. It does **not** add runtime features, language features, or syntax.

## M4 pressure-point set used for comparison

Behavior-local responsibilities carried over from M4:

- `ControlMode`
- `DisplayMode`
- `AlertStatus`
- `FaultLatched`
- interruption/resume memory (e.g. previous mode before `Inspect` / `FaultHold`)
- policy/arbitration outputs and supporting counters (e.g. commit/hysteresis memory)

Numeric/data-lane responsibilities intentionally excluded from the blackboard target:

- sample value generation
- time/tick advancement
- history buffer payloads

## Candidate A — Flow-local mutable bindings

Concept: each flow/state can declare mutable locals.

Pseudo sketch:

```oct
flow ControlDirector {
  state Running {
    mut fault_latched: Bool = false
    mut display_mode: String = "Scope"
    mut alert_status: String = "Nominal"
    mut resume_mode: String = "Running"

    when EventFaultTrip => {
      fault_latched = true
      resume_mode = remember_state_name()
      goto FaultHold
    }
  }
}
```

## Candidate B — Special flow blackboard record (preferred)

Concept: each flow has one explicit, typed behavior-working-state record (`board`) that is readable/writable only inside flow/state contexts.

Pseudo sketch:

```oct
record ControlBoard {
  FaultLatched: Bool
  DisplayMode: String
  AlertStatus: String
  ResumeMode: String
  PolicyCommitCount: Int
}

flow ControlDirector(board: ControlBoard) {
  state Running {
    when EventInspect => {
      board.ResumeMode = "Running"
      suspend "Inspect"
    }

    when EventFaultTrip => {
      board.FaultLatched = true
      board.AlertStatus = "Fault"
      suspend "FaultHold"
    }

    when EventAcknowledge and board.FaultLatched => {
      board.FaultLatched = false
      resume board.ResumeMode
    }
  }
}
```

## Candidate C — Typed slot/key model

Concept: a typed key registry and get/set API.

Pseudo sketch:

```oct
slot FaultLatched: Bool
slot DisplayMode: String
slot AlertStatus: String
slot ResumeMode: String

flow ControlDirector(board: Slots) {
  state Running {
    when EventFaultTrip => {
      board.set(FaultLatched, true)
      board.set(AlertStatus, "Fault")
      board.set(ResumeMode, current_state_name())
      goto FaultHold
    }
  }
}
```

## Candidate D (optional) — Specialized flow context object

Concept: strongly shaped APIs exposed as `ctx`, but values hidden behind methods.

Pseudo sketch:

```oct
flow ControlDirector(ctx: ControlContext) {
  state Running {
    when EventFaultTrip => {
      ctx.latch_fault()
      ctx.raise_alert("Fault")
      ctx.capture_resume_mode(current_state_name())
      goto FaultHold
    }
  }
}
```

## Narrowing decision

Preferred: **Candidate B (special flow blackboard record)**.

Rejected:

- Candidate A: too close to generic mutable locals; hard to constrain spread.
- Candidate C: explicit but review-hostile once key usage scales; weak locality.
- Candidate D: safe but too framework-like, hides writes behind methods and drifts from Oct directness.

Provisional shape recommendation:

- one explicit, typed `board` per flow instance
- board fields declared up front (fixed shape; no dynamic keys)
- write access allowed only inside flow/state transitions
- board passed/read out explicitly at flow boundaries
- data-lane numeric/history state remains immutable records outside the board

Open questions before implementation:

- exact boundary for board I/O at reducer seams
- whether utility-policy internal memory should be board fields or private flow internals
- reset/initialization semantics across machine restarts
- test-surface style for board expectations without overcoupling tests to internals
