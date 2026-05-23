# P15 M2 — Native Predictor State + Delayed Prediction Ring

## 1. References
- `internal/prometheus/P15_M0_PREDICTIVE_LEASE_AHEAD_DESIGN.md`
- `Experiments/PrometheusPredictiveLeaseAheadLab/M1/REPORT.md`
- `internal/prometheus/P14_M7_DOMINATUS_FILTERED_EVIDENCE.md`
- `internal/prometheus/P14_M8_SGEMM_FILTERED_EVIDENCE_SMOKE.md`

M2 closes the M1 fidelity gap by implementing a persisted fixed-size delayed prediction ring in native Dominatus code.

## 2. Native state contract
Added Vulkan-free predictor state in:
- `internal/prometheus/native/reactor_dominatus_predictor.h`
- `internal/prometheus/native/reactor_dominatus_predictor.c`

State is bounded and heap-free, with `PROM_DOM_PREDICTION_RING_CAP = 16`.

## 3. Delayed prediction ring behavior
- Ring is fixed-size and circular.
- Issue writes at `(ring_head + ring_count) % cap`.
- Mature consumes from `ring_head` only (deterministic one-entry-per-call maturity).
- Overflow never reallocates; issuance is blocked and stale/fallback diagnostics are raised.

## 4. Depth selection rules
Implemented `prom_dominatus_predictor_select_depth`:
- depth 0 when evidence invalid/warmup/low confidence.
- depth 0 on hard gates: runtime unsafe, invalid slot, depth cap reached, or memory budget fail.
- depth 1 when confidence >= 0.45.
- depth 2 when confidence >= 0.75, low outliers, and no stale/recent miss signal.
- final depth capped by `params.max_lookahead_depth`.

## 5. Issue / mature / update ordering
- `prom_dominatus_predictor_issue`: selects depth, checks gates, checks capacity, issues diagnostic future-lease-requested prediction entry.
- `prom_dominatus_predictor_mature`: matures one deterministic eligible entry (`target_tick <= tick`) from ring head.
- `prom_dominatus_predictor_update`: matures first, then issues.

## 6. Correction behavior
- Correct maturity (`predicted_ready == actual_ready`): confidence increases by `correction_reward` (default `+0.05`, clamped to [0,1]).
- Miss maturity: confidence decreases by `correction_penalty` (default `-0.20`), correction count increments, lookahead depth is reduced, and action is `REDUCE_DEPTH` or `LOWER_CONFIDENCE`.
- Hard-gate-at-maturity marks stale/fallback diagnostics and emits `MARK_STALE` action.

## 7. Future lease state (diagnostic-only)
Prediction entries include `future_lease_state` and set issued entries to `PROM_DOM_FUTURE_LEASE_REQUESTED` for diagnostics.
No real lease mutation/integration is performed in M2.

## 8. Safety gates
Hard gates prevent depth and issuance:
- `runtime_unsafe != 0`
- `slot_valid == 0`
- `memory_budget_ok == 0`
- `outstanding_depth >= outstanding_depth_cap` when cap is non-zero

## 9. Tests added
Added Marionette group `PrometheusDominatusPredictor` with tests for:
- default params
- depth selection cases
- issue behavior
- ring capacity behavior
- mature correct / mature miss correction behavior
- update ordering
- reset behavior

## 10. Deferred runtime lease seam
M2 intentionally does not call or alter real lease behavior, SGEMM dispatch, fallback execution path, or policy actuation.
The module is diagnostic-only and bounded-state only.

## 11. Validation results
Validated with native build and required Marionette/Go test commands listed in this milestone.

## Inconsistency notes
- M1 documented a structural-only `PredictionEntry`/`CorrectionEvent` model without a full persisted per-tick ring; M2 now implements that persisted ring in native code.
- Existing Dominatus modules primarily expose measurement/filter contracts; predictor state is newly introduced and therefore extends native diagnostics surface beyond current filter-only contracts.
