# FINDINGS: Prometheus Shadow Authority Rake Lab M3

## Question

M3 tests whether reason-binding EMA and stronger commit controls can keep recency responsiveness while avoiding boundary over-promotion and chatter.

## Why M3 exists after M2

M2 showed EMA recovery upside, but also specific rakes: BoundaryCanaryConfidence, BoundaryArrivalError, and BurstyRealistic with HEALTHY/RecentStale over-promotion pressure. M3 isolates those threshold/chatter failure modes.

## Policy variants tested

Variants: M2BaselineEMA control; ReasonBindingEMA (RecentFallback/RecentStale/HighArrivalError block HEALTHY eligibility); and ReasonBindingEMA_StrongerCommit (same binding plus stronger hysteresis/min_commit and lower alpha).

## Aggregate result

Reason-binding removed HEALTHY violations under RecentFallback and RecentStale in this lab run while preserving eventual recovery paths. Stronger commit further suppresses chatter in some boundary lanes but can delay promotion.

Recommendation candidate from this run: ReasonBindingEMA_StrongerCommit.

## Scenario-by-scenario interpretation

BoundaryCanaryConfidence and BoundaryArrivalError remain key over-promotion probes; reason-binding dampens premature HEALTHY when caution reasons are active.

BurstyRealistic is the decisive rake: reason-binding prevents HEALTHY while RecentStale or RecentFallback is active, which aligns with diagnostic safety intent.

RecoveryAfterBadPatch and MatchThenFallbackThenRecover still recover to CANARY/HEALTHY under improved variants, so caution binding did not permanently exile recovery.

## Rakes found

Remaining rakes are captured via overPromotionFlag and chatter deltas versus baseline; the lab keeps these visible for native diagnostic planning.

## Did reason binding help?

Yes in this run: it eliminates stale/fallback healthy violations and improves reason-gate alignment near mixed bursts.

## Did chatter improve?

Chatter totals are compared directly across variants. Stronger commit is expected to trade latency for fewer transitions; recommendation depends on measured totals.

## Did recovery remain possible?

Yes: recovery scenarios still reach CANARY/HEALTHY, satisfying the non-permanent-block requirement.

## Threshold / hysteresis recommendation

Use reason-binding as primary policy change candidate for the next diagnostic-only native counter lab. Consider stronger commit only if chatter totals remain above baseline.

## Native Prometheus recommendation

Do not change native dispatch/selector/lease/prestage authority. Do not change native M8/M9/M10 constants yet. If adopted, move only reason-binding logic into the next diagnostic would-act counter lab.

## Limitations

M3 remains an Oct simulation with synthetic sequences and no production authority path. Results are directional and should be validated against native telemetry before any constant changes.
