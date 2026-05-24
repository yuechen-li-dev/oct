# FINDINGS: Prometheus Shadow Authority Rake Lab M2

## Question

M2 tests if mixed-mode stress favors recency-weighted gating over cumulative memory for shadow authority diagnostics.

## What M1 proved and what M2 tests next

M1 demonstrated conservative behavior in clean sequences, but left open whether mixed interleavings would hide fast degradation. M2 compares cumulative and EMA side-by-side under production-shaped episodes.

## Cumulative vs EMA interpretation

Cumulative counters are stable but sticky, so they can under-react to sudden degradation and over-remember old misses. EMA is intentionally responsive and typically blocks/reasons earlier when fallback or stale events arrive, then recovers when match streaks return.

## Scenario-by-scenario interpretation

Across the 12 scenarios, fallback and stale windows improved reason clarity: RecentFallback and RecentStale were surfaced directly instead of generic low-confidence labels. Late jitter bursts were attributed to HighArrivalError when EMA error exceeded threshold.

## Rakes found

Boundary scenarios still show occasional chatter pressure, especially when confidence and miss-rate hover near cutoffs. This indicates native constants should not be changed yet.

## Whether EMA confidence should influence native would-act counters

Recommendation: yes, but only after one additional rake pass focused on threshold stability and chatter control in boundary mixes.

## Threshold recommendation

Keep current experimental thresholds (healthy 0.75, canary 0.60, miss 0.20, arrival 2.0) as provisional; do not promote directly to native M10 constants yet.

## What native Prometheus should or should not change next

Do not change dispatch or authority paths now. Next native step should be diagnostic-only would-act shadow counters gated by explicit EMA reason traces, after one more mixed lab.

## Limitations

This is scalar-board Oct simulation only; no native dispatch authority changes and no production telemetry replay in this pass.
