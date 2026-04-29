# P13 M14 — Lease Controller Audit Fixes

## 1) Claude audit summary
This pass resolves the M14 pre-actuator audit items: single-SGEMM lease behavior is now execution-gating, mutable function-static fairness state was removed from the judgment engine, and pressure-class inputs are no longer silent zero defaults.

## 2) Single-SGEMM gating audit outcome
Outcome: **A (gated)**.

Single-SGEMM now requests resource lease before slot preparation/dispatch and returns error on deny. This makes deny load-bearing rather than diagnostic-only.

## 3) Fairness race fix
Mutable function-static fairness arrays were removed from `prom_judgment_engine_decide_resource_lease(...)`.
The function now remains deterministic from facts to decision and no longer carries cross-call mutable state.

## 4) Pressure-class wiring or explicit deferral
Outcome: **A (basic conservative wiring)**.

Single-SGEMM lease facts now receive conservative non-zero pressure classes. Unknown/default is moderate (2), with escalation for aggressive recipe variants, register-constrained bands, and large/high-work shapes.

## 5) Diagnostics added/clarified
Lease decisions now reflect real single-call deny/grant semantics in runtime status paths. The policy no longer implies hidden fairness globals.

## 6) Tests added
Added judgment-engine determinism test to verify identical facts produce identical decisions across repeated calls (purity/no hidden mutable state behavior).

## 7) Validation results
Validated using native marionette suites listed in task request.

## 8) Remaining deferred scope
Runtime-owned fairness telemetry/counters are still deferred; this patch intentionally avoids hidden global mutation and keeps the judgment engine pure.

## 9) Readiness for P13 M15 bounded actuator implementation
Ready to proceed: lease controller now has explicit single-SGEMM gating semantics, no mutable static fairness race in decision logic, and non-zero pressure inputs for utility backpressure.

## M14 Follow-up 2 — Single-SGEMM Gate Compatibility

### Root cause observed
`PrometheusReactor_Sgemm` failures were caused by single-SGEMM lease deny at transfer-in stage before slot preparation/dispatch.

The deny was not due to unsafe runtime flags; it was caused by under-scored utility grant facts in the single-call path (slot readiness/attention were left at zero), while conservative pressure classes were non-zero by design.

### Selected fix path
**Path B (facts/defaults too strict)**.

Single-SGEMM lease fact construction now marks the selected work slot as ready + attention when the slot is known and about to be prepared for immediate dispatch. This preserves load-bearing gating while preventing accidental deny of safe baseline SGEMM calls.

### Behavior after fix
- Single-SGEMM lease remains gating (not diagnostic-only).
- Safe baseline single calls can grant and execute.
- Unsafe/failed/invalidated/cap deny behavior remains unchanged in lease engine hard-gates.

## M14 Follow-up 3 — Phase Placement Correction

### Confirmed issue
The readiness/attention under-score was real, but the deeper issue was compute-lease phase placement: compute lease was evaluated in transfer-in instead of near compute submit/dispatch.

### Fix applied
Single-SGEMM compute lease request is now deferred to submit phase, immediately before dispatch gating. Transfer/staging can proceed, then compute lease gates the compute critical section.

### Why pressure classes remain non-zero
Conservative non-zero pressure classes remain intact by design; compatibility is achieved by phase-correct lease placement and proper ready/attention fact timing, not by zeroing pressure.
