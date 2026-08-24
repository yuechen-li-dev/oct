# DifferentialEquations ODE M0

## Purpose

`Libraries/DifferentialEquations` provides a deterministic, fixed-step initial value problem surface for ordinary differential equations.

This package owns mathematical ODE integration methods. [`Simulation`](../Simulation/README.md) owns scenario transitions, observation traces, and parameter sweeps; physical equations belong in packages such as [`Thermofluids`](../Thermofluids/README.md) or [`Electromagnetism`](../Electromagnetism/README.md). Start with the RK4 progression below.

## Solver surface

- `EulerStep(f, t, y, dt: ODEStep) -> Float`
- `EulerSolve(f, t0, y0, dt: ODEStep, steps: ODEStepCount) -> ODESolution`
- `MidpointStep(f, t, y, dt: ODEStep) -> Float`
- `MidpointSolve(f, t0, y0, dt: ODEStep, steps: ODEStepCount) -> ODESolution`
- `RK4Step(f, t, y, dt: ODEStep) -> Float`
- `RK4Solve(f, t0, y0, dt: ODEStep, steps: ODEStepCount) -> ODESolution`

## State/time scope

ODE M0 is currently **scalar-only**:
- time: dimensionless `Float`
- state: dimensionless `Float`
- derivative callback shape: `f(t: Float, y: Float) -> Float`

## Output shape

All three solve functions return:
- `ODESolution.Times: Float[]`
- `ODESolution.Values: Float[]`

Both arrays include the initial condition at index `0` and then one appended entry per step.

## Deterministic fixed-step behavior

- `dt` is explicit and never adapted.
- `steps` is explicit and drives exact iteration count.
- validation rejects `dt == 0` and `steps <= 0`.

## Executable progression

For `y' = y`, `y(0) = 1`, one step of size `0.2` demonstrates the expected accuracy ladder:

```oct
fn Growth(t: Float, y: Float) -> Float { return y }

let euler = EulerStep(Growth, 0.0, 1.0, 0.2)
let midpoint = MidpointStep(Growth, 0.0, 1.0, 0.2)
let rk4 = RK4Step(Growth, 0.0, 1.0, 0.2)
// |rk4-exp(0.2)| < |midpoint-exp(0.2)| < |euler-exp(0.2)|
```

Euler is first order, explicit midpoint/RK2 is second order, and classical RK4 is fourth order. These are fixed-step teaching solvers, not stiff or adaptive production solvers.

`ODEStep` permits forward or backward integration but rejects zero. `ODEStepCount` requires at least one step. Literal arguments are statically proved; unknown runtime values use their checked Concept constructors. Solver bodies are therefore infallible after domain admission.

## Non-goals (M0)

No adaptive step size, implicit/stiff solvers, events, DAEs, PDEs, symbolic workflows, sensitivity analysis, or stochastic differential equations.
