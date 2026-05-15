# P15 M5 — Correction-Event Integration with Reservation Cancellation

## References
- `internal/prometheus/P15_M2_PREDICTOR_STATE_RING.md`
- `internal/prometheus/P15_M3_FUTURE_LEASE_SEAM_DIAGNOSTIC.md`
- `internal/prometheus/P15_M4_PREPLAN_RESERVATION_ACTUATOR.md`

## Mapping implemented
M5 integrates prediction correction events with reservation lifecycle through:
- `prom_dominatus_predictor_apply_correction_to_reservation(...)`
- synchronous invocation from `prom_dominatus_predictor_mature(...)`

Deterministic mapping used in M5:
- **Correct maturity** (`state_mismatch=0`): mature associated reservation and future lease diagnostic.
- **Miss at maturity** (`state_mismatch=1` and `tick >= target_tick`): expire associated reservation and mark future lease cancelled.
- **Hard gate at maturity** (`MARK_STALE`/fallback): cancel associated reservation and mark future lease cancelled.
- **No associated reservation** (`lease_request_id=0` or not found): no-op reservation decision, correction still emitted.

## Behavior summary
1. Correct predictions retire entry, increase confidence, and mature the associated reservation/future lease.
2. Mismatches decrement confidence and correction counts as before, and now expire the associated reservation at maturity.
3. Hard gate observations mark stale/fallback and cancel associated reservation.
4. Targeted reconciliation only affects the matching `lease_request_id`; unrelated reservations remain active.

## Diagnostics emitted
M5 keeps correction and reservation truths separate while surfacing:
- correction action (`NONE`, `REDUCE_DEPTH`, `LOWER_CONFIDENCE`, `MARK_STALE`)
- prediction error ticks
- correction count and post-update confidence/depth
- reservation decision validity and transition (`previous_state` → `new_state`)
- cancelled / expired / matured counters
- future lease state transition (`matured` or `cancelled`)
- fallback flags (`fallback_active`, `fallback_reason`)

## Tests added
Added Marionette test group: `PrometheusDominatusPredictorCorrection`
- correct maturity matures reservation
- miss expires reservation
- hard gate cancels reservation
- no reservation no-op
- targeted cancellation only
- confidence recovery after misses

Also retained existing suites for predictor/future lease/reservation/resource lease boundaries.

## ResourceLease unchanged
M5 changes are constrained to predictor/future-lease/reservation seam in native Dominatus predictor module.
No immediate `ResourceLease` grant/deny behavior is modified.

## Deferred actuator scope
Still deferred:
- any execution lease grant path
- SGEMM dispatch/variant changes
- transfer/GPU submission paths
- P14 policy changes

## Validation
Validated with native stub build, predictor/future-lease/reservation/predictor-correction groups, ResourceLease, SGEMM reactor tests, and `go test ./...`.
