# Continuum Computability Boundary M0 report

## Tiny problem chosen

A minimal 2D small-strain solid mechanics problem was chosen:

- rectangular 2D body (`Width = 2.0`, `Height = 1.0`)
- isotropic linear elastic material from (`E`, `nu`)
- plane stress assumption
- one unknown displacement field `u`
- constant body force `(0, -1000)`
- one displacement constraint (left edge fixed)
- governing relation intent: linear momentum balance
- query intent: solve for `u`

This is a cantilever-like boundary-value statement, intentionally without any finite lowering.

## What problem objects were introduced

Using ordinary Oct records/enums/arrays only:

- root problem object: `ContinuumProblem`
- support records: `Body`, `Material`, `Load`, `Constraint`
- support enums: `BoundaryCategory`, `MissingCapability` (problem tags are explicit strings in `ContinuumProblem`)
- probe result records/enums: `BoundaryProbeResult`, `BoundaryCategory`, `MissingCapability`

No parser, DSL, mesh, basis, assembly, or solver machinery was added.

## Layer split (explicit)

### A) Representation layer (already present)

The field-form mechanics substrate remains usable and direct:

- `SymGrad(u)`
- isotropic constitutive mapping in `StressFromMaterial(...)`
- `Div(stress) + body_force`

This demonstrates that constitutive and strong-form operator composition is representable now.

### B) Problem specification layer (added explicitly here)

`ContinuumProblem` is the typed carrier for:

- body/domain extent
- assumption/material
- unknown field declaration
- loads
- constraints
- governing relation intent
- query intent

This confirms that a tiny real continuum problem can be stated honestly without discretizing.

### C) Computational layer (not present)

When query intent is `SolveForField`, the first hard stop is:

- no finite computational lowering exists to realize `u(x)` over the body.

The probe marks this explicitly as `BoundaryCategory.Computational` with `MissingCapability.MissingFiniteLowering`.

## Required questions answered

1. **What is already expressible?**
   Constitutive algebra, strong-form residual composition, and typed problem components are all expressible.
2. **Is a structured problem-spec layer missing before discretization?**
   It was missing as an explicit first-class object; this probe adds it using plain records plus explicit query/governing tags.
3. **First exact missing capability?**
   Finite realization of the unknown field for `SolveForField` queries.
4. **Boundary type?**
   Computational.
5. **Is discretization actually next?**
   Yes, after problem specification is explicit.
6. **Honest role of discretization?**
   Not to define physics; to lower the already-specified continuum problem into a finite computable form that can produce numerical answers to solve/query requests.

## Blunt conclusion

Discretization is **not** the next layer if the problem object is still implicit. Once a typed problem object exists, the next honest missing layer is computational lowering.

In this probe, that boundary is now explicit and localized: `SolveForField` cannot be crossed without finite approximation machinery.

## Recommendation for next pass

Run a narrowly scoped **Computational Lowering M1** pass that:

- consumes `ContinuumProblem`
- introduces the minimal finite representation needed to realize `u`
- keeps constitutive/governing semantics in the continuum layer (no semantic duplication)
- reports lowered artifacts and solved-query outputs separately from model specification

Do not broaden into general FE infrastructure until this thin lowering seam is validated end-to-end.
