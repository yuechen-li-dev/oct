# P10 M3 — SGEMM Blackboard Adapter Migration

## 1) Migration slice chosen

M3 migrates the **M35 buffering-selector diagnostics/facts slice** (Option A) through a Dominatus adapter:

- selected buffering mode
- feasibility flags (fixed/pull-lag/serial)
- candidate scores (fixed/pull-lag/serial)
- selector reason + final reason code
- per-mode rejection reasons
- memory budget fact (slots-permille)
- per-mode headroom facts (fixed/pull-lag/serial)

## 2) Why this slice was chosen

This slice is policy/fact dominated, reason-code heavy, and already exported as diagnostics, making it a low-risk first migration that exercises staged/visible semantics without changing Vulkan object lifecycle behavior.

## 3) Files/modules added

Added:

- `internal/prometheus/native/reactor_dominatus_sgemm_adapter.h`
- `internal/prometheus/native/reactor_dominatus_sgemm_adapter.c`
- `internal/prometheus/native/Marionette/reactor_dominatus_sgemm_adapter_tests.cpp`
- `internal/prometheus/P10_M3_SGEMM_BLACKBOARD_ADAPTER.md`

Updated integration:

- `internal/prometheus/native/reactor_vulkan.c`
- `internal/prometheus/native/reactor_dominatus_blackboard.h`
- `internal/prometheus/native/reactor_dominatus_blackboard.c`
- `internal/prometheus/native/build_stub.sh`
- `internal/prometheus/native/build_windows.cmd`

## 4) Blackboard keys added/used

### Added keys

SGEMM domain:

- `PROM_DOM_KEY_SGEMM_M35_FIXED_FEASIBLE`
- `PROM_DOM_KEY_SGEMM_M35_PULL_LAG_FEASIBLE`
- `PROM_DOM_KEY_SGEMM_M35_SERIAL_FEASIBLE`
- `PROM_DOM_KEY_SGEMM_M35_FIXED_SCORE`
- `PROM_DOM_KEY_SGEMM_M35_PULL_LAG_SCORE`
- `PROM_DOM_KEY_SGEMM_M35_SERIAL_SCORE`
- `PROM_DOM_KEY_SGEMM_M35_REASON_CODE`
- `PROM_DOM_KEY_SGEMM_M35_FINAL_REASON_CODE`
- `PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTION_REASON`
- `PROM_DOM_KEY_SGEMM_M35_PULL_LAG_REJECTION_REASON`
- `PROM_DOM_KEY_SGEMM_M35_SERIAL_REJECTION_REASON`

MEMORY domain:

- `PROM_DOM_KEY_MEMORY_M35_FIXED_HEADROOM`
- `PROM_DOM_KEY_MEMORY_M35_PULL_LAG_HEADROOM`
- `PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM`

### Reused existing keys

- `PROM_DOM_KEY_SGEMM_BUFFERING_MODE`
- `PROM_DOM_KEY_MEMORY_BUDGET`

## 5) Ownership model for migrated fields

### Source of truth after M3

For migrated M35 fields, **Dominatus visible state is the source of truth**:

- Runtime writes through `prom_dom_sgemm_stage_m35(...)` (staged).
- Commit boundary is explicit via `prom_dom_sgemm_commit(...)`.
- Diagnostics export reads via `prom_dom_sgemm_read_visible_m35(...)`.

### Compatibility mirror

`rt->slot_diag` still receives compatibility mirror updates from committed visible snapshot in the SGEMM path, but ownership for migrated fields is blackboard-visible.

### Deferred direct-owned fields (not migrated in M3)

- M35 transition/rejection counters
- fixed/pull-lag/serial rejected flags
- required fixed/pull-lag/serial slots-permille
- pull-lag proxy-unit diagnostics and serial JIT counters
- slot lifecycle diagnostics (M29) and transfer diagnostics (M31)
- packed4/fp16 diagnostics

## 6) Staged/visible commit behavior

Adapter tests cover reactor-adjacent staged/visible behavior:

1. staged M35 writes are not visible before commit
2. commit promotes staged values to visible
3. visible snapshot becomes readable after commit

Runtime SGEMM path now stages M35 facts and commits at an explicit boundary before projection/diagnostics.

## 7) Dirty tracking validation

M3 validates:

- changed M35 values mark specific dirty keys
- SGEMM and MEMORY domains mark dirty as expected
- dirty-key last-commit masks receive promoted keys
- staged dirty masks clear after commit
- same-value staging does not dirty
- trace ring records adapter writes (including memory headroom keys)

To support key-precise assertions without index-coupling, M3 adds:

- `prom_dom_dirty_key_staged(...)`
- `prom_dom_dirty_key_last_commit(...)`

## 8) Public diagnostics compatibility

`prom_reactor_runtime_sgemm_policy_diagnostics_impl(...)` now reads migrated M35 fields from visible blackboard snapshot and preserves existing diagnostics struct/ABI layout and value semantics.

M35 Marionette tests remain valid and pass with this migration.

## 9) Tests added

Added focused tests in `reactor_dominatus_sgemm_adapter_tests.cpp`:

1. staged invisible / visible after commit
2. dirty key/domain behavior + same-value non-dirty behavior
3. visible projection snapshot correctness
4. trace emission and deterministic reset behavior

Also executed existing M35 buffering selector tests to verify compatibility.

## 10) Deferred fields/domains for M4+

Deferred intentionally:

- slot HFSM ownership/lifecycle bridge into Dominatus events (M4 target)
- broader transfer-queue diagnostics migration
- packed4/fp16 diagnostics migration
- N-slot concurrency/runtime expansion

## 11) Inconsistency/documentation gap callout

`Language/reference` governs Oct-language semantics and style, while this milestone is native C runtime work under `internal/prometheus/native`. No direct conflict was encountered, but this split in authority is explicit and intentional for this change.
