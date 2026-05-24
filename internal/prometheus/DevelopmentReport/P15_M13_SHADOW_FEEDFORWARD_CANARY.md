# P15 M13 Shadow Feedforward Canary (Meaningful Progression)

## Purpose
Wire initial shadow-canary authority enable propagation and bounded lookahead feedback seams for native runtime diagnostics.

## TDD method
- Added Marionette tests first in `reactor_p15_m13_shadow_feedforward_tests.cpp`.
- Confirmed initial failure for enabled propagation (`p15_shadow_canary_enabled` and `p15_shadow_authority_enabled` were both 0).
- Implemented runtime propagation fix and re-ran tests to pass.

## What landed
- Default-off behavior remains unchanged.
- `PrometheusReactorConfig.p15_shadow_canary_enabled` now propagates at runtime-create into:
  - canary params,
  - canary state exported diagnostics,
  - authority enabled exported diagnostics.
- During predictor/canary update, authority gate now mirrors feature-flag enablement.
- Added bounded lookahead-depth feedback application (`1..4`) when canary enabled and authority gate is `HEALTHY`.

## Non-goals preserved
- No default enablement.
- No pre-transfer action.
- No selector rewrite.
- No broad dispatch rewrite.

## Remaining blockers
Full M13 actuator wiring remains incomplete in this pass:
- No dispatch-time matured-reservation consume branch yet.
- No full shape/path validation consume pipeline.
- No llvmpipe SGEMM end-to-end smoke test yet.
- No PRESTAGE_ELIGIBLE reachability wiring change in dispatch path yet.

This pass is **Meaningful progression** per convergence rule: one blocker (authority-enable propagation gap) is removed and validated with native tests; next blockers are isolated above.

## M13b update (reservation maturity + consume seam)

### Tests added first
- Added M13 Marionette tests:
  - `PrometheusP15M13ShadowFeedforward_MatureReservationConsumeOnce`
  - `PrometheusP15M13ShadowFeedforward_MatureReservationShapeVariantMismatchFallsBack`
- These tests target the reservation maturity/consume seam directly (helper-level), and explicitly validate consume-once dedup and mismatch fallback behavior.

### Production changes
- Added `prom_dominatus_reservation_consume_matured(...)` in predictor reservation module (`reactor_dominatus_predictor.c/.h`).
- Behavior:
  - scans matured reservations,
  - matches by `(shape_class, variant_id)`,
  - deterministically chooses earliest `target_tick`,
  - consumes by transitioning reservation state to `PROM_DOM_RESERVATION_YIELDED`.
- Re-consume attempts on the same reservation now fail cleanly (no duplicate consume).

### What this unblocks
- Narrow, reusable consume seam now exists in the reservation module (not ad hoc in SGEMM path).
- This is the minimal primitive required for dispatch-time feedforward branch integration in the next patch.

### Remaining M13 blockers (still open)
- SGEMM dispatch branch is not yet wired to consume matured reservations.
- Runtime diagnostics for feedforward-used/block-reason/counters are still not fully implemented.
- No llvmpipe/software-Vulkan smoke in this patch.
- Full reason-binding + healthy-margin gate interaction at dispatch seam still pending integration.
