# Mechanics / Continuum M0 Report

## What was introduced

This pass adds a minimal, algebraic continuum mechanics core in `Libraries/Mechanics` (via `Mechanics.Continuum.M0.*`):

1. `RightCauchyGreen2D(F)` implementing `C = F^T F` through indexed contraction.
2. `SmallStrain2D(gradU)` implementing `eps = 0.5 * (gradU + gradU^T)`.
3. `FirstInvariantI1(tensor)` via explicit `Trace(...)`.
4. `LinearIsotropicStress2D(strain, lambda, mu)` implementing the small-strain skeleton
   `sigma = lambda * tr(eps) * I + 2 * mu * eps`.

This is intentionally local/algebraic and avoids PDE operators.

## Formulas successfully expressed

- Kinematics: right Cauchy-Green tensor from deformation gradient.
- Kinematics: symmetric small-strain extraction from displacement gradient.
- Invariant: first invariant through explicit trace.
- Constitutive skeleton: isotropic linear stress from strain and Lamé constants.

The formulas read in recognizable continuum mechanics structure (indices, trace, identity, contraction) rather than flattened matrix utilities.

## Tensor/math substrate relied on

This M0 relies directly on the current tensor/math foundation:

- First-class indices (`Idx`) and indexed tensor access (`A[i, j]`).
- Einstein contraction through repeated indices (`F[k, i] * F[k, j]`).
- Indexed addition with strict free-index matching.
- Scalar/tensor composition in indexed expressions.
- Explicit `Trace(...)` for invariants.

## What felt awkward or missing

- No transpose operator means transpose intent is encoded indirectly via index ordering (`F[k, i]` vs `F[i, k]`). It works, but a first-class transpose surface would improve readability.
- No determinant/J-invariant helper in this pass; volumetric finite-strain paths remain awkward without a canonical determinant/invariant API.
- This pass stayed 2D and local; moving beyond this needs either richer tensor utilities or deliberate differential operators next.

## What was deliberately excluded

- No `Grad`, `Div`, `SymGrad`, or other differential operators.
- No balance-law PDE forms or weak forms.
- No discretization, FE machinery, meshing, or solvers.
- No constitutive family expansion (hyperelastic/plastic/viscoelastic).

## Blunt recommendation

The tensor/math substrate is now sufficient for this minimal continuum mechanics algebra layer.

**Next step should be differential operators** (`Grad`, `Div`, and related field/operator surfaces), because the algebraic local core is now expressive enough that the next bottleneck is field-level formulation, not matrix plumbing.
