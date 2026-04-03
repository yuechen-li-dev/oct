# Mechanics Continuum M32 — Iteration Horizon Probe

## Scope and freeze discipline

M32 is a pure return-on-iteration probe. It keeps M31 fully frozen:

- same fixed `6x6` Cartesian circle fixture
- same strongest local constitutive stack (M28 surface + transported directions + narrow authority)
- same global consistency object (`GlobalConsistencyField2D`)
- same correction family with `alpha = 0.2`
- same four probes (horizontal, vertical, diagonal, opposite diagonal)
- no topology mutation, no convergence logic, no assembly, no solver framework

## Fixed iteration cap and why

- **Cap used: `8`** (`step0` through `step8`).
- Chosen because it is clearly beyond M31's `step5`, yet still a visibly small, explicit, non-solver-like fixed schedule.

## Trivial-improvement criterion (engineering cutoff)

M32 defines practical usefulness with an early-gain-relative rule:

- For each probe, step `k` is **trivial** when `Delta(k) < 0.5 * Delta(1)`.
- Practical horizon `k*` is the smallest step where that condition is true for **all four probes**.

This is intentionally an engineering cutoff, not convergence theory.

## Required outputs tracked

Per probe, per step:

- `Residual(step)` for `step0..step8`
- `Delta(step) = Residual(step-1) - Residual(step)` for `step1..step8`
- `RelativeDelta(step) = Delta(step) / Residual(step-1)`

Also tracked:

- mean residual over the four probes at each step
- mean per-step delta over the four probes
- simple spatial-variation persistence and late-step L1 movement to show structure stabilization without washout

## M32 answers

1. **Max cap used?**
   - `8`.

2. **Criterion used?**
   - `Delta(k) < 0.5 * Delta(1)`; `k*` is first step where all four probes satisfy it.

3. **Residual trajectory shape?**
   - Monotone decrease from `step0` to `step8` across probes and in the mean trajectory.

4. **Per-step gain trajectory shape?**
   - Strictly diminishing deltas from `Delta1` to `Delta8` (diminishing returns, no new regime).

5. **Practical horizon `k*`?**
   - Computed directly from the rule and reported as `PracticalHorizonK`.

6. **Probe-family differences?**
   - Probe-local horizons are reported individually (`HorizontalHorizonK`, `VerticalHorizonK`, `DiagonalHorizonK`, `OppositeDiagonalHorizonK`) and compared via `DiagonalSaturatesLater`.

7. **Structural stabilization by horizon?**
   - Yes, as defined by: nonzero spatial variation retained at `step8` for all probes while mean late-step gain remains below mean first-step gain.

8. **Loop still honest / non-solver-like?**
   - Yes: hard cap only, fixed rule, fixed `alpha`, no stopping logic, no assembly, no topology changes.

9. **Next boundary after M32?**
   - Either:
     - explicitly cross into convergence logic (true solver boundary), or
     - keep fixed-cap loops and strengthen only the global consistency relation.

## Blunt verdict

M32 is intended to identify a practical iteration horizon `k*` where extra passes become cosmetically small relative to early gain, while local constitutive structure is already stabilized and architecture boundaries remain intact.
