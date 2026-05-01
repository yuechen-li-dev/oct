# P14 M1 — Measurement Filtering / Evidence Robustness Lab

## M1 Rerun After Random M8b

### 1) Why rerun was required
P14 M1 pre-M8b results are now explicitly treated as **invalid historical output**. The prior run could execute `Random.Core.oct` stubs in some paths, producing degenerate randomness and suspicious recommendations. After Random M8b/M2 validation, M1 was rerun using real Random dispatch and regenerated artifacts.

### 2) Random usage audit (required pre-rerun questions)
1. **Does M1 use actual `Random.Core` / `Random.Distributions` APIs?**  
   **Yes.** Regime generation now uses `Random.RandNormal` plus `Random.RandBernoulli`-based spike generation with real draw records (`Value`, `Next`).
2. **Does it use real `Next` state threading?**  
   **Yes.** A per-regime RNG state is seeded once and advanced via `rng = draw.Next` after each draw.
3. **Does it accidentally reuse the same RNG state repeatedly?**  
   **No.** The loop advances RNG after jitter draw and after each Bernoulli draw used for spike trigger/sign.
4. **Does it compare filter estimates against `TrueValues`, not `Measurements`?**  
   **Yes.** `M1Score` still computes `MeanAbsError(estimate, series.TrueValues)`.
5. **Does it generate visibly non-degenerate jitter/spikes/drift?**  
   **Yes.** `M1RegimeNoiseSummaries` now quantifies non-degeneracy and tests assert non-zero jitter, spikes in spike-heavy, actual step metadata, and positive drift where expected.
6. **Are old constants/stubs/workarounds from broken Random era still present?**  
   **No known stub workarounds remain in M1 regime generation.** Prior per-sample reseeding paths were replaced by threaded RNG.

### 3) Regime non-degeneracy summary
M1 now computes `RegimeNoiseSummary` per regime (sample count, true/measurement min/max, noise mean, mean noise magnitude, spike count, step metadata, total drift) via `M1RegimeNoiseSummaries`.

Sanity expectations validated in tests:
- stable-low-jitter has low but non-zero noise magnitude,
- moderate-jitter has higher noise magnitude than stable-low-jitter,
- spike-heavy has non-zero spike count,
- step-change has non-negative step index and non-zero step delta,
- slow-drift has positive total drift,
- mixed-hostile includes spikes + step + drift.

### 4) Scoring formula / weights
Scoring remains transparent and regime-specific:
- `quality_score = M1QualityScore(regime, mae, spike, stepDelay, driftErr, stability)`
- `complexity_score = 0.03 * complexity`
- `composite_score = quality_score + complexity_score`

Per-regime weight formulas are emitted in `m1_scoring_weights.octagon`.

### 5) Sanity-test outcomes
M1 test suite enforces:
- robust filter sanity in spike-heavy (median or hybrid beats raw on spike sensitivity),
- EMA stability improvement over raw on jitter,
- fast EMA step response no slower than slow EMA,
- true-vs-observed scoring guard,
- random non-degeneracy guard,
- deterministic recommendations and score matrix.

### 6) Updated top 3 per regime
Updated rankings are regenerated in `m1_top3_by_regime.octagon` and include top-3 candidates for each regime from rerun data.

### 7) Final implementation recommendation
Per-regime recommendations are regenerated in `m1_final_recommendation.octagon`, including:
- composite winner,
- quality-only winner,
- composite score.

These outputs are computed from rerun metrics (not hardcoded) and should supersede pre-M8b recommendations.

### 8) Limitations
- Still synthetic-lab scenarios and seed-dependent.
- Complexity penalty is intentionally simple and policy-level.
- No production runtime diagnostics/hardware timing in scope for M1.

### 9) Next milestone recommendation
Proceed to P14 implementation planning using this rerun as the baseline, then validate selected filters against broader scenario sets and runtime constraints in subsequent milestones.
