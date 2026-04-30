# Tuple Support — M4 Lowering / Compiled-Mode Compatibility

Date: 2026-04-30

## Compiled/lowering audit findings

1. Compiled mode exists and is implemented in `internal/build/compiler.go` with MIR lowering and Go emission.
2. Lowering previously assumed single-target assignment (`AssignStmt`) and had no `DestructureAssignStmt` branch.
3. Call lowering previously assumed single return target (`MIRCall{Target: ...}`) and no tuple return unpack path.
4. Builtins are lowered separately in compiled mode through `goStmt(MIRCall)` builtin switch and `resolveCall`/`compiledBuiltinReturnType` decisions.
5. Proof builtins `TupleProbe`/`BoolIntProbe` existed in parser/typechecker/interpreter but were not supported in compiled lowering.
6. Minimal compiled proof path is `Compile(...)` + execute produced artifact and assert return code/output.

## Implementation approach

- Added tuple return type stringing in lowering (`typeRefStringForPackage`) for `ast.TypeRef.TupleOf`.
- Added a narrow lowering statement `MIRDestructureCall` for destructuring assignment from call RHS only.
- Destructuring lowering validates:
  - RHS is a call,
  - call return type is tuple,
  - arity matches LHS names,
  - target locals exist and type-match tuple element types.
- Added compiled lowering support for proof builtins in `resolveCall`:
  - `TupleProbe -> (Int, Int)`
  - `BoolIntProbe -> (Bool, Int)`
- Added Go emission for `MIRDestructureCall`:
  - builtins emit constants directly,
  - non-builtin calls emit multi-target assignment (`a, b = fn_Pkg_Name(...)`).

## Proof programs/tests

- Added compiled artifact tests in `internal/build/compiler_test.go`:
  - `a, b = TupleProbe()` returns `3`.
  - `flag, n = BoolIntProbe()` returns `7`.

## Single-evaluation guarantee

- Lowering emits one `MIRDestructureCall` statement per destructuring assignment.
- Go emission produces one call expression on RHS for non-builtin destructuring paths.
- Builtin proof paths are constants with no duplicated call.

## Non-goals preserved

No tuple literals/indexing/equality/arrays/storage/nested destructuring/multi-RHS/general tuple APIs were added.

## Random M1 readiness

Compiled mode now has a concrete tuple-return destructuring lowering path and execution proof for narrow return-packet semantics. Random M1 can proceed without compiled-mode uncertainty for this packet pattern.
