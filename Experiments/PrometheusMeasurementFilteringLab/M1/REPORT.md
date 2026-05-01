# P14 M1 — Measurement Filtering / Evidence Robustness Lab

## M1 Follow-up — Scoring Sanity Audit

### 1) Root cause of suspicious all-raw outcome
The previous setup used one global composite mix and weak regime-specific weighting, so raw could dominate by step responsiveness and low complexity carry-through. Follow-up introduces regime-specific quality weighting plus separate complexity accounting.

### 2) True vs observed signal audit
- True signal is generated in `M1GenerateRegimeSeries` as `current` and appended to `TrueValues`.
- Observed/noisy signal is `current + jitter + spike` appended to `Measurements`.
- Filter estimates are scored against `TrueValues` inside `M1Score` via `MeanAbsError(estimate, series.TrueValues)`.

### 3) Scoring formula
For each candidate:
- `quality_score = M1QualityScore(regime, mae, spike, stepDelay, driftErr, stability)`
- `complexity_score = 0.03 * complexity`
- `composite_score = quality_score + complexity_score`
Lower is better.

Weights are regime-specific and emitted via `m1_scoring_weights.octagon`.

### 4) Regime parameters
Configured in `M1RegimeConfigs`:
- stable-low-jitter: stddev 0.05, no spikes
- moderate-jitter: stddev 0.25, pSpike 0.04, amp 1.0
- spike-heavy: stddev 0.10, pSpike 0.12, amp 4.0
- step-change: step index 40, step +3.0, jitter+minor spikes
- slow-drift: drift +0.035/sample
- mixed-hostile: jitter 0.22, spikes p=0.08 amp=2.5, step +2.5, drift +0.02/sample

### 5) Per-metric results
Full per-metric breakdown is emitted in `m1_metric_breakdown.octagon` (MAE, spike sensitivity, step delay, drift error, stability, complexity, quality, composite).

### 6) Top 3 by regime
Emitted in `m1_top3_by_regime.octagon` (rank1/rank2/rank3 per regime).

### 7) Updated recommendations
Emitted in `m1_final_recommendation.octagon` and now include:
- composite winner,
- quality-only winner,
per regime.

### 8) Sanity tests
Added tests covering:
- spike-heavy robust filter beats raw on spike sensitivity,
- EMA smooths low-jitter more than raw,
- fast EMA step delay <= slow EMA,
- truth-vs-observed metric guard,
- deterministic recommendations.

### 9) Limitations
Still synthetic and seed/scenario dependent; no runtime diagnostic integration yet; weighting is transparent but still policy-initial and subject to recalibration.
