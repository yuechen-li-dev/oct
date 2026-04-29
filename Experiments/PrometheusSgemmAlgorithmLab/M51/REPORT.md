# P13 M14 / Prometheus SGEMM Algorithm Lab M51 — Bounded Lookahead Actuator Rake + Implementation Plan

## 1. Design stance

Lookahead must move from a diagnostic bit to an actuator because M13 already computes allow/block decisions; without actuation, the controller cannot materially change runtime preparation behavior. Exact optimal prefetch depth is not the M51 target because depth tuning is device/workload specific and can be deferred after safe-path implementation. Hardware benchmarking is not required before first implementation because M51's purpose is risk-bounded actuator selection, gate design, and implementation ordering, not performance claims. Bounded safety dominates overlap potential because stale data, ownership, and cancellation hazards can corrupt correctness. Before native implementation, M51 must prove deterministic recommendation, enforceable hard gates, and a clear rollback-safe first actuator.

## 2. Actuator candidates

Modeled candidates:
- Diagnostic-only.
- Pre-plan.
- Pre-stage host.
- Pre-transfer.
- Reservation-only.
- Hybrid pre-plan + reservation.

## 3. Context models

Modeled contexts include batch-known-future, single-call-unknown-future, S=2 depth cap, transfer queue unavailable, high memory pressure, and shape/layout churn invalidation.

## 4. Safety gates

All evaluations apply hard gates:
- unsafe runtime,
- slot failed,
- slot invalidated,
- outstanding depth cap,
- memory budget exceeded,
- future entry unavailable for data-touching actuators,
- transfer queue unavailable for pre-transfer,
- memory pressure blocks reservation/staging,
- shape/layout churn blocks staging/transfer,
- S=2 depth > 1 blocked.

## 5. Scoring model

Computed product score combines safety, correctness confidence, rollback simplicity, complexity cost, latency hiding potential, batch/single applicability, memory pressure behavior, diagnostics, and extensibility. Heavy penalties are applied to pre-transfer complexity/risk and to any blocked/unsafe actuation path.

## 6. Findings

The batch/S=2 context consistently scores `HybridPrePlanReservation` above diagnostic-only and above pre-transfer. Pre-transfer receives the strongest latency potential but loses on correctness confidence, rollback simplicity, and complexity, especially when queue availability or churn conditions are not ideal.

## 7. Final recommendation

First actuator: **hybrid pre-plan + reservation**, scoped to **batch context with S=2 worker-local slots**. Pre-transfer is deferred.

## 8. Implementation plan for P13 M15

1. Add runtime actuator reason codes mirroring modeled blocks (`future_entry_unavailable`, `transfer_queue_unavailable`, `memory_pressure_backpressure`, `s2_depth_cap_exceeded`, `shape_layout_invalidation_risk`).
2. Implement pre-plan metadata materialization for next batch entry when lookahead is allowed and depth <= 1.
3. Implement reservation-only second-slot lease budgeting (no data movement).
4. Add cancellation path on slot invalidation/churn that drops reserved/preplanned state.
5. Emit diagnostics for allowed/blocked actuation and fallback-to-diagnostic behavior.
6. Keep transfer actuation feature-flagged off in M15.

## 9. Deferred scope

Deferred intentionally:
- pre-transfer actuator,
- single-call future-hint API,
- queue-sync/cancel orchestration for in-flight transfer,
- hardware tuning and overlap benchmarking,
- depth > 1 lookahead (S=4 or cross-worker borrowing).

## Required final answers

1. First lookahead actuator: hybrid pre-plan + reservation.
2. Pre-transfer now: no, defer.
3. Batch-only first target: yes.
4. S=2 constraint: lookahead depth limited to one next slot, no cross-worker borrowing.
5. Memory pressure policy: block reservation/staging actuators and backpressure.
6. Shape/layout invalidation policy: cancel/forbid staging/transfer and recompute plan path later.
7. Diagnostics needed: explicit allow/block reason codes for each hard gate and context gate.
8. P13 M15 should implement: pre-plan + reservation actuator path with cancellation-safe diagnostics.
9. Deferred: transfer actuation, single-call hints, advanced overlap optimizations.

## Inconsistency and documentation-gap note

No direct syntax conflict was found while implementing M51. However, this experiment uses enum-based actuator/context modeling patterns not yet explicitly documented in `Language/reference` as a recommended style for control-rake labs; this should be documented to reduce drift across future milestones.
