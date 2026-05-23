# P15 M6 — Pre-Stage Diagnostic Scaffold (Native)

## 1) M6a lab reference
This scaffold follows the gate-first recommendation from `Experiments/PrometheusPredictiveLeaseAheadLab/M6a/REPORT.md`: strict Dominatus gating, diagnostics visibility, and pre-stage action default-off.

## 2) Pre-stage state contract
Native pre-stage scaffold introduces:
- `PROM_DOM_PRESTAGE_NONE`
- `PROM_DOM_PRESTAGE_ELIGIBLE`
- `PROM_DOM_PRESTAGE_BLOCKED`
- `PROM_DOM_PRESTAGE_SUBMITTED`
- `PROM_DOM_PRESTAGE_READY`
- `PROM_DOM_PRESTAGE_CANCELLED`
- `PROM_DOM_PRESTAGE_EXPIRED`
- `PROM_DOM_PRESTAGE_WASTED`
- `PROM_DOM_PRESTAGE_MATURED`

M6 behavior:
- Default path with passing gates and `action_enabled = 0` returns `ELIGIBLE` with `FEATURE_DISABLED` reason set.
- `SUBMITTED` is diagnostic-only and only produced when `action_enabled = 1`.
- `READY/CANCELLED/EXPIRED/WASTED/MATURED` are intentionally deferred for later milestones.

## 3) Params and defaults
`prom_dominatus_prestage_default_params()` defaults:
- `action_enabled = 0`
- `confidence_threshold = 0.75`
- `recent_miss_window = 5`
- `max_lead_ticks = 2`
- `cost_estimate_low = 0.10`
- `cost_estimate_medium = 0.25`

## 4) Gate list and semantics
Implemented block reasons:
- confidence below threshold
- warmup active
- reservation not RESERVED
- recent miss window active (`recent_miss_count > 0` when window configured)
- hard safety gate active (`runtime_unsafe != 0`, `slot_valid == 0`, `memory_budget_ok == 0`)
- resource pressure (`outstanding_depth >= outstanding_depth_cap` when cap nonzero, or `resource_pressure_low == 0`)
- lead-time invalid (`target_tick <= current_tick`) or too far (`lead > max_lead_ticks`)
- feature disabled
- invalid input

## 5) Action default-off policy
Even when all hard gates pass, pre-stage remains non-actuating by default:
- `allowed = 1`
- `submitted = 0`
- no transfer/dispatch/resource side effects

Only when `action_enabled = 1` does decision report diagnostic `SUBMITTED=1`, still without real GPU submission in M6.

## 6) Diagnostic output fields
`prom_dominatus_prestage_decision` exposes:
- validity and state
- `allowed`, `submitted`, lifecycle flags
- `block_reasons` bitset
- `request_id`, `target_tick`, `lead_ticks`
- `confidence`, `cost_estimate`, `benefit_estimate`

## 7) Tests added
Marionette suite `PrometheusDominatusPreStage` verifies:
1. safe defaults
2. all gates pass + feature disabled
3. confidence block
4. warmup block
5. reservation block
6. recent miss block
7. hard safety block
8. resource pressure block
9. lead-time block (non-future and too-far)
10. action-enabled diagnostic submit
11. multiple reasons accumulate
12. no lease/reservation mutation side effects

## 8) Intentionally not implemented in M6
Deferred deliberately:
- transfer submission
- buffer staging
- command buffer submission
- SGEMM dispatch/variant changes
- ResourceLease behavior changes
- real prefetch/pre-transfer operations
- hardware-timing integration and tuning

## 9) Validation results
Validated through native build + targeted and regression Marionette suites + `go test ./...` (see command log in task report).
