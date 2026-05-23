# P15 M7 — SGEMM Predictive Diagnostics Wiring

## 1. Integration location
Wired P15 diagnostic flow in `internal/prometheus/native/reactor_vulkan_sgemm.c` in the valid GPU-timing update path and SGEMM policy diagnostics export function.

## 2. State added
Added SGEMM runtime persisted fields:
- predictor state (`prom_dominatus_predictor_state`)
- prestage params (`prom_dominatus_prestage_params`, default action disabled)
- last correction/prediction/reservation/prestage snapshots for diagnostic export

State is session-persistent, reset on runtime creation, heap-free, and remains in native Dominatus-safe types.

## 3. Fields exported
Extended `PrometheusSgemmPolicyDiagnostics` with explicit P15 predictor/future-lease/reservation/prestage fields (valid flags + detailed values).

## 4. Update order
On valid timing sample:
1) P14 filter update
2) predictor evidence conversion
3) predictor mature/issue update
4) reservation attempt from future lease seam last request
5) prestage diagnostic gate evaluation
6) SGEMM diagnostics export of all P15 snapshots

## 5. Invalid timing behavior
When timing is invalid/missing, P14 evidence remains invalid and P15 update path does not run, so no new prediction issuance/advancement occurs.

## 6. Truth separation statement
Raw timing, filtered evidence, predictor/correction, future lease state, reservation state, and prestage decision are exported in distinct fields and are not collapsed into selector or lease authority fields.

## 7. No selector/lease behavior change
No SGEMM dispatch, selector recommendation/execution path, immediate lease grant behavior, or fallback lifecycle wiring was changed.

## 8. Tests added
Added Marionette SGEMM tests:
- `PrometheusReactor_Sgemm_P15_PredictiveDiagnostics_FieldsPresentAndDefaultOff`
- `PrometheusReactor_Sgemm_P15_PredictiveDiagnostics_InvalidTimingNoAdvance`

## 9. Deferred actuator enablement
Pre-stage real action remains disabled by default (`action_enabled = 0`). M7 only exposes diagnostics.

## 10. Validation results
Validated with native stub build, required Marionette suites, and Go test sweep.
