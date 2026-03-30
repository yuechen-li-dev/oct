# Optimization M0

## Scope

`Libraries/Optimization` provides a small deterministic unconstrained optimization core:

- `GoldenSectionSearch(f, a, b, tolerance, maxIterations)`
- `GradientDescentStep(x, gradient, stepSize)`
- `GradientDescentSolve(f, grad, x0, stepSize, tolerance, maxIterations)`

This is **not** a modeling DSL and **not** a solver framework.

## Algorithms and limits

- `GoldenSectionSearch`: 1D, bounded interval, derivative-free local minimization inside `[a, b]`
- `GradientDescentSolve`: fixed-step gradient descent with user-supplied gradient

Optimization M0 is currently **scalar-only (`Float`)** and **dimensionless-only**.

## Result records

- `OptimizationResult1D { Point, Value, Iterations, Converged }`
- `OptimizationResult { Point, Value, Iterations, Converged }`

Both result shapes are explicit and deterministic.

## Convergence and stopping

- Golden section converges when interval width `<= tolerance`, otherwise stops at `maxIterations`
- Gradient descent converges when `Abs(grad(x)) <= tolerance`, otherwise stops at `maxIterations`

## Invalid-input policy

Functions reject invalid setup with `Error`:

- invalid search interval (`a >= b`)
- `tolerance <= 0`
- `maxIterations <= 0`
- `stepSize <= 0` for gradient descent

## Explicit non-goals

Out of scope in M0: constrained optimization, LP/QP/MILP, Newton/quasi-Newton families,
automatic differentiation, numerical differentiation frameworks, stochastic optimizers,
line-search frameworks, and multi-start/global optimization features.
