# P15 M10 — Shadow Authority Canary Diagnostics

## Purpose
M10 adds a deterministic, heap-free, Vulkan-free **diagnostic-only** authority canary gate that evaluates whether shadow calibration health is strong enough for future canary consideration.

## Gate States and Reasons
States:
- `PROM_SHADOW_AUTHORITY_UNKNOWN`
- `PROM_SHADOW_AUTHORITY_BLOCKED`
- `PROM_SHADOW_AUTHORITY_CANARY_ELIGIBLE`
- `PROM_SHADOW_AUTHORITY_HEALTHY`
- `PROM_SHADOW_AUTHORITY_DISABLED`

Reasons:
- `PROM_SHADOW_AUTHORITY_REASON_NONE`
- `PROM_SHADOW_AUTHORITY_REASON_INSUFFICIENT_SAMPLES`
- `PROM_SHADOW_AUTHORITY_REASON_LOW_CONFIDENCE`
- `PROM_SHADOW_AUTHORITY_REASON_HIGH_MISS_RATE`
- `PROM_SHADOW_AUTHORITY_REASON_HIGH_ARRIVAL_ERROR`
- `PROM_SHADOW_AUTHORITY_REASON_LOOKAHEAD_DISABLED`
- `PROM_SHADOW_AUTHORITY_REASON_RECENT_FALLBACK`
- `PROM_SHADOW_AUTHORITY_REASON_RECENT_STALE`
- `PROM_SHADOW_AUTHORITY_REASON_INVALID_CALIBRATION`

## Thresholds / Constants
- `min_samples = 3`
- `min_confidence_for_canary = 0.60`
- `min_confidence_for_healthy = 0.75`
- `max_miss_rate = 0.20`
- `max_mean_abs_arrival_error_ticks = 2.0`

Recommended depth:
- blocked/disabled/unknown => 0
- canary-eligible => 1
- healthy => 1 (or 2 when mean abs error <= 1.0)

## Field Definitions (new SGEMM diagnostics)
- `p15_shadow_authority_valid`
- `p15_shadow_authority_state`
- `p15_shadow_authority_reason`
- `p15_shadow_authority_canary_allowed`
- `p15_shadow_authority_would_act`
- `p15_shadow_authority_enabled` (remains `0` in M10)
- `p15_shadow_authority_recommended_lookahead_depth`
- `p15_shadow_authority_confidence_gate_passed`
- `p15_shadow_authority_sample_gate_passed`
- `p15_shadow_authority_miss_rate_gate_passed`
- `p15_shadow_authority_arrival_error_gate_passed`
- `p15_shadow_authority_lookahead_gate_passed`
- `p15_shadow_authority_match_rate`
- `p15_shadow_authority_miss_rate`
- `p15_shadow_authority_mean_abs_arrival_error_ticks`

## Integration Point
SGEMM valid-timing path order is:
1. P14 filter update
2. predictor evidence conversion
3. predictor mature/issue update
4. reservation attempt from future lease seam
5. prestage diagnostic evaluation
6. M8 shadow snapshot evaluation
7. M9 shadow calibration update
8. M10 shadow authority gate evaluation
9. diagnostics export

Invalid timing does not advance calibration and therefore cannot create synthetic eligibility.

## Tests
- New Marionette unit coverage for defaults/healthy/canary/blocked reasons/lookahead disabled/recent stale.
- SGEMM diagnostic tests assert authority stays off and invalid timing does not report canary eligibility.

## Explicit Non-goals
- No dispatch authority changes
- No selector tuning changes
- No immediate lease/reservation/prestage authority changes
- No production behavior changes
- No actuator enablement for prestage/pretransfer
