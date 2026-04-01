# Tensor / Einstein Notation M0 Report

## Minimal model chosen

- Added an explicit `Index` value kind via `Idx(name: String) -> Index`.
- Kept storage concrete and existing: rank-2 `Matrix<T>` remains the backing tensor model.
- Added focused Einstein entry points instead of a broad tensor API:
  - `EinMul(A, i, k, B, k, j)`
  - `EinAdd(A, i, j, B, i, j)`

This is intentionally narrow M0 scope: explicit index entities + index-aware contraction on matrix terms.

## Contraction rule implemented

For `EinMul`:

- Labels appearing once across both matrix terms are **free indices** and define output axes.
- Labels appearing exactly twice are **contracted indices** and are summed.
- Labels appearing more than twice are rejected.
- M0 currently requires exactly two free indices (matrix output).

This directly supports matrix multiplication in Einstein form by contracting the repeated index.

## Examples that worked

- Matrix multiplication:
  - `EinMul(A, i, k, B, k, j)`
- Free-index renaming invariance:
  - `EinMul(A, p, s, B, s, q)` gives the same numeric result shape/values.
- Addition with matching free-index structure:
  - `EinAdd(A, i, j, B, i, j)`

## Failure boundaries enforced

Static (`.octfail`) boundaries:

- Non-`Index` arguments for index slots are rejected.
- Non-matrix tensor operands are rejected.

Runtime boundaries (explicit errors):

- Index extent mismatches are rejected.
- Index multiplicity > 2 is rejected.
- `EinMul` with non-matrix-output free-index structure is rejected in M0.
- `EinAdd` rejects contracted indices and rejects mismatched free-index order.

## Deliberately excluded in M0

- Full expression-level `A[i,k] * B[k,j]` parser/operator syntax.
- Higher-rank tensor storage and rank-polymorphic output typing.
- Symbolic simplification / algebraic rewriting.
- Differential geometry / coordinate-frame semantics.

## Verdict

This is **more than renamed arrays** because index labels now drive contraction and free-index mapping semantics.

It is still intentionally minimal and concrete: matrix-backed, explicitly bounded, and testable.

## Recommended next step

M1 should introduce parser-level indexed tensor terms so users can write:

- `A[i,k] * B[k,j]`
- `A[i,j] + B[i,j]`

while preserving the same M0 runtime contraction rules and failure boundaries. That yields math-native readability without expanding into symbolic algebra.
