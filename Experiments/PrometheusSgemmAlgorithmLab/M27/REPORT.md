# Prometheus SGEMM Algorithm Lab — M27

## 1) M25/M26 synthesis (required first step)

### What M25 proved about pull-lag vs fixed double

M25 established a directional split: pull-lag can run lean in stable regimes, while fixed double buffering is the robust default under jitter and burst disruptions.

### What M26 proved about noise mitigation

M26 showed that EWMA smoothing and control-chart style detection are useful practical tools, but fixed double remained the robust noisy-regime fallback when bursts and jitter dominate.

### Why Kalman is no longer a default candidate

M26 integrated Kalman successfully, but product score did not justify the extra complexity versus simpler EWMA + detector behavior. M27 therefore does **not** reopen Kalman tuning and does not treat Kalman as a default controller path.

### Why M27 hybrid is the next product-shaped candidate

Given M25/M26, the next product-shaped candidate is exactly one hybrid lifecycle:

- stable pull mode for low WIP,
- control-chart-triggered fallback to fixed double when noisy,
- explicit recovery hysteresis before returning to pull.

### What success/failure means for M28/M29

- **Success in M27:** carry this hybrid into M28 rake hardening, then consider implementation in M29.
- **Failure in M27:** pivot M28 toward fixed double as practical default and defer pull-lag controller path.

## 2) Corrected WIP model explanation

M26 noted WIP-separation compression. M27 corrected model shape by tracking:

1. ready-but-unused inventory-time,
2. total WIP inventory-time (depth × time),
3. average/peak WIP depth,
4. memory-pressure time proxy weighted by held ready/in-flight buffers.

Validation table shows fixed double holds materially more idle inventory-time than raw pull in every tested regime (e.g., stable-balanced ready-idle: 1233 vs 617; compute-bound-stable: 1968 vs 24).

## 3) Candidate strategies compared

- **A** fixed double buffering (robust baseline),
- **B** raw pull-lag (lean baseline),
- **C** hybrid pull/double controller (candidate),
- **D** fixed triple (reference row only).

## 4) Hybrid controller design (M27)

Three modes were implemented:

1. **Stable Pull mode**
   - pull-lag staging,
   - EWMA transfer estimate,
   - low cap/margin.
2. **Noisy Fallback mode**
   - fixed double behavior,
   - entered by control-chart noise signal.
3. **Recovery mode**
   - conservative settings,
   - minimum dwell,
   - stable-window gate before return to stable pull.

Octomata was used for mode-lifecycle transition edge accounting (`DetectEdge`) so transition events are explicit and auditable in mode-transition artifacts.

## 5) Workload regimes used

M27 ran all required regimes:

1. stable-balanced,
2. compute-bound-stable,
3. ordinary-jitter,
4. burst-shock,
5. calm-then-burst,
6. burst-then-recovery,
7. boundary-jitter.

## 6) Detector and mode-transition behavior

From detector + transition artifacts:

- False noisy detections remained 0 in this model.
- Missed burst detections remained present in burst-led regimes (burst-shock: 1, calm-then-burst: 1, burst-then-recovery: 7).
- Mode transitions stayed bounded (0–3 per regime for hybrid).
- Boundary-jitter recovery delay was 3 ticks.

Interpretation: thrash is controlled, but burst-then-recovery detection quality is still weak.

## 7) WIP vs starvation tradeoff

- Hybrid did **not** beat fixed double on product score in stable-balanced.
- Hybrid did **not** stay close enough to fixed double in burst-shock completion/starvation.
- Hybrid did reduce WIP versus fixed double in stable regimes, but with too much starvation/completion loss in noisy regimes.

## 8) Per-strategy findings

- **Fixed double (A):** strongest practical default across M27 product score rows.
- **Raw pull (B):** lean inventory but large starvation/completion penalties under noise.
- **Hybrid (C):** middle ground; mode behavior is structurally stable but still underperforms fixed double in this product score shape.
- **Triple reference (D):** high WIP/memory overhead; not a default candidate.

## 9) Final recommendation

M27 recommendation output is:

- WIP separation issue: **fixed** in this model.
- Hybrid vs fixed double (stable): **no**.
- Hybrid closeness under jitter/burst: **no**.
- Hybrid WIP reduction without unacceptable starvation: **no**.
- Mode thrash control: **yes (controlled)**.
- Carry hybrid to M28 rake lab: **no**.
- Practical default if not: **fixed double**.

## 10) M28/M29 direction

- **M28:** pivot to fixed-double default path; defer hybrid as non-carry candidate from M27.
- **M29:** implementation path should prioritize fixed-double operationalization, with pull-lag/hybrid retained only as optional guarded experiments.

## Required final answers (explicit)

1. **Did M27 fix WIP-separation from M26?** Yes, in this simulation model.
2. **Does hybrid beat fixed double in stable regimes?** No.
3. **Does hybrid stay close to fixed double under jitter/burst?** No.
4. **Does hybrid materially reduce WIP without unacceptable starvation?** No.
5. **Is mode thrash controlled?** Yes.
6. **Is hybrid worth carrying to M28 rake lab?** No.
7. **If not, should fixed double become practical default?** Yes.

## Scope note

M27 intentionally stayed narrow:

- no new estimator family bakeoff,
- no Kalman retest,
- no triple-buffer default reopening,
- no native C/Vulkan implementation claims,
- no hardware timing claims.
