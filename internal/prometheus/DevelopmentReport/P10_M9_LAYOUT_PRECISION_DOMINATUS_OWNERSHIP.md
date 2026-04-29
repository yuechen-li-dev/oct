# P10 M9 — Packed4 / FP16 Layout-Precision Dominatus Ownership

## 1) migration scope

M9 migrates the bounded layout/precision candidate slice (Packed4 + FP16 facts and diagnostics) into Dominatus staged/visible ownership while keeping the broader SGEMM path selector unchanged.

## 2) Packed4 facts migrated

Migrated as Dominatus SGEMM keys and projected via visible snapshot:

- packed4 available
- packed4 small-shape gate
- packed4 padding waste permille
- packed4 mode budget permille
- packed4 row-major validity
- packed4 tail validity

## 3) Packed4 decisions/diagnostics migrated

Migrated decision + diagnostics ownership:

- packed4 selected
- packed4 reject reason
- packed4 selected layout format
- packed4 tail count last/total
- packed4 padded-lane count last/total
- packed4 padding waste permille last
- packed4 selection count
- packed4 mode-budget denials
- packed4 row-major check failures
- packed4 fallback counters:
  - padding waste
  - small shape
  - capability missing
  - fallback required
  - mode budget denied

## 4) FP16 facts migrated

Migrated as Dominatus SGEMM keys and projected via visible snapshot:

- strict FP32
- tolerance known
- tolerance pass
- special values flag
- FP16 storage capability
- fallback available
- FP16 utility score

## 5) FP16 decisions/diagnostics migrated

Migrated decision + diagnostics ownership:

- fp16 selected
- fp16 reject reason
- fp16 max absolute error
- fp16 max relative error
- fp16 aggregate error
- fp16 worst-case element index
- fp16 K error growth
- fp16 cancellation risk
- fp16 tolerance known
- fp16 tolerance pass
- fp16 fallback reason detail
- fp16 selected candidate

## 6) keys added/used

New SGEMM keys:

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
- `PROM_DOM_KEY_SGEMM_PACKED4_SELECTED`
- `PROM_DOM_KEY_SGEMM_PACKED4_REJECT_REASON`
- `PROM_DOM_KEY_SGEMM_FP16_SELECTED`
- `PROM_DOM_KEY_SGEMM_FP16_REJECT_REASON`

New diagnostics keys:

- packed4 layout/tail/padding/selection/fallback counter keys
- fp16 error/tolerance/fallback/selected-candidate keys

## 7) source-of-truth ownership model

For migrated M9 fields:

1. runtime stages layout/precision facts via adapter
2. commit promotes staged → visible
3. judgment consumes visible projection for migrated facts
4. runtime stages layout/precision decision/diagnostics via adapter
5. commit promotes staged → visible
6. diagnostics export reads visible Dominatus snapshot first

## 8) staged/visible behavior

M9 keeps anti-tearing behavior:

- staged facts do not change visible projection until commit
- staged decisions/diagnostics do not change visible diagnostics until commit
- last-commit dirty masks cover migrated fact/decision keys
- same-value writes remain non-dirty

## 9) diagnostics export behavior

`prom_reactor_runtime_sgemm_policy_diagnostics_impl(...)` now prefers
`prom_dom_sgemm_read_visible_layout_precision_diagnostics(...)` for migrated Packed4/FP16 diagnostics.

Legacy controller fields remain fallback-only if visible snapshot is unavailable.

## 10) compatibility mirror behavior

`rt->sgemm_controller` remains as compatibility mirror/state carrier during this slice:

- runtime still updates controller counters as before
- migrated diagnostics are exported from visible Dominatus when available
- controller values are mirrored into Dominatus through staged decision updates
- controller fields are no longer the authoritative source for migrated exported fields

## 11) tests added

Added M9 adapter tests in `reactor_dominatus_sgemm_adapter_tests.cpp`:

1. layout/precision fact snapshot isolation across commit
2. layout/precision decision staging visibility and dirty coverage

## 12) deferred scope for M10+

Deferred explicitly:

- full path/compute selector migration
- requested/selected path migration
- final detail migration
- baseline/tiled selector ownership
- transfer queue ownership changes beyond M7/M8 scope
- slot lifecycle ownership migration beyond existing bridge
- decision caching
- N-slot/work-stealing/concurrency
- FFT migration

## 13) inconsistency callout

M9 stores FP16 float diagnostics in Dominatus as explicit `uint32` bit-pattern keys (`*_BITS`) because the blackboard value system is integer-typed. This is intentional and preserves exact payload values, but it is a representational documentation gap versus the public float diagnostics fields.

## 14) follow-up: M4 slot event observability regression after M9

Post-M9, Marionette smoke `PrometheusDominatusSlotAdapter_RuntimeSmokeFixedDoubleProducesCommittedSlotEvent` failed even though slot lifecycle bridge emission still occurred.

Root cause:

- SGEMM now performs additional Dominatus commits for migrated layout/precision facts + decisions.
- `prom_dom_dirty_slots_last_commit(...)` is a per-commit window, so later non-slot commits can clear dirty-slot bits.
- runtime diagnostics previously exported slot dirty mask directly from that last-commit window.

Resulting behavior:

- slot events remained staged and committed in the committed event ring,
- but the exported dirty-slot mask could be `0` when queried after later non-slot commits.

Follow-up contract/fix:

- `prom_dom_slot_read_last_commit(...)` now scans committed events backward and returns the latest slot lifecycle event (`source=slot_hfsm`, `domain=slot`).
- for runtime slot diagnostics, the dirty-slot mask is aligned with that retained slot event slot id, keeping slot observability stable across multi-commit SGEMM calls.
- Marionette coverage now verifies this behavior and also checks that migrated Packed4/FP16 diagnostics remain visible in the same runtime call.
