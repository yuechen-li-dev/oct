# P14 M7 — Dominatus Filtered Evidence Integration

## Sources
- `internal/prometheus/P14_M5_DOMINATUS_FILTER_PRIMITIVES.md`
- `internal/prometheus/P14_M6_DOMINATUS_FILTER_POLICY.md`
- `Experiments/PrometheusMeasurementFilteringLab/M4/REPORT.md`

## M6 policy reference
M7 keeps M6 policy defaults and contract:
- `min_commit_ticks = 8`
- `switch_margin = 0.15`
- `confidence_threshold = 0.45`
- warm transfer on switch

M7 integration now feeds policy updates with computed measurement facts from a real native rolling measurement stream.

## Measurement filter state contract
Added `prom_dominatus_measurement_filter_state` in:
- `internal/prometheus/native/reactor_dominatus_measurement_filter.h`
- `internal/prometheus/native/reactor_dominatus_measurement_filter.c`

State fields include:
- embedded `prom_dominatus_filter_policy_state`
- current `prom_dominatus_measurement_facts`
- bounded raw/filtered rolling buffers (`PROM_DOM_MEASUREMENT_WINDOW_MAX = 16`)
- bounded counters/index flags; no heap allocation

## Facts extraction approach
M7 adds bounded, Vulkan-free facts extraction from recent measurements:
- `sample_count`
- `recent_abs_residual`
- `recent_output_variation`
- `spike_rate_estimate`
- `jitter_estimate`
- `step_change_suspected`
- `drift_suspected`
- `confidence`
- `outlier_count`

Extraction uses:
- rolling mean / normalized variance proxy
- adjacent delta spike and outlier counters
- residual thresholds for step/drift suspicion
- confidence from window maturity + jitter penalty

This is intentionally simple and bounded to support M7 integration without overbuilding classifier logic.

## Filtered evidence output fields
Added `prom_dominatus_filtered_evidence` with preserved truth dimensions:
- `raw_value` (raw evidence)
- `filtered_value` (Dominatus interpreted evidence)
- `residual`
- selected/previous filter kinds
- switch/hold diagnostics (`held_by_min_commit`, `held_by_margin`, `held_by_confidence`, `warm_transferred`)
- confidence and utility diagnostics
- outlier/sample counters

## Truth-separation note
M7 explicitly preserves P13/M17 truth separation discipline:
- raw measurement is always retained
- filtered measurement is emitted as a separate evidence field
- policy recommendations/holds are surfaced separately from measured truth

M7 does not replace existing raw diagnostics streams and does not alter selector/lease behavior.

## Tests added
Added Marionette synthetic stream group `PrometheusDominatusMeasurementFilter`:
- stable stream valid filtered evidence
- spike stream outlier/spike detection and suppression
- step stream suspicion and tracking behavior
- confidence gating visibility
- explicit raw vs filtered truth separation
- reset/reinitialize behavior

## Deferred SGEMM/runtime integration scope
No SGEMM policy coupling was added in M7.
No Vulkan dependency was introduced into Dominatus filter primitives, policy, or measurement facts/filter integration.
SGEMM diagnostic channel integration remains deferred for a later milestone.

## Validation results
Validated by building native reactor tests and running required Marionette slices plus full Go tests.

## Inconsistency notes
No documented inconsistency found between M6 policy contract and M7 implementation scope.
