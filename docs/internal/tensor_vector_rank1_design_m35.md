# M35 — Vector rank-1 Einstein notation and tensor result-rank design audit

## 1. Executive summary

Post-M34 Oct has compiled/interpreted parity for the existing rank-2, matrix-backed Einstein notation surface. The supported indexed tensor path is intentionally narrow: `Idx("name")` constructs first-class `Index` labels; `A[i, j]` creates a symbolic rank-2 matrix term; `A[i, k] * B[k, j]`, `A[i, j] + B[i, j]`, and `A[i, j] - B[i, j]` produce `Matrix<T>` results; explicit `EinMul`/`EinAdd` remain supported; nested/reindexed rank-2 expressions compile; and M34 added ordinary compiled matrix/scalar and matrix/matrix element-wise arithmetic needed by Mechanics. Mechanics is now compiled-green without expanding the Einstein surface.

Vector rank-1 indexed terms are the next natural expansion because the language reference already defines `Vector<T>` as the rank-1 mathematical tensor value, `Matrix<T>` as rank-2, and `@` as contraction shorthand. However, `v[i]` where `i: Index` is still rejected today, so user-level notation cannot spell the same contraction that `A @ x` already represents mathematically. This creates an intentional but now prominent gap between the tensor model and source notation.

Recommended next implementation milestone after M35: **M36 — interpreted vector rank-1 indexed terms and result-rank contracts**. M36 should implement and test interpreted/typechecked behavior for `v[i]`, `a[i] * b[i]`, `a[i] * b[j]`, `A[i, j] * x[j]`, `x[i] * A[i, j]`, and rank-1 `+`/`-`, while preserving existing rank-2 behavior. It should not change `@` behavior yet, should not add rank-N tensors, should not make arrays tensor-indexable, and should continue rejecting `A[i, i]` trace sugar.

Deferred features: compiled parity for the rank-1 expansion, `@` expansion for `x @ A` and `x @ y`, scalar double contractions such as `A[i, j] * B[j, i]`, arbitrary rank-N storage/results, broadcasting, variance, raising/lowering indices, trace sugar, and GPU/reactor tensor kernels.

## 2. Current vector/matrix/tensor state

### Vector concrete indexing

- `v[n]` where `v: Vector<T>` and `n: Int` is concrete vector element access and returns scalar `T`.
- The typechecker requires exactly one index for vector indexing and currently requires that index to be `Int`.
- The interpreter mirrors that split: `ValueVector` accepts exactly one integer index and returns the selected element.
- Compiled lowering supports concrete vector indexing by lowering `Vector<T>` indexing to a scalar element expression.
- `v[i]` where `i: Index` is not supported today. It fails statically with the existing vector indexing diagnostic rather than becoming an indexed tensor term.

### Matrix concrete and symbolic indexing

- `A[r, c]` where both indices are `Int` is concrete matrix element access and returns scalar `T`.
- `A[i, j]` where both indices are `Index` creates a rank-2 indexed matrix term carrying labels.
- Mixed symbolic/concrete matrix indexing such as `A[i, 0]` is rejected.
- `A[i, i]` trace-style sugar is deliberately rejected; users must use `Trace(A)`.
- Materialized indexed-expression results are ordinary `Matrix<T>` values and can be explicitly reindexed in a later expression.

### Array concrete indexing

- Arrays remain general ordered storage values, not mathematical tensor values.
- `arr[n]` requires one `Int` index and returns the array element type.
- `Float[]` must not become tensor-indexable in the vector rank-1 milestone.

### Current `@` cases

- `A @ B` is supported for `Matrix<T> @ Matrix<U>` and returns `Matrix<T*U-result>`.
- `A @ x` is supported for `Matrix<T> @ Vector<U>` and returns `Vector<T*U-result>`.
- The reference documents these as contraction shorthand: `A @ B` is conceptually `A[i, k] * B[k, j]`, and `A @ x` is conceptually `A[i, j] * x[j]` even though `x[j]` is not source-expressible yet.
- Current unsupported `@` cases include `Vector @ Matrix`, `Vector @ Vector`, and arrays.

### Current element-wise vector/matrix operations

- `+`, `-`, `*`, and `/` on vectors are element-wise for matching vector lengths at runtime.
- `+`, `-`, `*`, and `/` on matrices are element-wise for matching matrix shapes at runtime.
- Numeric scalar expansion exists for vector/scalar, scalar/vector, matrix/scalar, and scalar/matrix forms.
- These element-wise operators are distinct from `@` and from indexed Einstein contraction.

### Current compiled support status

- M33 compiled the rank-2 matrix Einstein surface: `Idx`, explicit `EinMul`/`EinAdd`, infix rank-2 `*`, `+`, and `-`, and nested/reindexed rank-2 expression metadata.
- M34 compiled ordinary matrix/matrix and matrix/scalar element-wise arithmetic, and Mechanics now runs compiled without interpreted fallbacks.
- Compiled support still intentionally excludes vector rank-1 symbolic indexing, indexed dot/outer/matrix-vector notation, scalar/double contractions, rank-N tensors, trace sugar, broadcasting, variance, raising/lowering, and `@` behavior changes.

## 3. Current implementation constraints

### Typechecker constraints

- `ExprType.EinTerm` is rank-2-oriented. Its metadata stores a scalar type plus exactly two labels in `[2]string` and a `HasLabels` flag.
- Matrix symbolic indexing constructs `EinTerm` only for `[Index, Index]` matrix index expressions.
- Vector indexing currently accepts only one `Int` index, so `Vector[Index]` cannot reach Einstein expression checking.
- `checkEinsteinBinaryExpr` assumes two labels on each side, computes at most two free labels, and always returns `Matrix<T>`.
- Einstein multiplication currently rejects any result that does not have exactly two free labels. This is why scalar dot products, vector results, and double contractions are unavailable.
- Addition/subtraction require identical two-label order and always return a rank-2 matrix term.

### Interpreter constraints

- `evalResult.einTerm` stores an `einsteinIndexedTerm` with a `matrix Value` and `[]string` labels, so the runtime representation is matrix-specific even though labels are a slice.
- `evalEinsteinBinaryMatrices` enforces two labels for each operand and reads dimensions from `left.Matrix.Rows`, `left.Matrix.Cols`, `right.Matrix.Rows`, and `right.Matrix.Cols`.
- Multiplication requires exactly two free indices and allocates a `ValueMatrix` result.
- Addition/subtraction require distinct labels per rank-2 term, exact label order match, matching matrix shapes, and allocate a `ValueMatrix` result.
- Interpreter `@` already has a separate matrix-vector multiply path, but indexed Einstein evaluation cannot currently reuse it because vector indexed terms do not exist.

### Compiler constraints

- Compiled `einsteinTermMeta` stores only labels, but the lowering paths that use it require matrix operand types.
- `einsteinMulFreeLabels` requires two labels per operand and exactly two free result labels.
- Infix indexed compilation always emits `EinMul`, `EinAdd`, or `EinSub` MIR calls using six arguments: left matrix, two left labels, right matrix, two right labels.
- Generated helper code is rank-2 matrix-specific: `__octEinMulMM`, `__octEinAddMM`, and `__octEinSubMM` accept `[][]T` inputs and return `[][]T`.
- Compiled `@` is separate and lowers only matrix-vector and matrix-matrix cases through `MatMulMV`/`MatMulMM` helpers.

## 4. Desired rank-1 indexed term contract

### Valid contract

`v[i]` should become valid when all of the following hold:

- `v: Vector<T>`.
- The index list has exactly one index expression.
- That index expression has static type `Index`.
- The index value was constructed through `Idx("name")` or otherwise evaluates to a non-empty `Index` label.
- The expression is being used as an indexed tensor term rather than concrete element access.

The resulting term should carry:

- operand kind: vector;
- rank: 1;
- labels: `[i]`;
- scalar type: `T`;
- materialized value type: `Vector<T>` if the term is evaluated alone or assigned after composition.

### Concrete indexing remains unchanged

`v[0]` remains concrete vector element access. Static overload resolution should be based on the index expression type:

- `v[n: Int] -> T` concrete element;
- `v[i: Index] -> indexed rank-1 vector term`;
- any other index type is invalid.

### Arrays are not included

Array indexing must remain concrete storage indexing only:

- `Float[][Int] -> Float` remains valid.
- `Float[][Index]` remains invalid.
- No expected-type coercion should reinterpret `Float[]` as `Vector<Float>` for tensor notation.

### Label rules

- Labels must be non-empty at runtime; `Idx` already enforces this and generalized runtime code should keep a defensive check.
- Repeated-label rules should align with rank-2 terms: in a multiplication expression a label may appear once as free or twice as contracted; labels appearing more than twice are invalid.
- A single rank-1 term cannot contain a repeated label because it has only one slot.
- Trace-style repeated labels inside one rank-2 matrix term, `A[i, i]`, should remain rejected even after generalized counting is introduced.

## 5. Desired result-rank rules

General rule for multiplication of indexed rank-1/rank-2 terms:

1. Count labels across both operands in left-to-right slot order.
2. Labels appearing once are free labels.
3. Labels appearing twice are contracted labels.
4. Labels appearing more than twice are invalid.
5. Result rank is the number of free labels.
6. Supported result types for M36/M37 are:
   - 0 free labels: scalar `T`;
   - 1 free label: `Vector<T>`;
   - 2 free labels: `Matrix<T>`;
   - more than 2 free labels: reject until rank-N tensors exist.

| Expression | Free labels | Contracted labels | Result rank | Result Oct type | Recommendation |
| --- | --- | --- | ---: | --- | --- |
| `a[i] * b[i]` | none | `i` | 0 | scalar `T` | Support in M36 interpreted; compile in M37. This is dot product. |
| `a[i] * b[j]` | `i, j` | none | 2 | `Matrix<T>` | Support in M36 interpreted; compile in M37. This is outer product with row label from the left vector and column label from the right vector. |
| `A[i,j] * x[j]` | `i` | `j` | 1 | `Vector<T>` | Support in M36 interpreted; compile in M37. This matches existing `A @ x`. |
| `x[i] * A[i,j]` | `j` | `i` | 1 | `Vector<T>` | Support in M36 interpreted if diagnostics and loop order are clear; compile in M37. This is left vector/matrix contraction and is the natural future meaning of `x @ A`. |
| `A[i,k] * B[k,j]` | `i, j` | `k` | 2 | `Matrix<T>` | Already supported; preserve exact behavior and tests. |
| `A[i,j] + B[i,j]` | `i, j` | none | 2 | `Matrix<T>` | Already supported; preserve exact free-index-order behavior. |
| `A[i,j] - B[i,j]` | `i, j` | none | 2 | `Matrix<T>` | Already supported after M32/M33; preserve exact behavior. |
| `A[i,j] * B[j,i]` | none | `i, j` | 0 | scalar `T` | Recommend later, not M36. This is mathematically valid double contraction but adds scalar result paths and should be staged after rank-1 parity. |
| `A[i,j] * B[i,j]` | none | `i, j` | 0 | scalar `T` | Recommend later with the same double-contraction milestone as above. This is Frobenius inner product. |

Notes:

- The result scalar type should come from existing scalar binary `*` and `+` rules over element types, including dimension propagation.
- Runtime extent checks must ensure every occurrence of a repeated label has the same extent.
- Outer product uses no contraction; it should not be confused with broadcasting. It is a tensor product because both free labels are explicit.
- `A[i,j] * B[j,i]` and `A[i,j] * B[i,j]` are intentionally not recommended for the first rank-1 milestone to avoid mixing two independent expansions: vector notation and rank-2 scalar double contraction.

## 6. Addition/subtraction rank rules

Addition and subtraction should be rank-preserving and label-order-strict:

| Expression | Result | Recommendation |
| --- | --- | --- |
| `a[i] + b[i]` | `Vector<T>` | Support in M36 interpreted and M37 compiled. |
| `a[i] - b[i]` | `Vector<T>` | Support in M36 interpreted and M37 compiled. |
| `A[i,j] + B[i,j]` | `Matrix<T>` | Preserve current behavior. |
| `A[i,j] - B[i,j]` | `Matrix<T>` | Preserve current behavior. |
| `A[i,j] + x[i]` | invalid | Reject rank mismatch. |
| `a[i] + b[j]` | invalid | Reject mismatched free-label order/name. |
| `A[i,j] + B[j,i]` | invalid | Continue rejecting implicit transposition/reordering. |

Rules:

- Both operands must be indexed terms.
- Operators `+` and `-` require equal rank and exactly matching free-label order.
- For rank-1 vector terms, free-index order is a single label and must match exactly by name.
- No implicit reordering, transposition, broadcasting, or singleton expansion should be added.
- Addition/subtraction should not contract repeated labels. Any diagonal/trace-like single operand form remains invalid.

## 7. Relationship to `@`

### Current audit

`@` already represents a subset of Einstein contraction in ordinary linear algebra syntax:

- `A @ B` is equivalent to `A[i, k] * B[k, j]`.
- `A @ x` is equivalent to `A[i, j] * x[j]`.

The current implementation supports exactly those two cases and rejects vector-matrix, vector-vector, and arrays. This is coherent today because vector symbolic indexing is absent, but it will feel incomplete once `x[i]` exists.

### Future policy recommendation

Stage `@` alignment after vector rank-1 indexed terms have interpreted and compiled parity:

1. **M36/M37:** Do not change `@`. Add indexed notation and parity first.
2. **M38:** Add `@` equivalence tests and decide/implement expansion:
   - `x @ A` should be supported as `x[i] * A[i, j] -> Vector<T>` if the language accepts left vector/matrix contraction.
   - `x @ y` should be supported as `x[i] * y[i] -> scalar T` if dot product notation is stable.
3. Keep arrays rejected for `@`.
4. Keep `@` as contraction, not element-wise multiplication and not broadcasting.

Recommended final direction: support `x @ A` and `x @ y` in M38, but only after indexed vector notation has shipped in both execution modes. This keeps `@` a shorthand for supported contractions rather than an independently growing operator.

## 8. Typechecker design sketch

### Generalized term metadata

Replace the fixed two-label metadata with a generalized representation, for example:

```go
type einsteinTermType struct {
    ScalarType Type
    Rank       int
    Labels     []string
    HasLabels  bool
    Operand    einsteinOperandKind // scalar/vector/matrix if useful for diagnostics
}
```

The typechecker should create terms for:

- `Vector<T>[Index]` with rank 1, one label, and value type `Vector<T>`;
- `Matrix<T>[Index, Index]` with rank 2, two labels, and value type `Matrix<T>`.

Arrays should not produce `EinTerm`.

### Result type selection

For multiplication:

- Count labels across operands.
- Reject labels appearing more than twice.
- Reject unsupported operand ranks beyond 1 and 2.
- Reject result free-label count greater than 2 until rank-N tensors exist.
- Select result type by free-label count:
  - 0 -> scalar element result type;
  - 1 -> `Vector<element result type>`;
  - 2 -> `Matrix<element result type>`.

For addition/subtraction:

- Require both operands to be indexed terms.
- Require equal rank.
- Require exactly matching labels in order.
- Return the same rank: rank 1 -> vector, rank 2 -> matrix.

### Static rejection rules

- One-sided indexed binary expressions remain invalid.
- Labels appearing more than twice in multiplication are invalid.
- Free-label count greater than 2 is invalid until rank-N tensors exist.
- Addition/subtraction rank or label mismatch is invalid.
- Mixed concrete/symbolic indices are invalid: `v[i + 1]`, `A[i, 0]`, `A[0, i]`.
- Arrays indexed by `Index` are invalid.
- `A[i, i]` remains invalid as trace-style sugar.
- Empty labels are already rejected by `Idx`; keep defensive runtime validation.

### Diagnostics

The generalized checker should produce rank-aware diagnostics without exposing implementation internals:

- `vector indexing expects either [Int] element access or [Index] Einstein term access, got [String]`.
- `EinMul result rank 3 is not supported until rank-N tensors are implemented`.
- `EinAdd requires matching free-index order on both terms (left=[i], right=[j])`.
- `trace-style contraction '[i,i]' is not supported; use Trace(...)`.

## 9. Interpreter design sketch

### Generalized runtime representation

Replace the matrix-only term with an indexed term shape similar to:

```go
type indexedTerm struct {
    rank   int
    labels []string
    value  Value // ValueVector or ValueMatrix for M36/M37
}
```

Optional helper methods can expose slot extents and element lookup:

- vector slot 0 extent: `len(value.Vector)`;
- matrix slot 0 extent: rows;
- matrix slot 1 extent: cols;
- vector element lookup by assignment label;
- matrix element lookup by row/column assignment labels.

### Generalized binary evaluation

A generalized `evalEinsteinBinaryTerms` should:

1. Validate ranks and labels.
2. Build label extent maps from all operand slots.
3. Count labels to identify free and contracted labels.
4. Select result allocation by free-label count.
5. Iterate output free-label assignments.
6. Recursively or iteratively loop over contracted-label assignments.
7. Multiply operand elements and accumulate with scalar `+`.
8. Allocate scalar/vector/matrix output based on result rank.

### Output allocation

- Rank 0: return scalar `Value`.
- Rank 1: return `ValueVector` with length equal to the free label extent.
- Rank 2: return `ValueMatrix` with rows/cols equal to the two free label extents in free-label order.

### Preserving current behavior

- Existing rank-2 matrix multiplication/addition/subtraction should keep producing the same values and diagnostics where possible.
- Reindexed matrix intermediates should continue to work.
- New vector intermediates should be reindexable by `Vector[Index]` once implemented.
- The interpreter and compiled generated helpers should share the same conceptual algorithm and validation rules, even if they cannot literally share Go code because compiled helpers are emitted into generated programs.

## 10. Compiler design sketch

### Metadata changes

Compiled lowering should extend `einsteinTermMeta` to include rank and possibly operand/value type:

```go
type einsteinTermMeta struct {
    Labels []string
    Rank   int
    Type   string // optional, useful for helper selection
}
```

Lowering of `Vector[Index]` should set rank 1 term metadata without materializing an element. Lowering of `Matrix[Index, Index]` should continue setting rank 2 metadata.

### Helper strategy

Two viable strategies exist:

1. **Small specialized helpers first** (recommended for M37):
   - `__octEinMulVV` handles dot and outer depending on labels, or split into `__octEinDotVV` and `__octEinOuterVV` after typechecker result-rank selection.
   - `__octEinMulMV` handles `A[i,j] * x[j]` and related matrix-vector one-contraction forms.
   - `__octEinMulVM` handles `x[i] * A[i,j]`.
   - `__octEinAddVV`/`__octEinSubVV` handle vector indexed addition/subtraction.
   - Keep existing `__octEinMulMM`, `__octEinAddMM`, `__octEinSubMM` for rank-2 outputs.
2. **Generalized emitted helper** (better later):
   - A single helper over descriptor structs can reduce duplication but may be harder to keep readable in generated Go.

Recommended path: use specialized helpers for M37, backed by a shared helper for label counting/free-label decisions where practical. This limits blast radius and makes generated code easier to audit.

### Result type handling

- MIR calls need return types that may be scalar, `Vector<T>`, or `Matrix<T>`.
- Indexed binary lowering should choose helper and return type after generalized label analysis.
- If a multiplication result is scalar, do not attach `EinTerm` metadata to the temporary.
- If a multiplication/addition/subtraction result is vector or matrix, attach term metadata for reindexed intermediate workflows.

### Label preservation and reindexing

- Preserve free-label order exactly as determined by left-to-right slot order.
- Assigned vector intermediates should behave like ordinary `Vector<T>` values; explicit `partial[i]` should recreate term metadata.
- Assigned matrix intermediates should retain current behavior: materialized `Matrix<T>` values can be reindexed explicitly.
- Do not implicitly carry hidden label metadata across non-indexed value uses.

### No rank-N outputs yet

If generalized analysis finds three or more free labels, compiled mode should reject with the same static diagnostic as interpreted/typechecked mode. No rank-3/4 storage or helper emission should be added in M36/M37.

## 11. Test plan for future implementation

### Valid vector indexed terms

- `v[i]` can participate in `v[i] + w[i]` and materialize as `Vector<T>`.
- `v[0]` still returns scalar `T`.
- Reindexed vector intermediate: `let y = a[i] + b[i]; let z = y[i] - a[i]`.

### Invalid vector `Index` cases

- Wrong arity: `v[i, j]` remains invalid.
- Wrong type: `v["i"]` invalid.
- Mixed/computed symbolic-looking access: `v[i + 1]` invalid.
- Empty label via defensive runtime path if reachable.

### Dot product

- `a[i] * b[i]` returns scalar with expected numeric and dimension result.
- Length mismatch reports an extent mismatch for label `i`.
- Compiled/interpreted parity once M37 starts.

### Outer product

- `a[i] * b[j]` returns `Matrix<T>` with rows from `a` and cols from `b`.
- Non-square vector lengths prove this is not accidental element-wise behavior.

### Matrix-vector and vector-matrix

- `A[i,j] * x[j]` matches existing `A @ x` for values and dimensions.
- `x[i] * A[i,j]` returns a vector over label `j` if accepted.
- Shape/extent mismatch diagnostics are label-based.

### Addition/subtraction

- `a[i] + b[i]` and `a[i] - b[i]` return vectors.
- `a[i] + b[j]` rejects label mismatch.
- `A[i,j] + x[i]` rejects rank mismatch.

### `@` equivalence tests

- In M36/M37, add documentation-only or pending tests comparing `A @ x` with `A[i,j] * x[j]` if the harness supports it.
- In M38, add active tests for `x @ A` and `x @ y` only if implemented.

### No array tensor indexing

- `let a: Float[] = [1.0, 2.0]; let i = Idx("i"); a[i]` must remain invalid.
- Ensure expected type does not convert arrays to vectors for tensor notation.

### Regression suites

- Existing TensorEinsteinM0/M1/M3/M4 valid and invalid tests should remain green.
- Add future tests under `Language/Expressions/TensorEinsteinM5` or `TensorEinsteinRank1M36` with separate valid/invalid manifests.
- Run each new valid suite in interpreted first, then compiled once M37 is implemented.

## 12. Documentation plan

After implementation, update:

- `Language/reference/language/16-vectors-and-matrices.md`:
  - add vector rank-1 symbolic indexing section;
  - specify `v[Int]` versus `v[Index]` overload behavior;
  - add dot, outer, matrix-vector, and vector-matrix examples;
  - document result-rank rules for 0/1/2 free labels;
  - clarify double contractions remain deferred if not implemented;
  - keep arrays explicitly non-tensor-indexable;
  - update `@` relationship after M38 if `x @ A`/`x @ y` are added.
- `Language/reference/tensors.md` can remain a pointer unless the repo later consolidates all tensor docs elsewhere.
- `docs/COMPILED_SUPPORT.md`:
  - after M36, note interpreted-only rank-1 support and compiled gap;
  - after M37, note compiled parity;
  - after M38, document any `@` expansion.
- Internal milestone docs should record any intentionally staged inconsistencies between indexed notation and `@`.

## 13. Staged milestone plan

### M36 — interpreted vector rank-1 terms and result-rank contracts

Scope:

- Typechecker and interpreter support for `Vector[Index]` indexed terms.
- Interpreted dot, outer, matrix-vector, vector-matrix, and vector indexed `+`/`-`.
- Static result-rank selection for scalar/vector/matrix results.
- New language contract tests under `Language/`.
- Preserve arrays, trace rejection, rank-N rejection, and current `@` behavior.

Non-goals:

- Compiled parity.
- `@` expansion.
- Rank-2 double contractions unless deliberately split into a later scalar-contraction milestone.

### M37 — compiled parity for vector rank-1 terms

Scope:

- Compile `Vector[Index]` term metadata.
- Emit helpers for dot/outer/matrix-vector/vector-matrix and vector indexed `+`/`-`.
- Keep M36 tests green in compiled mode.
- Preserve M33/M34 compiled support.

### M38 — `@` alignment expansion

Scope:

- Decide and implement `x @ A` as vector-matrix contraction if M36/M37 accepted `x[i] * A[i,j]`.
- Decide and implement `x @ y` as dot product if M36/M37 dot product is stable.
- Add explicit equivalence tests between `@` and indexed notation.
- Keep arrays rejected and avoid broadcasting semantics.

### M39 — continuum/mechanics/vector friction tests

Scope:

- Exercise rank-1 notation in Mechanics-style vector/matrix formulas.
- Add friction/regression tests for dimensions and shape diagnostics.
- Confirm no regressions in LinearAlgebra, Mathematics, RF, and Mechanics compiled lanes.

### Later — rank-N tensor design

Scope:

- Design rank-3/4 storage and result types if needed.
- Revisit scalar double contractions for rank-2 matrices if not already implemented.
- Consider whether generalized helper descriptors are worth replacing specialized helpers.
- Keep variance/raising/lowering as separate design work, not incidental implementation.

## 14. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Arrays/vectors conflation | Keep `Float[]` and `Vector<Float>` distinct in typechecking, docs, tests, and diagnostics. Add invalid `Array[Index]` tests. |
| Broadcasting temptation | Require explicit labels for every free axis. Reject rank mismatch and label mismatch for `+`/`-`. Do not treat scalar/vector/matrix result-rank selection as broadcasting. |
| Scalar result path complexity | Stage scalar dot product first with vectors; defer rank-2 double contraction if needed. Ensure scalar results do not carry stale `EinTerm` metadata. |
| Result-rank diagnostics become confusing | Centralize label counting/free-rank analysis and produce rank-aware errors with expression labels. |
| Accidental `A[i,i]` trace sugar | Keep the single-term repeated-label check before generalized multiplication analysis. Maintain invalid tests. |
| Performance pitfalls | Use specialized compiled helpers for common rank-1/rank-2 cases before introducing descriptor-heavy generic helpers. Validate extents once per operation. |
| Documentation drift | Update language reference and compiled support docs in the same milestones that change behavior. Call out interpreted-only gaps between M36 and M37. |
| `@` expectations and user confusion | Do not change `@` in M36/M37. Document that `@` expansion is staged after indexed notation parity. Add equivalence tests in M38. |
| Dimension propagation mistakes | Reuse existing scalar `*`, `+`, and `-` typechecking for element result types and add dimension-qualified tests. |
| Reindexed intermediate bugs | Add tests for vector and matrix intermediates that are assigned, then explicitly reindexed. |

## 15. Final recommendation

The exact recommended next implementation milestone is **M36 — interpreted vector rank-1 indexed terms and result-rank contracts**.

M36 should implement only the interpreted/typechecked language contract for:

- `v[i]` where `v: Vector<T>` and `i: Index`;
- `a[i] * b[i] -> T` dot product;
- `a[i] * b[j] -> Matrix<T>` outer product;
- `A[i,j] * x[j] -> Vector<T>` matrix-vector contraction;
- `x[i] * A[i,j] -> Vector<T>` vector-matrix contraction if accepted by the same generalized rules;
- `a[i] + b[i]` and `a[i] - b[i] -> Vector<T>`;
- strict rejection of arrays, mixed concrete/symbolic indexing, unsupported rank-N outputs, label counts greater than two, and trace-style `A[i,i]`.

M36 non-goals:

- no production parser syntax changes beyond using existing index expression forms;
- no compiled parity;
- no `@` behavior changes;
- no arbitrary rank-N tensors;
- no rank-3/4 storage;
- no covariant/contravariant variance;
- no raising/lowering;
- no broadcasting;
- no trace sugar;
- no GPU/reactor tensor kernels.

Deferred features after M36 should be staged as M37 compiled parity, M38 `@` alignment expansion, M39 library friction tests, and later rank-N/scalar-double-contraction design as needs become concrete.
