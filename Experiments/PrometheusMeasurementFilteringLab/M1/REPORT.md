# P14 M1 — Measurement Filtering / Evidence Robustness Lab

## 1) Problem statement
This lab models synthetic timing-measurement regimes and computes filter rankings per regime so Prometheus can pick filter policy by regime instead of using one global filter.

## 2) Regimes
Implemented six regimes: stable-low-jitter, moderate-jitter, spike-heavy, step-change, slow-drift, mixed-hostile.

## 3) Candidate filters
Implemented: raw, EMA (0.1/0.2/0.4), sliding mean (3/5/9), sliding median (3/5/9), trimmed mean (5/9, trim=1 each side), hysteresis (small/medium), hybrid (median-3 + EMA-0.2).

## 4) Scoring criteria
Computed per regime/candidate:
- mean absolute error,
- spike sensitivity,
- step response delay,
- drift tracking error,
- stability (mean abs consecutive delta),
- implementation complexity,
- composite score for ranking.

## 5) Computed results
Scores and recommendations are computed (not hardcoded) via `M1ScoreMatrix` + `M1FinalRecommendations` and emitted as artifacts:
- `m1_regime_score_matrix.octagon`
- `m1_candidate_filter_matrix.octagon`
- `m1_final_recommendation.octagon`

## 6) Per-regime recommendations
Use `m1_final_recommendation.octagon` as source of truth for this seed/scenario setup.

## 7) Recommended P14 initial implementation set
Recommended initial set for controller selection:
- EMA family (smooth/fast variants),
- median-window family,
- hysteresis band,
- hybrid median+EMA.

These cover stable, outlier-heavy, and oscillation-sensitive selection contexts.

## 8) Deferred filters
Defer extra variants beyond implemented set (e.g., additional robust estimators) until runtime diagnostics are integrated and real-data priors refine weighting.

## 9) Judgment Engine framing
Dominatus/Judgment Engine should consume:
- jitter class,
- spike rate,
- step-change suspicion,
- drift suspicion,
- confidence and sample count,
and choose filter+parameters or reduce confidence/freeze updates when quality is poor.

## 10) Limitations
- Synthetic data only (no real hardware timings in this lab).
- Composite weighting is initial-policy heuristic and should be recalibrated with future diagnostics.

## 11) Next milestone recommendation
Integrate regime classification inputs from runtime diagnostics, then evaluate policy-switch robustness and confidence-gated update behavior.

## Note on earlier cast issue
Oct has no implicit casting; this lab now uses explicit `FloorToInt(...)` where float→int window conversion is required.
