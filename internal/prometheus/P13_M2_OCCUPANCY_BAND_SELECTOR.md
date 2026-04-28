# P13 M2 — Occupancy Band Selector Seam

## 1) M45 handoff summary

M45 proved that a deterministic feedforward policy (`device_band + shape_class -> variant`) is viable as a first production control law for SGEMM occupancy/WIP selection, including safety clamping and an override seam, without requiring first-pass per-device response-surface commissioning.

Concretely, M45 validated:
- deterministic device-band classification,
- deterministic shape classification,
- deterministic variant mapping,
- constrained-device aggressive-variant suppression,
- richer-device aggressive selection on compatible shapes,
- explicit unknown/fallback and override behaviors.

## 2) Enums/constants added

Added production selector vocabulary in `reactor_judgment_engine.h`:
- device bands:
  - `PROM_OCCUPANCY_DEVICE_BAND_REGISTER_CONSTRAINED`
  - `PROM_OCCUPANCY_DEVICE_BAND_BALANCED`
  - `PROM_OCCUPANCY_DEVICE_BAND_COMPUTE_RICH`
  - `PROM_OCCUPANCY_DEVICE_BAND_MEMORY_RICH`
- shape classes:
  - `PROM_OCCUPANCY_SHAPE_CLASS_SMALL_SQUARE`
  - `PROM_OCCUPANCY_SHAPE_CLASS_MEDIUM_SQUARE`
  - `PROM_OCCUPANCY_SHAPE_CLASS_LARGE_SQUARE`
  - `PROM_OCCUPANCY_SHAPE_CLASS_TALL_SKINNY`
  - `PROM_OCCUPANCY_SHAPE_CLASS_WIDE_SHORT`
  - `PROM_OCCUPANCY_SHAPE_CLASS_K_HEAVY`
  - `PROM_OCCUPANCY_SHAPE_CLASS_ML_FFN_LIKE`
- kernel variants:
  - `PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR`
  - `PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE`
  - `PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE`
  - `PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4`
  - `PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8`
- reason codes:
  - default selection,
  - manual override used,
  - low-register clamp,
  - shared-memory clamp,
  - small-shape clamp,
  - baseline fallback,
  - unknown-device fallback,
  - override rejected.

## 3) Selector facts/decision API

Added explicit facts/decision structs:
- `prom_occupancy_selector_facts`
- `prom_occupancy_selector_decision`

Added deterministic selector API:
- `prom_judgment_engine_select_occupancy_variant(...)`

## 4) Device band classifier

Implements M45-style banding using capability classes:
- derived tolerance from register/workgroup and shared/queue classes,
- compute-vs-memory bias from FP32-vs-bandwidth classes,
- unknown/incomplete facts fall back to balanced band with explicit fallback reason.

## 5) Shape classifier

Implements deterministic shape classing over `m/n/k/work_units`:
- small/medium/large square,
- tall-skinny,
- wide-short,
- K-heavy,
- ML/FFN-like.

Thresholds are intentionally simple and inspectable.

## 6) Variant selector + safety clamps

Implements deterministic mapping `(device_band, shape_class) -> variant`, then applies safety clamps:
- register-constrained and low-register classes clamp aggressive variants,
- low-shared-memory classes clamp aggressive variants,
- small-shape guard clamps over-aggressive variants.

## 7) Manual override seam

Selector supports manual override fields in facts:
- accepted only if it passes the same safety clamp rules,
- sets explicit `override_used` reason when accepted,
- explicit `override_rejected` reason when rejected.

Public end-user override API remains deferred.

## 8) SGEMM/runtime diagnostics integration

M2 integrates selector as diagnostics-first runtime seam:
- builds occupancy facts during SGEMM planning,
- executes occupancy selector,
- stores selector decision into runtime diagnostics,
- exports fields through `PrometheusSgemmPolicyDiagnostics`:
  - `p13_m2_occupancy_device_band`
  - `p13_m2_occupancy_shape_class`
  - `p13_m2_occupancy_selected_variant`
  - `p13_m2_occupancy_unclamped_variant`
  - `p13_m2_occupancy_clamp_reason`
  - `p13_m2_occupancy_override_used`
  - `p13_m2_occupancy_fallback_used`

No SGEMM kernel dispatch switching is performed in this milestone.

## 9) Tests added

Added tests for:
- occupancy determinism,
- register-constrained aggressive clamp behavior,
- compute-rich aggressive selection behavior,
- shape influence on variant mapping,
- unknown-device fallback,
- manual override accepted,
- manual override rejected,
- runtime diagnostics population with SGEMM output invariance check.

## 10) Behavior intentionally unchanged

This milestone does not switch SGEMM execution to occupancy variants. Selection is currently diagnostics-facing. Numerical behavior and kernel path behavior remain intentionally unchanged.

## 11) Deferred scope

Still deferred:
- new Vulkan SGEMM kernels,
- runtime dispatch switch to new variants,
- benchmark/performance claims,
- runtime autotune,
- per-device response-surface profile commissioning,
- exact GPU commissioning pipeline,
- kernel generation,
- public manual override surface.

## M45 consistency note surfaced

M45 documentation records a Language/reference vs experiment implementation gap (enum-typed tags vs string tags in the experiment model). M2 does not alter this gap and keeps it explicitly surfaced.
