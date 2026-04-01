# Differential Operators / Field Interop M1 report

## What was added

- Added a narrow representational field-term interoperability runtime surface:
  - representational field expression composition for `+`, `-`, `*`, `/` when at least one side is a differential/operator expression value.
  - representational projection of operator expressions through indexing (`expr[i]` / `expr[i, j]`) without numerical evaluation.
- Added `SymGrad(...)` as a tiny mechanics-adjacent helper that stays representational.

## Operator output surface decision

- **Composable:** yes (representational algebraic composition of field/operator terms).
- **Indexable:** yes (narrow projection surface for representational field/operator expressions).
- Implemented as a very small combination of both to unblock direct mechanics forms.

## What is now directly expressible

- Gradient projection path: `Grad(u)[i, j]` (runtime representational projection object).
- Symmetric-gradient-like path: `SymGrad(u)` and downstream usage such as `Div(SymGrad(u))`.
- Strong-form residual shape: `Div(sigma) + b` as a typed field-form expression.

## What remains intentionally excluded

- No discretization (`FE/FV/FD`), meshes, assemblers, or solvers.
- No PDE simplification engine or broad symbolic calculus.
- No equation-solving semantics.
- Differential operators remain representational, not evaluative.

## Answers to required M1 questions

1. Smallest honest typed interop layer: representational field expression composition + narrow projection.
2. `Grad(u)` continuum participation: yes, via projection and composition paths without evaluation.
3. `Div(sigma) + b` typed direct relation: yes.
4. `SymGrad(...)` justified: yes, now a compositional representational helper (not cosmetic).
5. Explicit and non-evaluative: preserved.
6. Next step: continuum mechanics expansion (Continuum M2) is now the natural next milestone.
