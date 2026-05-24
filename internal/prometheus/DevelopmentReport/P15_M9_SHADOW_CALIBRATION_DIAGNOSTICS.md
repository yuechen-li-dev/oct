# P15 M9 — Shadow HFSM calibration / confidence diagnostics

## Purpose
M9 converts individual M8 shadow-vs-physical outcomes into bounded, deterministic calibration diagnostics that quantify recent trustworthiness of the shadow model. This is diagnostics-only and does not change authority.

## Calibration state
Added `prom_dominatus_shadow_calibration_state` with bounded fixed-size fields:
- validity/init flags
- sample and outcome counters (match/miss/early/late/physical-not-ready/cancelled/stale/fallback)
- streak counters
- arrival-error aggregates (total abs, signed sum, max abs, last)
- last mismatch kind
- confidence `[0,1]`
- lookahead diagnostic state
- disabled/caution reason flags
- de-dup composite key (`issued_tick`, `target_tick`, `predicted_ready_tick`)

## Confidence update constants
- initial confidence: `0.50`
- match: `+0.05`
- early/late abs error <= 1 tick: `-0.02`
- early/late abs error > 1 tick: `-0.04`
- physical-not-ready miss: `-0.10`
- stale: `-0.05`
- fallback/hard-gate: `-0.05`
- cancelled: `-0.01`
- fallback/default diagnostic mismatch: `-0.03`
- clamp to `[0.0, 1.0]`

## De-duplication rule
Calibration updates only once per unique matured snapshot key:
- `issued_tick`
- `target_tick`
- `predicted_ready_tick`

Repeated diagnostics export of the same prediction does not increment counters.

## Lookahead diagnostic state
`PROM_SHADOW_LOOKAHEAD_{UNKNOWN,HEALTHY,CAUTION,UNRELIABLE,DISABLED}`

Classification:
- `DISABLED`: fallback observed (`fallback_count > 0`)
- `UNKNOWN`: sample count `< 3`
- `HEALTHY`: confidence `>= 0.75` and miss rate `<= 20%`
- `CAUTION`: confidence `>= 0.45`
- `UNRELIABLE`: otherwise

## SGEMM integration point
After M8 `prom_dominatus_shadow_snapshot_evaluate(...)` in the valid timing P15 path, runtime calls:
- `prom_dominatus_shadow_calibration_update(&rt->p15_shadow_calibration, &rt->p15_last_shadow)`

Invalid timing path still does not run P15 updates, so calibration is not advanced.

## Diagnostics export
`PrometheusSgemmPolicyDiagnostics` now exports separate `p15_shadow_calibration_*` fields and `p15_shadow_lookahead_state`.

## Tests
- Native Marionette calibration unit tests cover defaults, match/miss, arrival error tracking, stale/fallback/cancel semantics, de-duplication, and state classification.
- Existing SGEMM P15 predictive diagnostics test extended to assert calibration clamping and invalid-timing no-advance.

## Explicit non-goals
- no dispatch authority changes
- no selector tuning
- no lease/prestage authority changes
- no production/fallback behavior changes
