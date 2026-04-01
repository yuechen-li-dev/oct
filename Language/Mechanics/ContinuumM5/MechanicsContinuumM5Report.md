# Mechanics / Continuum M5 report

## What richer mechanics distinction was introduced

M5 introduces a first-class **plane stress vs plane strain constitutive split** for isotropic small-strain 2D mechanics, while remaining representational and pre-discretization.

Concretely, M5 adds an explicit effective Lamé helper for plane stress,

- `lambda_ps = (2 * mu * lambda) / (lambda + 2 * mu)`

and then exposes two narrow constitutive surfaces:

- plane strain in-plane stress: `sigma = LinearIsotropicStress2D(eps, lambda, mu)`
- plane stress in-plane stress: `sigma = LinearIsotropicStress2D(eps, lambda_ps, mu)`

This is a real mechanics distinction (assumption-level, not syntactic sugar), and it stays small.

## Why this was the right M5 choice

- It is a standard small-strain continuum distinction engineers expect.
- It materially increases usefulness without creating a constitutive zoo.
- It pressures the substrate with richer constitutive composition rather than additional operator seam repair.

## What substrate features it relied on

M5 uses the existing representational small-strain field-form substrate directly:

- `SymGrad(u)` for strain-like field-form construction.
- `Trace(eps)` for volumetric part extraction.
- indexed tensor composition for constitutive assembly.
- `Div(...) + b` for strong-form residual-shaped expression.

No new foundational operator or tensor plumbing was required.

## What remained awkward

- Assumption state is still encoded by function choice rather than an explicit typed assumption object/tag.
- There is still no canonical shared continuum package under `Mechanics/Continuum`; helpers currently live in the existing mechanics library file and in language-contract probes.
- Parameter admissibility (e.g., physically valid `lambda, mu` regimes) remains outside this pass, which is acceptable for representational M5 but will matter for later usability.

## Did this feel like real mechanics progress?

Yes. The pass adds an assumption-level constitutive distinction used routinely in continuum mechanics and shows it composes cleanly with the existing field-form chain. The remaining friction is now mechanics API design, not missing representation primitives.

## Recommendation for next step

Proceed with **richer continuum mechanics (still narrow)**, likely one additional helper pass focused on compact, explicit engineering parameter surfaces (e.g., a narrow Young's modulus / Poisson ratio bridge), while staying pre-discretization.
