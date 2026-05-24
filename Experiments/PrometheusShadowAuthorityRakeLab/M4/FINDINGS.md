# FINDINGS: Prometheus Shadow Authority Rake Lab M4

## Question

Can a small HEALTHY boundary guard fix BoundaryCanaryConfidence over-promotion while preserving recovery?

## Why M4 exists after M3

M3 left one known rake: BoundaryCanaryConfidence over-promoted to HEALTHY for all variants.

## Policy variants tested

ReasonBindingEMA_StrongerCommit baseline, HealthyMarginGate, HealthyStreakGate, and optional HealthyMarginAndStreak.

## Aggregate result with exact numbers

Expected behavior met=34/36, rakeSignal=2, boundary over-promotion baseline/margin/streak/both=1/0/1/0.

## BoundaryCanaryConfidence result

Improved variants are evaluated directly by boundaryOverPromotionFlag to verify elimination of final HEALTHY over-promotion.

## Sustained-good recovery result

BoundaryCanaryConfidenceSustainedGood checks whether stronger evidence still reaches HEALTHY for promoted recommendation candidates.

## Chatter result

Chatter is tracked per scenario and variant; oscillation scenario guards against excess transitions.

## Recovery preservation result

RecoveryAfterBadPatch and MatchThenFallbackThenRecover remain required to reach CANARY/HEALTHY after bad intervals.

## Stale/fallback/arrival-error regression check

RecentStale and RecentFallback HEALTHY violations remain explicitly tracked; high-arrival-error observation remains visible.

## Rakes found

Rakes are any boundary over-promotion or stale/fallback healthy contradiction events captured by rakeSignal.

## Recommended native M12 policy

Recommendation from this run: HealthyMarginGate. Keep native authority diagnostic-only until feature-flagged canary path in M12.

## Limitations

Simulation-only evidence from finite scenarios; no native constants, dispatch, selector, lease, or prestage authority changes are proposed here.
