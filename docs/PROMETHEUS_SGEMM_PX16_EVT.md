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
