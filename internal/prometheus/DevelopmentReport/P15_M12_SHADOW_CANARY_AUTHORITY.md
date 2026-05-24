# P15 M12 — Shadow Canary Authority Completion

## What landed in foundation pass
M12 foundation previously added default-off canary params/state/action-kind and helper APIs in the predictor layer.

## Runtime wiring completed
This pass wires M12 into SGEMM runtime after M11 would-act update in the valid timing path and keeps behavior default-off.

## Feature flag/default-off
- Config field: `PrometheusReactorConfig.p15_shadow_canary_enabled`
- Default: `0` (disabled)
- Disabled behavior: no canary reservation attempt; dispatch/selector semantics unchanged.

## HealthyMarginGate + reason-binding
Canary helper remains conservative:
- HEALTHY-only
- confidence >= 0.75 + healthy_margin (default margin 0.05)
- reason-binding requires no fallback/stale signal
- de-dup on issued/target/predicted-ready key

## Action scope
Only uses existing future-lease/pre-plan reservation seam (`prom_dominatus_predictor_try_reserve_future`).
No pre-transfer and no real prestage action is enabled.

## Diagnostics
`PrometheusSgemmPolicyDiagnostics` now exports `p15_shadow_canary_*` fields for validity, enablement, last decision, gating bits, and counters for allowed/applied/blocked/attempt/success/rejected.

## Non-goals preserved
- no production dispatch enablement
- no selector tuning
- no pre-transfer
- no real prestage action
- no default behavior change

## M12b dedicated Marionette/native test matrix and bugfix pass

### Coverage added
- Added `internal/prometheus/native/Marionette/reactor_p15_m12_shadow_canary_tests.cpp` with focused Vulkan-free canary tests for:
  - default-off no-op
  - enabled healthy allow path + de-dup
  - margin block
  - recent stale block
  - lookahead-disabled/recent-fallback class block
  - high arrival error block
  - low confidence block
  - high miss-rate block
  - insufficient samples block
  - diagnostics export visibility under invalid timing path

### Bugs found/fixed by tests
- `prom_dominatus_shadow_canary_should_attempt(...)` previously returned no-attempt for blocked cases without incrementing canary block counters and without consistently resetting last action fields per evaluation.
- Fixed narrowly in predictor layer by:
  - resetting last action fields each evaluation,
  - counting disabled blocks in helper,
  - mapping gate reasons to canary block counters when canary is enabled but blocked.

### Authority status after M12b
- Feature flag exists: `PrometheusReactorConfig.p15_shadow_canary_enabled`.
- Default remains off (`0`), with explicit disabled no-op behavior.
- M12 action remains strictly scoped to future lease/pre-plan reservation seam (`prom_dominatus_predictor_try_reserve_future(...)`).
- No pretransfer behavior enabled.
- No selector/dispatch behavior changes.
