# Differential Operators M0 Report

## Operators added

- `Grad(...)`
- `Div(...)`

Both are language builtins in the general operator substrate (not mechanics-specific library code).

## Exact semantic scope (M0)

### `Grad(x)`

Accepted operand kinds:
- numeric scalar (`Int`, `Float`, `Complex`)
- numeric vector (`Vector<Int|Float|Complex>`)

Rejected in M0:
- matrices
- arrays
- non-numeric operands

Type-level result shape:
- scalar -> vector
- vector -> matrix

### `Div(x)`

Accepted operand kinds:
- numeric vector
- numeric matrix

Type-level result shape:
- vector -> scalar
- matrix -> vector

## Representational vs evaluable

M0 operators are **representational/structural**.

- Runtime construction yields structural differential-operator values (`Grad(...)`, `Div(...)`) for explicit field-equation expression building.
- This pass does **not** implement numerical derivative evaluation.
- This pass does **not** implement symbolic simplification.

Unit dimensions are preserved from the operand in M0 because no spatial coordinate metric/discretization layer exists yet.

## What became expressible

- `Grad(temperature)`
- `Div(Grad(temperature))`
- `Grad(displacementVector)`
- `Div(stressLikeMatrix)`

These enable early field-equation-shaped expressions without implying solver/discretization behavior.

## Deliberately excluded

- finite difference / finite volume / finite element discretization
- meshing, assembly, solver tooling
- symbolic differentiation and simplification
- broader operator set (`Curl`, `Laplacian`, material derivative, weak forms)

## Findings

A small explicit operator layer (`Grad`, `Div`) composes with the tensor/container type substrate and is sufficient to represent early field-equation structure honestly.

## Recommendation for next step

Proceed with **richer continuum formulations** first (constitutive + balance-law surfaces using this representational layer), then add carefully scoped operators (`SymGrad` and optionally `Curl`) before any discretization groundwork.
