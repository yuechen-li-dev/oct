# P14 M8 — SGEMM Diagnostic-Channel Smoke Integration

## Selected timing channel
- Integrated on `p13_m5_last_gpu_duration_ns` (GPU timestamp elapsed duration in nanoseconds) from the existing SGEMM diagnostics path.

## Integration location
- Runtime integration point: `internal/prometheus/native/reactor_vulkan_sgemm.c`.
- State is persisted in runtime state (`prometheus_runtime`) via:
  - `p14_measurement_filter_state`
  - `p14_last_filtered_evidence`
  - `p14_measurement_tick`

## New diagnostics fields
Added to `PrometheusSgemmPolicyDiagnostics`:
- `p14_m8_filter_evidence_valid`
- `p14_m8_raw_gpu_duration_ns`
- `p14_m8_filtered_gpu_duration_ns`
- `p14_m8_filter_residual`
- `p14_m8_filter_confidence`
- `p14_m8_filter_selected_kind`
- `p14_m8_filter_previous_kind`
- `p14_m8_filter_switched`
- `p14_m8_filter_warmup`
- `p14_m8_filter_held_by_min_commit`
- `p14_m8_filter_held_by_margin`
- `p14_m8_filter_held_by_confidence`
- `p14_m8_filter_warm_transferred`
- `p14_m8_filter_sample_count`
- `p14_m8_filter_outlier_count`

## Truth-separation statement
- Raw measurement (`p13_m5_last_gpu_duration_ns`, and duplicated as `p14_m8_raw_gpu_duration_ns`) is preserved.
- Filtered evidence is emitted separately (`p14_m8_filtered_gpu_duration_ns`).
- Policy/filter diagnostics remain explicit in separate fields.
- Selector recommendation/request/execution/lease telemetry is unchanged.

## Update and invalid-measurement behavior
- On valid GPU timing sample: runtime updates Dominatus filter once and records latest evidence snapshot.
- On invalid/missing timing: runtime does not update filter state and marks `p14_m8_filter_evidence_valid=0`.
- Existing fallback/selector/lease behavior remains unchanged.

## Tests added
- `PrometheusReactor_Sgemm_P14_FilteredEvidenceFieldsPresentWhenTimingValid`
- `PrometheusReactor_Sgemm_P14_RawAndFilteredTruthSeparated`
- `PrometheusReactor_Sgemm_P14_FilterStatePersistsAcrossCalls`
- `PrometheusReactor_Sgemm_P14_InvalidTimingDoesNotUpdateFilter`

## Selector / lease behavior unchanged
- No selector path inputs were modified.
- No lease logic inputs were modified.
- M8 only appends diagnostics-channel evidence.

## Deferred scope
- No selector coupling to filtered evidence.
- No lease coupling to filtered evidence.
- No predictor/profiler integration.
- No policy weight tuning from production traces.
