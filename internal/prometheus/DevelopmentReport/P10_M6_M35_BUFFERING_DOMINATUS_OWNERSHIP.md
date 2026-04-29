# P10 M6 — Full M35 Buffering Selector Dominatus Ownership Migration

## 1) Migration scope

M6 completes Dominatus ownership for the full M35 buffering selector slice:

- all `prom_buffering_selector_facts` inputs are now staged and committed into Dominatus keys
- `prom_judgment_engine_select_buffering_mode(...)` now consumes a full visible projection from Dominatus
- all core M35 decision outputs are staged and committed through Dominatus keys
- diagnostics export prefers Dominatus visible projection for migrated M35 fields
- `slot_diag` fields are retained as compatibility mirrors/counters, not source-of-truth for migrated facts/outputs

## 2) Facts migrated

Migrated input facts now owned by Dominatus visible state:

- memory budget
- required fixed slots
- required pull-lag peak slots
- required serial slots
- fixed/pull-lag/serial headroom
- transfer variance class
- compute predictability class
- starvation risk
- pull-lag WIP waste exceeded gate
- fallback availability

These are staged via `prom_dom_sgemm_stage_m35_facts(...)`, committed, and read via `prom_dom_sgemm_build_buffering_selector_facts_from_visible(...)`.

## 3) Decision outputs migrated

Migrated outputs now staged/committed in Dominatus:

- success flag
- selected buffering mode
- fixed/pull-lag/serial feasible flags
- fixed/pull-lag/serial rejected flags
- fixed/pull-lag/serial scores
- fixed/pull-lag/serial rejection reasons
- reason code
- final reason code
- no-feasible-mode detail code (when applicable)

These are staged via `prom_dom_sgemm_stage_m35_decision(...)` and projected through `prom_dom_sgemm_read_visible_m35(...)`.

## 4) Keys added/used

### New SGEMM keys

- `PROM_DOM_KEY_SGEMM_M35_TRANSFER_VARIANCE_CLASS`
- `PROM_DOM_KEY_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS`
- `PROM_DOM_KEY_SGEMM_M35_STARVATION_RISK_HIGH`
- `PROM_DOM_KEY_SGEMM_M35_PULL_LAG_WIP_WASTE_EXCEEDED`
- `PROM_DOM_KEY_SGEMM_M35_FALLBACK_AVAILABLE`
- `PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTED`
- `PROM_DOM_KEY_SGEMM_M35_PULL_LAG_REJECTED`
- `PROM_DOM_KEY_SGEMM_M35_SERIAL_REJECTED`
- `PROM_DOM_KEY_SGEMM_M35_SUCCESS`
- `PROM_DOM_KEY_SGEMM_M35_NO_FEASIBLE_DETAIL`

### New MEMORY keys

- `PROM_DOM_KEY_MEMORY_M35_REQUIRED_FIXED`
- `PROM_DOM_KEY_MEMORY_M35_REQUIRED_PULL_LAG`
- `PROM_DOM_KEY_MEMORY_M35_REQUIRED_SERIAL`

## 5) Source-of-truth ownership model

For migrated M35 inputs and outputs, Dominatus visible state is the source of truth.

Execution order now follows:

1. runtime computes current M35 facts
2. runtime stages facts (`staged`)
3. commit promotes facts to `visible`
4. judgment reads full visible projection
5. runtime stages decision outputs (`staged`)
6. commit promotes decisions to `visible`
7. diagnostics export reads visible projection

Compatibility mirrors in `slot_diag` are updated from visible snapshot after commit and are no longer authoritative for migrated fields.

## 6) Staged/visible judgment behavior

M6 keeps anti-tearing explicit:

- staged M35 writes do not alter visible snapshot before commit
- full selector facts projection reads visible snapshot only
- next decision changes only after corresponding commit

## 7) Diagnostics export behavior

`prom_reactor_runtime_sgemm_policy_diagnostics_impl(...)` now reads migrated M35 decision/fact projection from `prom_dom_sgemm_read_visible_m35(...)` when available, including:

- selected mode/feasibility/scores/reasons
- rejected mode flags
- required slots and headroom
- memory budget

Legacy `slot_diag` fallback remains for pre-populated/empty-board compatibility.

## 8) Dirty tracking validation

M6 extends dirty dependency coverage to the full migrated input slice by including required slots and safety/gating facts in the dependency mask and per-key dirty assertions.

Same-value writes remain no-op (no staged dirty bits).

## 9) Compatibility mirror behavior

`slot_diag` M35 fields remain as compatibility mirrors and counters:

- mirrors are refreshed from visible Dominatus snapshot after commit
- pre-commit staged mutations do not alter visible-derived mirror export
- direct `slot_diag` mutation is not used as source-of-truth for migrated M35 fields

## 10) Tests added/updated

Updated `reactor_dominatus_sgemm_adapter_tests.cpp` with M6 coverage:

- full input snapshot isolation (A/B before vs after commit)
- representative migrated inputs only affecting decision after commit
- decision-output staging visibility (A visible while B staged)
- expanded dirty dependency bitmask and key-level dirty checks
- compatibility mirror no-drift proof pattern with pre/post-commit visible reads

Existing M35/Marionette tests are retained for compatibility regression checks.

## 11) Legacy compatibility fields (mirror-only)

For migrated ownership fields, `slot_diag` mirrors remain for compatibility export but are populated from Dominatus visible snapshot:

- selected mode, feasible flags, rejected flags, scores, reason codes, rejection reasons
- memory budget, required slots, and headroom

Non-migrated M35 counters/proxy telemetry remain owned by legacy diagnostics fields.

## 12) Deferred scope for M7+

Deferred explicitly:

- path/compute/precision selector migrations
- Packed4/FP16 selector migration
- transfer-queue selector migration
- full slot diagnostics ownership migration
- decision caching/reuse policy
- N-slot/work-stealing/concurrency expansion
- FFT-domain migration
