# Simulation M0

## Purpose

`Libraries/Simulation` provides explicit deterministic fixed-step simulation helpers authored on top of Octomata `flow` + `state`.

## M0 scope

- fixed-step (`dt` is explicit)
- scalar (`Float`) state and output
- deterministic progression with explicit step count
- trace collection through explicit append operations

## Core shapes

- `SimulationStepResult { Time, State, Output }`
- `SimulationTrace { Times, States, Outputs }`

## API surface

- `ValidateFixedStepSetup(dt, steps)`
- `InitializeTrace(initialTime, initialState, initialOutput)`
- `AppendStep(trace, stepResult)`
- `ValidateTraceShape(trace)`

## Architectural rule

Simulation is built on Octomata execution primitives. Canonical simulations are authored with `flow` and `state`, using explicit `goto`/`suspend`/`return` progression.

## Non-goals

Simulation M0 is **not**:

- a Simulink clone
- a block-diagram framework
- a hidden scheduler/execution-order engine
- an event system
- a variable-step or adaptive-time solver platform
