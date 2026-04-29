# P10 M13 — Dirty-Key Selector Decision Cache

## 1) Selector-slice audit (required pre-coding summary)

Audited slices before implementation:

- M6/M5 M35 migration + visible projection in `reactor_dominatus_sgemm_adapter.c` (`m35_dependency_mask_last_commit`, `prom_dom_sgemm_build_buffering_selector_facts_from_visible`).
- M7 transfer-policy migration + visible projection in `reactor_dominatus_sgemm_adapter.c` (`transfer_queue_dependency_mask_last_commit`, `prom_dom_sgemm_build_transfer_queue_facts_from_visible`).
- Runtime commit/projection invocation points in `reactor_vulkan.c`.
- Last-commit dirty key semantics in Dominatus blackboard (`prom_dom_dirty_key_last_commit`, staged-vs-visible commit boundaries).
- Existing adapter/runtime tests in `reactor_dominatus_sgemm_adapter_tests.cpp`, `reactor_m35_buffering_selector_tests.cpp`, and `reactor_m31_transfer_queue_tests.cpp`.

Explicit audit deliverable:

1. **Selectors cached in M13**
   - M35 buffering selector (`prom_judgment_engine_select_buffering_mode(...)`).
   - Transfer-queue policy selector (runtime transfer-policy decision synthesis from visible transfer projection + selected path gate).
2. **Dependency keys used**
   - M35 dependency mask bits map to:
     - `PROM_DOM_KEY_MEMORY_BUDGET`
     - `PROM_DOM_KEY_MEMORY_M35_REQUIRED_FIXED`
     - `PROM_DOM_KEY_MEMORY_M35_REQUIRED_PULL_LAG`
     - `PROM_DOM_KEY_MEMORY_M35_REQUIRED_SERIAL`
     - `PROM_DOM_KEY_MEMORY_M35_FIXED_HEADROOM`
     - `PROM_DOM_KEY_MEMORY_M35_PULL_LAG_HEADROOM`
     - `PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM`
     - `PROM_DOM_KEY_SGEMM_M35_TRANSFER_VARIANCE_CLASS`
     - `PROM_DOM_KEY_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS`
     - `PROM_DOM_KEY_SGEMM_M35_STARVATION_RISK_HIGH`
     - `PROM_DOM_KEY_SGEMM_M35_PULL_LAG_WIP_WASTE_EXCEEDED`
     - `PROM_DOM_KEY_SGEMM_M35_FALLBACK_AVAILABLE`
   - Transfer dependency mask bits map to:
     - `PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE`
     - `PROM_DOM_KEY_QUEUE_TRANSFER_FAMILY`
     - `PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY`
     - `PROM_DOM_KEY_QUEUE_FAMILIES_DIFFER`
     - `PROM_DOM_KEY_QUEUE_TRANSFER_SUPPORTED`
     - `PROM_DOM_KEY_QUEUE_TRANSFER_DISABLED_BY_CONFIG`
     - `PROM_DOM_KEY_QUEUE_TRANSFER_WORKLOAD_LARGE_ENOUGH`
     - `PROM_DOM_KEY_QUEUE_TRANSFER_SYNC_OWNERSHIP_SUPPORTED`
     - `PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_AVAILABLE`
     - `PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_ONLY_ELIGIBLE`
     - `PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_READBACK_SUPPORTED`
3. **Why these are safe first targets**
   - Both are deterministic selectors over bounded fact sets already projected from Dominatus visible snapshot.
   - Both already expose dependency dirty masks at projection time.
   - Both already stage decision outputs through Dominatus commit boundaries, so cached reuse can preserve downstream visibility semantics.
4. **Selectors intentionally not cached yet**
   - path/compute selector, layout/precision selector, async lifecycle decisions, slot readiness/lifecycle caches.
5. **How stale reuse is prevented**
   - Reuse requires: cache valid + projection from visible snapshot + dependent dirty mask last commit == 0 + cache enabled.
   - Recompute occurs when dirty dependency mask is non-zero, cache invalid, projection fallback path is used, or selector cache is disabled.
   - Changed visible generation alone does not force recompute.

## 2) Cache structs/fields

Added small explicit per-runtime caches:

- `prom_selector_cache_m35`
- `prom_selector_cache_transfer`

Both include:

- `valid`
- `visible_generation_when_computed`
- `dependency_mask`
- `last_dirty_dependency_mask`
- `reuse_count`
- `recompute_count`
- `invalidation_count`
- `last_decision_reused`
- selector-specific cached decision payload

No generic dynamic cache framework was introduced.

## 3) Reuse/recompute rules implemented

### Reuse requires

- cache enabled (`PROM_TESTCFG_DISABLE_SELECTOR_CACHE` not set)
- cache currently valid
- projection built from visible snapshot
- `dependent_dirty_key_mask_last_commit == 0`
- selector-specific completeness guard:
  - transfer selector also requires selected-path gate equality with cached path input

### Recompute occurs when

- cache invalid
- projection not from visible snapshot
- dependency dirty mask non-zero
- cache explicitly disabled by test flag
- transfer selector path-input gate changed

### Generation handling

- visible generation is recorded for diagnostics
- generation changes do **not** trigger recompute by themselves

## 4) Runtime integration flow

For both cached selectors, runtime flow is:

1. stage facts
2. commit
3. build visible projection + dependency dirty mask
4. decide reuse vs recompute
5. stage decision (cached or fresh)
6. commit decision

Decision outputs are still staged/committed even on reuse, preserving diagnostics/export coherence.

## 5) Diagnostics added

Added M13 diagnostics fields on `PrometheusSgemmPolicyDiagnostics` for both M35 and transfer caches:

- cache enabled
- cache valid
- reuse count
- recompute count
- invalidation count
- last dirty dependency mask
- last visible generation
- last decision reused flag

## 6) Test/config control

Added `PROM_TESTCFG_DISABLE_SELECTOR_CACHE` to force recompute and disable reuse for parity checks.

## 7) Tests added

Focused runtime integration coverage was added:

- `PrometheusReactor_M35_DirtyKeySelectorCache_ReusesAndCanBeDisabled`
- `PrometheusReactor_M31_TransferPolicyDirtyKeyCache_ReusesAndInvalidatesOnDependencyChange`

Coverage includes first recompute, unchanged-key reuse, dependency-change recompute/invalidation, and cache-disable behavior.

## 8) Deferred scope (explicit)

Still deferred:

- full SGEMM path/compute selector caching
- Packed4/FP16 layout-precision selector caching
- async lifecycle caching
- slot readiness caching
- N-slot/work-stealing
- concurrency
- FFT caching
- cross-runtime/global caches

## 9) Known limitations and surfaced inconsistency

- Transfer policy remains coupled to selected-path gating to preserve existing behavior parity with current judgment flow.
- Dominatus transfer facts include `upload_only_policy_eligible` and `upload_readback_supported`; current transfer selector parity logic remains aligned to existing judgment fallback ordering and does not yet differentiate upload+readback policy paths. This is an intentional behavior-preservation limit and should be revisited in future transfer-policy expansion work.
