# P15 M13 Shadow Feedforward Canary (Reservation Lifecycle Heartbeat + Diagnostics Correctness)

## Claude audit summary (2026-05-24)
- Root cause confirmed: production path created canary/future reservations but did not continuously advance reservation lifecycle.
- `prom_dominatus_reservation_mature(...)` was only reached through the test seed helper.
- `prom_dominatus_reservation_expire_stale(...)` was not called in production timing/dispatch flow.
- Dispatch consume only scanned `MATURED` entries (`prom_dominatus_reservation_consume_matured(...)`), so production `feedforward_used` remained 0 unless tests injected matured reservations.
- Stale `RESERVED` entries could accumulate and eventually block reservation capacity.

## Production heartbeat fix
- Added `prom_dominatus_predictor_advance_reservations(...)` in native predictor layer.
- Helper deterministically advances lifecycle in bounded steps:
  1. expire stale reserved entries,
  2. mature ready reserved entries.
- SGEMM now calls this helper:
  - once on dispatch path before any feedforward consume attempt (earliest safe point),
  - once on valid GPU timing update path after predictor evidence/correction update.

This resolves missing production lifecycle heartbeat without introducing override behavior.

## Expire-stale behavior
- Expiration is now part of production heartbeat; stale `RESERVED` entries are transitioned to `EXPIRED` using existing reservation params (`expiry_slack_ticks`).
- This prevents indefinite accumulation of stale reservations in the fixed-capacity ring.

## Authority enabled propagation hardening
- Added `prom_dominatus_shadow_authority_gate_evaluate_with_enabled(...)`.
- Production SGEMM now evaluates gate with `enabled` passed in directly, eliminating fragile post-evaluation patching.
- Legacy `prom_dominatus_shadow_authority_gate_evaluate(...)` remains as compatibility wrapper (enabled=0).

## Feedforward mode: agree-and-confirm (explicit)
- This pass keeps feedforward in agree-and-confirm mode.
- A matured reservation is consumed only if it matches selected occupancy/judgment shape+variant.
- On mismatch or absence, dispatch falls back to judgment path.
- No override semantics introduced.

## Mismatch diagnostics wired
- Dispatch path now increments mismatch counters when matured reservations exist but cannot be consumed due to disagreement:
  - `p15_shadow_feedforward_shape_mismatch_count`
  - `p15_shadow_feedforward_variant_mismatch_count`
- Existing fallback/no-matured attribution remains intact.

## Tests added
- Reservation heartbeat matures reserved entries through production helper.
- Reservation heartbeat expires stale entries through production helper.
- Authority enabled propagation verified at evaluation time via new API.
- Existing default-off guarantee and consume semantics retained.

## P15 closeout hardening (2026-05-24)
- Canary `evaluation_count` semantics are now explicit: it counts **enabled canary evaluations only**. Disabled-but-valid timing attempts increment `block_disabled_count` and do not increment `evaluation_count`.
- Added sized diagnostics export API:
  - `prometheus_reactor_runtime_sgemm_policy_diagnostics_sized(handle, out, out_size)`
  - performs bounded write by filling full diagnostics and copying only `min(out_size, sizeof(struct))` bytes.
  - existing unsized API remains a compatibility wrapper using full struct size.
- Added tests for truncated diagnostics buffers to ensure sentinel bytes are not overwritten.
- Startup diagnostics visibility remains deterministic: config-time `p15_shadow_canary_enabled` propagates to `p15_shadow_authority_enabled` prior to first dispatch.
- Feedforward remains **agree-and-confirm**:
  - matured reservation must match judgment-selected shape+variant to consume,
  - mismatch increments `p15_shadow_feedforward_variant_mismatch_count`/shape mismatch counters,
  - dispatch falls back to normal judgment; no override mode.

## Deferred items (explicitly not expanded in this pass)
- Real hardware Windows/RTX validation.
- Feedforward override mode (still deferred; agree-and-confirm only).
- Pretransfer/prestage real action path.
- P16 vendor-path split of authority/canary if required.
- HealthyMarginGate one-dispatch lag remains structural (dispatch reads prior post-dispatch canary state).
- Dedup remains not gate-aware by design in this phase.
- `authority_enabled` and `canary_enabled` remain aliased in P15 configuration (intentional for Phase 1).

## Structural note
- HealthyMarginGate and feedforward decision can still exhibit one-dispatch lag relative to newly observed timing due to dispatch-before-next-measurement ordering. This pass intentionally does not invent synthetic timing.
