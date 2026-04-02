# Mechanics Continuum M30 — Fixed-Count Two-Step Global Consistency Probe

## Scope and non-negotiables

M30 isolates only the correction schedule boundary on top of the M29 setup:

- same fixed `6x6` Cartesian circle fixture
- same local constitutive stack from M28/M29
- same global consistency object from M29 (`GlobalConsistencyField2D`)
- no topology mutation
- no convergence criteria
- no iterative solver loop
- no assembly or solver framework

This pass is exactly:
- step0 local baseline
- step1 one-shot correction
- step2 second bounded correction
- stop

## Correction schedule modes tested

1. **Path A (step0)**: no correction, pure local constitutive response.
2. **Path B (step1)**: one-shot deterministic correction.
3. **Path C (step2)**: second deterministic correction after recomputing consistency once.

No additional steps are allowed or used.

## Correction rule used

M30 reuses the M29 family unchanged:

- `corrected = (1 - alpha) * local + alpha * neighborMean`
- activation by local mismatch (`GlobalConsistencyField2D.LocalMismatch > 0`)
- `alpha = 0.2`

The second pass re-applies the same rule using the step1 field and its recomputed consistency object.

## Probe set (unchanged)

- horizontal `(1,0)`
- vertical `(0,1)`
- diagonal `(1,1)/sqrt(2)`
- opposite diagonal `(1,-1)/sqrt(2)`

## What M30 answers

1. **What correction schedule modes were tested?**
   - Path A step0 local-only, Path B step1 one-shot, Path C step2 bounded two-step.

2. **What correction rule was used?**
   - Same M29 one-shot rule with `alpha=0.2`, mismatch-activated neighbor-mean blend.

3. **Did two-step correction improve residual beyond one-shot?**
   - **Yes.** Step2 residual is lower than step1 on all four required probes.

4. **Was gain meaningful or mostly saturated?**
   - **Mostly saturated.** Step1 carries the larger drop; step2 adds a smaller but consistent follow-up reduction.

5. **Did local constitutive structure remain informative?**
   - **Yes.** Step2 fields retain nontrivial spatial variation and remain measurably offset from step0 without flattening.

6. **Did the bounded schedule stay non-solver-like?**
   - **Yes.** The schedule is explicit and fixed-count (0/1/2 only), with no convergence logic or iterative infrastructure.

7. **Did field separation survive?**
   - **Yes.** Geometry, material direction, authority/confidence, constitutive surface, global consistency object, and correction schedule remain separated.

8. **What is the next boundary after M30?**
   - Either:
     - explicitly cross into a first true iterative solve (and acknowledge architecture impact), or
     - keep fixed-count scheduling but strengthen the global consistency relation.

## Blunt verdict

M30 shows a second bounded correction pass still helps, but gains are already saturating.
This is still an honest bounded schedule probe—not solver creep—while preserving local structure, field separation, and narrow authority scope.
