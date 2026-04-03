# Mechanics / Continuum M2 report

## Field-form relations introduced

This pass adds a narrow strong-form small-strain continuum slice with explicit representational operators:

1. `u` (displacement-like vector field)
2. `eps = SymGrad(u)`
3. `sigma = (2 * mu * I) * SymGrad(u)` (narrow constitutive slice with `lambda = 0`)
4. `r = Div(SymGrad(u)) + b`

The contracts were added under `Language/Mechanics/ContinuumM2`.

In addition, `Libraries/Mechanics/Mechanics.Continuum.M2.octest` probes both:
- the `u -> SymGrad(u) -> sigma` representational constitutive path and `Div(SymGrad(u)) + b` balance path
- `LinearIsotropicStress2D(...)` on materialized strain tensors

## Features relied on

- tensor/index semantics and indexed projection (`expr[i, j]`)
- representational differential operators: `SymGrad(...)`, `Div(...)`
- field-term composition for strong-form balance shape (`Div(SymGrad(u)) + b`)
- existing continuum helper `LinearIsotropicStress2D(...)` (on materialized strain)

No discretization, weak forms, assembly, or solving semantics were introduced.

## Does this read like honest strong-form mechanics?

Yes for a minimal slice. A mechanics-aware reader can identify:

- displacement field `u`
- symmetric gradient strain path
- constitutive stress construction
- divergence-based balance residual shape
- explicit body-force-like term

## What remained awkward

The biggest awkwardness surfaced clearly:

- `SymGrad(u)` yields a representational operator value that is index-projectable and composable.
- But helper flows that depend on `Trace(...)` over that representational value are still blocked (`Trace` expects a materialized matrix).

Consequence: `LinearIsotropicStress2D(SymGrad(u), lambda, mu)` is not currently viable end-to-end in field form, so M2 uses explicit projected strain components for the field-form constitutive expression and keeps `LinearIsotropicStress2D(...)` validated on materialized strain.

## Answers to required M2 questions

1. **Can Oct express a real small-strain strong-form continuum relation cleanly?**
   - Yes: `u -> SymGrad(u) -> sigma` and `Div(SymGrad(u)) + b` are now expressible and typed.
2. **Does field interop fully unblock continuum notation at this stage?**
   - It unblocks the core strong-form notation, including composition and projection.
3. **Are `SymGrad(...)`, `Div(...)`, and existing constitutive helpers enough now?**
   - Almost. They are sufficient for the first honest slice, but direct helper composition through `Trace` on representational strain is still a friction point.
4. **What still feels awkward or missing?**
   - A narrow bridge for invariant-style helpers (especially trace-driven constitutive steps) over representational operator outputs.
5. **What should come next?**
   - One narrow helper/operator pass, not discretization.

## Recommendation

**Next step: one more narrow substrate/helper pass** focused on invariant support over representational field-operator tensors (e.g., trace-compatible pathway for `SymGrad(u)` outputs), then continue richer continuum mechanics helpers.
