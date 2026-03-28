# M48 — Concurrency Surface Pressure Test: Structured Batch/Map Shapes

This pressure test compares candidate user-facing surface families using concrete independent scientific/engineering workloads (parameter sweeps, trial batches, signal batches), plus near-miss and rejected-pattern checks. It does not implement concurrency.

## 1. Candidate Comparison

### Candidate
**A — Structured batch/map form** (`parallel map` / `batch map` conceptual family)

### Outcome
**Strong Fit**

### Best Workload Match
- Parameter Sweep
- Monte Carlo / Repeated Trials
- Independent Signal Batch

### Main Strength
It directly expresses Oct’s first-model contract: explicit independent-work mapping, ordered output collection, and implicit join at construct boundary.

### Main Risk
If underspecified, users may still try to treat it like tiny per-element kernel parallelism rather than coarse independent units.

---

### Candidate
**B — Parallel range/loop form** (`parallel for` / `parallel range` conceptual family)

### Outcome
**Acceptable But Weaker**

### Best Workload Match
- Monte Carlo descriptor arrays where indexed destination writes are natural
- Some signal/sample batches with preallocated outputs

### Main Strength
Deterministic result shape can be made explicit through indexed destination writes (`results[i] = ...`).

### Main Risk
Surface reads like imported worker/index culture. It foregrounds mechanics (indices, destination buffers) instead of the semantic unit (independent batch mapping), increasing drift toward low-level parallel idioms.

---

### Candidate
**C — Execution policy / annotation shape** (parallel block/policy wrapper)

### Outcome
**Misleading For First Concurrency Model**

### Best Workload Match
- Limited fit for teams already using policy-driven compute APIs

### Main Strength
Low syntactic disruption for existing sequential-looking code.

### Main Risk
Encourages “flip parallel mode” thinking without making independence constraints first-class. Highest chance of importing async/task/goroutine-style expectations and race-prone habits.

## 2. Workload-by-Workload Read

### Parameter Sweep
**Most natural: Candidate A (structured batch/map).**
Reason: the domain statement is n independent parameter inputs producing n ordered records; batch/map mirrors that shape directly.

### Monte Carlo / Repeated Trials
**Most natural: Candidate A, with Candidate B as secondary.**
Reason: trial descriptors map cleanly to one-summary-per-descriptor. B works but pushes attention to loop/index scaffolding rather than experiment descriptors.

### Independent Signal Batch
**Most natural: Candidate A.**
Reason: per-signal metric extraction is semantically a map over independent signals; ordered output comes “for free” in the mental model.

### Near-Miss Per-Element
**Most natural for first model: Candidate A (at coarse chunk level only).**
Reason: A can be constrained to independent coarse units; B and C more strongly invite low-level element scheduling that should remain future Prometheus/data-parallel territory.

### Rejected Pattern Check
**Most protective: Candidate A.**
Reason: shared accumulation and pipeline dependencies visibly do not fit map semantics. B/C both make it too easy to rationalize shared-state parallelism and “we’ll add synchronization later.”

## 3. Recommended Surface Direction

**structured batch/map**

This is the smallest surface family that stays explicit about independent work, preserves deterministic ordered outputs by default, and keeps Oct aligned with “correct way is easiest way.” It fits the highest-pressure real workloads (sweeps, repeated trials, signal batches) without importing general-purpose concurrency culture.

## 4. Minimality Check

The chosen direction must remain a constrained structured construct and must **not** become:

- goroutines (no user-visible spawned concurrent lifetimes)
- channels (no message-passing topology surface)
- async/await (no suspended control-flow model)
- general tasks/futures (no arbitrary task graph API)
- shared mutable parallelism (no implicit shared-state update model)
- low-level vector-kernel semantics (no pseudo-SIMD/OpenMP element-kernel programming surface)

Practical guardrail: keep the first surface strictly “independent input units -> ordered output units -> implicit join.”

## 5. Recommended Next Step

**begin formal syntax design for the chosen surface**

Rationale: this pressure test shows a clear winner (Candidate A) across primary workloads and boundary-protection criteria. Remaining work should narrow syntax and static constraints, not reopen model selection.
