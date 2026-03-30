# DifferentialEquations ODE M0

## Purpose

`Libraries/DifferentialEquations` provides a deterministic, fixed-step initial value problem surface for ordinary differential equations.

## Solver surface

- `EulerStep(f, t, y, dt) -> Float ! Error`
- `EulerSolve(f, t0, y0, dt, steps) -> ODESolution ! Error`
- `RK4Step(f, t, y, dt) -> Float ! Error`
- `RK4Solve(f, t0, y0, dt, steps) -> ODESolution ! Error`

## State/time scope

ODE M0 is currently **scalar-only**:
- time: dimensionless `Float`
- state: dimensionless `Float`
- derivative callback shape: `f(t: Float, y: Float) -> Float`

## Output shape

`EulerSolve` and `RK4Solve` return:
- `ODESolution.Times: Float[]`
- `ODESolution.Values: Float[]`

Both arrays include the initial condition at index `0` and then one appended entry per step.

## Deterministic fixed-step behavior

- `dt` is explicit and never adapted.
- `steps` is explicit and drives exact iteration count.
- validation rejects `dt == 0` and `steps <= 0`.

## Non-goals (M0)

No adaptive step size, implicit/stiff solvers, events, DAEs, PDEs, symbolic workflows, sensitivity analysis, or stochastic differential equations.
