# Mechanics Continuum M31 — First True Iterative Solve Boundary Probe

## Scope and non-negotiables

M31 only promotes the correction schedule from M30's bounded `step0/step1/step2` to a tiny explicit iterative loop, while preserving all prior boundaries:

- same fixed `6x6` Cartesian circle fixture
- same M28/M29/M30 local constitutive stack
- same global consistency object (`GlobalConsistencyField2D`)
- same correction family and `alpha=0.2`
- no topology mutation, no mesh changes, no assembly, no solver abstraction framework

## Iteration cap and why

- **Hard cap used: `5` iterations** (`step0` through `step5`).
- Chosen as the smallest cap that is clearly beyond M30's bounded `step2` while still staying visibly thin and explicit.

## Modes compared

- **Path A**: `step0` local-only control.
- **Path B**: `step1` one-shot correction (M29 baseline).
- **Path C**: `step2` bounded two-step correction (M30 baseline).
- **Path D**: explicit iterative loop reapplying the same correction from `step0` to `step5` with hard cap `5`.

## Correction rule (unchanged)

Per pass/iteration:

- `corrected = (1 - alpha) * local + alpha * neighborMean`
- mismatch-activated using `GlobalConsistencyField2D.LocalMismatch > 0`
- `alpha = 0.2`

No retuning and no new correction family.

## Probe set (unchanged)

- horizontal `(1,0)`
- vertical `(0,1)`
- diagonal `(1,1)/sqrt(2)`
- opposite diagonal `(1,-1)/sqrt(2)`

## What M31 answers

1. **Did tiny explicit iteration buy something beyond M30 step2?**
   - **Yes, quantitatively and consistently.** `step5` residual beats `step2` on all required probes.

2. **Did it unlock a qualitatively new regime?**
   - **Mostly no.** Behavior continues the same diminishing-return trend seen in M30, now extended across additional fixed iterations.

3. **What does residual trajectory look like under fixed cap?**
   - **Monotone drop** from `step0` to `step5` for the required probes and mean pair residual.

4. **Did local constitutive structure survive?**
   - **Yes.** Nontrivial spatial variation remains at `step5`; fields are not washed flat.

5. **Did the loop remain honest or become solver-like?**
   - **Still controlled/honest at this cap.** There is first solver smell (explicit iteration exists), but no convergence criterion, no assembly, no generic solver engine, and no abstraction explosion.

6. **Did field separation survive?**
   - **Yes.** Iterative state is just repeated application over existing carriers.

7. **Did authority remain narrow?**
   - **Yes.** Authority remains in local constitutive coupling only; it does not enter iteration policy.

8. **What is next boundary after M31?**
   - Either:
     - cross into real solver territory by adding explicit convergence logic (and acknowledge boundary crossing), or
     - keep hard-cap explicit loops but strengthen global consistency relation while still avoiding assembly.

## Blunt verdict

M31 proves a tiny explicit iterative loop does add measurable residual reduction beyond M30's bounded step2 baseline. However, it does **not** reveal a wholly new qualitative regime; it mostly extends the same saturation story across more fixed steps. Crucially, local structure, field separation, and narrow authority scope survive, and the loop still reads as a constrained correction layer rather than full solver machinery.
