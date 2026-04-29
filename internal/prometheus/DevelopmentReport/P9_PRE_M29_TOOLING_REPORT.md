# P9 Pre-M29 Tooling Report — Slot HFSM Helper

## Why add HFSM-style tooling now

M28 established that fixed double buffering needs an explicit slot lifecycle contract (states, guarded transitions, diagnostics, and failure isolation). A compact slot-focused HFSM helper removes ad hoc flag logic before M29 starts, so M29 can wire behavior onto a deterministic lifecycle seam rather than patching control flow directly inside `reactor_vulkan.c`.

## Why this is not a Dominatus/DragonGod port

This pass adds only a tiny fixed-depth stack machine with a domain-specific slot lifecycle enum and explicit transition guards. It does **not** add callback schedulers, mailbox/event systems, persistence, policy engines, or a generic runtime framework.

## Stack machinery added

New helper module:

- `internal/prometheus/native/reactor_slot_hfsm.h`
- `internal/prometheus/native/reactor_slot_hfsm.c`

Capabilities:

- fixed-size stack (`PROM_SLOT_HFSM_MAX_DEPTH = 8`)
- current-state query
- push / pop / replace operations
- depth and contains queries
- reset/init with no heap allocation

## Slot lifecycle states

The helper defines the explicit slot lifecycle domain:

- `PROM_SLOT_EMPTY`
- `PROM_SLOT_PREPARING`
- `PROM_SLOT_READY`
- `PROM_SLOT_CURRENT`
- `PROM_SLOT_IN_FLIGHT`
- `PROM_SLOT_CONSUMED`
- `PROM_SLOT_FAILED`
- `PROM_SLOT_CLEANUP` (explicitly included for failure recovery)

This state domain is separate from judgment-engine policy modes and compute/path modes.

## Legal transitions enforced

The helper enforces deterministic legality rules:

- `EMPTY -> PREPARING`
- `PREPARING -> READY`
- `READY -> CURRENT`
- `CURRENT -> IN_FLIGHT`
- `IN_FLIGHT -> CONSUMED`
- `CONSUMED -> EMPTY`
- `ANY (except FAILED) -> FAILED`
- `FAILED -> CLEANUP -> EMPTY`

Illegal transitions are rejected, counted, and leave current state unchanged.

## Diagnostics exposed

`prom_slot_hfsm_get_diagnostics` provides:

- current state
- previous state
- transition count
- invalid transition count
- max stack depth reached
- last invalid from/to
- failure count
- cleanup count

## Slot metadata seam prepared for M29

`prom_slot_metadata` provides a small lifecycle-adjacent metadata shape:

- slot id
- generation
- valid flag
- shape metadata (`m/n/k`)
- layout/precision metadata
- required capacity bytes
- failure reason

The helper includes metadata set/query and invalidation helpers without deep SGEMM execution integration.

## Tests proving helper behavior

Added `internal/prometheus/native/Marionette/reactor_slot_hfsm_tests.cpp` coverage for:

1. legal lifecycle sequence
2. illegal transition rejection
3. failure path requiring cleanup/reset
4. bounded stack behavior (push/pop/replace + overflow/underflow)
5. diagnostics counters and last-invalid tracking
6. deterministic replay (same sequence => same state and diagnostics)
7. metadata shape/invalidation seam for M29

## M29 integration guidance

M29 should instantiate one `prom_slot_hfsm` per fixed slot (two slots total) and route slot lifecycle operations through:

- `prom_slot_hfsm_init` / `prom_slot_hfsm_reset`
- `prom_slot_hfsm_current_state`
- `prom_slot_hfsm_transition`
- `prom_slot_hfsm_fail`
- `prom_slot_hfsm_cleanup`
- `prom_slot_hfsm_get_diagnostics`
- `prom_slot_hfsm_metadata` + metadata helpers for shape/layout/capacity invalidation

This keeps M29 focused on actual double-buffer execution logic while preserving explicit, inspectable lifecycle behavior.
