# Prometheus SGEMM Algorithm Lab — M28

## 1) M27 handoff summary (required first step)

M27's conclusion is carried forward unchanged:

1. **Why hybrid was rejected:** hybrid pull/double did not beat fixed double in stable regimes, did not stay close enough under jitter/burst, and did not reduce WIP without unacceptable starvation.
2. **Why fixed double became practical default:** it delivered the most reliable completion/starvation behavior with bounded complexity and bounded WIP.
3. **What fixed double means here:** exactly two SGEMM orchestration slots — one current/consumed and one next/prepared — with deterministic single-swap handoff and no speculative third slot.
4. **Failure modes to attack before native implementation:** swap-order errors, stale reuse, in-flight overwrite, shape/layout invalidation gaps, async ownership leaks, failure poisoning, and misleading diagnostics.
5. **What M29 should implement if M28 passes:** a native reactor state machine that enforces explicit slot lifecycle, mandatory invalidation rules, strict async ownership, and WIP<=2 invariants.

## 2) Fixed double model definition

M28 models fixed double buffering as:

- exactly one `current` slot when compute is active,
- at most one `next` slot in `preparing` or `ready`,
- deterministic `ready -> current` swap,
- bounded WIP depth `<= 2`,
- no hidden third slot and no unbounded outstanding work.

The second slot is pipeline-forward work, not a redundant backup copy.

## 3) Slot lifecycle model

Modeled slot state set:

- `empty`
- `preparing`
- `ready`
- `current`
- `in-flight`
- `consumed`
- `failed`

Per-slot metadata contract modeled in M28 tables:

- slot id,
- validity/generation tracking,
- shape + K + output dimension metadata,
- layout/precision metadata,
- required byte-capacity metadata,
- explicit ownership (current vs next vs in-flight).

## 4) Failure modes tested (rake categories)

M28 directly rakes the required categories:

1. swap-order bugs,
2. stale-buffer reuse,
3. in-flight overwrite,
4. shape-change invalidation,
5. layout/precision transition safety,
6. async interaction,
7. failure and recovery,
8. diagnostics truthfulness,
9. WIP/accounting invariants.

## 5) Findings by rake category

- **Swap order:** early/late/double/missing swap attempts are detected and rejected; no current/next alias violations were observed.
- **Stale reuse:** stale slot consumption is blocked by validity + invalidation checks.
- **In-flight overwrite:** overwrite attempts against in-flight ownership are rejected until completion/failure.
- **Shape/layout transitions:** transitions force invalidation and capacity/layout rebuild before swap.
- **Async ownership:** submit/complete/abandon paths retain slot ownership correctness; wrong-slot release remained zero.
- **Failure isolation:** next-slot and current-slot failures remain isolated; peer slot is not poisoned.
- **Recovery:** explicit cleanup and generation bump returns pipeline to valid operation.
- **Diagnostics:** counters for swap/invalidation/rejections/failure slot are required to make state transitions auditable.
- **WIP accounting:** observed max WIP depth remained 2 in all modeled scenarios.

## 6) Invariants that held

- WIP depth never exceeded 2.
- Max current slot count remained 1.
- Max ready-next slot count remained 1.
- No hidden third slot was observed.
- No consumer wrong-buffer reads were observed.
- No slot aliasing between current/next was observed.
- No ownership leaks were observed in async scenarios.

## 7) Invariants that failed or need guardrails

No hard invariant violation occurred in this M28 model; however, M28 surfaced **mandatory guardrails**:

- reject early/late/double swap attempts,
- reject overwrite while `in-flight`,
- invalidate on any shape/K/output/layout/precision change,
- rebuild capacity/layout before marking `ready`,
- block consumption from stale/failed generation,
- require explicit cleanup before resuming after failure.

## 8) Required M29 implementation contract

M29 should implement native fixed double buffering with this contract:

1. two explicit slots only; no hidden third slot path,
2. explicit slot-state machine (`empty/preparing/ready/current/in-flight/consumed/failed`),
3. generation counter + validity flag per slot,
4. deterministic swap preconditions (single swap, no duplicate handoff),
5. mandatory invalidation gates on shape/K/output/layout/precision transitions,
6. capacity/layout rebuild gate before `ready`,
7. async ownership ledger (submit/complete/abandon) with overwrite + double-submit rejection,
8. failure-isolated cleanup path with slot-id + reason diagnostics,
9. mandatory diagnostics fields:
   - `current_slot`, `next_slot`, `slot_state`,
   - `swap_count`,
   - `overwrite_rejection_count`,
   - `stale_buffer_rejection_count`,
   - `shape_invalidation_count`,
   - `layout_invalidation_count`,
   - `inflight_rejection_count`,
   - `failure_slot_id`, `failure_reason`.

## 9) Final verdict

Fixed double buffering is **safe to implement** as Prometheus SGEMM's practical default **if and only if** M29 enforces the explicit lifecycle/invalidation/ownership/diagnostics contract above.

---

## Required final answers

1. **Is fixed double buffering safe to implement?** Yes, with explicit lifecycle and guardrails.
2. **What slot lifecycle states are required?** `empty`, `preparing`, `ready`, `current`, `in-flight`, `consumed`, `failed`.
3. **What invalidation rules are mandatory?** Invalidate and rebuild on shape/K/output/layout/precision transitions; reject stale generation consumption.
4. **What async ownership rules are mandatory?** In-flight slot is immutable until completion/failure; reject overwrite/double-submit; explicit abandon/cleanup ownership release.
5. **What diagnostics are mandatory?** Slot ownership/state + swap/rejection/invalidation counters + failure slot/reason.
6. **What exact M29 implementation contract should be followed?** The 9-point contract in section 8.
7. **Is further rake lab needed before implementation?** No, not before M29, provided contract enforcement is complete.
