# FINDINGS: Prometheus Shadow Authority Rake Lab M3

## Question
Can reason-binding EMA and stronger commit controls keep recency responsiveness while avoiding boundary over-promotion and chatter?

## Summary answer
M3 tested **8 scenarios** across **3 variants** (24 scenario-variant rows).

- Recommended next candidate from measured aggregates: **ReasonBindingEMA_StrongerCommit**.
- Expected behavior met:
  - M2BaselineEMA: **6/8**
  - ReasonBindingEMA: **7/8**
  - ReasonBindingEMA_StrongerCommit: **7/8**
- Rake signal count:
  - M2BaselineEMA: **3**
  - ReasonBindingEMA: **2**
  - ReasonBindingEMA_StrongerCommit: **1**
- Chatter totals:
  - M2BaselineEMA: **33**
  - ReasonBindingEMA: **33**
  - ReasonBindingEMA_StrongerCommit: **30**
- Over-promotion count:
  - M2BaselineEMA: **3**
  - ReasonBindingEMA: **2**
  - ReasonBindingEMA_StrongerCommit: **1**
- RecentStale/RecentFallback HEALTHY violations:
  - M2BaselineEMA: **1 / 0**
  - ReasonBindingEMA: **0 / 0**
  - ReasonBindingEMA_StrongerCommit: **0 / 0**

Interpretation: reason-binding fixed the decisive stale/fallback HEALTHY violation and reduced over-promotion. StrongerCommit preserved those gains and further reduced aggregate chatter (33 -> 30), at the cost of more conservative final gates in some lanes.

## Why M3 exists after M2
M2 showed EMA recency upside but also boundary over-promotion and chatter risk, especially in boundary confidence/arrival scenarios and BurstyRealistic mixed stress. M3 isolates those risks by testing reason-binding and stronger commit controls while staying fully Oct diagnostic-only (no native authority path changes).

## Policy variants tested
1. **M2BaselineEMA**
   - Definition: M2 EMA control policy.
   - Expected behavior: preserve M2 responsiveness baseline.
   - Risk tested: known boundary over-promotion/chatter pressure.

2. **ReasonBindingEMA**
   - Definition: `RecentFallback`, `RecentStale`, and `HighArrivalError` make HEALTHY ineligible.
   - Expected behavior: remove stale/fallback HEALTHY violations while keeping recovery.
   - Risk tested: whether reason-binding is sufficient without extra commit tightening.

3. **ReasonBindingEMA_StrongerCommit**
   - Definition: ReasonBindingEMA + stronger hysteresis/min_commit and lower alpha.
   - Expected behavior: further reduce chatter/over-promotion.
   - Risk tested: potential over-conservatism and delayed promotion.

## Aggregate results by variant

| Variant | expectedBehaviorMet | rakeSignal | chatterTotal | chatterRegressionVsBaseline | overPromotionCount | recentStaleHealthyViolationCount | recentFallbackHealthyViolationCount | highArrivalErrorReasonCount | healthyCountTotal | canaryCountTotal | blockedCountTotal | disabledCountTotal | unknownCountTotal |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| M2BaselineEMA | 6 | 3 | 33 | 0 (baseline) | 3 | 1 | 0 | 1 | 51 | 25 | 33 | 0 | 0 |
| ReasonBindingEMA | 7 | 2 | 33 | 0 | 2 | 0 | 0 | 1 | 50 | 26 | 33 | 0 | 0 |
| ReasonBindingEMA_StrongerCommit | 7 | 1 | 30 | 1 | 1 | 0 | 0 | 1 | 36 | 38 | 35 | 0 | 0 |

Key readout:
- ReasonBindingEMA improves safety signals (3->2 rake, 3->2 over-promotion, 1->0 stale violation) but does **not** lower total chatter vs baseline.
- StrongerCommit improves safety signal again (2->1 rake, 2->1 over-promotion) and lowers total chatter (33->30), but shifts state mix toward canary/blocked (healthy 50->36, canary 26->38, blocked 33->35).

## Scenario-by-scenario interpretation

### BoundaryCanaryConfidence
- Baseline: final **HEALTHY**, chatter 2, overPromotionFlag true.
- ReasonBindingEMA: final **HEALTHY**, chatter 2, overPromotionFlag true.
- StrongerCommit: final **HEALTHY**, chatter 2, overPromotionFlag true.
- Meaning: all variants still over-promote here; this remains an unresolved boundary rake.

### BoundaryHealthyConfidence
- Baseline: final **HEALTHY**, chatter 2.
- ReasonBindingEMA: final **HEALTHY**, chatter 2.
- StrongerCommit: final **HEALTHY**, chatter 2.
- Meaning: stable healthy lane; no new conservatism cost.

### BoundaryMissRate
- Baseline: final **HEALTHY**, chatter 6, recover 5.
- ReasonBindingEMA: final **HEALTHY**, chatter 6, recover 5.
- StrongerCommit: final **HEALTHY**, chatter 5, recover 6.
- Meaning: StrongerCommit reduces chatter by 1 but delays recovery by 1 event.

### BoundaryArrivalError
- Baseline: final **HEALTHY**, chatter 6, overPromotionFlag true.
- ReasonBindingEMA: final **HEALTHY**, chatter 6, overPromotionFlag true.
- StrongerCommit: final **CANARY_ELIGIBLE**, chatter 3, overPromotionFlag false.
- Meaning: this is where StrongerCommit clearly helps; it suppresses HEALTHY over-promotion and halves chatter.

### BurstyRealistic (decisive stale/fallback rake)
- Baseline: final **HEALTHY / RecentStale**, chatter 6, overPromotionFlag true, recentStaleHealthyViolation true.
- ReasonBindingEMA: final **CANARY_ELIGIBLE / RecentStale**, chatter 6, overPromotionFlag false, no stale violation.
- StrongerCommit: final **BLOCKED / RecentStale**, chatter 6, overPromotionFlag false, no stale violation.
- Meaning: reason-binding is effective for the core stale/fallback safety issue; StrongerCommit is stricter but not chattier here.

### HealthyThenLateJitterBurst
- Baseline: final **HEALTHY**, chatter 5, recover 11.
- ReasonBindingEMA: final **HEALTHY**, chatter 5, recover 11.
- StrongerCommit: final **HEALTHY**, chatter 5, recover 13.
- Meaning: StrongerCommit preserves end state but delays recovery/promotion by 2 events.

### MatchThenFallbackThenRecover
- Baseline: final **HEALTHY**, chatter 4, recover 11.
- ReasonBindingEMA: final **HEALTHY**, chatter 4, recover 11.
- StrongerCommit: final **HEALTHY**, chatter 4, recover 11.
- Meaning: recovery-preservation probe passes for all variants.

### RecoveryAfterBadPatch
- Baseline: final **HEALTHY**, chatter 2, recover 10.
- ReasonBindingEMA: final **HEALTHY**, chatter 2, recover 10.
- StrongerCommit: final **HEALTHY**, chatter 3, recover 10.
- Meaning: eventual recovery preserved; StrongerCommit adds one extra transition.

## Rakes found
1. **Remaining over-promotion**: persists in BoundaryCanaryConfidence for all variants; persists in BoundaryArrivalError for baseline + ReasonBindingEMA; only StrongerCommit clears BoundaryArrivalError.
2. **RecentStale/Fallback HEALTHY violations**: baseline has 1 stale violation (BurstyRealistic), ReasonBinding variants have 0/0.
3. **Chatter profile**:
   - Totals: 33 (baseline), 33 (ReasonBindingEMA), 30 (StrongerCommit).
   - Scenario-level regression vs baseline count: 0 (ReasonBindingEMA), 1 (StrongerCommit; RecoveryAfterBadPatch 2->3).
4. **Conservatism cost**: StrongerCommit increases canary/blocked occupancy totals (canary 26->38, blocked 33->35) and delays some promotion/recovery lanes.
5. **Contradiction check**: no contradiction for recommendation; StrongerCommit is best on aggregate rake/chatter, but BoundaryCanaryConfidence remains unresolved.

## Did reason binding help?
Yes, with concrete gains:
- RecentStale HEALTHY violation: **1 -> 0**.
- RecentFallback HEALTHY violation: **0 -> 0** (no regression).
- Over-promotion: **3 -> 2**.
- Rake signals: **3 -> 2**.
- Recovery preserved in both recovery probes.
- Chatter unchanged in aggregate (**33 -> 33**).

## Did stronger commit help?
Yes, but with tradeoffs:
- Chatter total decreased: **33 -> 30**.
- Over-promotion decreased: **2 -> 1** vs ReasonBindingEMA.
- Rake signals decreased: **2 -> 1** vs ReasonBindingEMA.
- Recovery/promotion latency increased in some lanes (e.g., BoundaryMissRate recover 5->6, HealthyThenLateJitterBurst recover 11->13).
- State mix shifted toward more conservative occupancy (healthy 50->36, canary 26->38, blocked 33->35).

Assessment: extra conservatism appears justified for this M3 objective (chatter + over-promotion reduction), but BoundaryCanaryConfidence remains a tracked rake.

## Native Prometheus recommendation
- **Do not** change dispatch/selector/lease/prestage authority.
- **Do not** enable native authority.
- **Do not** change native M8/M9/M10 constants in this pass.
- Recommended next step: **A + B hybrid**
  1. Use **diagnostic-only native would-act counters** with ReasonBindingEMA-style reason arbitration as baseline instrumentation.
  2. Run one additional focused lab on the remaining BoundaryCanaryConfidence over-promotion rake before considering any constant changes.

Given M3 counts, if a single policy candidate is needed for next diagnostic simulation pass, use **ReasonBindingEMA_StrongerCommit**; if minimizing conservatism is prioritized for first native diagnostic counters, start with **ReasonBindingEMA** and keep StrongerCommit as the follow-up comparator.

## Limitations
- Oct simulation only.
- Synthetic event sequences.
- No native telemetry replay.
- No GPU/Vulkan involvement.
- No production authority path.
- Policy-shaping evidence only, not production-readiness proof.
