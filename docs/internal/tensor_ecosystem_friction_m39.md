# M39 — tensor ecosystem friction audit after vector/matrix/@ parity

## 1. Executive summary

M39 is an audit-only milestone. It changes no production behavior, no tests, and no language reference semantics.

After M38, Oct has a coherent interpreted+compiled tensor core for the first two mathematical tensor ranks:

- `Vector<T>` is the rank-1 mathematical tensor value.
- `Matrix<T>` is the rank-2 mathematical tensor value.
- Arrays remain storage/collection values and are not tensors.
- `Idx("name") -> Index` produces symbolic labels for indexed tensor notation.
- `Vector[Index]` and `Matrix[Index, Index]` produce indexed tensor terms.
- Native indexed tensor notation has interpreted and compiled parity for supported scalar, vector, and matrix results.
- `@` is shorthand for the supported vector/matrix Einstein contractions:
  - `A @ B == A[i, k] * B[k, j]`
  - `A @ x == A[i, j] * x[j]`
  - `x @ A == x[i] * A[i, j]`
  - `x @ y == x[i] * y[i]`

The tensor core should continue for exactly one more bounded milestone before switching lanes to package-manager federation design: **M40 — matrix/matrix scalar double contractions**. This is the smallest remaining high-value tensor gap because the current typechecker, interpreter, compiler metadata, generated helper style, and continuum library use-cases already orbit the missing shape. It should be implemented before rank-N tensors and before trace sugar, while keeping package federation next in the strategic queue after this bounded tensor closure.

Explicitly deferred after M39/M40 planning:

- rank-N tensor value/storage design and implementation;
- trace-style indexed sugar `A[i, i]`;
- broadcasting;
- arrays as tensors or implicit array/vector/matrix reinterpretation;
- covariant/contravariant variance;
- raising/lowering indices;
- Prometheus/reactor/GPU tensor kernels;
- package-manager federation/P2P implementation;
- `@` behavior changes.

## 2. Current capability table

| Feature | Interpreted | Compiled | Notes |
| --- | --- | --- | --- |
| `Idx("i")` | Yes | Yes | Returns `Index`; runtime/compiler helpers reject empty labels. |
| `Matrix[Index,Index]` | Yes | Yes | Produces rank-2 indexed term metadata; repeated labels in the single indexing expression are rejected as trace sugar. |
| `Vector[Index]` | Yes | Yes | Produces rank-1 indexed term metadata. |
| `Matrix[Int,Int]` | Yes | Yes | Concrete matrix element access. |
| `Vector[Int]` | Yes | Yes | Concrete vector element access. |
| array indexing | Yes | Yes | Concrete `Array[Int]` storage access only; no tensor labels. |
| `A[i,k] * B[k,j]` | Yes | Yes | Rank-2 matrix contraction returning `Matrix<T>`. |
| `A[i,j] + B[i,j]` | Yes | Yes | Rank-2 indexed elementwise addition; free-index order must match. |
| `A[i,j] - B[i,j]` | Yes | Yes | Rank-2 indexed elementwise subtraction; free-index order must match. |
| `a[i] + b[i]` | Yes | Yes | Rank-1 indexed vector addition. |
| `a[i] - b[i]` | Yes | Yes | Rank-1 indexed vector subtraction. |
| `a[i] * b[i]` | Yes | Yes | Vector dot product returning scalar. |
| `a[i] * b[j]` | Yes | Yes | Vector outer product returning `Matrix<T>`. |
| `A[i,j] * x[j]` | Yes | Yes | Matrix-vector indexed contraction returning `Vector<T>`. |
| `x[i] * A[i,j]` | Yes | Yes | Vector-matrix indexed contraction returning `Vector<T>`. |
| `A @ B` | Yes | Yes | Matrix-matrix contraction shorthand. |
| `A @ x` | Yes | Yes | Matrix-vector contraction shorthand. |
| `x @ A` | Yes | Yes | Vector-matrix contraction shorthand added in M38. |
| `x @ y` | Yes | Yes | Vector-vector dot shorthand added in M38. |
| `A[i,j] * B[j,i]` | No | No | Matrix/matrix scalar double contraction remains explicitly deferred. |
| `A[i,j] * B[i,j]` | No | No | Frobenius-style matrix/matrix scalar double contraction remains explicitly deferred. |
| `A[i,i]` | No | No | Trace-style sugar remains rejected; use `Trace(A)`. |
| arrays with tensor indices | No | No | Arrays remain storage values and require `Int` indices. |
| rank-N result expressions | No | No | Expressions with more than two free indices are rejected; no rank-3/4 storage/value type exists. |
| broadcasting | No | No | Element-wise operations require compatible lengths/shapes; `@` and indexed notation do not broadcast. |

## 3. Current docs audit

### What the docs explain clearly

- **Arrays vs vectors/matrices/tensors:** The vector/matrix reference explicitly separates arrays as storage values from vectors and matrices as mathematical values, and states that `Float[]`/`Float[][]` are not automatically `Vector<Float>`/`Matrix<Float>`.
- **Vector rank-1 tensor notation:** The reference now documents `v[i]` where `i: Index`, vector indexed add/sub, dot, outer, matrix-vector, and vector-matrix forms.
- **Matrix rank-2 tensor notation:** The reference documents `A[i, j]`, matrix indexed multiplication, addition, subtraction, and explicit `Trace(A)`.
- **Result-rank rules:** The reference documents scalar, vector, and matrix results for rank-1/rank-2 indexed multiplication and defers rank-N tensor outputs.
- **`@` shorthand:** The reference and compiled support docs describe `A @ B`, `A @ x`, `x @ A`, and `x @ y` as shorthand for supported Einstein contractions.
- **Unsupported double contractions:** The reference and compiled support docs explicitly defer `A[i, j] * B[j, i]` and `A[i, j] * B[i, j]`.
- **Unsupported rank-N:** The reference and compiled support docs explicitly defer arbitrary rank-N tensors.
- **`Trace(A)` instead of `A[i,i]`:** The reference documents trace sugar as unsupported and directs users to `Trace(A)`.
- **Interpreted/compiled boundaries:** `docs/COMPILED_SUPPORT.md` captures M33/M36/M37/M38 support boundaries and remaining compiled deferrals.

### Small documentation gaps for a later docs cleanup

- The vector/matrix reference still uses milestone phrasing such as “M36 supports interpreted result ranks” in a few places. That was accurate for M36 but now undersells M37/M38 compiled parity. A cleanup should rephrase the stable contract as “interpreted and compiled support scalar/vector/matrix results for these forms,” with milestone details moved to internal docs.
- `Language/reference/tensors.md` is intentionally only a compatibility pointer. That is fine, but a later cleanup could add a one-paragraph summary there for search/discovery without duplicating the full contract.
- The docs mention unsupported double contractions but do not include a compact “why deferred / likely next” note. If M40 implements them, this gap disappears; if not, the reference should add one sentence explaining that explicit loops/manual formulas remain required for Frobenius-style matrix inner products.
- There is no small “NumPy einsum vs Oct indexed notation” comparison. This would be useful after M40 but should not displace the implementation gap.
- Library docs for `Libraries/Mechanics` do not yet include a small example contrasting `Trace(T)`, `A @ x`, and indexed `F[k, i] * F[k, j]`. That is docs/example polish, not a language blocker.

## 4. Current test coverage audit

| Suite | Coverage | Interpreted? | Compiled? | Missing invalid tests | Missing rectangular/non-square cases | Missing dimensioned/complex/numeric mixed cases |
| --- | --- | --- | --- | --- | --- | --- |
| `TensorEinsteinM0` | Explicit `EinMul` matrix multiplication, free label renaming, explicit `EinAdd`; invalid non-`Index` and non-matrix arguments. | Yes | Yes after M33 | No explicit `EinSub` helper invalid/valid coverage in M0; later suites cover infix subtraction. | Mostly 2x2 square matrices. | No dimension-qualified, complex, or mixed numeric matrix tests. |
| `TensorEinsteinM1` | Infix rank-2 matrix `*` and `+`, nested composition, reindexed intermediates; invalid mismatched free indices, malformed one-sided indexed expression, mixed `Index`/`Int`, wrong vector arity. | Yes | Yes after M33 | Invalid matrix/matrix double contractions are not here; covered later by M5. | Mostly 2x2 square matrices; limited rectangular matrix-matrix shape pressure. | No dimension-qualified, complex, or mixed numeric tests. |
| `TensorEinsteinM3` | Nested/reindexed rank-2 behavior and trace rejection. | Yes | Yes after M33 | Good coverage of nested free-index count/order mismatch and trace rejection. | Mostly square examples. | No dimension-qualified, complex, or mixed numeric tests. |
| `TensorEinsteinM4` | Rank-2 indexed subtraction; invalid mismatched free indices, one-sided subtraction operand, trace rejection. | Yes | Yes after M33 | Adequate for rank-2 subtraction shape of contract. | Mostly square examples. | No dimension-qualified, complex, or mixed numeric tests. |
| `TensorEinsteinM5` | Vector rank-1 indexed terms: concrete vector indexing, vector add/sub, dot, outer, mixed matrix-vector/vector-matrix contractions, reindexed vector intermediate; invalid arrays with `Index`, index appears >2 times, double contraction deferred, rank mismatch, result rank >2, trace rejection, vector arity/type/label errors. | Yes | Yes after M37 | Static invalid coverage is strong; runtime mismatched vector lengths/matrix extents are exercised less directly. | Has vector length 3 and 2x2 mixed contractions, but limited rectangular mixed matrix/vector cases. | No dimension-qualified, complex, or mixed numeric tests. |
| `TensorEinsteinM6` | `@` equivalence with indexed notation for matrix-matrix, matrix-vector, vector-matrix, vector-vector; rectangular `A @ x` and `x @ A`; invalid arrays and scalar/vector. | Yes | Yes after M38 | Invalid `Matrix @ Array`, `Array @ Matrix`, and incompatible runtime shapes are not expressible as static `.octfail` in the current type system; runtime tests could be added if the test runner supports expected runtime failures. | Good rectangular coverage for matrix-vector/vector-matrix; matrix-matrix remains mostly 2x2. | No dimension-qualified, complex, or mixed numeric `@` tests. |

General test observations:

- The suites now cover the core interpreted+compiled parity path for current tensor syntax.
- Invalid tests correctly pin arrays as non-tensor storage values and keep trace sugar/double contractions/rank-N deferred.
- Rectangular coverage exists for M38 mixed `@`, but matrix-matrix indexed multiplication and addition/subtraction are still square-heavy.
- Dimension-qualified tensor tests would be valuable because `@` and indexed multiplication rely on scalar multiplication/addition result typing.
- Complex tensor tests are absent. This may be acceptable if current generic compiled helpers are numeric but broader complex matrix semantics are not a near-term tensor goal.

## 5. Library friction audit

### `Libraries/Mechanics`

Mechanics is the strongest current consumer of tensor notation.

Current tensor support already removes prior workarounds:

- `Mechanics.Core.InternalForce` uses `stiffness.K @ d`, which is now a concise matrix-vector tensor contraction with interpreted+compiled parity.
- `Mechanics.Continuum.RightCauchyGreen2D` uses `F[k, i] * F[k, j]`, a natural indexed matrix/matrix contraction.
- `Mechanics.Continuum.LeftCauchyGreen2D` uses `F[i, k] * F[j, k]`, another natural rank-2 contraction.
- `Trace(T)` keeps invariant helpers explicit and readable.

Friction that remains:

- `VonMisesStress2D` manually expands a deviatoric double contraction (`sxx*sxx + syy*syy + szz*szz + 2.0*sxy*sxy`). A matrix/matrix scalar double contraction would make the 2D in-plane portion of such expressions more direct. Full 3D out-of-plane accounting would still need explicit modeling because rank-N/3D tensor storage remains deferred.
- `SecondInvariantI2` uses an explicit 2x2 determinant formula. Trace sugar would not materially improve this; a matrix double contraction could support alternate invariant formulas later.
- Trace sugar `A[i, i]` would save very little because `Trace(A)` is already clear and avoids ambiguity.

### `Libraries/LinearAlgebra`

LinearAlgebra is older and intentionally array-backed:

- The README states that vectors are `Float[]` and matrices are flattened row-major `Float[]` with explicit `(rows, cols)` parameters.
- `MatrixTrace(A: Float[], rows: Int, cols: Int)` and matrix/vector helpers operate over flat arrays for explicit shape control.

Friction:

- Current tensor support does not automatically improve this library because arrays are deliberately not tensors.
- A future docs/example milestone could show when to choose `Libraries/LinearAlgebra` flat-array APIs versus native `Vector<T>`/`Matrix<T>` values.
- Redesigning LinearAlgebra around native matrices is out of scope for M39 and would be a library API migration, not a tensor core feature.

### `Libraries/RF`

RF is mostly array/record based. The MIMO channel helper stores coefficients as row-major `Float[]` with explicit row/column counts and loops over transmit vectors.

Friction:

- `ApplyMimoChannel` is conceptually `H @ x`, but its public API uses `Float[]`, so current tensor support cannot apply without redesigning the `MimoChannel` representation.
- Row/column power gain helpers are reductions over flattened coefficients; `A[i, j] * A[i, j]` would be the mathematical shape for total Frobenius power if the channel were represented as `Matrix<Float>`.
- This is an opportunity for examples or a future native-matrix RF API variant, not a reason to make arrays tensor-indexable.

### `Libraries/Mathematics`

Mathematics primarily uses scalar functions, calculus helpers, transforms, and arrays/complex traces. It does not create strong immediate pressure for tensor syntax.

Friction:

- No obvious matrix/matrix double contraction or rank-N pressure surfaced in this library.
- Complex trace/transform APIs are array-oriented; tensor changes should not be driven by this package.

### `Experiments/ContinuumComputabilityBoundary`

The continuum experiments are mostly records, enums, arrays, explicit local coupling surfaces, and reports about computability boundaries rather than native tensor kernels.

Friction and signals:

- Early continuum probes use explicit `Trace(strain)`, `SymGrad`, `Div`, and simple vector/matrix values. Existing vector/matrix tensor support is sufficient for the current small local mechanics algebra.
- Reports repeatedly frame richer constitutive anisotropy/stiffness tensors as future pressure, but the actual experiment corpus still avoids rank-3/rank-4 tensor storage and full solver frameworks.
- The strongest future rank-N example remains constitutive elasticity notation such as `C[i, j, k, l] * eps[k, l]`, but the experiments do not yet justify implementing storage/value semantics for rank-4 tensors.
- Matrix/matrix scalar double contractions would help invariant/energy-like local response examples without forcing rank-N design.
- Trace sugar is not necessary because explicit `Trace(...)` is already used and readable.

## 6. Candidate next milestones

### A. Matrix/matrix scalar double contractions

Examples:

```oct
A[i, j] * B[i, j]   // Frobenius inner product
A[i, j] * B[j, i]   // double contraction / trace-like contraction
```

Assessment:

- **Library/experiment demand:** Moderate and real. Mechanics has manually expanded invariant/stress formulas, RF matrix-channel power would naturally be a Frobenius reduction if represented as a native matrix, and Continuum experiments discuss tensor-like constitutive responses.
- **Mechanics/Continuum simplification:** Yes. It would simplify local invariant/energy-style expressions without requiring rank-N tensors.
- **Implementation fit:** Strong. The typechecker and interpreter already compute free labels, detect result rank 0, and explicitly reject matrix/matrix scalar double contractions. Compiled lowering has the same rank/free-label machinery and explicit defer point. Existing generated helper style can be extended with a scalar-returning `EinDoubleMM`/similar helper while preserving deterministic shape checks.
- **Diagnostics needed:** Keep current clarity: reject labels appearing more than twice; reject inconsistent extents; require both operands rank-2 matrices; reject unsupported result ranks; clarify scalar double contraction when labels produce zero free indices. Rectangular matrices need careful diagnostics because `A[i,j] * B[j,i]` can require transposed shape compatibility, while `A[i,j] * B[i,j]` requires same shape.
- **Should it come before rank-N tensors?** Yes. It closes an existing rank-2 gap, exercises scalar result lowering, and does not require a new storage type.

Verdict: **Recommended next milestone.**

### B. Trace-style sugar

Example:

```oct
A[i, i]
```

Assessment:

- **Worth adding now?** No. `Trace(A)` is already explicit, compiled-supported, and used in Mechanics/Continuum.
- **Ambiguity risk:** High enough to keep deferred. `A[i, i]` can be read as trace by Einstein convention, but users may also expect diagonal extraction in array/matrix languages. Oct currently avoids that ambiguity.
- **Footgun risk:** Allowing repeated labels inside a single indexed operand would weaken a clean validation rule and could complicate future diagonal-view design.

Verdict: Keep rejected; do not choose for M40.

### C. Rank-N tensor design

Example:

```oct
C[i, j, k, l] * eps[k, l]
```

Assessment:

- **Immediate library pressure:** Real but not yet strong enough. Continuum mechanics will eventually want rank-4 constitutive tensors, but current libraries and experiments are still productive with `Matrix<T>`, `Vector<T>`, explicit records, and helper functions.
- **Storage/value type gap:** Large. Oct has `Vector<T>` and `Matrix<T>`, but no canonical rank-3/rank-4 value type, literal syntax, constructors, shape API, or compiled representation.
- **Sequencing:** A design pass is required before implementation. This should likely happen after M40 and after package-manager federation decisions clarify how heavier numeric/tensor packages should be distributed.
- **Prometheus/reactor dependency:** Performance-sensitive rank-N syntax should not be introduced before there is a story for lowering hot tensor contractions outside naive interpreter/compiler helpers.

Verdict: Do not implement next; consider a later design audit.

### D. Documentation/example milestone

Examples:

- tensor examples in Mechanics/Continuum docs;
- “NumPy einsum vs Oct tensor notation” examples;
- small tutorial tests;
- examples combining enums/match with tensor notation.

Assessment:

- **Value:** Moderate. Docs have minor milestone-staleness and examples could help users.
- **Compared with implementation gap:** Lower. The current implementation has a narrow, known, tested rejection for matrix/matrix scalar double contractions that appears in natural mechanics/RF/continuum math. Closing that gap first would make examples more complete.
- **Best timing:** After M40, when examples can include matrix inner products without teaching a workaround.

Verdict: Useful, but not next.

### E. Package federation / registry design

Assessment:

- **Has tensor work reached a good stopping point?** Almost. M38 made the tensor core coherent enough that package-manager federation is strategically credible soon.
- **Should package manager design happen before rank-N tensors?** Yes. Rank-N tensors are a broad design/implementation/performance surface and should not block package federation.
- **Should package manager design happen before scalar double contractions?** Not quite. Matrix/matrix scalar double contractions are a small bounded parity closure within existing vector/matrix infrastructure; package federation is a broader architecture lane. One more tensor milestone is justified, but only one.

Verdict: Schedule after M40 unless new package-manager urgency appears.

## 7. Recommendation

Recommend exactly one next implementation milestone:

## **M40 — matrix/matrix scalar double contractions**

### Goal

Add interpreted and compiled support for rank-2 matrix/matrix indexed multiplication expressions that produce scalar results through double contraction:

```oct
A[i, j] * B[i, j]
A[i, j] * B[j, i]
```

### Scope

- Typecheck matrix/matrix indexed `*` with zero free indices when every label appears exactly twice.
- Interpret scalar matrix/matrix double contractions with deterministic extent checks.
- Compile scalar matrix/matrix double contractions through generated helpers consistent with existing `EinMulMM`, `EinAddMM`, `EinSubMM`, vector dot, and `@` helpers.
- Add `Language/Expressions/TensorEinsteinM7` or equivalent semantic contracts for interpreted and compiled valid/invalid behavior.
- Cover square and rectangular cases:
  - `A[i,j] * B[i,j]` requires matching shapes.
  - `A[i,j] * B[j,i]` requires transposed-compatible shapes.
- Preserve current diagnostics for arrays, trace sugar, rank-N outputs, labels appearing more than twice, and malformed one-sided indexed expressions.

### Non-goals

- No trace-style `A[i, i]`.
- No rank-N tensor value/storage/literal syntax.
- No broadcasting.
- No arrays as tensors.
- No covariant/contravariant variance.
- No raising/lowering indices.
- No Prometheus/reactor/GPU kernels.
- No `@` behavior changes.
- No package-manager federation implementation.

### Why this should be next

- It is the smallest remaining mathematical hole in the rank-1/rank-2 tensor contract.
- It matches natural mechanics/RF/continuum expressions better than trace sugar or rank-N tensors.
- Current implementation already has explicit defer points and most selection machinery needed to recognize the case.
- It gives scalar result coverage for matrix/matrix Einstein multiplication before any broader rank-N design.
- It avoids delaying package-manager work with an unbounded tensor expansion: after M40, package federation can become the next strategic lane with a cleaner tensor baseline.

### What remains deferred

Rank-N tensors, trace sugar, broadcasting, variance, raising/lowering, array tensor indexing, performance-specialized kernels, and package federation implementation remain deferred.

## 8. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Over-expanding the tensor system too quickly | Limit M40 to rank-2 matrix/matrix scalar double contractions only; reject rank-N, trace sugar, arrays, and broadcasting exactly as today. |
| Turning Oct into a NumPy broadcasting swamp | Preserve exact shape/extent requirements and keep arrays separate from tensors. Do not add implicit expansion. |
| Making trace sugar ambiguous | Keep `A[i, i]` rejected and keep `Trace(A)` explicit. Revisit only with a deliberate diagonal/trace design. |
| Delaying package-manager architecture too long | Treat M40 as the final immediate tensor implementation closure; then move to package federation unless strong new tensor evidence appears. |
| Docs lagging behind semantics | After M40, do a small docs cleanup to remove stale milestone wording and add examples that include double contractions. |
| Compiled/interpreted drift | Require interpreted and compiled semantic contracts for every M40 valid/invalid case; use the same extent/label rules in interpreter and generated helpers. |
| Introducing performance-sensitive tensor syntax before Prometheus/reactor lowering exists | Keep M40 scalar reductions simple and bounded. Do not market it as a high-performance tensor kernel; defer optimized lowering to Prometheus/reactor work. |

## 9. Test commands

M39 is audit-only. The required verification commands are:

```sh
go test ./internal/typecheck ./internal/interpret ./internal/build
go test ./cmd/oct -run 'Tensor|Einstein|Vector|Matrix|Mechanics'
go test ./internal/... ./cmd/oct

go run ./cmd/oct test Language/Expressions/TensorEinsteinM0/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/TensorEinsteinM1/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/TensorEinsteinM3/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/TensorEinsteinM4/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/TensorEinsteinM5/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/TensorEinsteinM6/valid --execution interpreted

go run ./cmd/oct test Language/Expressions/TensorEinsteinM0/valid --execution compiled
go run ./cmd/oct test Language/Expressions/TensorEinsteinM1/valid --execution compiled
go run ./cmd/oct test Language/Expressions/TensorEinsteinM3/valid --execution compiled
go run ./cmd/oct test Language/Expressions/TensorEinsteinM4/valid --execution compiled
go run ./cmd/oct test Language/Expressions/TensorEinsteinM5/valid --execution compiled
go run ./cmd/oct test Language/Expressions/TensorEinsteinM6/valid --execution compiled
```
