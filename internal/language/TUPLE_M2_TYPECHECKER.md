# Tuple Support — M2 Typechecker

## Implemented type representation

Tuple/product types are represented in the checker as:

- `Type{Tuple: &tupleType{Elements: []Type{...}}}`

Properties:

- fixed arity and ordered element types,
- string formatting as `(T1, T2, ...)`,
- enforced minimum arity of 2.

## Supported behavior in M2

- `ast.TypeRef.TupleOf` resolves to checker tuple types.
- Function signatures may return tuple types (e.g. `-> (Int, Int)`).
- Builtin proof hooks are available for typechecking:
  - `TupleProbe() -> (Int, Int)`
  - `BoolIntProbe() -> (Bool, Int)`
- Flat destructuring assignment is typechecked:
  - RHS must be tuple type,
  - target count must equal tuple arity,
  - each target receives the corresponding element type,
  - assigning tuple values to single-target assignment is rejected.

## Diagnostics added

- non-tuple RHS in destructuring: `destructuring assignment requires tuple return/value`
- arity mismatch: `destructuring assignment expected <n> targets, got <m>`
- tuple-to-single-assignment misuse: `tuple return values must be destructured`
- per-element mismatch includes index: `tuple element <i> ...`

## Explicit non-goals (still deferred)

- runtime tuple values,
- tuple literals,
- tuple indexing/equality,
- nested destructuring,
- multiple RHS assignment,
- general tuple storage as user container.

## M3 runtime next steps

- Introduce runtime tuple value carriage for call returns.
- Implement destructuring execution for tuple RHS.
- Add runtime invariant checks for tuple arity at assignment boundary.
