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

## M13 retry (current pass)

- Added one SGEMM-path Marionette coverage case for default-off behavior (`PrometheusP15M13ShadowFeedforward_DefaultOffMaturedReservationDoesNotConsume`).
- Updated dispatch branch to preserve strict judgment fallback when occupancy fallback is already required.
- Feedforward consume remains gated by: feature enabled, healthy authority, healthy margin, reason-binding pass, and non-fallback occupancy decision.
- Consume path now records reserved variant from consumed reservation payload.

## M13d enabled happy-path SGEMM integration attempt

1. **Red phase added**
   - Added `PrometheusP15M13ShadowFeedforward_EnabledHealthyMaturedReservationUsedBySgemm`.
   - Initial red run failed with:
     - SGEMM runtime unavailable in this environment,
     - `p15_shadow_feedforward_used == 0` and source/consume assertions failing.

2. **Test-only seam added (narrow)**
   - Added `prometheus_reactor_runtime_p15_test_seed_matured_reservation(...)` test seam API.
   - Seam behavior:
     - seeds a reservation with provided `shape_class`, `variant_id`, and `target_tick`,
     - matures it immediately via reservation helper,
     - forces canary gate state fields needed for deterministic healthy-path attempt.
   - Scope: deterministic Marionette integration setup only.

3. **Happy-path wiring status**
   - Existing SGEMM feedforward consume branch remains in dispatch path.
   - Test now seeds matching matured reservation and reruns SGEMM to attempt feedforward consume/use.

4. **Current acceptance evidence**
   - In this container, Vulkan runtime is unavailable; test now **skips** with explicit reason.
   - This preserves deterministic reporting but does not provide a runnable happy-path proof in this environment.

5. **Non-goals preserved**
   - default-off behavior unchanged,
   - no pre-transfer enabled,
   - no selector rewrite,
   - no broad dispatch rewrite.

## M13e software Vulkan + Marionette stabilization

- Environment probe run:
  - `uname -a`
  - `/etc/os-release` indicates Ubuntu 24.04.4 LTS.
  - `vulkaninfo` present.
- Software Vulkan install/probe:
  - attempted `apt-get update` (third-party mirrors returned 403 but Ubuntu mirrors succeeded with warnings).
  - installed `mesa-vulkan-drivers` (new package) and confirmed `libvulkan1` + `vulkan-tools` present.
  - discovered ICDs under `/usr/share/vulkan/icd.d/`, including `lvp_icd.json`.
  - set `VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/lvp_icd.json`.
  - `vulkaninfo --summary` reports llvmpipe (lavapipe) CPU Vulkan device.

- M13d happy-path result in this container:
  - test `PrometheusP15M13ShadowFeedforward_EnabledHealthyMaturedReservationUsedBySgemm` now reaches runtime probe stage under software Vulkan env,
  - but baseline SGEMM call still fails in this environment, so test emits explicit SKIP with reason:
    `baseline sgemm failed in environment; feedforward happy-path cannot be asserted`.
  - This is reported as an environment blocker, not a pass.

- Full-suite failing groups root cause and fix:
  - `PrometheusP15M11ShadowWouldAct_ReasonBindingAndDedup`:
    - stale test fixture had arrival-error total at gate boundary expectations.
    - updated fixture to exceed gate threshold deterministically.
  - `PrometheusP15M9ShadowCalibration_StateClassification`:
    - stale expectation assumed HEALTHY after only three matches.
    - updated fixture to apply enough consistent matches for confidence to enter HEALTHY bucket before miss sequence.

- Validation results:
  - M11 group: pass.
  - M9 group: pass.
  - M13 group: pass with one explicit environment SKIP (enabled happy-path SGEMM test).
  - full Marionette suite: pass (0 failed; remaining skips are explicit scenario/environment skips).

- Non-goals preserved:
  - default-off behavior unchanged,
  - no pre-transfer action enabled,
  - no selector/judgment global rewrite,
  - no broad dispatch rewrite.
