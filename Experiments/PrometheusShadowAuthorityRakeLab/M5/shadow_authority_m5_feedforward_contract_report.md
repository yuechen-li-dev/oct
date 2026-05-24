# Prometheus Shadow Authority Rake Lab (M5)

## Question

When a reservation matures, should dispatch consume first or validate first for feedforward actuator use?

## Aggregate

| key | value |
| --- | --- |
| scenarioCount | 16 |
| expectedBehaviorMet | 16 |
| feedforwardUseCount | 4 |
| judgmentFallbackCount | 12 |
| consumedCount | 4 |
| noConsumeOnBlockedCount | 12 |
| contractViolationCount | 0 |

Validate first, then consume only if dispatch source is ShadowReservation.
