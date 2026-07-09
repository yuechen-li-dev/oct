## Px16 M1

Px16 M1 wires the SGEMM occupancy judgment-engine decision into the production dispatch path. Production `prometheus_reactor_runtime_sgemm()` calls now bind the selector's clamped `selected_variant` as the dispatch variant used for tiled pipeline selection instead of staying pinned to baseline at the public API boundary.

The architecture rule for this milestone is:

- The judgment engine is the sole production dispatch authority for SGEMM occupancy variant selection.
- `occupancy_apply_safety_clamp` remains the live safety gate on that decision.
- P15 feedforward and matured reservations remain prestage, latency-hiding, and telemetry only; they do not override the production dispatch variant.
- Promotion lifecycle fields such as DVT, PVT, production eligibility, and dispatch-enabled remain diagnostic metadata only for this milestone and do not gate the new production dispatch wiring.

Deferred work:

- Hooking up the memory-conservative SPIR-V kernel is intentionally deferred to a later Px16 milestone.

## Px16 M4

Px16 M4 removes the blanket SAFE-mode SGEMM direct-path suppression that previously set `force_direct` solely because the controller was in `PROM_POLICY_MODE_SAFE`. SAFE mode now means guardrails, diagnostics, and concrete hazard fallback rather than automatic slow-path dispatch.

The architecture rule for this milestone is:

- SAFE policy may still reach tiled SGEMM production dispatch when the shape is eligible, the selected occupancy variant is wired, and no concrete hazard requires direct fallback.
- The judgment engine remains the production dispatch authority.
- `occupancy_apply_safety_clamp` remains the live engineering safety gate for occupancy variants.
- Direct fallback remains available for explicit overrides and concrete hazards, with path-level diagnostics exposing force-direct reason and selected path/compute state.
- DVT/PVT/promotion lifecycle fields remain telemetry only.
- P15 mismatch correction remains deferred.
- Selector performance tuning remains deferred.

## Px16 M5

Px16 M5 ports the Shadow Authority rake lab M5 feedforward validation model into the native SGEMM/P15 path. Native P15 reconciliation now compares the matured reserved/prestaged occupancy variant against the live judgment-engine-selected occupancy variant once per real SGEMM call, after the production decision exists.

The architecture rule for this milestone is:

- The judgment engine remains the sole production dispatch authority.
- P15 feedforward remains prestage, latency-hiding, and telemetry only.
- Matching matured reservations record a reconciliation hit and consume once.
- Variant mismatch never overrides dispatch; the live SGEMM call still requests and executes the judgment-engine-selected variant.
- Mismatch now feeds the existing predictor correction/confidence machinery and retires the stale reservation path instead of silently collapsing into a generic no-reservation bucket.
- Native block reasons now follow the Shadow Authority rake lab M5 taxonomy, including `VariantMismatch`, `StaleReservation`, `CancelledReservation`, `AlreadyConsumed`, and `ReservationNotReady`.

Deviation note:

- Native reuse of the existing reservation state machine still materializes stale mismatch cleanup as reservation expiry/cancellation transitions rather than adding a new reservation state enum.

## Px16 M6

Px16 M6 hardens the native SGEMM production diagnostics surface so one post-call snapshot can answer, without inference, what the selector recommended, what safety selected, what dispatch variant/path was requested, what path actually executed, whether direct was forced, and whether P15 agreed with the live decision.

The architecture rule for this milestone is:

- Diagnostics are the witness, not the dispatch authority.
- The judgment engine remains the sole production dispatch authority.
- `occupancy_apply_safety_clamp` remains the live safety gate.
- P15 remains prestage/telemetry/correction only and does not override the live dispatch variant.
- Wired EVT variants remain production-eligible and dispatch-enabled even when DVT/PVT/promotion lifecycle telemetry is false.
- SAFE policy remains hazard/feasibility based; it is not a blanket direct-path override.

Compact truth table:

- `px16_m6_selector_recommended_variant`: the selector recommendation before clamp. In M6 this mirrors `p13_m2_occupancy_unclamped_variant`.
- `px16_m6_selector_selected_variant`: the clamped live selector decision. In M6 this mirrors `p13_m2_occupancy_selected_variant`.
- `px16_m6_requested_dispatch_variant`: the occupancy variant handed to dispatch after selector authority is applied.
- `px16_m6_executed_dispatch_variant`: the occupancy variant identity actually bound by the executed compute mode. If the call ends up on a non-tiled compute path, this reports baseline rather than pretending a tiled occupancy pipeline ran.
- `px16_m6_requested_path` / `px16_m6_selected_path` / `px16_m6_executed_path`: requested, chosen, and actually used Vulkan path identity.
- `px16_m6_requested_compute_mode` / `px16_m6_selected_compute_mode` / `px16_m6_executed_compute_mode`: explicit compute-mode truth. M6 does not model a separate requested compute mode, so the requested field mirrors the selected mode.
- `px16_m6_force_direct_requested`: explicit caller/test-seam direct request.
- `px16_m6_force_direct_applied`: direct was actually forced by either explicit override or concrete fallback/hazard handling.
- `px16_m6_force_direct_reason`: `EXPLICIT_OVERRIDE`, `SAFE_CONCRETE_HAZARD`, or `NONE`. SAFE alone must still report `NONE`.
- `px16_m6_variant_path_status`, `px16_m6_variant_production_eligible`, `px16_m6_variant_dispatch_enabled`: factual EVT wiring/path truth for the requested occupancy variant.
- `px16_m6_variant_dvt_validated`, `px16_m6_variant_pvt_validated`, `px16_m6_variant_lifecycle_telemetry_only`: DVT/PVT/promotion lifecycle fields remain telemetry only and do not gate a wired variant.
- `px16_m6_p15_reservation_present`, `px16_m6_p15_reservation_matured`, `px16_m6_p15_reservation_consumed`: whether P15 had and used a relevant reservation.
- `px16_m6_p15_reserved_variant_id`: the P15 reserved/prestaged occupancy variant.
- `px16_m6_p15_live_selected_variant_id`: the live judgment-engine-selected occupancy variant used for reconciliation.
- `px16_m6_p15_reconciliation_match`: whether P15 matched the live decision.
- `px16_m6_p15_block_reason`, `px16_m6_p15_correction_action`, `px16_m6_p15_reservation_stale_or_expired`: mismatch/block/correction truth when P15 diverges from the live decision.
- `px16_m6_p15_confidence_before` / `px16_m6_p15_confidence_after`: cheap before/after confidence snapshots around reconciliation/correction.
