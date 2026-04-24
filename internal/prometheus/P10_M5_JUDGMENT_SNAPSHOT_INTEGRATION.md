# P10 M5 — Judgment Snapshot Integration

## 1) Integration scope

M5 integrates one real judgment path with Dominatus visible snapshot reads: the M35 buffering-selector judgment inputs for the migrated memory-budget/headroom fact slice.

This milestone does not migrate all judgment facts. It proves anti-tearing for the selected M35 input slice while preserving existing behavior.

## 2) Judgment path selected

Selected path: `prom_judgment_engine_select_buffering_mode(...)`.

Integration entrypoint:

- `prom_dom_sgemm_build_buffering_selector_facts_from_visible(...)` builds judgment facts from committed visible Dominatus values.
- Runtime path in `reactor_vulkan.c` now invokes this projection before M35 judgment selection.

## 3) Visible keys/facts consumed

M5 visible snapshot projection consumes these committed keys:

- `PROM_DOM_KEY_MEMORY_BUDGET` → `memory_budget_slots_permille`
- `PROM_DOM_KEY_MEMORY_M35_FIXED_HEADROOM` → `fixed_double_headroom_slots_permille`
- `PROM_DOM_KEY_MEMORY_M35_PULL_LAG_HEADROOM` → `pull_lag_headroom_slots_permille`
- `PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM` → `serial_jit_headroom_slots_permille`

Projection metadata also records:

- `visible_generation` used by judgment
- `from_visible_snapshot` (whether projection resolved from visible keys)
- `dependent_dirty_key_mask_last_commit` for this migrated dependency slice

Unmigrated fields remain sourced from the existing direct fact-construction path and are merged as fallback/template inputs.

## 4) Commit boundary semantics

M5 enforces this boundary order for M35:

1. Runtime builds current-step direct facts (legacy path for full shape).
2. Judgment projection is built from **visible** blackboard (or fallback template when visible keys are absent).
3. Judgment selects M35 buffering mode using the projected facts.
4. Reactor stages current-step M35 facts/decision into Dominatus staged state.
5. Commit promotes staged→visible for the next decision boundary.

This keeps judgment on stable, committed inputs while reactor writes next-boundary updates.

## 5) Staged vs visible isolation proof

Adapter tests prove non-tearing behavior:

- Visible value A committed.
- Staged value B written without commit.
- Projection still reads A and judgment output remains A-derived.
- After commit, projection reads B.

This satisfies the M5 anti-tearing invariant.

## 6) Compatibility behavior

Compatibility is preserved by design:

- On initial/empty board state, projection falls back to existing direct fact construction.
- Existing M35 direct path still provides unmigrated fields.
- M35 decisions and diagnostics remain ABI-compatible.
- Existing M35 Marionette tests continue to validate behavior.

## 7) Dirty dependency / future caching note

M5 does not add decision caching.

However, projection now exposes a dependency dirty mask over the migrated keys:

- bit0: memory budget
- bit1: fixed headroom
- bit2: pull-lag headroom
- bit3: serial headroom

Future optimization (deferred): judgment reuse can gate on `dependent_dirty_key_mask_last_commit == 0` and stable unmigrated dependencies.

## 8) Tests added

Added focused M5 tests in `reactor_dominatus_sgemm_adapter_tests.cpp`:

1. visible projection snapshot isolation across staged vs commit boundary (A/B proof)
2. projected-visible judgment compatibility with legacy facts for migrated slice
3. dirty dependency bitmask + in-step staged-mutation no-drift proof

Existing tests retained:

- M35 adapter staged/visible and dirty behavior tests
- M35 runtime integration tests

## 9) Deferred scope for M6+

Deferred intentionally:

- full migration of all buffering-selector inputs to blackboard ownership
- migration of non-M35 judgment paths
- dirty-key decision caching/reuse policy
- N-slot/concurrent judgment boundaries
- broader diagnostics ownership replacement of legacy direct fields

## 10) Inconsistency/documentation gap callout

No conflict was found with `Language/reference` authority because this milestone changes native C runtime code under `internal/prometheus/native`.

Explicitly deferred inconsistency: buffering-selector inputs are split between visible-owned migrated keys and legacy direct facts. This is intentional in M5 and should be converged in later milestones.
