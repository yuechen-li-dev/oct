# Prometheus SGEMM Algorithm Lab — M14

## Product candidate bakeoff for practical staging controller

## 1) Required synthesis from M12 + M13

### 1.1 What M12 says the controller should probably do

M12 findings imply the default product shape should:

- keep overlap enabled,
- keep lookahead modest (`1..2`),
- keep outstanding depth bounded,
- allow adaptive chunk sizing under strict min/max bounds,
- treat speculative waste as a first-class guardrail.

### 1.2 What M13 says the controller should probably not rely on

M13 showed estimator sophistication alone is not the answer:

- Kalman-level prediction complexity did not materially improve completion time,
- predictor families were often time-tied,
- extra speculation tended to raise waste.

So controller success should **not** depend on high-complexity prediction math.

### 1.3 Product-relevant characteristics for M14

M14 prioritizes controllers that are:

- bounded,
- explainable,
- robust across regimes (not just one best case),
- stable (limited thrash),
- explicit about retreat/safety behavior,
- plausible for Prometheus Judgment Engine implementation.

### 1.4 M14 success criteria used to pick a winner

The bakeoff selected candidates using a balanced product score composed of:

- completion time,
- wasted transfer work (weighted guardrail),
- mode instability transitions,
- retreat frequency,
- implementation complexity score.

This prevents choosing a winner on narrow single-metric speed only.

---

## 2) Candidate set and rationale

M14 compared six practical candidates:

1. **A — conservative bounded overlap**
2. **B — overlap + adaptive chunk sizing**
3. **C — waste-budgeted speculative controller**
4. **D — confidence-gated conservative controller**
5. **E — regime-classified controller**
6. **F — hysteresis-gated regime controller** (preferred optional extension)

All are bounded and avoid heavy estimator dependence.

---

## 3) Regimes, knobs, and metrics

### Regimes

- transfer-bound
- compute-bound
- balanced
- predictability-sensitive

### Knobs surfaced (explicitly visible per candidate)

- base chunk size,
- lookahead,
- outstanding depth,
- speculation budget,
- retreat/recover thresholds,
- hysteresis window (F only).

### Metrics tracked

- total completion time,
- transfer stall time,
- compute idle time,
- total transfer work,
- wasted transfer work,
- average chunk size used,
- mode/controller stability,
- retreat frequency,
- implementation complexity score,
- combined product score.

Artifacts:

- `m14_candidate_comparison.octagon`
- `m14_regime_winners.octagon`
- `m14_m15_handoff.octagon`

---

## 4) Bakeoff results

## 4.1 Per-regime winner (by product score)

- transfer-bound: **B-adaptive-chunk** (`4548`)
- compute-bound: **C-waste-budgeted-speculation** (`4324`)
- balanced: **C-waste-budgeted-speculation** (`3384`)
- predictability-sensitive: **C-waste-budgeted-speculation** (`3398`)

## 4.2 Cross-regime aggregate (lower is better)

- **C-waste-budgeted-speculation**: `15674`
- E-regime-classified: `19090`
- B-adaptive-chunk: `19386`
- A-conservative-bounded: `19392`
- F-hysteresis-regime: `19408`
- D-confidence-gated: `22294`

## 4.3 Why C wins this practical bakeoff

C is the best balanced product candidate in this simulation because it:

- remains bounded (`lookahead <= 2`, bounded depth, explicit waste budget),
- controls waste materially better than most alternatives,
- stays competitive on completion time across regimes,
- has explicit retreat semantics suitable for production safety policy,
- keeps logic understandable without estimator-heavy machinery.

C wins 3/4 regimes by product score and has the best aggregate score.

---

## 5) Per-candidate strengths and weaknesses

### A — conservative bounded

- Strengths: simplest, very explainable, stable.
- Weaknesses: too conservative under transfer pressure; weaker cross-regime score.

### B — adaptive chunk

- Strengths: best transfer-bound result; simple adaptation shape.
- Weaknesses: higher waste than C in non-transfer regimes.

### C — waste-budgeted speculation (**winner**)

- Strengths: best cross-regime tradeoff, strongest waste robustness with bounded retreat.
- Weaknesses: retreat tuning risk (could over-retreat in some workloads).

### D — confidence-gated conservative

- Strengths: explicit fallback concept.
- Weaknesses: unstable in this simulation (high transitions + high waste).

### E — regime-classified

- Strengths: very Prometheus-shaped policy selection model.
- Weaknesses: this bounded mapping still leaked too much waste under noisy predictability.

### F — hysteresis regime

- Strengths: reduced oscillation risk versus plain regime selection.
- Weaknesses: extra complexity and sticky-mode lag risk reduced score.

---

## 6) Remaining risks for winning candidate C

Primary risks to validate next:

1. over-retreat reducing overlap benefits,
2. under-retreat allowing waste spikes,
3. threshold brittleness at regime boundaries,
4. interaction between chunk dynamics and budget depletion,
5. behavior under abrupt regime shifts.

---

## 7) Required M15 rake-lab targets (explicit handoff)

M15 should directly attack these failure modes for **C-waste-budgeted-speculation**:

1. oscillation/thrash near regime boundaries,
2. over-retreat / under-retreat with degrading predictability,
3. pathological chunk-size adaptation with bursty transfer setup,
4. hysteresis/sticky-like lag behavior after abrupt shifts.

Concrete rake tests are provided in `m14_m15_handoff.octagon`.

---

## 8) M16 implementation target if C survives M15

If M15 confirms robustness, M16 should implement:

- a bounded overlap controller with fixed small lookahead (`1..2`),
- explicit speculation budget accounting,
- retreat/recovery threshold controls,
- observability counters for waste, retreat events, and mode stability,
- policy hooks compatible with Prometheus Judgment Engine integration.

This provides a practical, productizable controller shape without estimator complexity creep.

---

## 9) Octomata usage note

M14 kept control logic explicit in bounded tables/branching rather than introducing a larger Octomata state graph. This was intentional for interpretability at bakeoff scope.
