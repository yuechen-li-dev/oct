# P14 M5 — Native Dominatus Filter Primitives

## Sources
- `Experiments/PrometheusMeasurementFilteringLab/M3/REPORT.md`
- `Experiments/PrometheusMeasurementFilteringLab/M4/REPORT.md`
- `Experiments/PrometheusMeasurementFilteringLab/M3/m3_selected_implementation_set.octagon`
- `Experiments/PrometheusMeasurementFilteringLab/M4/m4_selected_policy_contract.octagon`

## Implemented filters
- EMA (`alpha` parameter)
- Sliding median (`window` 3/5/9)
- Hysteresis/deadband (`band` parameter)
- Hybrid median+EMA (`window` + `alpha`)

## Deferred filters
- Trimmed mean is deferred in M5. M3 selected initial implementation set did not require trimmed mean.

## API / state / diagnostics contract
- Added generic filter kind enum and param/state/output structs in `reactor_dominatus_filter.h`.
- Added `init`, `reset`, `update`, and parameter constructor helpers.
- Added `valid` output contract for invalid configuration or null-state update attempts.

## Warm transfer semantics
- `prom_dominatus_filter_warm_start` initializes state with prior estimate.
- Warm start sets `initialized=1`, `estimate=prior_estimate`, and keeps `sample_count=0` until first live sample.

## Validation behavior
- Rejects:
  - EMA/HYBRID `alpha <= 0` or `alpha > 1`
  - MEDIAN/HYBRID unsupported window (only 3/5/9)
  - HYSTERESIS `band < 0`
  - unknown kind
- Rejected init sets kind to `PROM_DOM_FILTER_KIND_NONE`.
- update on invalid/null state returns `valid=0` output.

## Tests added
- Marionette test group `PrometheusDominatusFilter` covers:
  - EMA constant input stability
  - EMA fast-vs-slow step response
  - median spike suppression
  - hysteresis hold/update behavior
  - hybrid smoothing+spike suppression
  - reset behavior
  - warm transfer behavior
  - invalid parameter behavior

## Future Judgment Engine integration
- Judgment Engine / policy layer should select `prom_dominatus_filter_params` from facts and call `update` per measurement tick.
- M4 policy constants (`min_commit_ticks`, `switch_margin`, `confidence_threshold`) remain policy-layer concerns and are intentionally not embedded in filter primitives.

## Validation results
- Built and executed native Marionette target set including filter tests.
- Existing ResourceLease and PrometheusReactor_Sgemm test slices were executed to check no regressions.

## M5 Follow-up: warmup + warm-start corrections
- Root cause (warmup): previous implementation used a generic `sample_count < 3` warmup rule for all kinds, which under-reported warmup for windowed filters.
- Root cause (warm-start): previous warm-start only seeded scalar estimate and did not seed median window state for windowed kinds.

### Corrected warmup semantics
- MEDIAN/HYBRID now report warmup based on window readiness: `window_count < params.window`.
- EMA/HYSTERESIS retain warmup behavior based on sample count threshold (`sample_count < 3`).

### Corrected warm-start behavior
- Warm-start for MEDIAN/HYBRID now seeds all window entries with the prior estimate, sets `window_count=params.window`, and `window_index=0`.
- This makes warm transfer truthful and prevents immediate cold-window artifacts after policy handoff.

### Additional tests added
- median window-9 warmup progression until full window.
- hybrid window-5 warmup progression until full window.
- warm-started median spike suppression with immediate non-warmup status.
- warm-started hybrid spike suppression with immediate non-warmup status.
- explicit EMA/hysteresis warmup contract preservation tests.

### Follow-up validation
- Native build and Marionette filter suite rerun.
- Existing ResourceLease and PrometheusReactor_Sgemm slices rerun.
- `go test ./...` rerun.
