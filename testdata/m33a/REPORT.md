# M33a — Meta Friction Test Run Report: Generics Pressure

Scope: `testdata/m33a/GenericsPressure`.

## A. Reusable Pair / Tuple-Like Structures

### Task
Define reusable pair-like records that differ only in field types (`Int/Float`, `Float/Float`, `Int/Int`).

### Outcome
Awkward

### Current Oct Expression
Used three explicit records: `PairIntFloat`, `PairFloatFloat`, and `PairIntInt`.

### Friction
The code is simple and clear, but mostly mechanical duplication. Renaming/changing shared structure means touching multiple declarations.

### Rejected Workaround
Rejected a pseudo-generic “pair framework” using tag fields plus manual access dispatch; it adds ceremony and hides concrete types.

### Recommendation
Better library/package pattern is enough

---

## B. Same Algorithm Across Multiple Types

### Task
Implement `min`, `max`, and `clamp` for `Int` and `Float`.

### Outcome
Awkward

### Current Oct Expression
Duplicated functions as `MinInt/MaxInt/ClampInt` and `MinFloat/MaxFloat/ClampFloat`.

### Friction
Algorithm logic is identical, and drift risk appears when one copy is updated while another is not.

### Rejected Workaround
Rejected collapsing numeric logic through string/tag dispatch; this would build an ad-hoc runtime type layer and weaken static clarity.

### Recommendation
A small language feature may be justified

### Minimal Needed Surface (only if justified)
Allow exactly one type parameter on pure functions for same-type numeric operations, e.g. `fn Min[T](a: T, b: T) -> T`, restricted to built-in numeric scalar types only (`Int`, `Float`) and no trait/typeclass surface.

---

## C. Array / Collection Helper Pressure

### Task
Write transform/filter/sum helpers for arrays across `Int[]` and `Float[]`.

### Outcome
Awkward

### Current Oct Expression
Implemented separate helpers: `DoubleInts`, `ScaleFloats`, `KeepPositiveInts`, `KeepPositiveFloats`, `SumInts`, and `SumFloats`.

### Friction
This produced the strongest concrete duplication signal: helper families multiply quickly by element type, operation, and return-shape records.

### Rejected Workaround
Rejected callback-heavy pseudo-generic utilities and code-generation-like scaffolding; those obscure intent and violate the “pressure probe, not abstraction tower” goal.

### Recommendation
A small language feature may be justified

### Minimal Needed Surface (only if justified)
If anything is added, keep it narrow: permit single-parameter type-generic functions where every occurrence uses the same `T` (for example `fn Sum[T](xs: T[]) -> T`), initially limited to `Int`/`Float` instantiation only.

---

## D. Record Family Duplication

### Task
Create record families with shared shape but different numeric field types (`IntRange` and `FloatRange`).

### Outcome
Clean

### Current Oct Expression
Defined explicit record pairs and corresponding helper functions (`CenterInt`, `CenterFloat`).

### Friction
Low friction at this scale. The duplication is obvious and readable.

### Rejected Workaround
Rejected introducing meta-record encoding with tagged unions to avoid two small declarations; complexity exceeds benefit.

### Recommendation
No language feature needed

---

## E. API Surface Duplication

### Task
Provide small APIs with equivalent behavior for `Int` and `Float` (`ShiftIntByDelta`, `ShiftFloatByDelta`).

### Outcome
Clean

### Current Oct Expression
Kept explicit type-specific function names.

### Friction
Minimal for small, user-facing APIs; explicit names improve readability at call sites.

### Rejected Workaround
Rejected unifying via generalized “number API” wrappers; wrappers add indirection without reducing meaningful complexity.

### Recommendation
No language feature needed

---

## F. “Looks Like It Wants Generics But Maybe Shouldn’t”

### Task
Probe open-ended generic-container pressure (type-erased/`Any`-style utilities).

### Outcome
Blocked

### Current Oct Expression
Stopped at explicit code and intentionally did not implement a faux type-erased container.

### Friction
Open-ended heterogeneous container patterns push directly toward tag-dispatch frameworks and manual runtime typing.

### Rejected Workaround
Rejected implementing a record + tag + giant dispatch matrix “generic box”; it is effectively a mini runtime and a poor fit for Oct’s explicit model.

### Recommendation
This pattern should not be encouraged in Oct

---

## Global Summary

### 1. Overall Assessment
Oct does not currently justify full generics. The real signal is a narrow slice: duplicated numeric/array helper algorithms.

### 2. Strongest Real Pressure
Collection helper families (`map/filter/reduce`-like logic over `Int[]` and `Float[]`) are the strongest pressure point.

### 3. Strongest False Pressure
Open-ended reusable container/type-erasure designs look like generics pressure, but they mainly produce abstraction noise and hidden dispatch.

### 4. Nature of Pressure
Primarily algorithm reuse + API duplication in numeric helper families, not broad modeling capability gaps.

### 5. Recommended Next Step
Consider a very small feature
