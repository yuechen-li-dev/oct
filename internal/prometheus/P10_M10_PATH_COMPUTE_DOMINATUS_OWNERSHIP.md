# P10 M10 — Path/Compute Dominatus Ownership

## 1) Migration scope

This slice migrates the **core SGEMM path/compute selector facts and decision outputs** from legacy in-function state to Dominatus-owned staged/visible keys via `reactor_dominatus_sgemm_adapter`.

Non-goals remained unchanged: no policy rewrite, no Vulkan execution changes, no transfer-policy semantics changes, no Packed4/FP16 rule changes, no async ownership migration.

## 2) Facts migrated

Migrated into Dominatus keys and adapter APIs:

- `m`, `n`, `k`
- `work_units`
- `can_stage`, `can_direct`
- `allow_fallback`
- `readback_required`
- `force_direct`, `force_staged`, `force_tiled`
- `tiled_shape`
- `software_vulkan`
- `policy_mode`

Adapter APIs:

- `prom_dom_sgemm_stage_path_compute_facts(...)`
- `prom_dom_sgemm_build_path_compute_facts_from_visible(...)`

## 3) Decision outputs migrated

Migrated into Dominatus keys and adapter APIs:

- `success`
- `error_detail`
- `requested_path`
- `selected_path`
- `compute_mode`
- `final_detail`
- `used_fallback_to_direct`
- `winning_candidate_index`
- `winning_score`

Adapter APIs:

- `prom_dom_sgemm_stage_path_compute_decision(...)`
- `prom_dom_sgemm_read_visible_path_compute_diagnostics(...)`

## 4) Relationship to M7/M8/M9 sub-slices

M10 explicitly reuses prior Dominatus-owned slices from visible snapshot projection:

- M9 layout/precision facts are still staged+committed first and projected from visible keys into `prom_judgment_facts`.
- M7 transfer queue facts are still staged+committed first and projected from visible keys into `prom_judgment_facts`.
- M8 transfer telemetry ownership is unchanged.

M10 does not duplicate ownership for those fields; it composes the full judgment input by combining:

- path/compute projection (M10),
- layout/precision projection (M9),
- transfer queue projection (M7).

## 5) Keys added/used

Added SGEMM keys for this slice:

Facts:

- `PROM_DOM_KEY_SGEMM_FACT_SHAPE_M/N/K`
- `PROM_DOM_KEY_SGEMM_FACT_WORK_UNITS`
- `PROM_DOM_KEY_SGEMM_FACT_CAN_STAGE/CAN_DIRECT`
- `PROM_DOM_KEY_SGEMM_FACT_ALLOW_FALLBACK`
- `PROM_DOM_KEY_SGEMM_FACT_READBACK_REQUIRED`
- `PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT/FORCE_STAGED/FORCE_TILED`
- `PROM_DOM_KEY_SGEMM_FACT_TILED_SHAPE`
- `PROM_DOM_KEY_SGEMM_FACT_SOFTWARE_VULKAN`
- `PROM_DOM_KEY_SGEMM_FACT_POLICY_MODE`

Decision:

- `PROM_DOM_KEY_SGEMM_JUDGMENT_SUCCESS`
- `PROM_DOM_KEY_SGEMM_JUDGMENT_ERROR_DETAIL`
- `PROM_DOM_KEY_SGEMM_JUDGMENT_REQUESTED_PATH`
- `PROM_DOM_KEY_SGEMM_JUDGMENT_SELECTED_PATH`
- `PROM_DOM_KEY_SGEMM_JUDGMENT_COMPUTE_MODE`
- `PROM_DOM_KEY_SGEMM_JUDGMENT_FINAL_DETAIL`
- `PROM_DOM_KEY_SGEMM_JUDGMENT_USED_FALLBACK_TO_DIRECT`
- `PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_CANDIDATE_INDEX`
- `PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_SCORE`

Infrastructure update required by key growth:

- Expanded dirty-key bitset words from 2 to 3.
- Expanded storage capacity from `128 * slots` to `256 * slots`.

## 6) Source-of-truth ownership model

- Runtime writes path/compute facts through Dominatus adapter staging API.
- Runtime commits, then builds path/compute facts from **visible** snapshot.
- Judgment runs from that visible projection.
- Runtime stages path/compute decision through Dominatus adapter.
- Runtime commits, then mirrors decision from visible snapshot for downstream use.

Legacy in-function fields are no longer authoritative for migrated path/compute state.

## 7) Staged/visible judgment behavior

Verified behavior:

1. Staged path/compute facts do not alter current visible projection before commit.
2. Committed facts affect the next selection.
3. Staged decision outputs do not become visible before commit.
4. Decision outputs become visible after commit.
5. Dirty masks cover migrated path/compute dependencies and decision keys.

## 8) Diagnostics/export behavior

- New adapter snapshot `prom_dom_sgemm_path_compute_snapshot` reads visible facts + decision.
- Snapshot also exposes already-migrated subcandidate outputs when present:
  - Packed4 selected/reject reason
  - FP16 selected/reject reason
  - transfer queue used/fallback reason
- Runtime now mirrors selected path/compute decision from visible path/compute snapshot after commit.

## 9) Compatibility mirror behavior

- Compatibility mirror behavior remains commit-gated: pre-commit reads remain pinned to visible state.
- No direct legacy mutation path was introduced that overrides visible Dominatus values for migrated fields.

## 10) Tests added

Added Marionette tests in `reactor_dominatus_sgemm_adapter_tests.cpp`:

1. `..._M10PathComputeInputSnapshotIsolation`
2. `..._M10PathComputeFactsAffectDecisionOnlyAfterCommit`
3. `..._M10PathComputeDecisionOutputStaging`
4. `..._M10PathComputeDirtyCoverage`
5. `..._M10PathComputeCompatibilityMirrorNoDrift`

Also re-ran full Marionette suite and Go Prometheus package tests.

## 11) Deferred scope for M11+

Explicitly deferred:

- async lifecycle ownership migration
- full slot diagnostics ownership migration
- decision caching
- N-slot/work-stealing
- memory suballocation
- FFT migration
- public external event stream API

## Inconsistency / documentation notes

- No policy/semantics inconsistency was intentionally introduced between existing behavior and this ownership migration slice.
- This slice required blackboard storage/bitset capacity growth to support additional SGEMM keys; previous fixed capacities were insufficient once this migration keyset was added.
