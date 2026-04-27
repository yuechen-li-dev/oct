# P11 M11 — Real Worker Thread Bridge Hardening

## 1) M10 handoff summary

M10 introduced opt-in real worker threads with immutable central plans, worker no-judgment/no-Dominatus mutation boundaries, per-worker event rings, first-failure capture, and a serialized Vulkan execution bridge. The fallback lane-simulated path remained available and explicit. Single-SGEMM behavior remained unchanged.

## 2) Audit findings

Classification against required audit areas:

1. **Worker thread creation/join** — implemented and tested in M10; M11 adds destroy-after-failure validation.
2. **Worker shared failure state** — implemented but under-tested for concurrent failures; M11 adds multi-failure stress and counter publication.
3. **First-failure-wins logic** — implemented but under-tested for races; M11 validates stability under dual-failure hook.
4. **Event ring writes under real threads** — implemented but under-tested for overflow race pressure; M11 adds real-thread critical overflow coverage.
5. **Event drain timing** — implemented and conservative (join then drain), under-tested for active-emitter failure timing; M11 adds failure-while-other-worker-active coverage.
6. **Output staging and commit** — implemented and tested for atomicity; M11 adds out-of-order completion pressure while preserving entry-order commit.
7. **Worker-local state reset** — implemented but under-tested across repeated mixed outcomes; M11 adds success/failure/success sequence coverage.
8. **Repeated batch execution on same runtime** — implemented but under-tested; M11 adds explicit regression test.
9. **Runtime destroy after success/failure** — success already covered broadly; M11 adds explicit failed-real-thread destroy safety case.
10. **Diagnostics publication** — implemented but under-tested for race/bridge counters; M11 adds diagnostics assertions and introduces failure count/stability fields for truthfulness.

## 3) concurrent failure race handling

M11 adds dual-failure stress via test hook (`PROM_BATCH_FLAG_TEST_DUAL_FAIL_FIRST_TWO`) and validates:

- failure terminal state remains deterministic,
- first-failure metadata remains stable (`first_failure_stable=1`),
- additional failures are counted (`failure_count`),
- output commit remains false.

## 4) event-ring overflow behavior

M11 validates real-thread critical ring overflow behavior under low capacity:

- overflow fails batch explicitly (`PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW`),
- overflow count is surfaced,
- output commit remains false,
- drain still runs and reports event drain count.

Diagnostics-only lossy stream behavior remains deferred; current implementation treats ring overflow as critical for this bridge path.

## 5) drain/join behavior

Bridge remains conservative by design:

- workers complete/join before event drain,
- no partial ring reads during worker writes,
- failure while another worker remains active still drains coherently,
- worker active mask is zero after drain completion.

## 6) output ordering behavior

M11 introduces deterministic delay hook (`PROM_BATCH_FLAG_TEST_DELAY_ENTRY0`) to force out-of-order internal completion opportunities in real-thread mode while preserving:

- staged per-entry writes,
- caller-visible commit only on full success,
- commit in entry-id order,
- no partial output on failure.

## 7) repeated batch reset behavior

M11 validates same-runtime sequence:

1. success,
2. failure,
3. success.

Per-batch state resets correctly (batch state, failure stage/detail, output commit), while lifetime behavior remains unchanged.

## 8) destroy-after-failure behavior

M11 adds explicit failed real-thread batch then destroy test and verifies clean destroy without crash.

## 9) diagnostics added/validated

Added diagnostics fields:

- `failure_count`
- `first_failure_stable`

Validated diagnostics truthfulness for:

- execution mode (`real_threads_serialized_vulkan`),
- real vs lane worker counts,
- serialized bridge flags/counters,
- max concurrent serialized entries (`<=1`),
- hardware parallelism claimed (`false`),
- worker judgment count (`0`),
- output commit semantics,
- failure metadata stability.

## 10) unchanged/deferred scope

Unchanged/serialized-conservative behavior:

- Vulkan execution remains serialized through one bridge mutex,
- workers still do not run judgment,
- workers still do not mutate Dominatus directly,
- immutable plan model remains central,
- single-SGEMM path semantics remain unchanged.

Explicitly deferred (unchanged):

- true multi-queue Vulkan execution,
- per-worker Vulkan queue/resource path,
- SPMC/MPMC queues,
- work stealing,
- lock-free queueing,
- parallel judgment,
- public event stream,
- performance tuning.

## 11) validation results

M11 coverage added under Marionette batch suite for:

1. concurrent dual-failure race stability,
2. real-thread critical ring overflow,
3. failure while another worker remains active,
4. out-of-order completion with ordered commit,
5. repeated success/failure/success reset,
6. destroy after failed real-thread batch,
7. worker restriction enforcement,
8. diagnostics truthfulness for serialized real-thread execution.

## Required summary deliverable

1. **What M10 implemented:** opt-in real worker threads plus serialized Vulkan bridge, preserving M6/M7/M8 boundaries.
2. **What M11 hardens:** race failure stability, overflow handling, drain safety, ordered commit under completion skew, repeated-run reset, failed-run destroy safety, and diagnostics truthfulness.
3. **What remains serialized/conservative:** Vulkan submission serialization, centralized planning, no worker policy/judgment, no worker Dominatus mutation.
4. **What remains deferred:** true multi-queue/parallel submission, queue topology expansion, stealing/MPMC/lock-free runtime, parallel judgment, performance work.
5. **How single-SGEMM remains unchanged:** no single-entrypoint semantic changes; M11 modifies only batch-path hardening and tests.

## Language/reference consistency note

This milestone changes native C/C++ runtime and Marionette native tests only. No `Language/reference` semantics were changed.
