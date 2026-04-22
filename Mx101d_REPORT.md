# Mx101d report — matrix write parity (`m[r, c] = value`)

## 1) What was required

To close the read/write mismatch for matrices, index assignment had to be extended from a single-index array-only shape to support a two-index matrix shape:

- AST index assignment now carries `Indices []Expr` (not a single `Index` expression).
- Parser index-assignment target parsing now accepts comma-separated indices inside one bracket pair (`x[i]`, `m[r, c]`).
- Typechecker index assignment now supports:
  - array targets with exactly one `Int` index
  - matrix targets with exactly two `Int` indices
  - element-type compatibility checks against the indexed element type
- Interpreter index assignment execution now mutates:
  - arrays by one-dimensional index
  - matrices by row/column element address

This keeps scope narrow to indexed assignment parity for existing matrix indexing semantics.

## 2) Interpreted mode support

Interpreted execution now performs concrete matrix element writes for `m[r, c] = value`:

- indices are evaluated and required to be dimensionless `Int`
- arity is validated at runtime (`2` for matrices)
- matrix bounds checks use existing matrix shape semantics
- assignment mutates `Matrix.Elements[r*cols + c]`

So matrix read/write now uses the same surface (`m[r, c]`) in interpreted mode.

## 3) Compiled mode support

Compiled lowering now supports matrix index assignment by lowering assignment targets to Go-style nested indexing:

- array assignment lowers to `target[idx] = value`
- matrix assignment lowers to `target[row][col] = value`

Lowering enforces index arity based on lowered local type shape and preserves existing compiled indexing conventions.

## 4) Prometheus SGEMM lab follow-through

Prometheus SGEMM Algorithm Lab M0/M1 was updated to use matrix-native accumulation outputs:

- output buffers are now `Matrix.zeros<Float>(rows, cols)` instead of flat row-major `Float[]`
- accumulation writes now use `out[r, c] = ...`
- M1 comparison assertions now compare matrix shapes and `expected[r, c]` vs `actual[r, c]`

This removes the flat-buffer workaround that was only present because matrix writes were missing.

## 5) Deferred scope confirmation

Anonymous/lambda callback support remains deferred and unchanged in this milestone.
No lambda or anonymous-function implementation work was introduced.
