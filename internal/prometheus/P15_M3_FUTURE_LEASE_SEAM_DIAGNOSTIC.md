# P15 M3 — Future Lease Request Seam (Diagnostic-Only)

## References
- `internal/prometheus/P15_M0_PREDICTIVE_LEASE_AHEAD_DESIGN.md`
- `internal/prometheus/P15_M2_PREDICTOR_STATE_RING.md`

## Contract implemented
M3 adds a diagnostic-only future lease seam around predictor state in `reactor_dominatus_predictor.{h,c}`.
It models request lifecycle transitions without invoking or mutating real `ResourceLease` decisions.

## Lifecycle states
Implemented diagnostics use existing M2 state enum values:
`none`, `requested`, `granted`, `denied`, `cancelled`, `matured`, `yielded`, `expired`.

Implemented helpers in M3:
- init/reset
- request issue
- grant
- deny
- cancel
- mature

Deferred helper APIs:
- explicit expire and yield transition helpers (state values exist; helpers deferred to keep M3 minimal).

## Diagnostic-only boundary
- No call from seam into real lease controller.
- Existing immediate lease path is unchanged.
- SGEMM dispatch, variant selection, fallback behavior, and promotion lifecycle are unchanged.

## Predictor integration
`prom_dominatus_predictor_issue` now calls `prom_dominatus_future_lease_request_issue` when a prediction is actually issued.
The seam-assigned `request_id` is written back into the issued prediction entry (`lease_request_id`) for traceability.

## Diagnostics surfaced
Seam state stores:
- `requested_count`
- `granted_count`
- `denied_count`
- `cancelled_count`
- `matured_count`
- `expired_count`
- `yielded_count`
- `last_request`
- `last_matured`

This provides lifecycle counters and last-state snapshots including target tick, confidence, and reason fields.

## Tests added
Marionette group: `PrometheusDominatusFutureLeaseSeam`
- request from active prediction
- no request for inactive/depth-zero prediction
- grant transition
- deny transition
- cancel transition
- mature transition
- reset behavior
- predictor integration smoke

Also reran:
- `PrometheusDominatusPredictor`
- `ResourceLease`
- `PrometheusReactor_Sgemm`
- `go test ./...`

## Validation summary
M3 validates the future lease seam as observable-only state machine scaffolding.
Real lease reservation/pre-plan actuator remains deferred to P15 M4.
