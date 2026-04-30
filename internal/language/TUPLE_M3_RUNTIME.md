# Tuple Support — M3 Runtime/Interpreter

Date: 2026-04-30

## Runtime tuple representation

M3 introduces a minimal runtime tuple packet carrier in `internal/interpret`:

- `ValueKind` adds `ValueTuple`.
- `Value` adds `Tuple []Value`.

This representation is intentionally narrow:

- fixed ordered elements,
- arity preserved by slice length,
- used for call/builtin return packet carriage into destructuring assignment.

No public tuple API is introduced.

## Proof builtins runtime behavior

M2 checker proof builtins now execute at runtime with tuple packet values:

- `TupleProbe()` returns `(1, 2)`
- `BoolIntProbe()` returns `(true, 7)`

These values are test proof hooks only, not user-facing semantic commitments.

## Destructuring execution behavior

`ast.DestructureAssignStmt` now executes in interpreter statement dispatch.

Behavior:

1. Evaluate RHS exactly once.
2. Require runtime value kind `ValueTuple`.
3. Require arity match between tuple elements and target names.
4. Assign each element to matching target via normal mutable binding assignment path.

Runtime guards are kept even though typechecker should reject public mismatch paths.

## Explicit non-goals preserved

M3 does **not** add:

- tuple literals,
- tuple indexing,
- tuple equality,
- tuple arrays,
- nested destructuring,
- multiple RHS assignment,
- general tuple storage features,
- public tuple APIs.

Tuple remains a temporary return packet.

## Random M1 readiness

M3 completes runtime prerequisites for narrow multi-return packet carriage and unpacking.

This unblocks Random M1 return-packet plumbing work (e.g., `RandInt(...) -> (Rng, Int)` style APIs) without broadening tuple into a general container.
