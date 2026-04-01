# Tensor / Einstein Notation M1 Report

## What M1 introduced

M1 introduces parser-level indexed tensor terms using existing bracket surface syntax over matrix values with `Index` values:

- `A[i, k]`
- `B[k, j]`

When both indices are `Index`, the form is treated as an Einstein term, not ordinary element lookup.

M1 then maps ordinary arithmetic operators over these terms onto the M0 Einstein semantics:

- `A[i, k] * B[k, j]` → `EinMul(A, i, k, B, k, j)` semantics
- `A[i, j] + B[i, j]` → `EinAdd(A, i, j, B, i, j)` semantics

## Boundary and semantics status

M1 keeps M0 boundaries:

- matrix-backed only (`Matrix<T>`)
- explicit first-class `Index` values (still created via `Idx("...")`)
- strict contraction/addition rules delegated to the same Einstein runtime core used by M0

No symbolic algebra, no coordinate/frame system, no rank-N generalization.

## Coexistence with ordinary indexing

Bracket indexing now has explicit matrix split behavior:

- `A[r, c]` where `r,c : Int` remains element indexing
- `A[i, j]` where `i,j : Index` is Einstein-term indexing
- mixed forms like `A[i, 0]` are rejected

Array/vector indexing remains ordinary `Int` indexing.

## What worked

- Parser-level indexed multiplication and addition read materially closer to math.
- Runtime behavior for contraction and free-index matching remains strict and explicit.
- Renaming invariance is preserved because labels are value-level `Index` names.

## Deliberately out of scope

- rank-polymorphic tensor output types
- arbitrary-rank storage
- symbolic tensor manipulation/simplification
- coordinate transforms / continuum mechanics machinery

## M2 recommendation

If M1 is accepted, M2 should focus on composition ergonomics while preserving strictness:

1. Allow named indexed intermediate bindings without losing Einstein-term structure.
2. Add minimal diagnostics that pinpoint the offending index positions in mismatches.
3. Evaluate a narrow trace-style contraction surface (`A[i, i]`) only if matrix-only invariants stay intact.
