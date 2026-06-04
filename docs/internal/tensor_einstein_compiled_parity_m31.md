# M31 — Tensor / Einstein compiled-parity and language-contract audit

## 1. Executive summary

Oct already has a narrow but real tensor-indexing surface in interpreted mode. The implemented path is matrix-backed and rank-2 only: users create first-class symbolic labels with `Idx("name")`, use values of type `Index` in matrix indexing forms such as `A[i, k]`, and combine indexed matrix terms with `*` or `+`. Multiplication follows Einstein contraction over repeated labels and addition is elementwise when the free-index order matches exactly. The earlier explicit builtins `EinMul(A, i, k, B, k, j)` and `EinAdd(A, i, j, B, i, j)` remain public/reachable and share the same runtime core.

Compiled mode does not have parity for this indexed tensor surface. The current compiled lowerer understands concrete vector/array indexing and concrete matrix indexing through integer indices, and it has special lowering for `@` matrix-matrix and matrix-vector multiplication. It does not lower `Idx`, `Index` values, `EinMul`, `EinAdd`, or `[Index, Index]` Einstein terms. Running the current tensor suites with `--execution compiled` fails immediately with diagnostics such as `compiled mode does not yet support builtin Idx`.

Recommended language-contract clarifications:

- Merge the tensor reference content into the vector/matrix reference page and retitle it to **Vectors, Matrices, and Tensors**.
- State explicitly that arrays are general ordered collections/storage values, while vectors and matrices are mathematical tensor values: vectors are rank-1 tensors and matrices are rank-2 tensors.
- Document tensor notation as an index-aware expression surface over vector/matrix/tensor mathematical values, not as array indexing with fancier names.
- Retain `Language/reference/tensors.md` as a short pointer during the transition unless repo convention later prefers removal.
- Document `@` as a special-case Einstein contraction shorthand, not as an unrelated generic matrix-multiplication operator.

Recommended next implementation milestone after M31: **M32 — documentation merge plus interpreted Einstein subtraction**. M32 should not attempt compiled lowering or vector rank-1 terms. It should make the reference contract coherent, then add `A[i, j] - B[i, j]` in the interpreter and typechecker by mirroring `EinAdd` validation and using elementwise subtraction.

Deferred features remain: compiled rank-2 indexed lowering, vector rank-1 indexed terms, vector/matrix mixed Einstein contractions, arbitrary rank-N tensors, index variance, raising/lowering indices, broadcasting, trace-style `A[i, i]`, and new GPU/reactor kernels.

## 2. Current interpreted tensor surface inventory

### First-class indices

- `Idx("name")` exists as a builtin constructor for first-class index labels.
- The static type is `Index` (`BaseTypeIndex` in the typechecker).
- Runtime `Idx` requires exactly one `String` argument and rejects empty/whitespace names.
- The current implementation stores the label text in a runtime `ValueIndex`.

### Matrix indexed terms and concrete indexing

Current matrix indexing is split by index operand types:

- `A[r, c]` where both operands are `Int` is concrete matrix element access and returns the matrix scalar element type.
- `A[i, j]` where both operands are `Index` is an Einstein indexed matrix term and returns a matrix-typed expression carrying `EinTerm` metadata.
- Mixed matrix forms such as `A[i, 0]` are rejected with: `matrix indexing expects either [Int, Int] element access or [Index, Index] Einstein term access, got [Index, Int]`.
- Matrix indexed terms are rank-2 only. The tracked labels are exactly two labels.

### Vector and array indexing

- `v[n]` where `v` is `Vector<T>` and `n: Int` is concrete vector element access.
- `v[i]` where `i: Index` is **not currently supported**. The typechecker rejects it with `vector indexing index must be Int, got Index`.
- `v[i, j]` is rejected before any tensor semantics because vector indexing requires exactly one index. The existing invalid TensorEinsteinM1 fixture expects: `vector indexing requires exactly 1 index, got 2`.
- Array indexing remains concrete storage access only: `a[n]` requires one `Int` index. Arrays are not tensor-indexed values today.

### Explicit builtins

The M0 explicit builtins are still public/reachable:

- `EinMul(A, i, k, B, k, j)`
- `EinAdd(A, i, j, B, i, j)`

They require six arguments: matrix, index, index, matrix, index, index. Static validation rejects non-matrix operands and non-`Index` label arguments. Runtime dispatch shares `evalEinsteinBinaryMatrices` with infix indexed expressions.

### Infix indexed operators

Current infix indexed tensor operators:

- `A[i, k] * B[k, j]` contracts repeated labels and produces a matrix when exactly two free indices remain.
- `A[i, j] + B[i, j]` performs elementwise matrix addition when both terms have the same free-index order.
- Nested composition works, including `(A[i, k] * B[k, m]) * (C[m, j] + D[m, j])`.
- Reindexed intermediates work by materializing the intermediate matrix and indexing it again, e.g. `partial = A[i, k] * B[k, j]` followed by `partial[i, j] + C[i, j]`.

Current infix indexed operators do **not** include subtraction. `A[i, j] - B[i, j]` currently routes to the indexed-expression checker/interpreter and fails because indexed tensor expressions only support `+` and `*`.

### Trace-style rejection

Trace-style indexed sugar is intentionally rejected:

- `A[i, i]` is rejected with `trace-style contraction '[i,i]' is not supported in M3; use Trace(...) for now`.
- This is also protected by the TensorUtilitiesM4 invalid fixture.
- The documented/user-facing path for traces is explicit `Trace(A)`.

### Representative invalid/error diagnostics

The current test corpus and implementation establish these important diagnostics:

- Non-index argument to explicit `EinMul`: `function 'EinMul' argument 3 expects Index, got Int`.
- Non-matrix argument to explicit `EinMul`: `function 'EinMul' argument 1 expects Matrix, got Int[]`.
- Mixed matrix index types: `matrix indexing expects either [Int, Int] element access or [Index, Index] Einstein term access, got [Index, Int]`.
- Indexed term on only one side of a binary expression: `indexed tensor expressions must appear on both sides of '+' (left indexed=false, right indexed=true)`.
- Addition with different free-index order: `EinAdd requires matching free-index order on both terms (left=[i,j], right=[j,i])`.
- Multiplication with a label appearing more than twice: `index 'i' appears 3 times in [i,i]*[i,j]; only 1 (free) or 2 (contracted) are allowed in M0`.
- Multiplication that would not produce a rank-2 matrix in M0/M3: `EinMul requires exactly 2 free indices in M0, got 0 for [i,j]*[j,i]`.
- Runtime shape mismatch for addition: `runtime error: EinAdd requires matching matrix shapes`.
- Runtime extent mismatch for labels: `runtime error: index '<label>' has inconsistent extents`.

## 3. Interpreter implementation summary

The interpreter implementation is centered on `evalEinsteinBinaryMatrices(op, left, leftLabels, right, rightLabels)` in `internal/interpret/einstein.go`.

### `evalEinsteinBinaryMatrices`

The helper accepts:

- an operation string currently limited to `EinMul` or `EinAdd`;
- two runtime matrix values;
- two rank-2 label lists.

It first enforces runtime invariants:

- each side must have exactly two labels;
- every label must be non-empty;
- every repeated label must refer to a consistent matrix extent.

Extent consistency is label-based. For a left matrix term, slot 0 maps to `Rows` and slot 1 maps to `Cols`; the right matrix term uses the same slot mapping. If a label appears in multiple slots, all those slots must have the same extent.

### Free vs contracted labels

For multiplication, the implementation forms an ordered sequence `[left0, left1, right0, right1]`, counts labels, and classifies labels as:

- **free** when the count is exactly 1;
- **contracted** when the count is exactly 2;
- invalid when the count is greater than 2.

The ordering of the free labels is derived from their first appearance in that ordered sequence. The ordered free labels define output rows and columns.

### Repeated label count rules

A label appearing more than twice is rejected. A label appearing twice is contracted. A label appearing once is free. Current M0/M1/M3 matrix output support requires exactly two free indices after multiplication. Therefore:

- `A[i, k] * B[k, j]` is valid and produces free labels `[i, j]`.
- `A[i, j] * B[j, i]` is rejected because there are zero free indices and the current implementation has no scalar trace/double-contraction result for indexed multiplication.
- `A[i, i] * B[i, j]` is rejected because `i` appears three times. In practice `A[i, i]` is also independently rejected as trace-style sugar.

### Why M0 multiplication requires exactly two free indices

M0 intentionally stayed matrix-backed. The runtime result type for `EinMul` is always a `Matrix`. There is no current scalar result path, vector result path, or rank-N tensor result path. Requiring exactly two free indices is the guard that keeps multiplication inside the current rank-2 matrix result model.

### Multiplication evaluation

For valid multiplication:

1. allocate an output matrix with rows from the first free label extent and columns from the second free label extent;
2. iterate over every output `(row, col)` assignment for the two free labels;
3. recursively iterate every contracted-label value;
4. read the corresponding left and right matrix elements;
5. multiply the two scalar values through normal `evalBinaryExpr("*", ...)`;
6. accumulate terms through normal `evalBinaryExpr("+", ...)`.

Because scalar multiplication/addition use the ordinary evaluator, numeric/dimensioned scalar behavior is inherited from the existing scalar operations.

### `EinAdd` validation and evaluation

For addition, the implementation does not contract. It enforces:

- labels within each matrix term must be distinct;
- left and right free-index order must match exactly;
- matrix shapes must match exactly.

It then loops over matrix elements and applies ordinary `evalBinaryExpr("+", ...)` elementwise. The returned labels are the left labels.

This means `A[i, j] + B[j, i]` is rejected today. The implementation does not perform index-aligned transposition or reordering.

### Current lack of `EinSub`

There is no `EinSub` operation in `evalEinsteinBinaryMatrices`, no public builtin branch for `EinSub`, and `evalEinsteinIndexedBinaryExpr` only accepts `+` and `*`. Subtraction must therefore be a follow-on feature, not a documented current capability.

### Rank-2 matrix limitation

The interpreter’s Einstein term payload stores a matrix value and exactly two labels. There is no representation for rank-1 vector terms, scalar terms, or rank-N tensor terms. Nested expressions stay usable by materializing rank-2 matrix results and carrying rank-2 label metadata through subsequent indexed operations.

## 4. Typechecker implementation summary

### `BaseTypeIndex`

The typechecker declares `Index` as a base type (`BaseTypeIndex`). It registers built-in type names including `Index`, validates `Idx(...)` as returning `Index`, and allows `Index` as a proper non-array/non-vector/non-matrix scalar type category.

### `ExprType.EinTerm`

Expression checking carries optional Einstein metadata on `ExprType`:

```go
type ExprType struct {
    ValueType Type
    Fallible  bool
    EinTerm   *einsteinTermType
}
```

The current `einsteinTermType` stores:

- the scalar element type;
- exactly two labels;
- a `HasLabels` flag.

This shape is the core reason the current typechecker model is rank-2 only.

### Matrix `[Index, Index]` vs `[Int, Int]`

When checking an `ast.IndexExpr` over a matrix:

- two `Int` indices produce concrete scalar element access;
- two `Index` indices produce an `ExprType` whose value type is still `Matrix<element>` but whose `EinTerm` records scalar type and labels;
- any mixed pair is rejected.

If the labels are statically recoverable from identifier expressions and both names are equal, the checker rejects `A[i, i]` with the explicit trace-style diagnostic.

### Binary-expression routing

Binary expression checking evaluates both sides first. If either side has `EinTerm != nil`, it routes to `checkEinsteinBinaryExpr` instead of ordinary binary checking. That helper currently enforces:

- both sides must be indexed terms;
- the operator must be `+` or `*`;
- addition labels must be distinct within each term and must match in order across terms;
- multiplication labels may appear once or twice but not more than twice;
- multiplication must leave exactly two free labels.

The returned expression remains a matrix and carries updated `EinTerm` metadata, allowing nested indexed expressions to compose.

### Current `Vector[Index]` behavior

Vector indexing is currently separate from matrix Einstein indexing. For vectors, the checker requires exactly one `Int` index. Therefore `x[i]` where `i: Index` is rejected today. This is an intentional audit finding and a documentation gap to close in the proposed future contract: vector rank-1 indexed terms are desired, but not implemented.

### Current `@` typing/lowering relation

The typechecker treats `@` as part of linear algebra binary expressions. It supports:

- `Matrix<T> @ Matrix<U> -> Matrix<T*U scalar result>`;
- `Matrix<T> @ Vector<U> -> Vector<T*U scalar result>`.

It rejects unsupported left/right pairs such as vector-matrix and vector-vector with `operator '@' not defined for ...`.

The interpreter mirrors those two supported runtime cases with `evalMatrixMultiply` and `evalMatrixVectorMultiply`. The compiler mirrors them with `MatMulMM` and `MatMulMV` helper calls. This is already semantically Einstein-like, but the reference docs currently describe `@` as “matrix multiplication” rather than explicitly as contraction shorthand.

## 5. Current compiled gap

### Why compiled mode currently only supports concrete indexing

The compiled lowerer has no MIR or lowering representation for `Index` values or indexed matrix terms. Its matrix `ast.IndexExpr` lowering unconditionally expects exactly two indices and lowers both with expected type `Int`, then emits Go nested slice access: `target[row][col]`. Non-matrix indexing similarly expects one concrete `Int` index for arrays, bytes, and vectors.

There is no compiled equivalent of the interpreter’s `einsteinIndexedTerm`, no label-preserving compiled expression type, and no codegen helper for `EinMul`/`EinAdd`.

### Compiling current TensorEinstein tests

Observed baseline commands and failures:

- `go run ./cmd/oct test Language/Expressions/TensorEinsteinM0/valid --execution compiled` fails all 3 tests with `compiled mode does not yet support builtin Idx`.
- `go run ./cmd/oct test Language/Expressions/TensorEinsteinM1/valid --execution compiled` fails all 5 tests with `compiled mode does not yet support builtin Idx`.
- `go run ./cmd/oct test Language/Expressions/TensorEinsteinM3/valid --execution compiled` fails both tests with `compiled mode does not yet support builtin Idx`.

Because every current TensorEinstein valid fixture creates labels with `Idx`, compiled execution fails before reaching `[Index, Index]` lowering or explicit `EinMul`/`EinAdd` lowering.

### Explicit `EinMul` / `EinAdd` compiled status

Explicit M0 calls also fail in compiled mode today because `Idx` is unsupported. Even if `Idx` were lowered, the compiler still has no generated helper or builtin lowering for `EinMul`/`EinAdd`; Mx101b explicitly deferred those families from compiled tensor builtin parity.

### Mechanics/Continuum compiled impact

`Libraries/Mechanics/Mechanics.Continuum.oct` uses indexed Einstein notation in helpers such as `RightCauchyGreen2D(F) = F[k, i] * F[k, j]` and `LeftCauchyGreen2D(F) = F[i, k] * F[j, k]`. Compiled Mechanics tests that transitively call those helpers fail with `compiled mode does not yet support builtin Idx`.

A compiled run of `Libraries/Mechanics` also shows other generated-Go linear algebra gaps unrelated to Einstein notation, such as scalar-matrix multiplication in `LinearIsotropicStress2D` lowering to invalid Go (`float64 * [][]float64`). M31 should not solve those, but a Mechanics/Continuum compiled-parity milestone must account for them separately from indexed Einstein lowering.

## 6. Language contract proposal: arrays vs vectors/matrices/tensors

Proposed reference wording:

> Arrays are general ordered collection/storage values. An array type such as `Float[]` stores a sequence of `Float` values and supports collection-style operations and concrete integer indexing.
>
> Vectors and matrices are mathematical value categories. A `Vector<T>` is a rank-1 mathematical tensor value. A `Matrix<T>` is a rank-2 mathematical tensor value. They may use storage representations internally, but their language semantics are linear-algebra/tensor semantics, not generic collection semantics.
>
> Tensor notation is an index-aware expression surface over mathematical tensor values. Concrete integer indexing (`v[0]`, `A[0, 1]`) accesses stored elements. Symbolic index values (`i: Index`) describe tensor expression structure (`A[i, j]`) and drive contraction/free-index validation.
>
> Arrays and tensors can share storage-like representations, but they do not have the same meaning. `Float[]` is not automatically the same thing as `Vector<Float>`, and `Float[][]` is not automatically the same thing as `Matrix<Float>`.

Contract implications:

- Do not teach vector/matrix/tensor features as array features.
- Do not make `Float[]` implicitly behave as `Vector<Float>` to make tensor notation convenient.
- Future vector rank-1 indexed terms should apply to `Vector<T>`, not arrays.
- Matrix tensor notation should remain over `Matrix<T>`, not `T[][]`.

## 7. Language contract proposal: `@` as Einstein shorthand

### Current supported cases

Current `@` support is:

- `Matrix<T> @ Matrix<U> -> Matrix<V>`
- `Matrix<T> @ Vector<U> -> Vector<V>`

where `V` is the scalar multiplication/addition result type, including dimension propagation.

Current unsupported cases:

- `Vector<T> @ Matrix<U>`
- `Vector<T> @ Vector<U>`
- array forms such as `Float[][] @ Float[]`

### Proposed documentation framing

Proposed wording:

> `@` is Oct’s compact notation for common Einstein contractions over vectors and matrices. It is not a generic array multiplication operator. It is equivalent to writing the standard indexed contraction when the corresponding indexed tensor surface exists.

Document current conceptual meanings:

- `A @ B` means matrix-matrix contraction, conceptually `A[i, k] * B[k, j]`.
- `A @ x` means matrix-vector contraction, conceptually `A[i, j] * x[j]`.

Because vector rank-1 indexed terms are not implemented yet, the `A @ x` conceptual example should be marked as conceptual/future-equivalence rather than a source-level equivalence users can write today.

### Future/unsupported cases

- `x @ A` could conceptually mean `x[i] * A[i, j]` and return a vector, but it is unsupported today. It should remain future work until vector rank-1 terms exist and row-vector semantics are deliberately specified.
- `x @ y` could conceptually mean `x[i] * y[i]` and return a scalar dot product, but it is unsupported today. It should remain future work until scalar Einstein results and vector rank-1 terms exist.

## 8. Language contract proposal: Einstein subtraction

Expected future behavior:

- `A[i, j] - B[i, j]` is valid when both operands are rank-2 indexed matrix terms with exactly matching free-index order.
- Validation mirrors `EinAdd`:
  - both sides must be indexed terms;
  - labels inside each matrix term must be distinct;
  - free-index order must match exactly;
  - shapes must match exactly at runtime;
  - scalar element types must support ordinary `-`.
- `A[i, j] - B[j, i]` remains rejected unless a later design explicitly supports index-aligned reordering/transposition.
- Shape mismatch errors should mirror `EinAdd` wording, probably `runtime error: EinSub requires matching matrix shapes` or a generalized shared `EinAdd/EinSub requires matching matrix shapes` if implemented as one helper.
- Users should not be expected to write implicit negation workarounds such as `A[i, j] + (-1 * B)[i, j]`.

Public surface recommendation:

- Add infix indexed `-` first.
- Do not add public `EinSub` unless compatibility with explicit M0-style builtins is judged important. The user-facing goal is coherent source tensor notation, and adding another explicit builtin may preserve a transitional API longer than necessary.
- Internally, it is reasonable to implement a shared addition/subtraction helper or private operation string to avoid divergence from `EinAdd` validation.

## 9. Language contract proposal: vector rank-1 indexed terms

### Expected future behavior

- `v[i]` where `v: Vector<T>` and `i: Index` creates a rank-1 Einstein term.
- `v[n]` where `n: Int` remains concrete vector element access.
- `a[i] * b[i]` eventually contracts to a scalar dot product.
- `a[i] * b[j]` eventually produces a rank-2 outer-product matrix.
- `A[i, j] * x[j]` eventually produces a rank-1 vector.
- `x[i] * A[i, j]` eventually produces a rank-1 vector if row-vector/left-contraction semantics are accepted.
- `A[i, j] + x[i]` should remain invalid because free rank/label structure does not match.

### M0/M32/M33 scoping recommendation

Do **not** include vector rank-1 terms in the next milestone. The first compiled parity milestone should support the existing rank-2 matrix contract before expanding the language surface. Recommended staging:

1. M32: docs merge + interpreted rank-2 subtraction only.
2. M33: compiled parity for existing rank-2 matrix indexed expressions and subtraction.
3. M34: vector rank-1 indexed terms and vector/matrix mixed contractions.

### Required changes when implemented

Parser likely needs no syntax change because `IndexExpr` already represents `v[i]`. Required implementation changes:

- Extend typechecker `einsteinTermType` beyond exactly two labels, or introduce a general rank-aware term representation.
- Allow vector `Index` indexing to return a rank-1 indexed term while preserving `Int` element access.
- Generalize binary Einstein checking to produce scalar, vector, or matrix result types depending on free-index count.
- Extend interpreter `einsteinIndexedTerm` to store rank and either vector or matrix operands, or introduce a uniform tensor-term wrapper.
- Extend runtime contraction loops to support rank-1 operands and scalar/vector/matrix outputs.
- Extend compiled lowering after rank-2 parity is stable.

## 10. Documentation merge plan

Recommended documentation changes after M31:

1. Retitle `Language/reference/language/16-vectors-and-matrices.md` from **Vectors and Matrices** to **Vectors, Matrices, and Tensors**.
2. Keep the file path unless the reference index convention allows renaming without breaking links. A title-only change is safer as the first doc implementation milestone.
3. Move the substantive tensor content from `Language/reference/tensors.md` into the retitled vector/matrix page.
4. Convert `Language/reference/tensors.md` into a short pointer page during the transition:
   - “Tensor notation is documented in [Vectors, Matrices, and Tensors](./language/16-vectors-and-matrices.md).”
   - Include a brief note that the page is retained for link stability.
5. If a later docs cleanup has a clear link-update convention, remove the pointer only then.

Sections to include in the merged page:

- Arrays vs vectors/matrices/tensors.
- Vector and matrix literals/constructors.
- Concrete integer indexing.
- Symbolic `Index` values and matrix indexed terms.
- Einstein multiplication.
- Einstein addition.
- Einstein subtraction status: future/desired until implemented; current lack should be explicit.
- `@` as common Einstein contraction shorthand.
- `Trace(...)` instead of trace-style `A[i, i]`.
- Differential representational operators: `Grad`, `Div`, `SymGrad`, `Trace`.
- Interpreted vs compiled support status, with `Idx`/`EinMul`/`EinAdd`/indexed terms clearly marked interpreted-only until compiled parity lands.

Documentation gaps/inconsistencies to surface:

- The current vector/matrix page says `@` is matrix multiplication, distinct from elementwise operators. That is accurate operationally but misses the intended tensor-derived explanation.
- The current tensors page says vectors are “rank-1 tensor-like” rather than directly establishing the desired rank-1 mathematical tensor contract.
- The current reference does not clearly state that `Vector[Index]` is future/unsupported while `Vector[Int]` is current concrete access.

## 11. Compiled parity implementation plan

Recommended staged milestones:

### M32 — documentation merge + interpreted Einstein subtraction

Scope:

- Merge/retitle reference documentation as described above.
- Add interpreted/typechecked `A[i, j] - B[i, j]` for rank-2 matrix indexed terms.
- Add valid/invalid language contracts under `Language/Expressions/TensorEinstein...`.

Non-goals:

- compiled indexed lowering;
- vector rank-1 indexed terms;
- rank-N tensors;
- trace-style `A[i, i]`;
- `@` behavior changes.

### M33 — compiled parity for existing rank-2 matrix Einstein expressions

Scope:

- Compile `Idx`/`Index` enough to support current tensor tests.
- Compile explicit `EinMul`/`EinAdd` and infix `A[i,k] * B[k,j]`, `A[i,j] + B[i,j]`, and M32 subtraction if present.
- Preserve current validation and diagnostics as much as possible.
- Keep outputs rank-2 matrices only.

Non-goals:

- vector rank-1 terms;
- scalar dot products/double contractions;
- rank-N storage;
- index variance.

### M34 — vector rank-1 indexed terms and vector/matrix Einstein contractions

Scope:

- Add `Vector[Index]` indexed terms.
- Support dot product, outer product, matrix-vector, and possibly vector-matrix contractions.
- Align `@` equivalence tests with source-level indexed expressions.

### M35 — Mechanics/Continuum compiled parity and friction tests

Scope:

- Run and fix Mechanics/Continuum compiled tests using real helper paths.
- Separate Einstein gaps from other linear algebra compiled gaps such as scalar-matrix multiplication lowering.
- Add friction tests for continuum helpers that motivated subtraction and index notation clarity.

### Later

- Rank-N tensor design.
- Performance/reactor lowering.
- Covariant/contravariant index variance.
- Raising/lowering index operations.
- GPU/Prometheus tensor kernels, only after semantics are stable.

## 12. Implementation sketch for compiled rank-2 parity

### Representation strategy options

Option A: MIR representation for indexed matrix terms.

- Introduce a MIR-level value for rank-2 indexed terms carrying matrix expression plus two labels.
- Binary MIR lowering can inspect labels, choose `EinMul`/`EinAdd`/`EinSub` helper calls, and preserve result labels for nested expressions.
- This mirrors the interpreter/typechecker model and makes traces/debugging clearer.

Option B: direct lowering to helper calls during AST lowering.

- Detect indexed terms in `lowerExpr` and lower immediately to helper calls.
- Carry labels in a side-channel compiled expression type rather than MIR.
- Smaller initial patch, but more fragile for nested expressions and diagnostics.

Recommendation: use an explicit compiled expression metadata structure, and consider MIR support if direct lowering starts to obscure nested label preservation. The core risk is losing labels after a temporary matrix result; the implementation must preserve free-index labels for nested expressions just as the interpreter does.

### Helper semantics

The compiled helper should mirror `evalEinsteinBinaryMatrices` closely:

- inputs: operation, left matrix, left labels, right matrix, right labels;
- validate non-empty labels;
- validate extent consistency by label;
- compute counts/free/contracted labels deterministically;
- require exactly two free labels for multiplication in M33;
- require matching labels and shapes for addition/subtraction;
- return matrix plus free-label metadata to the compiler lowering path, or return only matrix if labels are tracked outside the runtime call.

Potential generated Go helper shape:

```go
func __octEinMulMM[T __octNumber](left [][]T, left0 string, left1 string, right [][]T, right0 string, right1 string) [][]T
func __octEinAddMM[T __octNumber](left [][]T, left0 string, left1 string, right [][]T, right0 string, right1 string) [][]T
func __octEinSubMM[T __octNumber](left [][]T, left0 string, left1 string, right [][]T, right0 string, right1 string) [][]T
```

If dimensioned element types are represented as Go numeric aliases, existing scalar helper constraints may suffice. If not, the helper should follow the same generic numeric constraints as `__octMatMulMM` and elementwise linear-algebra helpers.

### Generated loop nests vs shared helper

Use shared generated helpers first. They provide:

- one implementation to compare against interpreter behavior;
- centralized diagnostics;
- lower codegen complexity;
- easier future extension to subtraction.

Generated specialized loop nests can be considered later for performance once semantics and tests are stable.

### Element types

M33 should support the matrix element types that existing compiled matrix helpers can support:

- `Matrix<Int>`
- `Matrix<Float>`
- dimension-qualified `Matrix<Float<D>>` if represented compatibly by compiled scalar operations
- possibly `Matrix<Complex>` only if current compiled matrix helpers already support Complex sufficiently

Do not widen Complex support as part of M33 unless the audit at that time shows it is already available through ordinary compiled scalar multiplication/addition.

### Diagnostics

Preserve static diagnostics where possible:

- unsupported mixed index types;
- one-sided indexed binary expressions;
- mismatched free-index order;
- repeated label count > 2;
- non-rank-2 multiplication result.

Runtime diagnostics should mirror interpreter wording for shape/extent errors. If exact wording diverges, tests should assert stable user-facing substrings rather than generated-Go implementation details.

## 13. Test plan

Future implementation should add or run:

### M32 interpreted subtraction tests

- Valid: `A[i, j] - B[i, j]` produces elementwise difference.
- Valid: subtraction composes with multiplication/addition in nested expression trees.
- Invalid: `A[i, j] - B[j, i]` rejects mismatched free-index order.
- Invalid: one-sided indexed subtraction rejects with one-sided indexed expression diagnostic.
- Invalid/runtime: shape mismatch mirrors `EinAdd` shape validation.

### M33 compiled parity tests

- Existing TensorEinsteinM0 valid tests under `--execution compiled`.
- Existing TensorEinsteinM1 valid tests under `--execution compiled`.
- Existing TensorEinsteinM3 valid tests under `--execution compiled`.
- Compiled explicit `EinMul`/`EinAdd` tests.
- Compiled infix `*`, `+`, and M32 `-` tests.
- Compiled invalid/error diagnostics where compiled mode can report at typecheck/build time.

### M34 vector rank-1 tests

- `v[i]` produces rank-1 term while `v[0]` remains element access.
- `a[i] * b[i]` dot product.
- `a[i] * b[j]` outer product.
- `A[i, j] * x[j]` matrix-vector contraction.
- `x[i] * A[i, j]` vector-matrix contraction if accepted.
- Invalid rank/free-index mismatches.

### `@` equivalence tests

- `A @ B` equals `A[i, k] * B[k, j]` once compiled indexed matrix terms exist.
- `A @ x` equals `A[i, j] * x[j]` once vector rank-1 terms exist.
- If `x @ A` or `x @ y` become supported, add equivalence tests at the same time as the feature.

### Mechanics/Continuum compiled tests

- `RightCauchyGreen2D` compiled parity.
- `LeftCauchyGreen2D` compiled parity.
- `GreenLagrangeStrain2D` compiled parity.
- Continuum tests that combine `Trace`, indexed multiplication, scalar/matrix arithmetic, and future subtraction.

## 14. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Conflating arrays with vectors | Reference docs must state that arrays are storage collections and vectors are rank-1 mathematical tensors. Do not add tensor indexing to `T[]`. |
| Accidentally making `Float[]` behave like `Vector<Float>` | Keep typechecker paths separate. Require explicit vector literals/constructors or conversion APIs if such APIs are later designed. |
| Teaching `@` as arbitrary magic multiplication | Document it as common Einstein contraction shorthand and cross-link to indexed examples. |
| Over-expanding into rank-N tensors too soon | Keep M32/M33 rank-2 only; require a separate rank-N design milestone. |
| Breaking existing interpreted behavior | Run existing TensorEinsteinM0/M1/M3 interpreted suites before and after changes. Keep helper validation shared for addition/subtraction. |
| Adding subtraction with divergent validation | Implement subtraction by sharing `EinAdd` label/order/shape checks or by an explicitly paired helper with tests for identical failure modes. |
| Losing labels through nested compiled expressions | Preserve labels in compiled expression metadata or MIR. Add nested/reindexed compiled tests. |
| Performance traps from naive loop nests | Use shared helpers first for correctness. Optimize later with benchmarks and generated specialized loops only after semantics stabilize. |
| Documentation drift between tensor and vector/matrix pages | Merge substantive docs into one page and keep `tensors.md` as a pointer. |
| Mechanics compiled parity hiding non-Einstein failures | Track Mechanics failures by category: `Idx`/Einstein, scalar-matrix lowering, matrix element type mismatch, and unrelated generated-Go gaps. |

## 15. Final recommendation

The exact recommended next milestone after M31 is:

> **M32 — merge tensor documentation into the vector/matrix reference page and add interpreted rank-2 Einstein subtraction.**

M32 should include:

- retitle/merge docs into **Vectors, Matrices, and Tensors**;
- make arrays vs vectors/matrices/tensors explicit;
- document `@` as Einstein contraction shorthand;
- add interpreted/typechecked `A[i, j] - B[i, j]` with `EinAdd`-aligned validation;
- add language contracts for valid and invalid subtraction.

M32 non-goals:

- no compiled indexed tensor lowering;
- no vector rank-1 indexed terms;
- no arbitrary rank-N tensors;
- no covariant/contravariant variance;
- no automatic raising/lowering;
- no broadcasting;
- no trace-style `A[i, i]`;
- no new Prometheus/reactor/GPU kernels;
- no public array redesign.

Defer compiled parity to M33, vector rank-1 terms to M34, Mechanics/Continuum compiled parity hardening to M35, and rank-N/performance/variance work to later explicit design milestones.
