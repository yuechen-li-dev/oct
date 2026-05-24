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
