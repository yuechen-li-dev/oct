# M31b — Meta Friction Test Run Report

Scope: `testdata/m31b/FunctionValuePressure`.

## A. Callback-Like Array Processing Revisited

### Task
Apply selectable transforms and keep/discard rules across a fixed array (`MapByMode`, `FilterByRule`).

### Outcome
Improved By Non-Capturing Function Values

### Current Oct Expression
Used enum-selected `switch` behavior in each helper plus explicit loop bodies and explicit output buffer management for filter.

### Friction
The transform/filter logic is straightforward, but reusable "apply this named operation" composition gets repetitive. The pressure is mostly at call-site composition boundaries, not inside the scalar math itself.

### Rejected Workaround
Rejected building a fake callback registry (records + dispatcher switch tower). That would reintroduce dynamic indirection manually and hide behavior ownership.

### Recommendation
Non-capturing function values may be justified

### Minimal Needed Surface (only if justified)
Allow passing named package functions as values with explicit function-type parameters and returns, no literals/lambdas, and no capture.

---

## B. Parameterized Behavior Without Capture

### Task
Re-test threshold/gain and affine rules where callers often want partially applied helpers (`ScoreWithThreshold`, `ApplyAffine`).

### Outcome
Still Wants Capture

### Current Oct Expression
Threaded parameter records (`ThresholdRule`, `AffineRule`) explicitly through each call.

### Friction
The friction appears when callers want to "freeze" parameters and reuse a unary function shape repeatedly. Non-capturing function values alone do not bind `rule`; callers would still pass rule every time.

### Rejected Workaround
Rejected global mutable rule state and rejected hand-written pseudo-closures (record + apply dispatcher). Both approaches obscure data flow and are error-prone.

### Recommendation
Full closures still not justified

---

## C. Small Scientific Hook Patterns

### Task
Test residual-model choice, normalization rule choice, and one-step updates (`OneStep`, `Normalize`).

### Outcome
Clean Without New Feature

### Current Oct Expression
Used package-owned enums with explicit `switch`es for closed scientific model families.

### Friction
No meaningful friction. The explicit model set is a strength because it is auditable and domain-owned.

### Rejected Workaround
Rejected introducing open callback hooks where model sets are intentionally closed for clarity and reproducibility.

### Recommendation
No language feature needed

---

## D. Bad Pattern Risk Check

### Task
Probe pipeline-like control flow that starts to resemble callback chains (`ApplyPipelineAttempt`).

### Outcome
Should Not Be Encouraged

### Current Oct Expression
Used explicit two-stage orchestration via enums and helper calls.

### Friction
As soon as control flow trends toward open-ended operation chaining, abstraction pressure rises quickly and readability drops.

### Rejected Workaround
Rejected constructing pseudo-pipeline engines (operation lists + dynamic dispatch). That pattern would import indirection-heavy style Oct should resist.

### Recommendation
This pattern should not be encouraged in Oct

---

## E. Distinguish Feature Pressure From Library Pressure

### Task
Separate language pressure from helper/ergonomic pressure by testing simple reusable mapping cases (`MapByMode`).

### Outcome
Clean Without New Feature

### Current Oct Expression
A small helper plus enum mode already keeps call-sites readable.

### Friction
Most remaining pain in these simple cases is array ergonomics (fixed buffers/manual sizes in other tasks), not inability to pass function values.

### Rejected Workaround
Rejected adding multiple one-off helper variants to mimic generic callback APIs; that would add boilerplate without improving semantics.

### Recommendation
Better package/library pattern is enough

---

## Global Summary

### 1. Overall Assessment
Oct does **not** appear to need broad function values. There is a narrow callback-composition pressure where a tiny non-capturing function-value surface could help, but much of the day-to-day expression remains clean with enums + switch + small package helpers.

### 2. Strongest Real Pressure
Composing reusable transform/filter helpers across call sites without repeating enum wiring is the strongest case for a minimal non-capturing function-value capability.

### 3. Strongest False Pressure
Pipeline/callback-chain architecture appears to want function values, but this is mostly pressure toward abstraction styles that reduce explicitness and should stay discouraged.

### 4. Interaction With M31a
M31a already showed: closed strategy selection and solver hooks are clean; full closures are not justified; callback-like plumbing is the main awkward zone. M31b clarifies that this remaining pressure splits in two:
- a narrow slice may improve with **non-capturing named function values**
- capture-heavy parameter freezing still wants closures/partial application, which remains unjustified for v1

This slightly strengthens the case for a tiny middle-ground experiment, while still rejecting full closure scope.

### 5. Recommended Next Step
Consider a very small non-capturing function-value feature
