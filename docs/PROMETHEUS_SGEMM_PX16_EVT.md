## Px16 M1

Px16 M1 wires the SGEMM occupancy judgment-engine decision into the production dispatch path. Production `prometheus_reactor_runtime_sgemm()` calls now bind the selector's clamped `selected_variant` as the dispatch variant used for tiled pipeline selection instead of staying pinned to baseline at the public API boundary.

The architecture rule for this milestone is:

- The judgment engine is the sole production dispatch authority for SGEMM occupancy variant selection.
- `occupancy_apply_safety_clamp` remains the live safety gate on that decision.
- P15 feedforward and matured reservations remain prestage, latency-hiding, and telemetry only; they do not override the production dispatch variant.
- Promotion lifecycle fields such as DVT, PVT, production eligibility, and dispatch-enabled remain diagnostic metadata only for this milestone and do not gate the new production dispatch wiring.

Deferred work:

- Hooking up the memory-conservative SPIR-V kernel is intentionally deferred to a later Px16 milestone.
- SAFE-mode force-direct policy relaxation is intentionally deferred.
