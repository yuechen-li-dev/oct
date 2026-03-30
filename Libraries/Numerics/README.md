# Numerics M0

## Scope

`Libraries/Numerics` provides a deterministic scalar root-finding core:

- `BisectionRoot(f, a, b, tolerance, maxIterations)`
- `NewtonRoot(f, df, x0, tolerance, maxIterations)`

Numerics M0 is intentionally **scalar-only (`Float`)** and dimensionless-only.

## Result shape

Both solvers return the same explicit record:

- `RootResult { Root, Value, Iterations, Converged }`

## Convergence and stopping

- Primary convergence check: `Abs(f(x)) <= tolerance`
- Bisection also accepts interval-width convergence when `(right - left) / 2 <= tolerance`
- Newton also accepts update-size convergence when `Abs(delta) <= tolerance`
- If no convergence criteria are met, the solver stops at `maxIterations` with `Converged = false`

## Validation policy

`BisectionRoot` rejects with `Error` when:

- `a >= b`
- `tolerance <= 0`
- `maxIterations <= 0`
- `f(a)` and `f(b)` do not form a sign-change bracket

`NewtonRoot` rejects with `Error` when:

- `tolerance <= 0`
- `maxIterations <= 0`
- derivative magnitude is too small for a safe update

Newton's derivative safety threshold is explicit and deterministic:

- `Abs(df(x)) <= 1e-12` is rejected

## Explicit non-goals (M0)

Out of scope: multidimensional root finding, Jacobian-based solves, automatic differentiation,
polynomial/root-family frameworks, continuation methods, interval arithmetic, and solver-zoo designs.
