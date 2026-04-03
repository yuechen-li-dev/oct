# Mechanics Continuum M26 — Signed / Off-Axis Coupling Semantics Probe

## Scope and constraints respected

M26 keeps the same fixed Cartesian lattice and circle fixture, remains local/cell-evaluable, and does not introduce mesh/topology mutation, global solve machinery, matrix assembly, or reusable constitutive frameworks.

Field separation is preserved:
- geometry carrier: circle-membership/SDF-like local inclusion
- material-direction carrier: transported tangent/normal frame
- authority/confidence carrier: independent scalar field
- constitutive response object: `ConstitutiveResponseField2D { TT, NN, TN }`

## Three coupling semantics tested

With probe `p`, tangent `t`, normal `n`, projections
- `pt = dot(p, t)`
- `pn = dot(p, n)`

and fixed coefficients
- `R.tt = 1.8`
- `R.nn = 0.2`
- `R.tn = 0.15`

M26 compares:

1. **Path A — Magnitude-only coupling (M25 control)**

   `response = R.tt*abs(pt) + R.nn*abs(pn) + R.tn*abs(pt*pn)`

2. **Path B — Signed coupling**

   `response = R.tt*abs(pt) + R.nn*abs(pn) + R.tn*(pt*pn)`

3. **Path C — Off-axis semantic variant (narrow explicit extension)**

   Signed coupling plus a mixed-sign diagonal gate:

   - if `(pt*pn < 0)` and `abs(pt) > 0.2` and `abs(pn) > 0.2`, scale coupling by `1.5`
   - otherwise keep signed coupling scale `1.0`

   Formula:

   `response = R.tt*abs(pt) + R.nn*abs(pn) + asymmetryScale*R.tn*(pt*pn)`

This is intentionally small, deterministic, interpretable, and distinct from both `abs(product)` and raw signed-product semantics.

## Probe set (exactly four)

- horizontal `(1, 0)`
- vertical `(0, 1)`
- diagonal `(1, 1)/sqrt(2)`
- opposite diagonal `(1, -1)/sqrt(2)`

## Evidence summary

- **Semantic unlock (A):** signed coupling produces measurable L1 differences vs magnitude-only on diagonal and opposite-diagonal probes.
- **Beyond signed (A):** the off-axis variant produces additional measurable L1 differences beyond signed coupling.
- **Probe sensitivity (B):** diagonal and opposite-diagonal probes expose coupling-semantic differences more strongly than horizontal/vertical probes.
- **Spatial coherence (C):** transported material direction continues to produce structured spatial variation; static direction collapses it.
- **Noise vs meaning (D):** sign-sensitive behavior is structured, not random: magnitude-only has near-zero diagonal family asymmetry, signed adds nonzero asymmetry, and off-axis explicitly increases that asymmetry under mixed-sign conditions.
- **Static inadequacy (E):** static frame remains less informative than transported direction.
- **Architectural cleanliness (F):** field separation remains intact with richer local semantics.
- **Need for authority (G):** authority still does not need to participate in constitutive evaluation for this boundary.

## Required blunt answers

1) **What three coupling semantics were tested?**
- Magnitude-only, signed, and mixed-sign-thresholded off-axis signed coupling.

2) **What exact formula did each use?**
- Listed above in the “Three coupling semantics tested” section.

3) **Was magnitude-only coupling too limited?**
- Yes for the next boundary: it cannot distinguish diagonal sign families (`(1,1)` vs `(1,-1)`) in coupling contribution.

4) **Did signed coupling add meaningful behavior?**
- Yes. It introduces diagonal-family sensitivity and measurable constitutive differences.

5) **Did the off-axis variant add anything beyond signed coupling?**
- Yes. It adds explicit mixed-sign asymmetry amplification under interpretable gating.

6) **Which probes exposed the differences most clearly?**
- Diagonal and opposite-diagonal probes.

7) **Did transported material direction remain useful?**
- Yes. It remains the informative constitutive input path; static direction under-resolves spatial structure.

8) **Did authority need to enter yet?**
- No. Authority remains architecturally present but not constitutively required in M26.

9) **What is the next boundary after M26?**
- First priority from evidence: richer local coupling semantics are now validated; the next honest boundary is whether authority-modulated constitutive participation adds interpretable signal **without** breaking locality/field separation. Global solve and full tensor mechanics remain premature for this step.
