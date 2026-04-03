# Mechanics Continuum M34 — Controlled Convergence Boundary Probe

M34 freezes M32/M33 mechanics (same circle, carriers, constitutive surface, global consistency object, correction family, `alpha=0.2`, and 8-step hard cap) and changes only execution policy.

## Explicit early-exit rule used

- **Fixed cap remains hard:** maximum 8 iterations.
- **Rule basis:** M32 diminishing-return criterion `Delta(k) < 0.5 * Delta(1)`.
- **Operationalization in M34:** for each probe, stop at the smallest `k` in `[6..8]` that satisfies the criterion.
  - This keeps the M32 rule explicit, tiny, and hand-written.
  - It avoids introducing any generic convergence framework.

## Aggregate comparison table

| Metric | Fixed-8 | Early-Exit |
|---|---:|---:|
| MeanIterationsUsed | 8.000000 | 7.000000 |
| MeanResidualFinal | 4.773679 | 5.415061 |
| MeanResidualReduction | 7.934328 | 7.292946 |
| MeanResidualReductionPerIteration | 0.991791 | 1.041849 |
| MeanSavedIterations | 0.000000 | 1.000000 |
| MaxIterationsUsed | 8 | 8 |
| MinIterationsUsed | 8 | 6 |

### Direct read

- **Iteration savings:** yes (mean savings = **1.0** iteration).
- **Residual quality retention:** early-exit keeps about **91.9%** of fixed-8 mean reduction (`7.292946 / 7.934328`).
- **Efficiency:** reduction-per-iteration improves from **0.991791** to **1.041849**.

## Per-probe comparison table

| Probe | IterUsed | Residual@Exit | Residual@8 | SavedIterations | EarlyExitAcceptable? |
|---|---:|---:|---:|---:|---|
| Horizontal | 8 | 3.767721 | 3.767721 | 0 | Yes |
| Vertical | 8 | 3.767721 | 3.767721 | 0 | Yes |
| Diagonal | 6 | 7.062400 | 5.779636 | 2 | Yes |
| Opposite diagonal | 6 | 7.062400 | 5.779636 | 2 | Yes |

## Required M34 judgments

1. **Did early exit preserve most useful reduction?**
   - **Yes, mostly.** Mean retained reduction is ~91.9% with fewer iterations.
2. **How many iterations were saved?**
   - **1.0 mean iteration saved** (8.0 → 7.0).
3. **Did any probe family degrade disproportionately?**
   - **No severe collapse observed.** Axial probes stay at cap; diagonal-family exits earlier with moderate residual penalty.
4. **Did practical efficiency improve?**
   - **Yes.** Mean reduction-per-iteration increased (~+5.0%).
5. **Did policy stay explicit and non-solver-like?**
   - **Yes.** Hard cap + tiny explicit rule; no tolerances framework, no solver interfaces, no assembly.
6. **Did field separation survive?**
   - **Yes.** Geometry/material/authority/constitutive/global/policy objects remain separate.
7. **What is the next boundary after M34?**
   - If this bounded regime is acceptable, treat it as the operational default for this line.
   - If higher diagonal fidelity is required under tighter budgets, the next honest boundary is the **first true convergence/solver boundary** (explicitly acknowledged as a different regime).

## Blunt verdict

**M34 supports bounded early-exit as an operational policy:** it saves compute, keeps most residual reduction, stays stable and architecturally honest, and does so without solver-framework creep.
