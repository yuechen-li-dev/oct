# M48 Workload Sketches — Candidate Surface Pressure Inputs

This file contains non-executable design sketches used to pressure-test candidate concurrency surface families against concrete scientific/engineering batch workloads.

## Candidate Legend

- **A: structured batch/map form**
- **B: parallel range/loop form**
- **C: execution policy / annotation shape**

---

## A. Parameter Sweep (model scoring over independent parameter vectors)

### Domain workload
Given 10,000 independent parameter vectors, evaluate `ScoreModel(params)` and return one record per input in input order.

### Candidate A sketch
- `results = parallel map params in parameter_set -> ScoreModel(params)`
- Reads as direct batch-in, batch-out contract.

### Candidate B sketch
- `allocate results[count(parameter_set)]`
- `parallel for i in 0..count(parameter_set) { results[i] = ScoreModel(parameter_set[i]) }`
- Requires index destination mechanics.

### Candidate C sketch
- `with execution(policy: parallel) { for params in parameter_set { append results, ScoreModel(params) } }`
- Requires users to infer ordered output behavior from policy semantics.

---

## B. Monte Carlo / Repeated Trials (independent trial descriptors)

### Domain workload
Given `TrialDescriptor[]` (seed + trial_count + scenario knobs), run one independent trial bundle per descriptor and produce one summary per descriptor.

### Candidate A sketch
- `summaries = parallel map trial in trials -> RunTrialBundle(trial)`
- Explicitly exposes independence at descriptor boundary.

### Candidate B sketch
- `allocate summaries[count(trials)]`
- `parallel for i in 0..count(trials) { summaries[i] = RunTrialBundle(trials[i]) }`
- Works, but loop mechanics are foregrounded.

### Candidate C sketch
- `parallel policy block around ordinary loop`
- Risk: users perceive mode switch rather than constrained independent-map abstraction.

---

## C. Independent Signal/Sample Batch (per-signal metrics)

### Domain workload
Given `Signal[]`, compute `{rms, peak, band_energy}` per signal and preserve input ordering.

### Candidate A sketch
- `metrics = parallel map s in signals -> AnalyzeSignal(s)`
- Shape mirrors domain statement exactly.

### Candidate B sketch
- `allocate metrics[len(signals)]`
- `parallel for i in 0..len(signals) { metrics[i] = AnalyzeSignal(signals[i]) }`
- Semantically clear but more mechanical.

### Candidate C sketch
- `AnalyzeSignalBatch(signals) with execution=parallel`
- Output ordering and allowed side effects depend on extra policy rules.

---

## D. Near-Miss Per-Element / Chunk Transform

### Domain workload
Transform very large arrays element-wise, and compute chunk-wise operations for throughput.

### Candidate A sketch
- `parallel map` can describe chunk-level independent units cleanly.
- Risk if users push it toward tiny element-level kernels where future Prometheus-style data-parallel ergonomics should dominate.

### Candidate B sketch
- `parallel for i in ...` strongly suggests low-level element scheduling and index micromanagement.
- High risk of accidental drift into pseudo-OpenMP/vector-kernel culture.

### Candidate C sketch
- Policy flags on normal loops can become a blanket “make this fast” knob.
- Highest risk of silent overreach into low-level data parallel assumptions.

---

## E. Rejected Pattern Check (shared histogram / producer-consumer instincts)

### Domain workload
Histogram accumulation with shared bins and pipeline-style interdependent stages.

### Candidate A sketch
- Mismatch is visible: map wants independent input->output units.
- Easier to explain as “not first-model eligible” without introducing channels/tasks.

### Candidate B sketch
- `parallel for` invites “just add atomics/reduction” instincts.
- Encourages worker-thinking earlier than Oct should support.

### Candidate C sketch
- Policy annotation invites “toggle to parallel and fix races later” behavior.
- Weakest footgun resistance for first-model boundaries.
