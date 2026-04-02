# Mechanics Continuum M33 — Strengthened Global Consistency Probe

M33 keeps the M32 loop frozen and tests only the global consistency relation.

## What changed (and what did not)

- **Path A (control):** unchanged M32 baseline consistency operator.
- **Path B (M33):** one strengthened consistency operator with:
  - direction-weighted pair influence from transported tangents,
  - authority-gated propagation via existing confidence carrier,
  - tiny bounded two-hop contribution (`beta=0.1`).

Frozen from M32:

- same local constitutive model and transport/authority construction,
- same correction family and `alpha=0.2`,
- same fixed iteration cap (`8`) and same horizon criterion,
- same probe set: horizontal, vertical, diagonal, opposite diagonal,
- no convergence logic, no assembly, no topology change.

## Required M33 answers

1. **Early gain improvement (`Delta(1)`, `Delta(2)`)**
   - Reported explicitly as `MeanDelta1_M32 vs MeanDelta1_M33` and `MeanDelta2_M32 vs MeanDelta2_M33`.

2. **Practical horizon (`k*`)**
   - Reported as `PracticalHorizonK_M32 vs PracticalHorizonK_M33` and mean residual at each path's `k*`.

3. **Probe-family behavior**
   - `DiagonalImprovementVsAxial` indicates whether diagonals gain more than axial probes under strengthened consistency.

4. **Stability**
   - `StableMonotone` enforces monotone residual and monotone diminishing deltas over the fixed schedule.

5. **Local structure survival**
   - `LocalStructurePreserved` requires spatial variation persistence while late-step movement shrinks.

6. **Architectural cleanliness**
   - `FieldSeparationIntact` and `NonSolverLike` remain true by construction: explicit records/functions, no solver framework.

## Blunt boundary statement

M33 succeeds only if smarter consistency improves early trajectory behavior (or equal horizon with lower residual quality) while retaining monotone stability and explicit non-solver architecture.

Next boundary after M33 is evidence-driven:

- if gains are clear and stable: one more explicit global-operator strengthening pass,
- otherwise: first explicit convergence boundary (acknowledged solver step),
- do not guess beyond measured comparison.
