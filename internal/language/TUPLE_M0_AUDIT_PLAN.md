# Tuple Support — M0 Audit & Implementation Plan

## Scope and intent

This document is **Tuple M0 (audit + plan only)** for narrow multi-return support needed by runtime-backed APIs like:

```oct
rng, x = RandInt(rng, 1, 6)
```

with a function/builtin signature shape like:

```oct
RandInt(rng: Rng, min: Int, max: Int) -> (Rng, Int)
```

No tuple feature implementation is included in this milestone.

---

## 1) Current pipeline summary

Current language pipeline (relevant slices):

1. **Parser** builds `ast.TypeRef` for a single return type and assignment statements headed by one identifier.
2. **Typechecker** lowers `ast.TypeRef` to `typecheck.Type` and tracks function signatures as exactly one `returnType Type`.
3. **Builtins** are identified by name (`internal/builtin/builtin.go`), but typing/arity/return contracts are hardcoded in `typecheck.checkBuiltinCallExpr` and runtime behavior in `interpret.evalBuiltinCallExpr`.
4. **Interpreter** evaluates calls to one `evalResult.value` and assignment writes one binding (`AssignStmt`) or index/field target.

Observations:

- The architecture currently assumes **single-return** values in parser AST shape, type signatures, call typing, and runtime call result carriage.
- Builtins do not use a shared declarative signature table; type and runtime paths diverge in implementation location (checker switch + interpreter switch/wrappers).

---

## 2) Parser / AST audit

### Current behavior

- Function declarations parse exactly one return type (`parseFunctionDecl` -> `parseTypeRef`).
- `ast.FunctionDecl.ReturnType` is a single `TypeRef`.
- `ast.TypeRef` has array/vector/matrix/function subforms but no tuple/product form.
- Assignments parse only identifier-leading forms:
  - `x = expr` (`AssignStmt`)
  - `x[i] = expr` (`IndexAssignStmt`)
  - `x.f = expr` (`FieldAssignStmt`)
- There is no AST node for destructuring assignment targets.

### Direct answers

- **Does parser already understand `(A, B)` as a return type?** No.
- **Does parser already support `a, b = expr`?** No.
- **What AST nodes represent these?** None today.
- **What is missing?**
  - A type-level product node (likely in `ast.TypeRef`) or equivalent dedicated return-shape node.
  - A destructuring assignment node carrying ordered target names.

### Complex number note

Complex values are first-class scalar values (`Complex`) and are unrelated to tuple packing; no existing “pair” representation can be reused for multi-return packets safely.

---

## 3) Typechecker audit

### Current behavior

- `typecheck.Type` has flags for array/vector/matrix/function/flow-instance but no tuple/product representation.
- `functionSignature` stores one `returnType Type`.
- `checkFunctionCallArguments` always returns exactly one `ExprType{ValueType: signature.returnType}`.
- Assignment checker (`case ast.AssignStmt`) validates one target binding against one expression type.
- Builtin typing is centralized in `checkBuiltinCallExpr`/helper methods, not via a shared built-in signature registry object.

### Direct answers

- **Is there a `TupleType`/`ProductType` already?** No.
- **Can user-declared functions return multiple values today?** No (single `ReturnType`).
- **Can builtins return multiple values today?** No typing path supports tuple/product return types.
- **Where does builtin typing diverge from declared function typing?**
  - User functions are typed via registered `functionSignature` from declarations.
  - Builtins are typed by specialized checker logic (`checkBuiltinCallExpr` switch paths), not by declared AST signatures.

### Missing pieces

- A checker-level tuple/product type representation.
- Destructuring typing rules:
  - arity match (`len(targets) == tuple arity`)
  - per-element assignability checks.
- Clear diagnostics for mismatch cases (non-tuple RHS, arity mismatch, element mismatch).

---

## 4) Interpreter/runtime audit

### Current behavior

- Runtime value model (`interpret.Value`) has many variants but no tuple/product variant.
- Call evaluation (`evalCallExpr`, builtin call helpers) yields one `evalResult.value`.
- Statement execution assignment path handles singular assignment/index/field assignment only.

### Direct answers

- **Can runtime already carry multiple return values?** No.
- **Can builtin implementations return more than one value?** Not as a first-class multi-value packet.
- **Can assignment destructure returned values?** No.
- **What runtime representation is needed?**
  - Minimal tuple packet value kind (ordered `[]Value` with fixed arity) *or*
  - equivalent internal multi-value carrier constrained to call/assignment boundary.

Recommendation: use an explicit `ValueTuple` runtime kind for diagnosability and consistent error messages.

---

## 5) Builtin system audit

### Current behavior

- `internal/builtin/builtin.go` only reserves builtin names.
- Builtin argument/return typing is implemented in `typecheck.checkBuiltinCallExpr` and helper paths.
- Builtin execution is implemented in `interpret.evalBuiltinCallExpr` plus wrapper registries.
- Fallible builtins have checker/runtime special handling where `ExprType.Fallible` / `evalResult.hasError` participate.

### Direct answers

- **Smallest change for `(Int, Int)` builtin returns?**
  1. Add tuple/product type in checker.
  2. Add tuple runtime value in interpreter.
  3. Add one builtin checker case returning tuple type and one runtime builtin case returning tuple value.
  4. Add destructuring assignment syntax+typing+runtime handling for assignment-only consumption.
- **Should builtin signatures use `TupleType` or existing function-signature machinery?**
  - For M1–M3 minimality: use existing builtin checker path but return the new tuple/product `Type` where needed.
  - Medium-term hardening: unify builtin metadata into a declarative signature table to reduce drift.

---

## 6) Test audit

### What exists now

- Parser tests cover single return types and assignment forms, including index/field assignment.
- Typechecker tests cover assignment typing, builtin argument validation, and many error strings.
- Interpreter tests cover builtin execution and runtime assignment invariants.
- Language reference currently documents single return form and scalar assignment form.

### Gaps for tuple work

No coverage for:

- tuple return type parsing,
- destructuring assignment parsing,
- tuple/product type checking,
- tuple arity/type mismatch diagnostics,
- runtime tuple return and destructuring application.

### Placement for future tests

- **Language semantics tests**: `Language/...` (`.octest` / `.octfail`) for public contracts.
- **Host implementation tests**: focused Go parser/typechecker/runtime tests for internal invariants.

Proposed staging:

- **Tuple M1 tests**: parser + AST shape tests.
- **Tuple M2 tests**: checker tests for arity/type mismatch and accepted builtin multi-return destructuring.
- **Tuple M3 tests**: interpreter tests for tuple-producing builtin + destructuring runtime behavior.

---

## 7) Risk assessment

1. **Cross-layer coupling risk**: parser/typechecker/runtime all currently single-return centric.
2. **Builtin divergence risk**: checker/runtime builtin behavior can drift because contracts are not centralized.
3. **Feature creep risk**: tuple packets accidentally become general-purpose user containers.
4. **Diagnostic regression risk**: existing clear errors may degrade if tuple logic is bolted in without dedicated mismatch messages.

Mitigation: constrain tuple support to call-return + assignment destructuring only, with explicit non-goal checks.

---

## 8) Staged implementation plan

## Tuple M1 — Parser/AST only

Deliverables:

1. Add AST representation for fixed product type (e.g., `TypeRef.TupleOf []TypeRef`).
2. Extend type parser to parse return type/group type syntax `(T1, T2, ... )` where allowed by grammar.
3. Add AST statement for destructuring assignment target list (e.g., `DestructureAssignStmt{Names []string, Value Expr}`).
4. Parse `a, b = expr` only for identifier targets (no nested patterns).
5. Add parser tests for valid/invalid syntax and shape.

Out of scope in M1: typing/runtime semantics.

## Tuple M2 — Typechecker only

Deliverables:

1. Add checker type representation for tuple/product (e.g., `Type{IsTuple bool, TupleElems []Type}`).
2. Extend `resolveTypeRef` and string formatting to support tuple type text.
3. Support function signatures whose return type is tuple/product.
4. Support builtin checker return of tuple/product type.
5. Implement destructuring assignment type rules:
   - RHS must be tuple/product
   - arity must match target count
   - each element assignable to target binding type.
6. Add dedicated diagnostics:
   - not tuple RHS,
   - arity mismatch,
   - element mismatch (include index).
7. Add typechecker tests.

Out of scope in M2: runtime value carriage/execution.

## Tuple M3 — Interpreter/runtime only

Deliverables:

1. Add runtime tuple packet representation (recommended `ValueTuple`).
2. Extend call evaluation paths to return tuple value when callee/builtin returns product.
3. Extend statement execution to apply destructuring assignment from tuple values.
4. Add runtime invariant checks for arity mismatches (should be unreachable post-typecheck but guarded).
5. Add interpreter tests with one proof builtin returning tuple packet.

Out of scope in M3: tuple literals/indexing/equality/collections.

## Random M1 — Random.Core enablement

After M1–M3 pass:

1. Add `Rng` runtime-backed behavior.
2. Implement `RandInt(rng,min,max)->(Rng,Int)` and related APIs.
3. Add `Language/` contracts and package-level usage tests.

---

## 9) Explicit non-goals

Deferred intentionally:

- tuple literals,

---

## Tuple M1 status update (parser/AST only)

Date: 2026-04-30

Implemented in M1:

1. **AST form added**
   - `ast.TypeRef` now supports tuple/product return packets via `TupleOf []TypeRef`.
   - `ast.DestructureAssignStmt` represents flat destructuring assignment with `Names []string` and single `Value Expr`.
2. **Syntax accepted**
   - Tuple return type syntax: `fn F() -> (A, B) { ... }`
   - Flat destructuring assignment: `a, b = expr` and `a, b, c = expr`
3. **Syntax rejected**
   - Invalid tuple return types: `()`, `(A)`, `(A, )`, `(, A)`
   - Invalid destructuring forms: `(a, b) = expr`, `a, (b, c) = expr`, `a, b = foo(), bar()`
   - Nested tuple types in type positions are explicitly rejected in M1.
4. **Parser tests added**
   - Positive/negative coverage for tuple return syntax and destructuring assignment AST shape.
5. **Explicit non-goals preserved**
   - No typechecker tuple semantics.
   - No runtime tuple values or execution semantics.
   - No tuple literals/indexing/equality/storage semantics.
6. **M2 next step**
   - Add checker-level tuple/product type representation and destructuring assignment type rules (tuple RHS requirement, arity checks, per-element assignability diagnostics).

Clarification carried forward:

- `x = PairReturningFunction()` is **not** made valid by M1 in semantic terms. M1 only parses syntax and AST shapes. M2/M3 will define checker/runtime behavior.
- tuple indexing,
- tuple equality,
- tuple arrays / heterogeneous arrays,
- `Any`,
- general tuple pattern matching,
- broad tuple ergonomics outside return-packet/destructuring assignment.

---

## 10) Required explicit answers

1. **Is tuple syntax already parsed anywhere?** No.
2. **Is destructuring assignment already parsed anywhere?** No.
3. **Are user function multi-returns already supported?** No.
4. **Are builtin multi-returns supported?** No.
5. **What exact type representation is missing?** A fixed-arity tuple/product type in AST+checker.
6. **What exact runtime representation is missing?** A fixed-arity tuple packet value kind (or equivalent internal multi-value carrier).
7. **What is the smallest safe proof builtin?** A dedicated internal builtin like `TupleProbe() -> (Int, Int)` used only to validate parser/typechecker/runtime planks.
8. **What should Tuple M1 implement?** Parser/AST tuple return type + destructuring assignment syntax and tests.
9. **What should Tuple M2 implement?** Type representation + call/destructure typing + diagnostics + tests.
10. **What should Tuple M3 implement?** Runtime tuple value + builtin tuple returns + destructure execution + tests.
11. **What must remain deferred?** All general-purpose tuple features (literals/indexing/equality/collections/patterns).

---

## 11) Reference consistency notes

`Language/reference` currently documents:

- function declarations with a singular `-> ReturnType` form,
- assignment as `name = expr`,
- enums page explicitly marks tuple destructuring patterns as out-of-scope.

Therefore tuple return/destructuring support is a **documentation expansion** that must be introduced intentionally alongside implementation milestones; existing code should not silently diverge from reference authority.
