# FINDINGS: Prometheus Shadow Authority Rake Lab M2

## Question

Can a recency-weighted EMA gate distrust quickly, recover quickly, and preserve reason precision under mixed interleavings versus cumulative policy?

## Summary answer

12 scenarios were tested and 12/12 met expected behavior. EMA blocked faster in 0 scenarios, recovered faster in 2 scenarios, showed chatter regression in 5 scenarios, and improved reason precision in 1 scenario. Threshold recommendation: one more lab pass before native would-act counters.

## What M1 proved and what M2 tested

M1 proved conservative behavior in clean sequential flows. M2 stressed mixed interleavings and threshold boundaries, directly comparing cumulative long-memory behavior versus EMA recency behavior.

## Cumulative vs EMA behavior

Cumulative is stable but sticky, while EMA is more responsive and sometimes recovers faster. RecoveryAfterBadPatch shows the recency upside (cumulative ends BLOCKED/HighMissRate; EMA ends HEALTHY with emaTimeToRecover=10). BoundaryCanaryConfidence and BoundaryArrivalError show over-promotion pressure (cumulative CANARY_ELIGIBLE while EMA HEALTHY). BurstyRealistic is the central mixed-stress rake (cumulative BLOCKED/RecentStale versus EMA HEALTHY/RecentStale).

## Scenario-by-scenario interpretation

MatchThenFallbackThenRecover: both end HEALTHY (cumulative confidence 0.95; EMA 0.9682); EMA blocks/recovers later (5/11 vs 1/2), so EMA is not uniformly faster.

HealthyThenLateJitterBurst: both end HEALTHY; HighArrivalError observed; EMA chatter regresses (5 vs cumulative 3).

AlternatingMatchMiss: both end BLOCKED/HighMissRate; no recovery; expected protective behavior under oscillation.

StaleThenMatches: both end HEALTHY; RecentStale observed; EMA recovers faster (3 vs 7).

FallbackSingleSpike: both end HEALTHY; RecentFallback observed; both absorb spike safely.

BoundaryCanaryConfidence: cumulative CANARY_ELIGIBLE (0.68) versus EMA HEALTHY (0.8576), indicating boundary over-promotion risk.

BoundaryHealthyConfidence: both HEALTHY; EMA confidence higher (0.9287 vs 0.86) with lower chatter (2 vs 3).

BoundaryMissRate: both HEALTHY near miss threshold; EMA chatter lower (6 vs cumulative 7), but timing remains boundary-sensitive.

BoundaryArrivalError: cumulative CANARY_ELIGIBLE versus EMA HEALTHY; EMA chatter regression (6 vs 4).

BurstyRealistic: cumulative BLOCKED/RecentStale versus EMA HEALTHY/RecentStale; mixed-stress over-promotion plus chatter regression (6 vs 3).

CancelledStorm: cumulative CANARY_ELIGIBLE versus EMA HEALTHY; low chatter, but EMA remains materially less conservative.

RecoveryAfterBadPatch: cumulative BLOCKED/HighMissRate versus EMA HEALTHY; EMA recovers in 10 events; chatter regression is present (2 vs 1).

## Rakes found

EMA chatter regression count is 5: HealthyThenLateJitterBurst, AlternatingMatchMiss, BoundaryArrivalError, BurstyRealistic, and RecoveryAfterBadPatch. Over-promotion risk appears in BoundaryCanaryConfidence, BoundaryArrivalError, and BurstyRealistic. Reason precision improved in 1 aggregate lane, with RecentFallback and RecentStale helping diagnosis, but arrival-error boundary distinctions still need sharpening.

## Threshold recommendation

Do not change thresholds yet. Current values are plausible, but boundary scenarios still show chatter and over-promotion pressure. Next pass should target threshold stability and chatter control before any native constant adoption.

## Native Prometheus recommendation

Do not change dispatch or native authority. Do not enable prestage/pretransfer. Do not add native would-act counters yet unless they are diagnostic-only and explicitly gated by EMA reason traces. Recommended next step: one more rake lab pass focused on threshold/chatter stability, then reevaluate native would-act counters.

## Limitations

Oct simulation only, synthetic event sequences, no native telemetry replay, no GPU/Vulkan involvement, and no production authority path. These results are policy-shaping evidence, not deployment proof.
