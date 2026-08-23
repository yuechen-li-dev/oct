# Optimization

## Relationship to Numerics

`Optimization` is the canonical larger-solver package: multivariate objectives, gradient methods, line search, simplex search, and nonlinear least squares. `Numerics` remains the canonical home for transparent scalar reference methods, including golden-section and Brent bounded one-dimensional minimization. This distinction is compatibility-first; no working API was deleted or renamed.

Mathematical optimization library for Oct. 40 tests, all compiled.

## Packages

- `Optimization.Core` — shared types, vector utilities (Scale, VecAdd, VecSub, Dot, L2Norm, LInfNorm, RSS)
- `Optimization.LineSearch` — Armijo backtracking, Wolfe condition checks
- `Optimization.GradientDescent` — steepest descent + Armijo line search, momentum variant
- `Optimization.NelderMead` — derivative-free simplex for 2-10 parameters
- `Optimization.LeastSquares` — Gauss-Newton, Levenberg-Marquardt, curve fitting

## Quick Reference

```oct
// Gradient descent (requires analytic gradient)
let result = GradientDescent(f, grad, x0, gTol, maxIter)!

// Nelder-Mead (derivative-free, 2-10 parameters)
let result = NelderMead(f, x0, xTol, fTol, maxIter)!

// Curve fitting (Levenberg-Marquardt)
let result = FitCurve(model, params0, xData, yData, rTol, maxIter)!

// Line search building block
let ls = DefaultArmijoLineSearch(x, d, f, fx, gd)!
```

## Dimensional Boundary

All parameters are dimensionless `Float`. Strip units before passing,
restore after. This is explicit by design — see Core.oct comments.

## Known Issues

See `FRICTION.md` for upstream fixes needed:
- Float[] assignment is reference copy (critical — requires deep copy fix in compiler)
- fn(Float[]) -> X parameters broken in interpreter (works in compiled mode)
- Nested index assignment not supported for board fields
