# Prometheus Shadow Authority Rake Lab (M3)

## Question

Can EMA reason arbitration and threshold stability controls reduce boundary chatter and over-promotion while preserving recovery?

## Aggregate

| key | value |
| --- | --- |
| scenarioCount | 8 |
| variantCount | 3 |
| expectedBehaviorMet | 20 |
| rakeSignalCount | 6 |
| chatterBaseline | 33 |
| chatterReasonBinding | 33 |
| chatterStrongerCommit | 30 |
| overPromotionCount | 6 |
| recentStaleHealthyViolationCount | 1 |
| recentFallbackHealthyViolationCount | 0 |
| recommendation | ReasonBindingEMA_StrongerCommit |

Variant-level evidence: baseline chatter=33, reason-binding chatter=33, stronger-commit chatter=30. Over-promotion totals trend down across variants in this run.

See FINDINGS.md for full scenario-by-scenario interpretation and recommendation rationale.
