# M34a — Collection Iteration Pressure Report

Scope: `testdata/m34a/CollectionIteration`.

## A1. Sum(xs: Int[]) -> Int

Classification: CLEAN_NOW

Observation:
- Straight counted traversal is short and direct.

Why:
- The logic is accumulation-only and uses `i` only for indexing.

Rejected Workarounds:
- None.

---

## A2. Max(xs: Float[]) -> Float

Classification: FRICTION_VALUE_ITERATION

Observation:
- The body repeatedly reads `xs[i]` and carries `best`.

Why:
- This is value-oriented logic; index machinery is incidental.
- The current loop is still possible, but noisier than the conceptual task.

Rejected Workarounds:
- Did not introduce helper callbacks or pseudo-iterators.

---

## A3. CountPositive(xs: Int[]) -> Int

Classification: FRICTION_VALUE_ITERATION

Observation:
- Count logic is simple, but every check is `xs[i] > 0`.

Why:
- Value-only conditionals suffer repeated indexing boilerplate.

Rejected Workarounds:
- Did not build custom traversal helper APIs.

---

## A4. AllPositive(xs: Int[]) -> Bool

Classification: FRICTION_VALUE_ITERATION

Observation:
- Early exit is clear, but value checks are index-addressed.

Why:
- The task cares about element values, not positions.

Rejected Workarounds:
- Did not emulate predicates via enums/switch dispatch.

---

## B1. AddIndex(xs: Int[]) -> Int[]

Classification: SHOULD_REMAIN_COUNTED_LOOP

Observation:
- Current counted loop is explicit and naturally expresses `x + i`.

Why:
- Both index and value are semantically central.

Rejected Workarounds:
- None.

---

## B2. DiffAdjacent(xs: Float[]) -> Float[]

Classification: SHOULD_REMAIN_COUNTED_LOOP

Observation:
- Adjacent differencing is index math by nature (`i` and `i-1`).

Why:
- Relative-position arithmetic is clearer with direct counted indexing.

Rejected Workarounds:
- Did not force value-only traversal with external state variables.

---

## B3. FindFirstGreaterThan(xs: Int[], threshold: Int) -> Int

Classification: FRICTION_INDEX_VALUE_ITERATION

Observation:
- The implementation is simple, but repeatedly does `xs[i]` only to return `i`.

Why:
- This pattern simultaneously consumes value and index; paired iteration would reduce repetition.

Rejected Workarounds:
- Did not create an artificial two-pass approach (find value then scan index).

---

## C1. KeepPositive(xs: Int[]) -> Int[]

Classification: FRICTION_BUILD_APPEND

Observation:
- Main burden is output buffer management (`out` + `kept`), not source traversal.

Why:
- Without append/growth ergonomics, filtering must manually track write position.

Rejected Workarounds:
- Rejected over-allocating elaborate sentinel schemes beyond a simple `Values + Kept` result.

---

## C2. SquareAll(xs: Float[]) -> Float[]

Classification: CLEAN_NOW

Observation:
- One-to-one transform with fixed-size preallocation is straightforward.

Why:
- No element dropping; output index mirrors input index.

Rejected Workarounds:
- None.

---

## C3. KeepAbove(xs: Float[], t: Float) -> Float[]

Classification: FRICTION_BUILD_APPEND

Observation:
- Same pressure as integer filter: compaction requires manual `kept` index.

Why:
- The critical pain is building variable-length results, not loop traversal.

Rejected Workarounds:
- Did not invent hypothetical append syntax.

---

## D1. SumAll(matrix: Float[][]) -> Float

Classification: CLEAN_NOW

Observation:
- Nested counted loops are explicit and understandable for two-level traversal.

Why:
- Pure accumulation across rows/columns does not require extra abstraction.

Rejected Workarounds:
- Did not substitute matrix-specific abstractions not already present.

---

## D2. CountAbove(matrix: Float[][], t: Float) -> Int

Classification: FRICTION_VALUE_ITERATION

Observation:
- Double indexing (`matrix[r][c]`) makes value checks verbose.

Why:
- The condition is value-driven at both loop levels; index mechanics are mostly incidental.

Rejected Workarounds:
- Did not flatten matrices into custom record wrappers just to avoid nested indexing.

---

## E1. Case where counted loop is clearer: AddIndex

Classification: SHOULD_REMAIN_COUNTED_LOOP

Observation:
- The algorithm is literally defined in terms of index arithmetic.

Why:
- A counted loop states intent directly and avoids hiding index semantics.

Rejected Workarounds:
- Did not force a value-oriented expression for an index-first operation.

---

## E2. Case where counted loop is clearer: DiffAdjacent

Classification: SHOULD_REMAIN_COUNTED_LOOP

Observation:
- Boundary control (start at second item) and prior-element access are explicit now.

Why:
- This is positional math; counted loops preserve correctness visibility.

Rejected Workarounds:
- Did not emulate rolling state with extra mutable temporaries to avoid index arithmetic.

---

## Global Summary

1. Does value iteration (H1) appear justified?
- **Yes, narrowly.** It would reduce repeated indexing in value-only scans (`Max`, `CountPositive`, `AllPositive`, nested value predicates).

2. Does index+value iteration (H2) appear justified?
- **Partially.** Clear benefit appears in mixed-use patterns like `FindFirstGreaterThan`, where both index and value are immediately needed.

3. How many tasks were actually blocked by build/append instead of iteration?
- **2 tasks** (`KeepPositive`, `KeepAbove`).

4. Did nested traversal reveal a separate matrix/vector problem?
- **No strong separate blocker in this run.** Nested arrays were traversable with current counted loops.

5. Identify at least one case where current loops are already optimal.
- `AddIndex` and `DiffAdjacent` are already optimal as counted loops.

6. Recommend ONE next step.
- **Implement append/build ergonomics instead.**
