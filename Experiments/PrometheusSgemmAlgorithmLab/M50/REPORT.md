# P13 M12 / Prometheus SGEMM Algorithm Lab M50 — Resource-Lease Utility Policy Rake Lab

## 1) M49/M9/M10/M11 handoff summary
M49 established hybrid control (feedforward recipe + runtime lease controller). M9 defined lease seams and reasons. M10 integrated request/grant/deny/execute/yield flow. M11 hardened lease invariants (no execute without grant, bounded depth, correct yield semantics).

## 2) Model shape
Hybrid model: hard-gated HFSM lifecycle decisions with deterministic utility scoring for grant/backpressure/lookahead.

## 3) Policy inputs
Modeled slot state, failed/invalidated/unsafe flags, held lease, readiness, outstanding/max depth, feedforward recipe and pressure classes, transfer/multi-queue/lookahead facts, contention, latency dominance, memory pressure, and fairness skew.

## 4) Hard gates
Hard deny on unsafe/failed/invalidated/depth at hard cap. Hard yield only with held lease + critical section complete. No yield without held lease. Lookahead hard block at lookahead cap, unsafe/failed/invalidated, and transfer-prefetch without transfer overlap.

## 5) Utility scoring
Utility computes grant/backpressure/lookahead scores from readiness, pressure penalties, contention, latency dominance, transfer availability, and feedforward priors (device band/shape/pipeline characteristics).

## 6) Candidate policies
A conservative-hard-gate, aggressive-lookahead, balanced-utility, fairness-biased, and hybrid-feedforward-utility policies are evaluated from same scenario catalog.

## 7) Scenario coverage
Model includes safe-ready, failed, latency+transfer, high contention + high memory pressure (degraded), and starvation-risk scenarios. (Documentation gap: broader 14-scenario matrix is partially represented; remaining scenarios are deferred to M13 extension pass.)

## 8) Findings
Aggressive policy improves local throughput proxy but incurs elevated overgrant and occupancy risk under pressure. Conservative policy is safe but underutilizes capacity. Fairness policy reduces starvation but can over-throttle. Hybrid policy produced strongest robust aggregate with deterministic reason-coded decisions.

## 9) Final recommendation
Implement **hybrid-feedforward-utility** for P13 M13.

## 10) P13 M13 implementation contract
1. Preserve hard gates exactly.
2. Utility-score non-hard paths: grant/backpressure/lookahead.
3. Apply feedforward priors to utility (device band, shape class, recipe pressure profile).
4. Clamp depth via hard max + lookahead limit.
5. Bound lookahead to latency-dominant + transfer-capable + below cap.
6. Add fairness skew term to prevent starvation.
7. Emit deterministic reason codes:
   - `hard_deny_safety_or_cap`
   - `hard_yield_critical_section_complete`
   - `no_yield_without_held_lease`
   - `hard_block_lookahead_limit_or_transfer`
   - `utility_grant_ready_and_safe`
   - `utility_backpressure_pressure_or_contention`
   - `utility_backpressure_default`
   - `utility_allow_lookahead_latency_dominant`

## 11) Deferred scope
No native runtime actuation changes, no kernel changes, no hardware benchmarking, no scheduler redesign. Expand full 14-scenario matrix and richer fairness metrics in M13 follow-up.
