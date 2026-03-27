# M31a — Meta Friction Test Run Report

Scope: `testdata/m31a/ClosurePressure`.

## A. Strategy Selection Without Closures

### Task
Select one of several scalar transform strategies at runtime.

### Outcome
Clean

### Current Oct Expression
Used `enum Strategy` + `switch` in `ApplyStrategy`, with runtime choice via ordinary control flow.

### Friction
No meaningful friction for a closed strategy set.

### Rejected Workaround
Function values were considered but rejected as unnecessary for a finite, package-owned strategy set.

### Recommendation
No language feature needed

---

## B. Small Callback-Like Processing

### Task
Model callback-like map/filter behavior over a small array.

### Outcome
Awkward

### Current Oct Expression
Used explicit loops plus `switch`-selected behavior (`TransformArray`, `FilterArray`).

### Friction
Map-like behavior is still readable, but filter-like behavior requires manual buffer management (`Values` + explicit `Kept` count) because there is no direct callback-style abstraction and no convenient growth primitive.

### Rejected Workaround
Rejected building a pseudo-generic callback framework with copied per-type/per-shape helpers; it would normalize boilerplate instead of solving pressure.

### Recommendation
Better library/package pattern is enough

---

## C. Parameterized Behavior With Captured Context

### Task
Express threshold/gain behavior that would often be a closure capture.

### Outcome
Awkward

### Current Oct Expression
Used explicit `ThresholdRule` record and threaded it into `ScoreWithRule`.

### Friction
Single call sites are fine; repeated partial-application style usage gets noisy because caller context must be carried manually at every use-site.

### Rejected Workaround
Rejected ad-hoc global state for thresholds/gains; hidden mutable state would be worse than explicit arguments.

### Recommendation
A small language feature may be justified

### Minimal Needed Surface (only if justified)
A minimal function-value surface without capture: allow passing named package functions as values where signatures are explicit. This relieves callback pressure without introducing closure state.

---

## D. Solver / Workflow Hook Pressure

### Task
Choose one of several residual/update models in a tiny iterative step.

### Outcome
Clean

### Current Oct Expression
Used `enum ResidualModel` + `switch` in `IterateOnce`.

### Friction
No real friction when the model set is closed and package-owned.

### Rejected Workaround
Rejected callback/plugin indirection because it weakens explicitness and offers little gain for fixed solver families.

### Recommendation
No language feature needed

---

## E. “Looks Like It Wants Closures But Maybe Shouldn’t”

### Task
Test mini transform pipeline style pressure with preset behaviors.

### Outcome
Blocked

### Current Oct Expression
Implemented only fixed presets (`PipelinePreset`) via explicit `switch` in `ApplyPreset`.

### Friction
Open-ended chaining/stateful function values cannot be represented directly. Scaling this pattern requires combinatorial presets or pseudo-abstractions, both of which are pretzel-shaped for Oct v0.

### Rejected Workaround
Rejected “stateful function object” simulation via records plus large dispatch trees; this is effectively a manual VM layer and violates the spirit of explicit Oct code.

### Recommendation
This pattern should not be encouraged in Oct

---

## Global Summary

### 1. Overall Assessment
Current pressure does **not** justify full closures with capture. Most real tasks are handled well with enums, switches, and named functions. There is some ergonomic pressure around callback-like plumbing.

### 2. Strongest Real Pressure
The strongest signal is small callback-like data processing (especially filter-style workflows) where boilerplate accumulates.

### 3. Strongest False Pressure
Dynamic callback-heavy chains/stateful function values look attractive but quickly produce hidden-state or combinatorial designs that are poor fits for Oct’s explicit model.

### 4. Recommended Next Step
Consider a very small language feature

Concretely: evaluate a narrowly-scoped function-value experiment (named function references only, no capture) in a follow-up pressure run before any closure design.
