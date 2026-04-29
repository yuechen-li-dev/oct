# P10 M7 — Transfer Queue Policy / Diagnostics Dominatus Ownership Migration

## 1) Migration scope

M7 migrates the transfer-queue policy slice into Dominatus ownership for queue-policy facts, policy decision outputs, and diagnostics export sourcing.

In-scope:

- transfer-policy input facts stage/commit through Dominatus queue keys
- transfer-policy judgment reads transfer facts from visible Dominatus projection
- transfer-policy decision outputs stage/commit through Dominatus queue keys
- diagnostics export reads migrated M31 fields from visible Dominatus snapshot
- legacy `slot_diag` transfer fields remain compatibility mirrors and runtime telemetry containers

Out-of-scope behavior and scheduler redesign remain unchanged.

## 2) Facts migrated

Migrated transfer facts now staged and committed into Dominatus:

- dedicated transfer queue availability
- transfer queue family index
- compute queue family index
- queue-families-differ flag (pseudo/shared implication)
- transfer queue supported flag
- transfer queue disabled-by-config gate
- workload-large-enough gate
- sync/ownership supported gate
- fallback availability flag
- upload-only policy eligibility marker
- upload+readback support marker (currently `0` / unsupported)

These facts are staged via `prom_dom_sgemm_stage_transfer_queue_facts(...)` and projected via `prom_dom_sgemm_build_transfer_queue_facts_from_visible(...)`.

## 3) Decision outputs migrated

Migrated transfer decision outputs now stage/commit through Dominatus:

- transfer policy selected flag
- selected transfer policy code (`0` fallback / `1` dedicated upload-only)
- transfer queue used flag
- transfer fallback reason

These outputs are staged via `prom_dom_sgemm_stage_transfer_queue_decision(...)` and read via `prom_dom_sgemm_read_visible_transfer_queue_diagnostics(...)`.

## 4) Keys added/used

### Existing queue keys now actively used in M7

- `PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY`
- `PROM_DOM_KEY_QUEUE_TRANSFER_FAMILY`
- `PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE`
- `PROM_DOM_KEY_QUEUE_TRANSFER_POLICY`

### New queue keys added for M7

- `PROM_DOM_KEY_QUEUE_FAMILIES_DIFFER`
- `PROM_DOM_KEY_QUEUE_TRANSFER_SUPPORTED`
- `PROM_DOM_KEY_QUEUE_TRANSFER_DISABLED_BY_CONFIG`
- `PROM_DOM_KEY_QUEUE_TRANSFER_WORKLOAD_LARGE_ENOUGH`
- `PROM_DOM_KEY_QUEUE_TRANSFER_SYNC_OWNERSHIP_SUPPORTED`
- `PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_AVAILABLE`
- `PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_ONLY_ELIGIBLE`
- `PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_READBACK_SUPPORTED`
- `PROM_DOM_KEY_QUEUE_TRANSFER_POLICY_SELECTED`
- `PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_REASON`
- `PROM_DOM_KEY_QUEUE_TRANSFER_QUEUE_USED`

## 5) Source-of-truth ownership model

For migrated transfer-policy fields, Dominatus visible queue state is source-of-truth.

Flow:

1. runtime stages transfer facts
2. commit promotes staged facts to visible
3. judgment consumes visible transfer projection
4. runtime stages transfer decision outputs
5. commit promotes staged decisions to visible
6. diagnostics export reads visible transfer snapshot

`slot_diag` transfer fields remain compatibility mirrors for migrated policy fields and telemetry holders for deferred runtime counters.

## 6) Staged/visible judgment behavior

M7 preserves anti-tearing semantics:

- staged transfer facts do not affect current transfer-policy judgment before commit
- committed transfer facts affect subsequent judgment
- staged transfer decision outputs are not externally visible before commit
- committed transfer decision outputs are externally visible after commit

## 7) Diagnostics export behavior

`prom_reactor_runtime_sgemm_policy_diagnostics_impl(...)` now exports migrated M31 fields from visible Dominatus transfer snapshot when available:

- queue used
- policy selected
- dedicated availability
- queue family indices
- queue families differ
- transfer fallback reason
- upload policy marker

Fallback to `slot_diag` remains for compatibility when visible transfer snapshot is unavailable.

## 8) Dirty tracking validation

M7 adds dirty validation over migrated queue facts and decisions:

- per-key staged dirty checks for transfer facts/outputs
- last-commit dirty checks for transfer queue keys
- same-value writes remain non-dirty

## 9) Compatibility mirror behavior

For migrated transfer policy fields:

- mirror fields in `slot_diag` are refreshed from visible transfer snapshot
- staged mutations do not change mirror-visible diagnostics pre-commit
- post-commit diagnostics and mirrors update together from visible Dominatus state

## 10) Tests added

`reactor_dominatus_sgemm_adapter_tests.cpp` now includes M7 coverage:

1. transfer input snapshot isolation across commit boundary
2. representative transfer input changes affecting decision only after commit
3. transfer decision output staging visibility
4. transfer dirty-key coverage and same-value non-dirty behavior
5. compatibility-mirror no-drift pre/post commit

Existing M31 transfer behavior tests are retained.

## 11) Legacy compatibility mirrors remaining

The following remain legacy-owned runtime telemetry in M7 (not policy-owned in Dominatus yet):

- queue family handoff count
- transfer/compute wait count
- transfer failure slot id/reason
- async transfer completion marker

These are intentionally deferred because they are lifecycle/runtime telemetry, not pure policy fact/decision state.

## 12) Deferred scope for M8+

Deferred intentionally:

- upload+readback transfer policy enablement
- path/compute/precision selector migration
- Packed4/FP16 selector migration
- full slot diagnostics ownership migration (including transfer failure + async completion)
- queue lifecycle correlation beyond policy slice
- decision caching
- N-slot/work-stealing
- memory suballocation
- FFT migration

## 13) Inconsistency surfaced

`PROM_DOM_KEY_QUEUE_HANDOFF_COUNT` existed before M7, but M7 keeps handoff/wait/failure/async telemetry deferred. This creates a temporary mixed state where some queue diagnostics are Dominatus-owned (policy) while runtime telemetry remains legacy-owned. This is intentional in M7 and should be resolved by later diagnostics/lifecycle migration work.
