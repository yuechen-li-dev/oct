# Mechanics / Continuum M6 report

## What engineering parameter bridge was added

M6 adds one narrow, explicit engineering-facing bridge for isotropic small-strain 2D:

- `MuFromYoungPoisson(E, nu)`
- `LameLambdaFromYoungPoisson(E, nu)`
- `LinearIsotropicStressPlaneStrain2DFromYoungPoisson(strain, E, nu)`
- `LinearIsotropicStressPlaneStress2DFromYoungPoisson(strain, E, nu)`

This is intentionally a single parameter surface (`E`, `nu`) mapped into the existing Lamé-based constitutive form.

## Why this was the right next step

M5 already established that the representational substrate can express a real constitutive distinction (plane stress vs plane strain). The remaining friction was API ergonomics: engineers commonly think in (`E`, `nu`) first, not (`lambda`, `mu`).

M6 addresses exactly that friction without adding broader constitutive infrastructure.

## Internal mapping

The bridge is explicit and direct:

- `mu = E / (2 * (1 + nu))`
- `lambda = E * nu / ((1 + nu) * (1 - 2 * nu))`
- plane strain: `sigma = LinearIsotropicStress2D(eps, lambda, mu)`
- plane stress: `sigma = LinearIsotropicStress2D(eps, PlaneStressLambda2D(lambda, mu), mu)`

Assumption state remains visible at call sites through distinct function names for plane stress vs plane strain.

## Physical admissibility policy in M6

No new physical-admissibility framework was added.

M6 intentionally relies on the existing typed language boundary (e.g., scalar argument types) and keeps this pass representational. It does **not** enforce range checks like `nu != 0.5` or positivity constraints.

## Usability result

For narrow isotropic small-strain 2D field-form authoring, the library now feels materially more usable:

- users can enter with `E`, `nu`
- plane stress vs plane strain remains explicit
- formulas compose directly with `SymGrad`, `Trace`, `Div`, and residual-shaped expressions

This is meaningful modeling-layer progress, not substrate repair.

## What still feels awkward

- Assumption state is still encoded by function naming rather than a dedicated assumption type.
- Parameter admissibility is still entirely user-managed.

Both are currently acceptable for the narrow representational scope.

## Recommendation for next step

One more narrow mechanics/API pass is the cleanest next move:

- add lightweight assumption typing for plane stress vs plane strain while preserving explicitness
- keep constitutive scope narrow
- stay pre-discretization

Discretization groundwork should come after this assumption-surface cleanup.
