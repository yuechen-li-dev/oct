# M33b — Meta Friction Test Run Report: Tiny Generic-Function Slice

Scope: `testdata/m33b/TinyGenericFunctionSlice`.

## A1. Min over `Int`/`Float`

### Task
Implement `MinInt` and `MinFloat` with identical control flow.

### Outcome
Improved By Tiny Generic-Function Slice

### Current Oct Expression
Two explicit functions (`MinInt`, `MinFloat`) with the same body and only scalar type changes.

### Friction
Mechanical duplication with drift risk when one copy is edited and the other is forgotten.

### Rejected Workaround
A runtime tag-dispatch helper was rejected because it hides concrete types and rebuilds ad-hoc polymorphism.

### Recommendation
Tiny generic-function slice may be justified

### Minimal Needed Surface (only if justified)
Allow one function type parameter used consistently in parameters/return (`fn Min[T](a: T, b: T) -> T`) with instantiation limited to plain `Int` and plain `Float`.

---

## A2. Clamp over `Int`/`Float`

### Task
Implement `ClampInt` and `ClampFloat`.

### Outcome
Improved By Tiny Generic-Function Slice

### Current Oct Expression
Two explicit clamps with duplicated branch structure.

### Friction
Same algorithm, same parameter shape, same return shape; duplication is pure ceremony.

### Rejected Workaround
Code-generation-style duplication was rejected for this run because it would mask true language pressure.

### Recommendation
Tiny generic-function slice may be justified

### Minimal Needed Surface (only if justified)
Same as A1: single-parameter same-type generic functions over plain `Int`/`Float` only.

---

## B1. `Sum(xs)` over `Int[]`/`Float[]`

### Task
Implement `SumInts` and `SumFloats`.

### Outcome
Improved By Tiny Generic-Function Slice

### Current Oct Expression
Two loops that differ only in element/accumulator type.

### Friction
This is the highest-volume duplication pattern and appears repeatedly in numeric helper families.

### Rejected Workaround
A function-value abstraction layer was rejected because it introduces extra call-shape complexity without removing type-family duplication.

### Recommendation
Tiny generic-function slice may be justified

### Minimal Needed Surface (only if justified)
Permit `fn Sum[T](xs: T[]) -> T` where `T` can only be `Int` or `Float`, and all uses are same-type.

---

## B2. `AllNonNegative(xs)` over `Int[]`/`Float[]`

### Task
Implement `AllNonNegativeInts` and `AllNonNegativeFloats`.

### Outcome
Improved By Tiny Generic-Function Slice

### Current Oct Expression
Two structurally identical loops and predicates with scalar literal changes (`0` vs `0.0`).

### Friction
Another repeated, boring duplication hotspot in numeric validation helpers.

### Rejected Workaround
Manual normalization to `Float[]` first was rejected because it changes API intent and introduces conversion noise.

### Recommendation
Tiny generic-function slice may be justified

### Minimal Needed Surface (only if justified)
Single generic parameter for same-type arrays and scalar comparisons over `Int`/`Float` only.

---

## C1. `AllTrue(xs: Bool[])`

### Task
Probe non-numeric array helper pressure with a boolean helper.

### Outcome
Clean Without Generics

### Current Oct Expression
A single explicit `Bool[] -> Bool` function is concise and readable.

### Friction
Low. No meaningful duplication pressure emerged from this case.

### Rejected Workaround
No workaround pursued; direct explicit code is already clear.

### Recommendation
No language feature needed

---

## C2. `IsEmptyString(s: String)`

### Task
Probe non-numeric pressure with a string helper.

### Outcome
Clean Without Generics

### Current Oct Expression
Direct `String -> Bool` helper remains compact and explicit.

### Friction
Low. This does not create repeated families that would justify type-parameterization.

### Rejected Workaround
A pseudo-generic “AnyMatches” helper was rejected as over-abstraction for trivial string checks.

### Recommendation
No language feature needed

---

## C3. Record midpoint helpers (`IntPoint`/`FloatPoint`)

### Task
Probe record duplication pressure with `MidpointInt` and `MidpointFloat`.

### Outcome
Should Not Be Encouraged

### Current Oct Expression
Two explicit record types and two explicit midpoint functions are straightforward.

### Friction
Duplication exists but this pattern quickly pressures generic records, which is outside the slice and outside current goals.

### Rejected Workaround
Generic-like record emulation via tagged records was rejected because it effectively builds a mini type-erasure framework.

### Recommendation
This pattern should not be encouraged in Oct

---

## D1. `Int[] -> Float[]` normalization

### Task
Probe mixed-type relationships with `NormalizePercentIntToFloat`.

### Outcome
Still Wants Broader Generics

### Current Oct Expression
An explicit dedicated conversion function is easy to read and currently the right shape.

### Friction
Any generic abstraction here would require mixed-type parameterization (input/output differ), exceeding same-type-only.

### Rejected Workaround
Rejected faking it with scalar wrappers plus duplicated conversions; it obscures intent and still does not generalize cleanly.

### Recommendation
The pressure already exceeds what should be added

---

## D2. `T -> Bool` predicates (`IsStrictlyPositive*`)

### Task
Probe single-input boolean predicate families.

### Outcome
Still Wants Broader Generics

### Current Oct Expression
`IsStrictlyPositiveInt` and `IsStrictlyPositiveFloat` are explicit and short.

### Friction
A generic `fn IsStrictlyPositive[T](value: T) -> Bool` requires cross-kind return behavior and quickly pressures constraints/traits.

### Rejected Workaround
Rejected centralized predicate registry patterns; they imitate typeclass dispatch and violate the tiny-slice boundary.

### Recommendation
The pressure already exceeds what should be added

---

## E1. Dimensioned clamp helpers (`Float<m>` and `Float<kg*m/s^2>`)

### Task
Probe dimensioned numeric duplication with `ClampLength` and `ClampForce`.

### Outcome
Still Wants Broader Generics

### Current Oct Expression
Two explicit helpers remain clear and type-safe.

### Friction
The tiny slice (`Int`/`Float` only) does not help; real pressure here is about dimension-parameterized numerics, not plain scalar numerics.

### Rejected Workaround
Rejecting dimension stripping (`/ 1m` and reapply units) because it weakens type guarantees and is error-prone.

### Recommendation
The pressure already exceeds what should be added

---

## E2. Dimension-awareness as a design boundary

### Task
Check whether the strongest numeric helper pattern naturally extends to units-aware numerics.

### Outcome
Still Wants Broader Generics

### Current Oct Expression
Explicit per-dimension helpers preserve intent and maintain strict dimension typing.

### Friction
If this use case is considered in-scope, tiny scalar-only generics immediately become incomplete.

### Rejected Workaround
Rejected introducing “dimensionless adapter” helper layers because they hide the real units contract.

### Recommendation
The pressure already exceeds what should be added

---

## Global Summary

### 1. Overall Assessment
Oct does not need broad generics. The tiny generic-function slice appears useful for plain `Int`/`Float` helper duplication, but it is not a stable endpoint if dimensioned numeric helpers are considered part of the same pressure family.

### 2. Strongest Real Pressure
`Min`/`Clamp` and `Sum`/`AllNonNegative` families over plain `Int`/`Float` show repeated, same-shape duplication where a single-parameter same-type function generic would give direct relief.

### 3. Strongest Leak Beyond The Slice
Dimensioned numerics are the strongest immediate leak: real scientific helpers often want reuse across `Float<...>` dimensions, which the tiny slice explicitly excludes.

### 4. Interaction With M33a
M33a already rejected broad generics and showed that most pressure is fake or tolerable. M33b confirms the narrow numeric helper hotspot is real, but clarifies that the hotspot splits into two layers: plain scalar duplication (slice helps) versus dimension-aware duplication (slice does not help). This makes the tiny slice conditionally useful but potentially unstable if interpreted as the start of a broader path.

### 5. Recommended Next Step
Run one more narrower pressure test
