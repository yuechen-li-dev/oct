# P15 M4 — Pre-Plan + Reservation Actuator

## References
- `internal/prometheus/P15_M0_PREDICTIVE_LEASE_AHEAD_DESIGN.md`
- `internal/prometheus/P15_M3_FUTURE_LEASE_SEAM_DIAGNOSTIC.md`
- `internal/prometheus/P15_M2_PREDICTOR_STATE_RING.md`

## Reservation state contract
M4 adds a bounded, heap-free reservation actuator in `reactor_dominatus_predictor.{h,c}` via:
- `prom_dominatus_reservation_state`
- `prom_dominatus_reservation_request`
- `prom_dominatus_reservation_state_set`
- `prom_dominatus_reservation_params`
- `prom_dominatus_reservation_decision`

Capacity is fixed (`PROM_DOM_RESERVATION_CAP=16`) with no dynamic allocation.

## Lifecycle states
Reservation lifecycle supports:
- requested (diagnostic input only)
- reserved
- denied
- cancelled
- matured
- expired
- yielded (enum state currently exposed; explicit yield helper deferred)

M4 chooses **deny** for `target_tick <= tick` to keep maturity semantics deterministic and explicit.

## Safety gates
Reservation denies when:
- request invalid
- confidence below threshold
- depth is zero or above configured max depth
- target tick is not future
- target tick exceeds max future tick horizon
- capacity is full
- duplicate active request id

## Predictor / future-lease integration
`prom_dominatus_predictor_try_reserve_future` consumes M3 future lease requests and attempts reservation.
- on reserve success: future-lease seam marked `granted` diagnostically
- on denial: future-lease seam marked `denied` diagnostically

Reservation state remains separate from immediate lease truth.

## Diagnostic-only vs actuator boundary
M4 is the first actuator seam, but still bounded to pre-plan reservation only.
M4 does not:
- mutate immediate `ResourceLease` grant behavior
- submit transfers
- stage GPU work
- alter SGEMM dispatch
- alter variant selection

## Why this is not prefetch
Reservation records a cancellable intent marker only. No execution-side submission/staging is triggered. Maturity marks target-tick reach, not execution permission.

## Tests added
Marionette group: `PrometheusDominatusReservation`
- valid reservation
- safety gates (low confidence/depth-zero/non-future target)
- capacity full deny
- cancel lifecycle
- mature lifecycle
- predictor integration smoke

## Deferred work
- explicit reservation yield helper API (`prom_dominatus_reservation_yield`)
- richer deny reason taxonomy constants
- direct bridge from matured reservation to later M5/M6 corrective/actuation seams

## Validation results
Validated via native build, Marionette suites, and `go test ./...`.

## Inconsistency notes
M3 introduced diagnostic lifecycle states including yielded/expired but deferred explicit helpers. M4 keeps this partial asymmetry: expired helper implemented, yield helper still deferred while enum state exists.
