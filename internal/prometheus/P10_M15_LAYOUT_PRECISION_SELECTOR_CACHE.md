# P10 M15 — Layout/Precision Selector Dirty-Key Cache Extension

## 1) M36/M13/M14 handoff summary

M36 recommended layout/precision as the next selector-cache slice after M13 because Packed4/FP16 facts were already Dominatus-owned (M9) and because M14 established safe per-artifact invalidation contracts for representation transitions.

M15 follows the M13 dirty-key pattern and adds a bounded cache only for the Packed4/FP16 subdecision used by SGEMM selection.

## 2) Selector slice cached

M15 caches only the layout/precision subdecision payload:

- `packed4_selected`
- `packed4_reject_reason`
- `fp16_selected`
- `fp16_reject_reason`

M15 does **not** cache full path/compute (`requested_path`, `selected_path`, `compute_mode`, `final_detail`, winner index/score).

## 3) Dependency keys/masks

### 3.1 Dominatus layout/precision fact keys

- `PROM_DOM_KEY_SGEMM_PACKED4_AVAILABLE`
- `PROM_DOM_KEY_SGEMM_PACKED4_SMALL_SHAPE`
- `PROM_DOM_KEY_SGEMM_PACKED4_PADDING_WASTE_PERMILLE`
- `PROM_DOM_KEY_SGEMM_PACKED4_MODE_BUDGET_PERMILLE`
- `PROM_DOM_KEY_SGEMM_PACKED4_ROW_MAJOR_VALID`
- `PROM_DOM_KEY_SGEMM_PACKED4_TAIL_VALID`
- `PROM_DOM_KEY_SGEMM_FP16_STRICT_FP32`
- `PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_KNOWN`
- `PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_PASS`
- `PROM_DOM_KEY_SGEMM_FP16_HAS_SPECIAL_VALUES`
- `PROM_DOM_KEY_SGEMM_FP16_CAPABILITY_STORAGE`
- `PROM_DOM_KEY_SGEMM_FP16_FALLBACK_AVAILABLE`
- `PROM_DOM_KEY_SGEMM_FP16_UTILITY_SCORE`

### 3.2 Dominatus shape/policy/path guard keys included for M15

- `PROM_DOM_KEY_SGEMM_FACT_SHAPE_M`
- `PROM_DOM_KEY_SGEMM_FACT_SHAPE_N`
- `PROM_DOM_KEY_SGEMM_FACT_SHAPE_K`
- `PROM_DOM_KEY_SGEMM_FACT_WORK_UNITS`
- `PROM_DOM_KEY_SGEMM_FACT_TILED_SHAPE`
- `PROM_DOM_KEY_SGEMM_FACT_POLICY_MODE`
- `PROM_DOM_KEY_SGEMM_FACT_CAN_DIRECT`
- `PROM_DOM_KEY_SGEMM_FACT_ALLOW_FALLBACK`

M15 computes a combined dirty dependency mask by OR-ing:

1. layout/precision projection dependency mask, and
2. relevant path-compute dependency bits for shape/policy guards.

### 3.3 M14 interaction guard

M15 additionally guards reuse with native `m14_layout_precision_invalidation_count` equality at cache-compute boundary.

This is used because there is currently no first-class Dominatus key for M14 layout/precision artifact invalidation count.

## 4) Cache data structures

Added `prom_selector_cache_layout_precision` to runtime with:

- `valid`
- `last_decision_reused`
- `visible_generation_when_computed`
- `dependency_mask`
- `last_dirty_dependency_mask`
- `reuse_count`
- `recompute_count`
- `invalidation_count`
- `layout_precision_invalidation_count_when_computed`
- cached `prom_judgment_layout_precision_decision`

## 5) Reuse/recompute rules

### Reuse requires

- selector cache enabled (`PROM_TESTCFG_DISABLE_SELECTOR_CACHE` not set)
- cache valid
- visible projection available
- combined dependency dirty mask == 0
- M14 invalidation counter unchanged since cached decision compute

### Recompute occurs when

- cache invalid
- projection unavailable
- combined dependency dirty mask != 0
- cache disabled by config
- M14 invalidation counter changed

Visible generation is recorded for diagnostics but does not force recompute by itself.

## 6) M14 buffer invalidation interaction

M14 made layout/precision transitions safe by invalidating incompatible artifacts under explicit A/B/C representation+capacity contracts.

M15 uses two protections against stale representation reuse:

1. dependency dirty keys for shape/policy/fallback/layout/precision facts, and
2. M14 layout/precision invalidation counter guard in cache reuse contract.

This prevents stale cached Packed4/FP16 subdecisions from being reused across representation-invalidation boundaries.

## 7) Diagnostics added

Added M15 diagnostics fields:

- `p10_m15_layout_precision_selector_cache_enabled`
- `p10_m15_layout_precision_selector_cache_valid`
- `p10_m15_layout_precision_selector_reuse_count`
- `p10_m15_layout_precision_selector_recompute_count`
- `p10_m15_layout_precision_selector_invalidation_count`
- `p10_m15_layout_precision_selector_last_dirty_dependency_mask`
- `p10_m15_layout_precision_selector_last_visible_generation`
- `p10_m15_layout_precision_selector_last_decision_reused`

## 8) Tests added

Added focused Marionette tests in:

- `reactor_m15_layout_precision_selector_cache_tests.cpp`

Coverage includes:

- first-run recompute + cache-valid transition
- unchanged facts reuse
- same-value write non-dirty reuse case
- packed4/shape dependency invalidation
- fp16 fact dependency invalidation
- cache disable forcing recompute
- cached payload parity checks via exported packed4/fp16 diagnostics fields

## 9) Selectors intentionally deferred

Still deferred:

- full path/compute selector caching
- async lifecycle caching
- slot readiness caching
- N-slot/work-stealing
- FFT selector caching
- cross-runtime/global caches

## 10) Known limitations

- M15 cache is limited to Packed4/FP16 subdecision reuse and still runs full SGEMM path/compute selection after injecting cached/recomputed subdecision.
- No first-class Dominatus key exists yet for M14 layout/precision invalidation counter; M15 uses a native runtime guard for this dependency.
- Public diagnostics expose subdecision-cache behavior; no additional public API for direct packed4/fp16 reject reason parity was added beyond existing fields.
