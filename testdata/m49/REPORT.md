# M49 — Structured Batch/Map Syntax Design

This milestone selects concrete syntax for Oct’s first concurrency model without changing the model itself. The target remains strict structured batch/map over independent work with ordered outputs and implicit join.

## 1. Candidate Comparison

### Candidate
**Candidate A — Expression-style map**

Example syntax:
- `results = batch_map params as p => run_trial(p)`

### Outcome
**Acceptable But Weaker**

### Best Workload Match
- Parameter Sweep where each item is already expression-oriented

### Main Strength
Very concise and easy to read in one-line transforms.

### Main Risk
Reads like a generic FP library call; independence and structured-join constraints can feel incidental instead of central.

---

### Candidate
**Candidate B — Structured block form**

Example syntax:
- `results = batch params as p { return run_trial(p) }`

### Outcome
**Strong Fit**

### Best Workload Match
- Parameter Sweep
- Monte Carlo / Trials
- Independent Signal Batch

### Main Strength
Makes the independence boundary explicit (`batch ... as item { ... }`) while still reading as “one output per input item.”

### Main Risk
More verbose than expression-only forms; requires tight syntax discipline so it is not mistaken for a mutable loop.

---

### Candidate
**Candidate C — For-like constrained form**

Example syntax:
- `results = parallel for p in params { return run_trial(p) }`

### Outcome
**Misleading For First Model**

### Best Workload Match
- Teams already biased toward loop syntax

### Main Strength
Immediate familiarity.

### Main Risk
Strongly imports loop mental models (indices, mutation, worker mechanics), which conflicts with Oct’s “independent mapping” first model.

---

### Candidate
**Candidate D — Pipeline/transform style**

Example syntax:
- `results = params |> batch_map(p => run_trial(p))`

### Outcome
**Misleading For First Model**

### Best Workload Match
- Composed transformation-heavy pipelines

### Main Strength
Composability for transform chains.

### Main Risk
Too abstract and library-shaped; hides execution boundary and encourages FP/pipeline framing over explicit independent-work semantics.

## 2. Workload-by-Workload Judgment

### A. Parameter Sweep
**Most natural: Candidate B.**
It exposes the input batch and item binding explicitly, while keeping one-result-per-item obvious in the block return.

### B. Monte Carlo / Trials
**Most natural: Candidate B.**
Trial descriptors are mapped independently with clear per-trial local scope; ordered summaries are naturally collected.

### C. Independent Signal Batch
**Most natural: Candidate B.**
Per-signal metrics fit directly into `batch signals as s { ... }` with no loop/index noise.

### D. Near-Miss Per-Element
**Most protective: Candidate B.**
Its explicit “batch over independent items” phrasing supports chunk-level or record-level units and discourages kernel-style element mutation patterns.

### E. Rejected Pattern Check (shared histogram / pipeline dependencies)
**Most protective: Candidate B.**
Shared accumulation and stage dependencies look visibly out-of-model inside a per-item return block, making misuse easier to reject during review.

## 3. Final Syntax Recommendation

### Chosen Form
**Candidate B (structured block), concrete proposal:**

- `results = batch <input_expr> as <item_name> { <item_body> }`
- The block must produce exactly one value per item (typically via `return <expr>`).

### Example (Parameter Sweep)
```oct
params = [
    {alpha: 0.1, beta: 1.0},
    {alpha: 0.2, beta: 1.0},
    {alpha: 0.3, beta: 1.0},
]

results = batch params as p {
    outcome = simulate(p.alpha, p.beta)
    return {alpha: p.alpha, beta: p.beta, score: outcome.score}
}
```

### Example (Monte Carlo)
```oct
seeds = [101, 102, 103, 104]

summaries = batch seeds as seed {
    trial = run_trial(seed)
    return {seed: seed, mean: trial.mean, variance: trial.variance}
}
```

### Example (Signal Batch)
```oct
signals = load_signal_batch("run-17")

metrics = batch signals as sig {
    m = analyze_signal(sig)
    return {snr: m.snr, peak: m.peak, drift: m.drift}
}
```

## 4. Minimal Semantics

- **Input evaluation:** `<input_expr>` is evaluated once to a finite ordered collection.
- **Item processing:** each element is bound to `<item_name>` in an isolated per-item scope and evaluated independently.
- **Result collection:** each item evaluation yields one result value.
- **Completion:** leaving `batch` implies an implicit join; the assignment target receives the full output collection only after all items complete successfully.
- **Ordering:** output order matches input order exactly.

## 5. Explicit Non-Goals

This syntax intentionally does **not** provide:

- shared mutable accumulation across items
- partial completion visibility or streaming result exposure
- task handles, futures, or cancellation APIs
- async/await behavior
- message passing or channels
- loop-style mutation patterns (`results[i] = ...` worker mechanics as the core model)

## 6. Edge Case Behavior

- **Empty input:** produces an empty output collection of the inferred result element type.
- **Single-element input:** produces a single-element output collection; semantics are identical.
- **Error in one item:** the `batch` expression fails as a whole; no partial output value is produced.
- **Type mismatch in result:** compile-time error when item result types are inconsistent or cannot unify to one output element type.

## 7. Rejection of Alternatives

- **Candidate A (expression map) rejected as final choice:** concise, but too easy to perceive as a library transform rather than a language-level structured execution boundary.
- **Candidate C (for-like) rejected:** imports loop/index/mutation expectations and drifts toward parallel-loop culture, which is specifically out of scope.
- **Candidate D (pipeline style) rejected:** over-abstracts execution and encourages FP/pipeline framing that can hide independence and implicit-join semantics.

## Decision

Choose **Candidate B structured block syntax** for M49.

It is the smallest form that keeps the correct model obvious:
**independent input items in, one ordered output per item out, implicit join at the boundary.**
