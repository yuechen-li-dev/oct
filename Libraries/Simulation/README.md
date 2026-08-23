# Simulation M0

## Purpose

`Libraries/Simulation` provides a compact deterministic scalar simulation runner, aligned trace analysis, and ordered parameter sweeps. Octomata `flow` remains available when a model genuinely needs suspension or explicit behavioral states.

## M0 scope

- fixed-step (`dt` is explicit)
- scalar (`Float`) state and output
- deterministic progression with explicit step count
- trace collection through explicit append operations
- callback-defined transition and observation functions
- trapezoidal integration of recorded outputs
- deterministic `batch` parameter sweeps

## Core shapes

- `SimulationStepResult { Time, State, Output }`
- `SimulationTrace { Times, States, Outputs }`
- `FixedStepConfig { InitialTime, Dt: PositiveSimulationStep, Steps: PositiveSimulationStepCount }`

## API surface

- `ValidateFixedStepSetup(dt, steps)`
- `InitializeTrace(initialTime, initialState, initialOutput)`
- `AppendStep(trace, stepResult)`
- `ValidateTraceShape(trace)`
- `RunFixedStep(transition, observe, initialState, config)`
- `FinalStep(trace)`
- `OutputIntegral(trace)`
- `EvaluateParameterSweep(parameters, experiment)`

## Executable decay example

```oct
fn Decay(t: Float, state: Float, dt: Float) -> Float {
    return state - dt * state
}

fn Observe(t: Float, state: Float) -> Float { return state }

let base = FixedStepConfig { InitialTime: 0.0 Dt: 0.2 Steps: 5 }
let refined = base with { Dt: 0.1 Steps: 10 }
let trace = RunFixedStep(Decay, Observe, 1.0, refined)!
```

Ordinary bounded fixed-step models should use `RunFixedStep`: the transition equation stays visible and the trace is deterministic. Use `flow` only when resumability, turns, or explicit state-machine progression is itself part of the scientific model. This separates numerical stepping from behavioral orchestration.

The config's refined Concepts reject non-positive time steps and step counts at compile time when known, or at one explicit checked-construction boundary when values arrive at runtime. Trace shape, monotonic recorded times, and non-empty final/integral operations remain fallible because they are relational runtime invariants.

## Non-goals

Simulation M0 is **not**:

- a Simulink clone
- a block-diagram framework
- a hidden scheduler/execution-order engine
- an event system
- a variable-step or adaptive-time solver platform
- a replacement for `DifferentialEquations` ODE algorithms
