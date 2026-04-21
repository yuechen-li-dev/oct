# Tensors

## Overview

Oct treats tensors as a first-class mathematical concept for typed scientific/mechanics expression.

- A matrix is a rank-2 tensor.
- A vector is a rank-1 tensor-like object.
- Tensor expressions in current Oct are centered on:
  - indexed Einstein-style matrix expressions (`m[i, j]` with index symbols), and
  - representational differential operators (`Grad`, `Div`, `SymGrad`, `Trace`) used in continuum-mechanics contracts.

This page documents only currently implemented behavior from the language corpus and implementation.

## Conceptual model in Oct

Oct tensor work is representational and typed:

- You can build tensor expressions that preserve index structure and composition.
- Differential operators in this phase compose as typed symbolic/representational terms rather than forcing numerical discretization.
- Continuum mechanics contracts use this to express strain/stress/balance forms directly in language-level tests.

## Supported operations

### Einstein-style indexed matrix operations

Current indexed tensor expression surface supports:

- index symbols via `Idx("name")`
- indexed matrix terms like `a[i, k]`
- indexed multiplication with contraction through repeated indices, e.g. `a[i, k] * b[k, j]`
- indexed addition when free-index structure matches, e.g. `a[i, j] + b[i, j]`
- nested composition across expression trees

Current constraints:

- indexed tensor expressions require indexed operands on both sides
- indexed tensor expressions currently support only `+` and `*`
- trace-style indexed sugar `[i, i]` is intentionally rejected; use `Trace(...)`

### Differential tensor-aware operators

Current tensor/differential surface includes:

- `Grad(x)`
- `Div(x)`
- `SymGrad(x)`
- `Trace(x)`

These operators are used directly in continuum mechanics contracts to express field-form equations (e.g., strain/stress and balance skeletons).

## Continuum mechanics examples (from existing corpus)

Representative use patterns in [Language/Mechanics/ContinuumM1](../Mechanics/ContinuumM1), [ContinuumM2](../Mechanics/ContinuumM2), and later continuum milestones:

- strain-like construction: `eps = SymGrad(u)`
- constitutive-like assembly (small-strain):
  - `sigma = (lambda * Trace(eps)) * I + (2*mu) * eps`-style composition
- strong-form residual skeleton:
  - `r = Div(sigma) + b`
- tensor inspection/projection paths via index access where applicable

## Relationship to matrices

- Matrices are 2D tensors.
- Tensor expressions generalize matrix operations with index-aware composition.
- Use matrix literals/constructors for concrete numeric objects.
- Use indexed tensor operators when expression clarity depends on free/repeated index structure.

See also [16 vectors and matrices](./language/16-vectors-and-matrices.md).

## Compiled mode support

Compiled parity evidence currently shows tensor/indexed differential surface is primarily interpreter-path functionality:

- Compiler indexing rules currently enforce concrete matrix element access (`m[r, c]`) and do not implement indexed-symbol tensor terms in compiled lowering.
- Differential tensor operators are not documented as compiled-parity guarantees in current corpus.

Use the compiled parity corpus in `internal/build/compiler_test.go` and compiler diagnostics/lowering in `internal/build/compiler.go` as SSOT for compiled support status.
