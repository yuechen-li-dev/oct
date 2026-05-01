# P14 M6 — Native Dominatus Filter Policy State + Synthetic Stream Integration

## Sources
- `Experiments/PrometheusMeasurementFilteringLab/M4/REPORT.md`
- `internal/prometheus/P14_M5_DOMINATUS_FILTER_PRIMITIVES.md`

## Contract added
Implemented native policy contract:

`PolicyState + MeasurementFacts + Measurement -> PolicyState + FilterDecision`

Files:
- `internal/prometheus/native/reactor_dominatus_filter_policy.h`
- `internal/prometheus/native/reactor_dominatus_filter_policy.c`

## Policy state / params / facts / decision
- `prom_dominatus_measurement_facts` captures M4-style synthetic facts.
- `prom_dominatus_filter_policy_params` includes `min_commit_ticks`, `switch_margin`, `confidence_threshold` and default kind routing.
- `prom_dominatus_filter_policy_state` stores current selected filter kind/state, commit window, switch diagnostics fields.
- `prom_dominatus_filter_decision` reports selected/previous kinds, switch/hold reasons, warm-transfer flag, utility values, and filter output.

## Defaults (M4 recommendation)
- `min_commit_ticks = 8`
- `switch_margin = 0.15`
- `confidence_threshold = 0.45`
- warm transfer on switch (enabled by policy update path)

## Utility scoring summary
Deterministic utility scoring uses:
- spike rate and jitter
- step/drift suspicion bits
- confidence
- outlier hint

Regime mapping:
- stable -> hysteresis
- spike-heavy -> median
- mixed hostile -> hybrid median+EMA
- step change -> EMA
- drift -> EMA

## Min-commit behavior
After a successful switch, `min_commit_remaining` is set to `min_commit_ticks`. While positive, alternate better candidates are blocked and `held_by_min_commit=1` is emitted.

## Switch-margin behavior
Switching requires candidate utility to beat current utility by at least `switch_margin`; otherwise `held_by_margin=1`.

## Confidence gating
If confidence is below threshold and current filter is initialized, switching is blocked with `held_by_confidence=1`.

## Warm transfer behavior
Switch path uses `prom_dominatus_filter_warm_start` with prior estimate to seed the new filter and avoid cold jumps.

## Tests added
Marionette group: `PrometheusDominatusFilterPolicy`
- stable selects smooth
- spike-heavy selects robust
- min-commit blocks immediate thrash
- switch-margin blocks weak switch
- low confidence blocks switch
- warm transfer avoids cold jump
- step/drift prefer tracking filter
- reset behavior

## Deferred scope
- No Vulkan integration.
- No Prometheus benchmark wiring.
- No parallel candidate states.
- No tuning against real hardware traces.

## Validation
Validated via native Marionette slices and `go test ./...` (see command log in task output).

## Inconsistency notes
No language-reference inconsistency observed for this C-native work item. No documented disagreement found between M4 recommendations and implemented M6 defaults.
