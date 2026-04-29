# P11 M19 — Worker-Local Two-Slot Runtime

## 1) M44 handoff summary

M44 recommended the first native N-slot implementation as **worker-local S=2** with static partition, immutable plans, readiness-attention-driven refill, first-failure-wins drain, and ordered atomic output commit. Shared slot pools, stealing, and S=4+ remained deferred.

## 2) Audit findings (follow-up)

Classification against M19 follow-up targets:

1. **slot assignment rotation** — implemented and tested (owner-local rotation into 2 slots).
2. **slot lifecycle updates** — implemented but under-tested in original M19; strengthened tests added for success/failure/invalidation paths.
3. **ready/failed/invalidated/attention masks** — implemented but under-tested in original M19; explicit mask truth tests added.
4. **refill behavior** — implemented (static pre-plan + owner-local assignment), but still scaffolded vs fully dynamic runtime refill loop.
5. **owner-local enforcement** — implemented and tested via per-slot owner diagnostics and wrong-owner resource checks.
6. **failure before submit** — scaffolding-only in original M19; now explicit pre-submit failure hook + tests.
7. **failure after submit** — implemented from M17 and now re-validated with S=2 diagnostics expectations.
8. **drain behavior** — implemented from M17; slot drain counter remains aggregate and conservative.
9. **output staging/commit ordering** — implemented and tested with delayed completion pressure.
10. **execution-mode compatibility** — implemented but under-tested in original M19; now covered across lane, real-thread serialized, and forced fallback.

## 3) Implemented slot model

M19 adds explicit batch slot runtime state with:

- slot id / owner worker id,
- lifecycle state,
- generation,
- assigned plan and entry ids,
- queue / command / arena / output staging ids,
- in-flight / ready / invalidated flags,
- failure stage/detail.

Lifecycle mapping:

- `PROM_BATCH_SLOT_STATE_EMPTY`
- `PROM_BATCH_SLOT_STATE_PREPARING`
- `PROM_BATCH_SLOT_STATE_READY`
- `PROM_BATCH_SLOT_STATE_IN_FLIGHT`
- `PROM_BATCH_SLOT_STATE_COMPLETE`
- `PROM_BATCH_SLOT_STATE_FAILED`
- `PROM_BATCH_SLOT_STATE_CLEANUP`
- `PROM_BATCH_SLOT_STATE_INVALIDATED`

## 4) Worker-local ownership

Slots are worker-local contiguous blocks:

- worker `w` owns `[w*S, w*S+(S-1)]`,
- `S target = 2`,
- assignment rotation is owner-local only,
- no cross-worker borrowing path exists.

## 5) Readiness / attention model

Diagnostics expose:

- `dirty_slot_mask`
- `ready_slot_mask`
- `failed_slot_mask`
- `invalidated_slot_mask`
- `attention_slot_mask`

Contract kept:

- `attention = ready ∪ failed ∪ invalidated`.

Follow-up added explicit invalidated-ready rejection behavior and tests proving invalidated and failed slots remain in attention.

## 6) Follow-up behavior completed

This follow-up completed/proved:

- both slots can be used by one worker with repeated assignments,
- owner-local slot usage diagnostics remain truthful,
- pre-submit slot failure sets failed+attention masks and fails atomically,
- invalidated-ready slot is rejected before dispatch and marked invalidated+attention,
- delayed-completion success still commits outputs atomically,
- S=2 diagnostics remain consistent across lane / real-thread serialized / forced fallback modes.

## 7) Execution-mode compatibility

Preserved and re-validated:

- single-worker fallback,
- lane-simulated multi-worker,
- real-thread serialized Vulkan,
- true-multi topology-gated behavior from M16/M17,
- serialized fallback when gates fail.

## 8) Typed arena interaction

Arena ownership remains worker-local and unchanged in authority.

Slot diagnostics now consistently report:

- target slots per worker,
- effective slots per worker,
- total slot count,
- slot cap reason when reduced by memory budget.

## 9) Failure / drain behavior

M17 semantics preserved:

- first failure wins,
- no output commit on failure,
- drain/failed terminal behavior preserved,
- unsafe-to-reuse on timeout/device/global classes preserved.

M19 follow-up adds explicit slot-level pre-submit and invalidated-ready failure paths with mask updates.

## 10) Output ordering

Caller-visible commit remains atomic and entry-ordered:

- success commits all staged outputs,
- failure commits none.

## 11) Diagnostics verified

Verified diagnostics cover:

- slots-per-worker target/effective,
- total slot count and cap reason,
- per-slot owner/state/generation/entry/queue/resource ids,
- dirty/ready/failed/invalidated/attention masks,
- boundary generation,
- refill/poll/failure/drain counters,
- unsafe-to-reuse.

## 12) Tests added/strengthened

Follow-up adds Marionette coverage for:

1. both slots used by one worker,
2. pre-submit failure mask + attention truth,
3. invalidated-ready slot rejection,
4. ordered output commit under delayed completion,
5. S=2 compatibility across lane/real-thread/fallback modes,
6. prior M19 ownership and budget-cap diagnostics tests retained.

## 13) Deferred scope

Still explicitly deferred:

- S=4+,
- shared slot pool,
- work stealing,
- SPMC/MPMC queues,
- lock-free runtime redesign,
- parallel judgment,
- cross-worker slot borrowing,
- public event stream redesign,
- scheduler redesign,
- benchmark/performance claims.

## Language/reference consistency note

This milestone changes native runtime/tests/docs only and does not alter Oct language semantics under `Language/reference`.
