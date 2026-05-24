# Prometheus Shadow Authority Rake Lab (M1)

## Question

Can a scalar-board Octomata model of P15 M8/M9/M10-style shadow authority diagnostics hold conservative gating under deterministic synthetic scenario stress?

## Why this lab exists

Rake-lab policy shaping before native authority changes; this is simulation evidence only.

## Findings at a glance

| key | value |
| --- | --- |
| scenarioCount | 10 |
| expectedly conservative scenarios | 7/10 |
| scenarios with notable rake behavior | 3/10 |
| policy construct | Octomata `when policy` with `hysteresis: 2`, `min_commit: 2` |
| recommendation | Keep current threshold family for now; add payload-error and recency refinements before native would-act counters |

## Scenario-by-scenario interpretation

### 1) PerfectMatches
- **Observed:** final gate `HEALTHY`, confidence `1.0`, first canary at tick 2, first healthy at tick 4.
- **Interpretation:** baseline calibration path works; healthy promotion is reachable without oscillation.
- **Assessment:** ✅ expected.

### 2) ColdStart
- **Observed:** final gate `UNKNOWN` at exactly 2 samples.
- **Interpretation:** `minSamples = 3` cold-start hold works.
- **Assessment:** ✅ expected and conservative.

### 3) LateJitterSmall
- **Observed:** final gate `HEALTHY` with moderate mean arrival error (`0.25`), with interim blocked/canary time.
- **Interpretation:** gate tolerates mild timing jitter while converging to healthy.
- **Assessment:** ✅ expected.

### 4) LateJitterLarge
- **Observed:** final gate `BLOCKED`, reason `LowConfidence`.
- **Interpretation:** this does block as desired, but the dominant reason is confidence drop, not explicit high-arrival-error gating.
- **Assessment:** ⚠️ **rake signal**: large timing errors should ideally map to `HighArrivalError` more directly.

### 5) PhysicalNotReadyBurst
- **Observed:** final gate `BLOCKED`, reason `HighMissRate`.
- **Interpretation:** burst miss handling is conservative and stable.
- **Assessment:** ✅ expected.

### 6) RecoveryAfterMisses
- **Observed:** final gate `HEALTHY`, recovery time recorded at tick 9.
- **Interpretation:** model supports deterministic recovery after early misses.
- **Assessment:** ✅ expected and useful for future would-act recovery counters.

### 7) FallbackBurst
- **Observed:** final lookahead state `DISABLED`, final gate `BLOCKED`, reason `LookaheadDisabled`.
- **Interpretation:** fallback-to-lookahead-disable behavior is active and sticky enough for safety.
- **Assessment:** ✅ expected (conservative).

### 8) StalePredictions
- **Observed:** final gate `BLOCKED`, reason `HighMissRate`.
- **Interpretation:** stale currently contributes to miss-rate blocking rather than a dedicated stale reason path.
- **Assessment:** ⚠️ **rake signal**: stale should likely carry explicit recency reasoning (`RecentStale`) in a future pass.

### 9) CancelledNoise
- **Observed:** final gate `HEALTHY`, confidence remains high (`0.92`).
- **Interpretation:** cancellation penalty is mild and does not behave as a hard miss.
- **Assessment:** ✅ expected.

### 10) MixedRealistic
- **Observed:** final gate `CANARY_ELIGIBLE`, confidence `0.68`, miss rate `0.0833`.
- **Interpretation:** realistic mixed conditions settle into canary rather than over-promoting.
- **Assessment:** ✅ expected and policy-conservative.

## Cross-scenario patterns

1. **Conservative posture is present.**  
   Scenarios with misses/fallback/stale tend to remain blocked or canary rather than prematurely healthy.

2. **Healthy path is reachable but not trivial.**  
   Perfect and recovery-rich paths can reach healthy, while noisy mixed paths stay canary.

3. **Reason attribution is currently coarse in edge conditions.**  
   Some high-jitter and stale cases resolve to `LowConfidence` or `HighMissRate` instead of reason-specialized outcomes.

4. **Policy stability appears acceptable in this deterministic set.**  
   No obvious chatter path was observed in summary metrics with current hysteresis/min_commit.

## Suggested policy adjustments (lab-phase, not native changes yet)

### Keep now
- Keep `minSamples = 3`, `canaryConfidence = 0.60`, `healthyConfidence = 0.75`, and `maxMissRate = 0.20` for the next lab iteration.
- Keep fallback-driven lookahead disable behavior conservative.

### Improve next (before native would-act counters)
1. **Reintroduce payload arrival error modeling** for `Early(arrivalErrorTicks)` and `Late(arrivalErrorTicks)` so large jitter can produce direct `HighArrivalError` reasoning rather than only confidence collapse.
2. **Add recency windows** for stale/fallback so reason codes can represent `RecentStale` / `RecentFallback` explicitly.
3. **Split reason arbitration from gate code finalization** using an inspectable `when utility`/decision trace lane for postmortem clarity.
4. **Add scenario variants around boundaries** (`confidence≈0.60/0.75`, `missRate≈0.20`, `meanAbsArrivalError≈2.0`) to validate hysteresis edge behavior.

## Readiness judgment

- **Native dispatch authority readiness:** **Not yet.**
- **Native would-act counter readiness:** **Conditionally close**, but only after one additional lab pass that restores payload error modeling and sharpens reason attribution.

## Next step

Run M2 of this rake lab with:
1. payload `Early/Late` error ticks restored,
2. explicit recent-fallback/recent-stale tracking,
3. threshold-boundary sweep scenarios,
4. unchanged native code paths.
