# P14 M3 — Measurement Filtering Lab

## 1) Problem statement
Define implementation-ready filter primitives for Prometheus/Dominatus with explicit state-transition shape: `FilterState + Measurement -> FilterState + FilterOutput`.

## 2) Regime definitions
Used six regimes: stable-low-jitter, moderate-jitter, spike-heavy, step-change, slow-drift, mixed-hostile. All use seeded `Random` Gaussian jitter + Bernoulli spikes (+ optional step/drift).

## 3) Candidate filter definitions
Raked candidates:
- EMA: alpha {0.1,0.2,0.4,0.6}
- Sliding median: window {3,5,9}
- Trimmed mean: (5,1), (9,1), (9,2)
- Hysteresis/deadband: band {0.05,0.10,0.20}
- Hybrid: median3+ema0.2, median5+ema0.2, reject{1,2}+ema0.2

## 4) Parameter rake grid
See `m3_parameter_rake_scores.octagon`.

## 5) Scoring formula
Per candidate/regime:
- Quality metrics: MAE, Stability, SpikeSensitivity, StepResponseDelay, DriftTrackingError, WarmupCost
- `quality_score = 1.4*mae + 0.9*stability + 0.9*spike + 0.5*step_delay + 0.8*drift_error`
- `implementation_cost_score = 0.08 * state_complexity`
- `diagnostic_value_score = 0.04 * (6 - diagnostic_usefulness)`
- `composite_score = quality_score + implementation_cost_score + diagnostic_value_score`

Implementation cost is a tie-break/modest penalty, not dominant.

## 6) Computed top candidates per regime
Generated in selected set artifact; selection excludes dominated candidates first.

## 7) Dominated candidate analysis
Dominance rule: candidate A is dominated if candidate B has `<= quality`, `<= stability`, `<= step response delay`, and `<= complexity` in same regime. Emitted in `m3_dominated_filters.octagon`.

## 8) Selected initial implementation set
See `m3_selected_implementation_set.octagon` and `m3_final_recommendation.octagon`.

## 9) Implementation contracts
Defined in `m3_filter_contracts.octagon` for EMA, SlidingMedian, TrimmedMean, HysteresisDeadband, and Hybrid variants with:
- state fields
- parameters
- init/update/reset behavior
- diagnostics
- judgment-selection hints
- weaknesses

## 10) Dominatus / Judgment Engine selection policy draft
Proposed facts:
- jitter_class
- spike_rate
- step_change_suspected
- drift_suspected
- sample_count
- confidence
- outlier_count
- filter_warmup_state

Draft mapping:
- stable/moderate jitter + low spike_rate -> EMA or Hysteresis
- spike-heavy -> SlidingMedian / Hybrid
- mixed-hostile -> Hybrid
- slow-drift low spike -> EMA alpha 0.2/0.4
- step-change suspected -> faster EMA or median3+EMA

## 11) Limitations
- Lab uses synthetic regimes only.
- Regime-specific weighting is fixed (single formula) and should be tuned in M4.
- No native C/runtime integration yet (intentional non-goal).

## 12) Next milestone recommendation
M4 should wire selected contracts into Dominatus policy simulation and validate live filter-selection switching under regime transitions.

## Inconsistency notes
- Existing experiment code commonly uses helper patterns not explicitly documented in `Language/reference` (e.g., compact all-in-one function formatting). This lab kept repository style for consistency but this should be normalized against `Language/reference` in a follow-up documentation/style pass.
