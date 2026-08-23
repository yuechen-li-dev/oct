# Numerics M0

## Scope

`Libraries/Numerics` provides readable deterministic scalar numerical methods:

- `BisectionRoot(f, a, b, tolerance, maxIterations)`
- `NewtonRoot(f, df, x0, tolerance, maxIterations)`
- `SecantRoot` and the bracketed hybrid `BrentRoot`
- forward, backward, central, second, and Richardson finite differences
- trapezoid, Simpson, five-point Gauss-Legendre, and adaptive Simpson quadrature
- golden-section and Brent scalar minimization

Numerics M0 is intentionally **scalar-only (`Float`)** and dimensionless-only.

## Result shape

Root solvers return the same explicit record:

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

## Adaptive quadrature diagnostics

`AdaptiveSimpson` returns `IntegrationResult { Value, Iterations, Converged }`, where `Iterations` is the actual function-evaluation count. Refinement compares one Simpson panel with its two half-panels. If the error estimate misses tolerance when `maxDepth` is exhausted, the best corrected estimate is returned with `Converged = false`; depth exhaustion is never reported as success.

These are transparent reference algorithms suitable for moderate scalar problems. They do not claim production behavior for singular integrands, ill-conditioned roots, or extreme floating-point scales.
