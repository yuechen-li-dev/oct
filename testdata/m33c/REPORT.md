# M33c — Meta Friction Test Run Report: Dimension-Aware Numeric Reuse Pressure

Scope: `testdata/m33c/DimensionAwareNumericReusePressure`.

## A. Same-Shape Scalar Helpers Across Different Dimensions

### Task
Implemented same-shape scalar helpers across dimensions: `MinLength`, `MinForce`, `MaxTime`, `MaxDistance`, `ClampLength`, and `ClampForce`.

### Outcome
Awkward But Acceptable

### Current Oct Expression
Use explicit per-dimension helper functions where each function keeps one concrete dimension in parameter and return positions.

### Friction
The duplication is mechanical and easy to spot; branch structure and parameter shapes are nearly identical.

### Rejected Workaround
Considered a pseudo-"any dimension" helper surface. Rejected because it immediately asks for dimension-parameterized function abstraction, not just scalar `Int`/`Float` reuse.

### Recommendation
Keep explicit per-dimension code

### Minimal Needed Surface (only if justified)
Not justified for this task.

---

## B. Same-Shape Array Helpers Across Different Dimensions

### Task
Implemented dimensioned array helpers: `SumLengths`, `SumForces`, `AllNonNegativeLengths`, and `ClampArrayLengths`.

### Outcome
Real Pressure For Reuse

### Current Oct Expression
Write separate loops with dimension-specific literals and accumulator types.

### Friction
Array helpers amplify repetition quickly: each helper family duplicates loop bodies, accumulator initialization, and clamp/predicate logic per dimension.

### Rejected Workaround
Considered normalizing all values to plain `Float` and then reapplying units. Rejected because unit stripping undermines compile-time dimension safety and creates audit risk.

### Recommendation
This pressure shows the tiny slice is unstable

### Minimal Needed Surface (only if justified)
The smallest useful abstraction would need a dimension-parameterized numeric type variable, not merely `Int`/`Float` scalar reuse. That exceeds the tiny scalar-only slice.

---

## C. Boundary Probe: Different Dimensions Should Stay Distinct

### Task
Probed whether helpers should safely prevent cross-dimension misuse (e.g., treating length and force as interchangeable or trying one helper that accepts any dimension).

### Outcome
Should Stay Explicit For Safety/Clarity

### Current Oct Expression
Dimension-specific helpers naturally encode domain intent (`ClampLength` vs `ClampForce`) and keep misuse visually and type-level obvious.

### Friction
The pressure to unify these helpers is convenience-driven, but the domain boundary is meaningful and protects correctness.

### Rejected Workaround
Rejected dimension-erasing conversion patterns and generic-like wrappers that would allow accidental cross-dimension call sites.

### Recommendation
Keep explicit per-dimension code

### Minimal Needed Surface (only if justified)
Not justified for this task.

---

## D. Boundary Probe: Plain Numeric vs Dimensioned Numeric

### Task
Compared plain scalar duplication (`ClampInt`, `ClampFloat`) with dimensioned duplication (`ClampLength`, `ClampForce`).

### Outcome
Would Require Too-Broad A Feature

### Current Oct Expression
Plain scalar helpers duplicate over value kind (`Int` vs `Float`), while dimensioned helpers duplicate over dimension algebra (`m`, `s`, `kg*m/s^2`, `m/s`, ...).

### Friction
These are related but not equivalent pressure families. Scalar-only reuse can stay tiny; dimension-aware reuse naturally expands toward richer typing machinery.

### Rejected Workaround
Rejected claiming they are the same problem and solving both with one tiny feature; this would hide the fact that dimension polymorphism is a broader system question.

### Recommendation
This pressure shows the tiny slice is unstable

### Minimal Needed Surface (only if justified)
Any meaningful abstraction here needs dimension-aware type parameterization and safe operator constraints, which is outside the tiny scalar-only design.

---

## E. Scientific Readability / Auditability Probe

### Task
Built tiny scientific flows (`SafeSupportForce`, `SafeSpan`) reusing explicit helpers in mechanics-style scenarios.

### Outcome
Should Stay Explicit For Safety/Clarity

### Current Oct Expression
Function names include domain meaning and dimensions at the boundary; call sites remain easy to audit (`kg*m/s^2`, `m/s`, `m`).

### Friction
There is some repetition, but explicit names and dimensions improve reviewability for safety-critical scientific calculations.

### Rejected Workaround
Rejected consolidation into abstract numeric helpers that hide unit intent at call sites.

### Recommendation
Keep explicit per-dimension code

### Minimal Needed Surface (only if justified)
Not justified for this task.

---

## Global Summary

### 1. Overall Assessment
Dimension-aware pressure does **not** support broad generics, but it does show that the tiny scalar-only generic-function slice does not naturally scale into unit-aware scientific helpers. Explicit per-dimension code remains a strong default style.

### 2. Strongest Real Pressure
Array helper families over dimensioned values (`Sum*`, `ClampArray*`, predicate loops) are the strongest real duplication pressure.

### 3. Strongest Explicitness Win
Safety and auditability are strongest when dimensions remain explicit in helper naming and signatures, especially for mechanics and measurement workflows.

### 4. Interaction With M33a/M33b
- M33a settled that broad generics are not justified and open-ended abstractions are a poor fit.
- M33b narrowed the viable space to a tiny same-type scalar helper slice (`Int`/`Float`).
- M33c resolves that dimension-aware reuse is a distinct pressure seam: real in spots, but tightly coupled to safety semantics and likely requiring broader machinery.
- Result: the tiny generic-function slice can still be coherent **for plain scalar numerics only**, but it is unstable as a "general numeric" story once dimensions are included.

### 5. Recommended Next Step
Reconsider the problem through a different mechanism, not generics
