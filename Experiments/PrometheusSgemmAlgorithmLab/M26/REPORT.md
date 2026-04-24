# Prometheus SGEMM Algorithm Lab — M26

## Noise-Mitigation Lab for Pull-Based Buffering

## 1) Required first step: M25 recap mapped to M26

M25 concluded that pull-lag is promising in stable regimes, but fragile under jitter/burst, while fixed double buffering remained the robust baseline. M26 therefore runs a bounded bakeoff focused on making pull-lag robust enough to productize via filtering, burst detection, and controlled fallback rather than reopening kernel design.

Explicit mapping to M26:
1. **Why raw pull-lag is promising:** in M25 it reduced inventory waste in stable/compute-bound conditions.
2. **Why raw pull-lag is fragile:** under jitter/burst it can stage late (starvation) or oscillate (margin/cap changes).
3. **Why fixed double remains baseline:** it is the known robust fallback with low control complexity and consistent behavior.
4. **Techniques tested in M26:** EWMA, Kalman input smoothing, alpha-beta trend, median/MAD robustification, CUSUM shift detection, control-chart detection, and PI margin feedback.
5. **Success definition for productization:** pull-lag should beat raw pull and approach fixed-double robustness in jitter/burst while preserving lean WIP behavior in stable regimes.

## 2) Candidate methods and rationale

Implemented candidates:
- A: fixed double baseline.
- B: raw pull-lag.
- C: pull-lag + EWMA lead-time estimate.
- D: pull-lag + Octomata Kalman predict/correct path.
- E: pull-lag + alpha-beta trend filter.
- F: pull-lag + median/MAD robust filter.
- G: pull-lag + CUSUM burst detector.
- H: pull-lag + control-chart detector.
- I: pull-lag + PI safety-margin controller (clamped + anti-windup accounting).

## 3) Workload regimes

M26 runs all required regimes:
1. stable-balanced
2. transfer-bound
3. compute-bound
4. ordinary-jitter
5. burst-shock
6. calm-then-burst
7. burst-then-recovery
8. false-stability

## 4) Metrics and product score

Captured metrics include completion time, starvation, ready-idle, total/avg/peak WIP, memory proxy, safety margin min/avg/max, mode transitions, false/missed burst detections, recovery time, WIP cap changes, and per-controller diagnostics. Product score penalizes completion, starvation, WIP waste, memory pressure, transition instability, and strategy complexity.

## 5) Per-method findings

- **Fixed double (A)** won both ordinary-jitter and burst-shock product score in this M26 model.
- **Raw pull (B)** remained close in stable regimes but did not surpass baseline under noise.
- **EWMA (C)** reduced estimate noise but did not change ranking enough to displace baseline.
- **Kalman (D)** integration worked with Octomata predict/correct; result quality did not beat baseline or raw enough for defaulting.
- **Alpha-beta (E)** behaved similarly to EWMA in this bounded model.
- **Median/MAD (F)** rejected outliers, but could underreact around true burst transitions.
- **CUSUM (G)** detected aggressively but produced many false detections in this tuning.
- **Control-chart (H)** produced fewer false detections than CUSUM but still did not beat fixed double.
- **PI margin (I)** remained bounded but did not produce a decisive product-score gain versus baseline.

## 6) Jitter and burst findings

- Ordinary jitter winner: **A-fixed-double-baseline**.
- Burst-shock winner: **A-fixed-double-baseline**.
- CUSUM was sensitive (many detections) but noisy in false alarms.
- Control-chart was calmer but still not enough to outperform baseline.

## 7) Stable-regime findings

Stable/transfer/compute rows stayed close in completion. Pull variants did not produce enough WIP advantage in this specific model to offset complexity penalties, so stable-regime upside was not strong enough yet for product default.

## 8) Complexity/productization assessment

- Kalman adds integration and tuning burden; in this simulation it did not earn default complexity cost.
- EWMA is cheap and understandable; useful as a candidate input smoother, but not sufficient alone for burst robustness.
- PI margin can be kept as optional extension only if tightly clamped and justified by measurable gain.
- Fixed double should remain explicit fallback because it still dominates noisy regimes in this bakeoff.

## 9) Final recommendation

1. **Which method best stabilizes pull-lag under ordinary jitter?** Fixed double baseline remains best in this run.
2. **Which method best handles burst shock?** Fixed double baseline remains best.
3. **Does Kalman help as lead-time estimator?** It integrates successfully, but did not beat simpler methods enough for default.
4. **Is EWMA enough?** Not enough alone for burst robustness.
5. **Is PI margin control worth carrying forward?** Defer by default; only continue with strict clamp/anti-windup if follow-up tests show real gain.
6. **Should fixed double remain fallback?** Yes.
7. **Is adaptive pull-lag implementation-worthy now?** Defer until detector+recovery tuning closes robustness gap.
8. **What should M27 do next?** Implement a narrow hybrid: EWMA-smoothed pull mode in stable windows + control-chart-triggered fallback to fixed double with explicit hysteresis and recovery gate.

## 10) M27 direction

M27 should implement and test only one product-candidate hybrid policy path (not a broad framework):
- stable mode: pull-lag with simple smoothing
- noisy mode: fixed double fallback via control-chart gate
- recovery mode: hysteresis + minimum dwell before returning to pull

## Explicit inconsistency surfaced

M25 motivation emphasized low-WIP wins for pull-lag. In this M26 implementation, peak WIP remained uniformly low across strategies in many rows, which compresses separation and likely understates pull-vs-buffer tradeoffs. This is a model-shape inconsistency versus M25’s stronger WIP spread and should be refined in M27 scenario tuning instead of interpreted as a final product truth.
