# P13 M13 — Hybrid Lease Utility Policy Implementation

## 1) M50 handoff summary
M13 implements the M50 hybrid policy in runtime lease judgment: hard gates first, utility scoring second, deterministic reason-coded outputs, and diagnostics-safe integration.

## 2) hard gate implementation
Implemented hard-deny for unsafe runtime, slot failed, slot invalidated, and outstanding depth cap. Added explicit no-yield-without-held-lease gate and hard-yield only when a held lease is present.

## 3) utility scoring design
Replaced simple pressure deny with deterministic score competition:
- `grant_score`
- `backpressure_score`
- `lookahead_score`

Decision order: hard gates first; then grant if highest otherwise backpressure.

## 4) feedforward priors
Utility bias now consumes feedforward occupancy priors:
- device band (register constrained / compute rich)
- shape class latency/compute tilt
- selected recipe variant pressure profile

## 5) fairness mechanism
Added minimal per-worker fairness skew via bounded bucketed request/grant counters. Over-served workers are penalized; under-served workers receive grant-score relief.

## 6) lookahead policy
Lookahead is allowed only when latency-dominant signal is present, request is active, and hard lookahead blocks are clear (lookahead limit, safety masks, transfer availability constraints).

## 7) reason codes
Adopted deterministic hybrid reason codes:
- `hard_deny_safety_or_cap`
- `hard_yield_critical_section_complete`
- `no_yield_without_held_lease`
- `hard_block_lookahead_limit_or_transfer`
- `utility_grant_ready_and_safe`
- `utility_backpressure_pressure_or_contention`
- `utility_backpressure_default`
- `utility_allow_lookahead_latency_dominant`

## 8) integration points
Lease policy replacement is applied at:
- `prom_judgment_engine_decide_resource_lease(...)`
- existing batch lease decision helper call path (no structural scheduler rewrite)

## 9) tests
Added focused Marionette lease utility coverage (`P13_M13`) for:
- hard-gate override,
- safe grant,
- pressure backpressure,
- lookahead allow/block diagnostics,
- fairness skew sanity,
- yield held-lease invariant.

## 10) unchanged behavior
No changes to HFSM topology, kernel dispatch switching, kernel implementations, or prefetch execution.

## 11) deferred scope
Still deferred:
- kernel variants rollout,
- dispatch actuation,
- prefetch execution,
- autotune,
- response-surface fitting,
- performance claims.

## Consistency notes
Inconsistency surfaced: legacy lease deny reasons were specific (`DENIED_SLOT_FAILED`, etc.), while M13 policy reasons are consolidated/hybrid-oriented. Compatibility aliases are retained for non-M13 call sites/tests.

## 12) final clean validation
Validated on April 29, 2026 (UTC) with the required command sequence:

- `bash internal/prometheus/native/build_stub.sh`
- `out/prometheus/native/marionette_tests P13_M13`
- `out/prometheus/native/marionette_tests ResourceLease`
- `out/prometheus/native/marionette_tests P11_M20`
- `out/prometheus/native/marionette_tests PrometheusReactor_Sgemm`
- `out/prometheus/native/marionette_tests`

Results:
- `P13_M13`: pass
- `ResourceLease`: pass (with expected backend skip for single-SGEMM lease diagnostics path)
- `P11_M20`: pass (with expected lane-lifecycle backend skip)
- `PrometheusReactor_Sgemm`: pass
- Full Marionette suite: `237` total, `220` passed, `17` skipped, `0` failed.

Reason-code / legacy compatibility resolution (verified clean in this run):
- `decision.detail` carries deterministic M13 utility/hard-policy reason codes (grant, backpressure, lookahead, and hard-gate diagnostics).
- `decision.deny_reason` preserves legacy deny semantics for compatibility-sensitive consumers/tests.
- Backpressure outcomes preserve legacy deny compatibility (`DENIED_RESOURCE_PRESSURE`) while exposing M13 utility specificity in `decision.detail` (pressure/contention vs default).
- Hard-deny outcomes preserve legacy deny compatibility (`DENIED_SLOT_FAILED`, `DENIED_SLOT_INVALIDATED`, `DENIED_UNSAFE_RUNTIME`, `DENIED_OUTSTANDING_LIMIT`) while `decision.detail` carries the M13 hard-gate diagnostic code.
- No policy-semantic changes were required in this follow-up; validation confirms mapping/diagnostic coherence.
