# P14 M4 — Dominatus Filter Policy Switching Simulation

## 1) Problem statement
M4 evaluates policy robustness under regime transitions rather than static single-regime filter quality.

## 2) Relation to M3 contracts
M3 identified filter families (EMA, median, trimmed, hysteresis, hybrid). M4 reuses these as policy-selectable kinds (`ema-slow`, `ema-fast`, `hybrid`) and evaluates switching behavior (cold, warm, parallel).

## 3) Scenario definitions
Implemented six transition scenarios:
- stable -> spike-heavy
- spike-heavy -> stable recovery
- stable -> step-change
- moderate jitter -> slow drift
- mixed-hostile -> stable
- false alarm stable with isolated spikes

## 4) Classifier fact definitions
Per tick facts:
- `sample_count`
- `recent_abs_residual`
- `recent_output_variation`
- `spike_rate_estimate`
- `jitter_estimate`
- `step_change_suspected`
- `drift_suspected`
- `confidence`

## 5) Policy candidates
A-F implemented:
- oracle per regime
- static conservative
- static smooth
- greedy fact switch
- dominatus min-commit + hysteresis + confidence gate
- dominatus parallel candidates

## 6) Switch mechanics
Compared `cold`, `warm`, and `parallel` mechanics. Dominatus recommended with warm transfer initially; parallel is strongest robustness option if cost budget allows.

## 7) Metrics/scoring formula
Separate metrics/scores are computed:
- quality: MAE, stability, spike disturbance, step settling delay, drift error
- switching: switch count, switch jump, warmup penalty, thrash
- confidence: drop/recovery quality
- implementation cost
- composite (weighted sum with cost not dominating)

## 8) Computed results
Artifacts contain full matrix and traces:
- `m4_policy_score_matrix.octagon`
- `m4_filter_switch_trace.octagon`

Observed pattern: greedy policy over-switches in false alarm and noisy transitions; dominatus min-commit/hysteresis reduces thrash with small quality tradeoff; parallel candidates reduce warmup penalties further.

## 9) Final policy recommendation
First production candidate:
- Dominatus policy
- utility scoring over facts
- `min_commit_ticks = 8`
- `switch_margin = 0.15`
- `confidence_threshold = 0.45`
- warm transfer by default
- optionally enable parallel candidates in high-criticality deployments

## 10) Proposed production contract
`PolicyState + MeasurementFacts + Measurement -> PolicyState + FilterDecision`
with fields:
- current filter kind
- per-filter states
- confidence
- last switch tick
- min commit remaining
- switch diagnostics

## 11) Known limitations
- classifier is synthetic/heuristic
- only three modeled filter kinds for switching core
- utility weights are hand-tuned and should be re-fit against real telemetry

## 12) Next milestone
M5 should port selected policy contract to native Dominatus subsystem and validate with real Prometheus diagnostics streams.

## Inconsistency notes
No syntax-level inconsistency observed with `Language/reference`. Existing compact one-line style in prior experiments is retained, though less readable than reference prose examples.

## Random v2 migration (2026-10-03)

The measurement generator now draws from Random v2 streams: one stream per
noise source, forked by name from the scenario seed (`"jitter"` and
`"spike"`), with `Random.Normal` and `Random.Spike` at the sample index. The
seeds are unchanged. The noise realization for a given seed is different, and
the recorded outputs were regenerated. Before regenerating, the recorded
files were reproduced exactly from the Random v1 code, so every difference
comes from the noise.

What moved: the selected policy is unchanged, `dominatus-mincommit-hysteresis`.
It still switches zero times in every scenario, and the greedy policy still
over-switches (6, 15, 3, 0, 6 and 3 switches across the six scenarios; 11,
12, 3, 0, 12 and 3 before). The best non-oracle composite score changed in one
scenario, `spike-heavy-to-stable`, from `static-smooth` to
`static-conservative`.

Because each noise source has its own stream, a segment with spike
probability 0 or jitter 0 no longer shifts the draws of the segments after
it. With Random v1 it did.
