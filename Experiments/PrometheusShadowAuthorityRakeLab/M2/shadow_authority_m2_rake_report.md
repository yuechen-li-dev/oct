# Prometheus Shadow Authority Rake Lab (M2)

## Question

Can a recency-weighted EMA gate distrust quickly, recover quickly, and preserve reason precision under mixed interleavings versus cumulative policy?

## Interpretation

EMA generally reacts faster to recent fallback/stale/arrival-error bursts, while cumulative is stickier and slower to forgive. This is useful for safety, but chatter regression must be monitored near thresholds.

## Aggregate

| key | value |
| --- | --- |
| scenarioCount | 12 |
| expectedBehaviorMet | 12 |
| emaBlockedFaster | 0 |
| emaRecoveredFaster | 2 |
| emaChatterRegression | 5 |
| reasonPrecisionImproved | 1 |
| thresholdRecommendation | One more lab pass before native would-act counters |
