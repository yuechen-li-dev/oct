# M36a — Collection Iteration Re-Evaluation (Post-M35)

Scope: `testdata/m36a/CollectionIterationPostAppend`.
Baseline: `Append(xs, x)` is available and used for variable-length array construction.

## A1. Sum(xs: Int[]) -> Int

Classification: CLEAN_NOW

Observation:
- No meaningful change from M34a. This was already direct.

Why:
- Counted loop + accumulation is explicit and short.

Rejected Workarounds:
- Did not introduce pseudo-iterator helpers.

---

## A2. Max(xs: Float[]) -> Float

Classification: MINOR_VALUE_ITERATION_IMPROVEMENT

Observation:
- Append does not affect this workload; the loop still reads `xs[i]` repeatedly.

Why:
- Value iteration would reduce minor indexing noise, but current code is still clear and correct.

Rejected Workarounds:
- Did not add helper APIs that hide traversal.

---

## A3. CountPositive(xs: Int[]) -> Int

Classification: MINOR_VALUE_ITERATION_IMPROVEMENT

Observation:
- Append does not materially change this value-only scan.

Why:
- Repeated `xs[i]` is mildly noisy, not structurally problematic.

Rejected Workarounds:
- Did not simulate higher-order predicates.

---

## A4. AllPositive(xs: Int[]) -> Bool

Classification: MINOR_VALUE_ITERATION_IMPROVEMENT

Observation:
- Early return stays clear, but value checks remain index-addressed.

Why:
- Value iteration would be a readability polish only.

Rejected Workarounds:
- Did not build ad hoc helper functions for boolean scans.

---

## B1. AddIndex(xs: Int[]) -> Int[]

Classification: SHOULD_REMAIN_COUNTED_LOOP

Observation:
- Append can build output ergonomically, but index arithmetic is still the semantic center.

Why:
- This task is position-defined (`x + i`), so counted loops remain the clearest model.

Rejected Workarounds:
- Did not force value-oriented traversal for index-defined logic.

---

## B2. DiffAdjacent(xs: Float[]) -> Float[]

Classification: SHOULD_REMAIN_COUNTED_LOOP

Observation:
- Append reduces fixed-size preallocation pressure, but adjacent differencing still depends on explicit offsets.

Why:
- Correctness depends on index relationships (`i`, `i-1`) and boundary control.

Rejected Workarounds:
- Did not emulate adjacency with extra rolling-state machinery.

---

## B3. FindFirstGreaterThan(xs: Int[], threshold: Int) -> Int

Classification: MINOR_INDEX_VALUE_ITERATION_IMPROVEMENT

Observation:
- Append is irrelevant here; pattern remains index+value probing.

Why:
- Paired `(i, x)` iteration would remove minor boilerplate, but counted form is still straightforward.

Rejected Workarounds:
- Did not split into multiple passes.

---

## C1. KeepPositive(xs: Int[]) -> Int[]

Classification: CLEAN_NOW

Observation:
- Major change from M34a: Append removes manual write-index tracking (`kept`).

Why:
- Variable-length build is now expressible directly with append-driven construction.

Rejected Workarounds:
- Did not keep legacy `Values + Kept` record shape once Append made it unnecessary.

---

## C2. SquareAll(xs: Float[]) -> Float[]

Classification: CLEAN_NOW

Observation:
- Already acceptable before; still clean. Append also supports concise grow-build style.

Why:
- One-to-one transform has no structural friction either way.

Rejected Workarounds:
- Did not introduce any abstraction beyond direct loop + assignment/append.

---

## C3. KeepAbove(xs: Float[], t: Float) -> Float[]

Classification: CLEAN_NOW

Observation:
- Major change from M34a: Append removes compaction bookkeeping pressure.

Why:
- Filtering into variable-length output is now direct and readable.

Rejected Workarounds:
- Did not retain preallocated buffers + manual kept-index management.

---

## D1. SumAll(matrix: Float[][]) -> Float

Classification: REQUIRES_FUTURE_MATRIX_VECTOR_MODEL

Observation:
- Re-attempting this task in current Oct fails at parsing because nested array types (`Float[][]`) are not yet supported.

Why:
- This is blocked by type/model surface, not by value-iteration syntax.

Rejected Workarounds:
- Refused to fake matrices as ad hoc flattened arrays with shape side channels for this language-pressure task.

---

## D2. CountAbove(matrix: Float[][], t: Float) -> Int

Classification: REQUIRES_FUTURE_MATRIX_VECTOR_MODEL

Observation:
- Same blocker as D1: `Float[][]` is currently unavailable in parser/type surface.

Why:
- Cannot evaluate nested traversal ergonomics until matrix-like collection modeling exists.

Rejected Workarounds:
- Refused to reframe this as one-dimensional synthetic data because that would dodge the actual task.

---

## E1. Case where counted loop is clearest: AddIndex

Classification: SHOULD_REMAIN_COUNTED_LOOP

Observation:
- Even after Append, intent is fundamentally index arithmetic.

Why:
- Counted form keeps index semantics explicit and auditable.

Rejected Workarounds:
- Did not rewrite into value-only traversal with hidden counter state.

---

## E2. Case where counted loop is clearest: DiffAdjacent

Classification: SHOULD_REMAIN_COUNTED_LOOP

Observation:
- Boundary start and neighbor access are naturally expressed with explicit indices.

Why:
- Positional differences are clearer in counted loops than in sugar forms.

Rejected Workarounds:
- Did not force synthetic tuple/state patterns.

---

## Global Summary

1. After Append, is value iteration (H1):
- **nice-to-have**

2. After Append, is index+value iteration (H2):
- **situational**

3. Did Append eliminate most previously observed friction?
- **Yes.** The highest-pressure filter/build friction is largely resolved; remaining pressure is smaller and a separate matrix modeling blocker exists.

4. Are remaining iteration complaints structural or minor syntactic noise?
- **Mostly minor syntactic noise**, plus one **structural matrix-model gap** unrelated to iteration syntax.

5. Are counted loops still the dominant correct form?
- **Yes.**

6. Recommend EXACTLY ONE next step:
- **do nothing (iteration not needed)**
