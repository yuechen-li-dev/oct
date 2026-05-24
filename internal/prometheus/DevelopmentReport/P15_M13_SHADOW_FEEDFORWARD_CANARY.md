# P15 M13 Shadow Feedforward Canary (Meaningful Progression)

## 2026-05-24 audit snapshot
- SGEMM path still uses normal occupancy/judgment selectors and does not invoke dispatch-time `prom_dominatus_reservation_consume_matured(...)`.
- Reservation consume API remains helper-level only (M13b seam).
- Canary/authority diagnostics export exists; feedforward-specific dispatch diagnostics are still not present.

## Exact unresolved blocker
End-to-end SGEMM dispatch actuator wiring for matured reservation consume/use/fallback is not yet integrated in `reactor_vulkan_sgemm.c`.

## Next patch plan (tests-first)
1. Add Marionette red tests for dispatch-time default-off/fallback/consume-success integration.
2. Insert feedforward seam at SGEMM variant/path-selection point with healthy+reason-binding+margin gate.
3. Add feedforward diagnostics fields/counters for used/block/fallback attribution.
4. Add software Vulkan smoke (run or precise skip reason).
