# M46 — Concurrency Pressure Test: Structured CPU Parallelism

This pressure run uses current Oct only, with no simulated task runtime, no channels, and no async scaffolding. Probes live in `testdata/m46/concurrency_pressure.octest` and were executed sequentially through `oct test`.

## A. Parameter Sweep / Batch Evaluation

### Task
Evaluate a compact engineering-style scoring model over six independent parameter values and preserve per-parameter result records.

### Outcome
**Real Pressure For Structured Parallelism**

### Current Oct Expression
A sequential counted loop builds a `SweepBatch` score vector, evaluating one independent point at a time with no cross-item dependency.

### Friction
The logic is straightforward, but this is classic embarrassingly parallel work. Expression is clear; wall-clock scaling pressure appears as sweep size grows.

### Rejected Workaround
Manual “worker-like” splitting into multiple hand-authored functions (for first half/second half/etc.) was rejected because it duplicates logic, harms readability, and imitates concurrency plumbing in user code.

### Recommendation
**Structured batch/map-style execution seems justified**

### Minimal Needed Surface (only if justified)
A minimal explicit form that maps one pure/independent function over an input collection and returns ordered outputs, e.g. “parallel map over range/array with deterministic result order and implicit join at block end.” No user-visible tasks/channels.

---

## B. Independent Signal / Sample Processing

### Task
Analyze a batch of independent small signals, producing per-signal RMS and peak metrics.

### Outcome
**Real Pressure For Structured Parallelism**

### Current Oct Expression
`AnalyzeSignalBatch` loops over a flattened batch buffer (`Float[]` with explicit width/count) and computes each record independently.

### Friction
Per-record analysis is isolated and naturally parallelizable; sequential loops are readable but become throughput bottlenecks for large sample batches.

### Rejected Workaround
Inlining all analysis logic into one giant loop body was rejected: it slightly reduces call overhead but worsens maintainability and does not address independent-work parallelism.

### Recommendation
**Structured parallel loop/range seems justified**

### Minimal Needed Surface (only if justified)
A bounded parallel range form for independent loop iterations with deterministic completion before leaving scope. No shared mutable cross-iteration writes unless explicitly reduced.

---

## C. Monte Carlo / Repeated Trial Style Work

### Task
Run multiple independent trial groups (`RunTrialBatch`) with distinct seeds and fixed sample count.

### Outcome
**Real Pressure For Structured Parallelism**

### Current Oct Expression
A sequential outer loop runs each trial group; each group has its own internal sample loop and local state.

### Friction
The repeated-trial seam is highly independent and frequently CPU-bound; sequential outer-loop execution is the main bottleneck for larger study sizes.

### Rejected Workaround
Creating ad-hoc staged files or multiple CLI invocations per seed was rejected. It externalizes scheduling to shell scripts, fragments reproducibility, and moves orchestration burden to users.

### Recommendation
**Structured batch/map-style execution seems justified**

### Minimal Needed Surface (only if justified)
Batch execution over independent trial descriptors (e.g., seeds or parameter tuples), returning one summary per descriptor in input order, with an implicit synchronization boundary.

---

## D. Independent Per-Element / Per-Chunk Work

### Task
Compute per-chunk summary metrics (mean and max) over four independent chunk arrays.

### Outcome
**Wants Better Data/Array Ergonomics Instead**

### Current Oct Expression
A sequential loop over flattened chunk buffers calls a small helper and writes each chunk result.

### Friction
The strongest day-to-day pain here is not parallelism first; it is data ergonomics (fixed-size buffer setup, shape boilerplate, and manual assembly). At this scale, sequential expression is acceptable.

### Rejected Workaround
Forcing pseudo-parallel chunk splitting with duplicated helper variants was rejected because it compounds array-shape boilerplate without improving core readability.

### Recommendation
**No concurrency feature needed**

---

## E. “Looks Parallel But Probably Shouldn’t Be The First Model”

### Task
Build a 3-bucket histogram with shared mutable counters updated per sample.

### Outcome
**Should Not Be The First Concurrency Model**

### Current Oct Expression
Single sequential pass with explicit shared counter updates (`Low`, `Mid`, `High`).

### Friction
This invites shared-state update races under parallelization and quickly suggests atomics/locks/reduction semantics, which are broader than the intended first-step model.

### Rejected Workaround
Simulated message-passing accumulation or manual two-phase merge scaffolding in user code was rejected: it introduces concurrency architecture and hides the core domain logic.

### Recommendation
**This should not shape Oct’s first concurrency model**

---

## Global Summary

### 1. Overall Assessment
Oct appears to need **a small structured CPU parallelism surface** for independent batch/range work, while keeping general concurrency models out of scope.

### 2. Strongest Real Pressure
Independent parameter sweeps and repeated-trial batches are the strongest pressure points: high-volume, embarrassingly parallel, and naturally expressed as same-shape work over many inputs.

### 3. Strongest False Pressure
Shared-state accumulation patterns (histogram-like updates, producer/consumer instincts) look concurrent but would drag in synchronization models that are too broad for first-step Oct concurrency.

### 4. Most Natural Surface Shape
The most natural first shape is **structured batch/map execution** (optionally paired with a constrained parallel range form), with deterministic ordering and implicit join.

### 5. Interaction With Prometheus
Large dense numeric kernels and throughput goals dominated by vectorized/tensor-like math should remain future **Prometheus/GPU path** territory; first-step CPU parallelism should target independent coarse-grain units, not deep data-parallel kernel design.

### 6. Recommended Next Step
**Begin designing a structured batch/map model**
