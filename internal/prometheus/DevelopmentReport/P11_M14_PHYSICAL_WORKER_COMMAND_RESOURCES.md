# P11 M14 — Physical Per-Worker Vulkan Command Resources (Serialized Submit)

## 1) M13 handoff summary

M13 introduced per-worker command/resource ownership diagnostics and ownership checks while keeping queue submission serialized and explicitly not claiming hardware parallelism.

M13 modeled per-worker command pool/command buffer/fence identity with deterministic per-worker IDs plus wrong-owner rejection checks, but did not allocate per-worker Vulkan command objects.

## 2) Audit findings

Classification against the M14 audit checklist:

1. **Command pool create/reset/destroy**
   - Prior state: one runtime-level command pool for single-SGEMM path, batch path only diagnostic IDs.
   - M14: **safe to split now** in real-thread serialized mode; per-worker pools created with `VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT`, destroyed in batch cleanup.

2. **Command buffer allocate/record/reset**
   - Prior state: one runtime-level command buffer used by single-SGEMM path; batch path had no physical worker command buffers.
   - M14: **safe to split now**; per-worker primary command buffers allocated from each worker pool, reset/recorded worker-locally.

3. **Fence ownership reset/wait behavior**
   - Prior state: one runtime-level submit fence for single-SGEMM path; batch path only diagnostic fence IDs.
   - M14: **safe to split now**; per-worker fences are reset/waited by worker owner inside serialized submit gate.

4. **Current command buffer reused by batch workers**
   - Prior state: no physical batch recording; diagnostic-only ownership.
   - M14: **fixed**; each worker uses its own command buffer handle when physical mode is active.

5. **Per-worker command pools from current queue family**
   - Prior state: deferred.
   - M14: **implemented** using runtime compute queue family index.

6. **Per-worker command buffers independently recordable**
   - Prior state: deferred.
   - M14: **implemented under serialized submit gate** (independent handles, still serialized by bridge policy).

7. **Per-worker fences independently reset/waited**
   - Prior state: deferred.
   - M14: **implemented under serialized submit gate**.

8. **Destroy path cleanup ordering**
   - Prior state: no per-worker physical command resources in batch path.
   - M14: **implemented**; batch cleanup destroys worker fence then command pool for every created worker resource.

9. **Failure path cleanup ordering**
   - Prior state: first-failure-wins and drain semantics were present; no physical worker command resources.
   - M14: **implemented** for per-worker command resources; create failures and submit-path failures flow to shared failure path and batch cleanup.

10. **Interaction with serialized bridge**
   - Prior state: serialized mutex bridge in place.
   - M14: **preserved**; physical worker command resources execute through same serialized gate, max concurrent serialized entries remains bounded to <=1.

## 3) What became physical per-worker

When running real-thread serialized mode and Vulkan device/queue are available:

- per-worker `VkCommandPool`,
- per-worker `VkCommandBuffer`,
- per-worker `VkFence`,
- per-worker local reset/record/submit/wait counters,
- per-worker physical-valid diagnostics.

Wrong-owner checks remain active and unchanged.

## 4) What remains diagnostics-only / deferred

If Vulkan device/queue is unavailable in the current runtime, M14 keeps diagnostics-only per-worker IDs/mapping and reports simulated-per-worker resource mode.

Still deferred (unchanged scope):

- true multi-queue parallel submit,
- per-worker independent queue execution,
- work stealing,
- SPMC/MPMC queues,
- lock-free queueing,
- parallel judgment,
- cross-worker arena borrowing,
- public event stream,
- performance tuning.

## 5) Serialized submit bridge behavior

M14 does not change bridge semantics:

- submit path remains serialized,
- maximum concurrent serialized submit remains <= 1,
- hardware parallelism claim remains false,
- serialized enter/wait/execution counters remain truthful.

Physical per-worker command resources are ownership hardening inside serialized submit, not queue-level parallel execution.

## 6) Diagnostics added / validated

Added diagnostics fields:

- `resource_creation_failure_count`,
- per-worker `worker_command_pool_valid`,
- per-worker `worker_command_buffer_valid`,
- per-worker `worker_fence_valid`,
- per-worker `worker_reset_count`,
- per-worker `worker_record_count`.

Existing counters retained and still used:

- per-worker submit/wait/in-flight,
- ownership violation count,
- serialized bridge counters,
- worker resource mode + queue mapping/topology diagnostics.

## 7) Failure / cleanup behavior

M14 adds explicit batch failure detail codes for command-resource path:

- `PROM_DETAIL_BATCH_COMMAND_RESOURCE_CREATE_FAILED`,
- `PROM_DETAIL_BATCH_COMMAND_RECORD_FAILED`,
- `PROM_DETAIL_BATCH_FENCE_RESET_FAILED`,
- `PROM_DETAIL_BATCH_FENCE_WAIT_FAILED`,
- `PROM_DETAIL_BATCH_QUEUE_SUBMIT_FAILED`.

Contracts preserved:

- first-failure-wins,
- no output commit on failure,
- drain/cleanup remains safe,
- per-worker command resources are always destroyed in batch cleanup.

## 8) Tests added

Added M14 Marionette coverage for:

1. physical-mode-or-simulated-mode reporting with serialized submit invariants,
2. physical validity + distinct identities when physical mode is active,
3. fence-reset failure injection behavior in physical mode,
4. compatibility of fallback diagnostics-only mode when physical allocation is unavailable.

Also updated M13 resource diagnostics test to accept explicit simulated fallback mode when physical allocation is not available in the environment.

## 9) Unchanged / deferred scope

Unchanged:

- single-SGEMM execution path behavior,
- SGEMM selection semantics,
- judgment boundary (workers still do not run judgment or mutate Dominatus),
- typed arena design.

Deferred as above: true multi-queue and scheduler/performance expansions remain out of scope for M14.

## Required summary deliverable

1. **What M13 modeled with deterministic IDs**
   - Per-worker ownership identity diagnostics for command pool/buffer/fence + ownership violation checks.
2. **What is physically split in M14**
   - Worker-local Vulkan command pool/buffer/fence in real-thread serialized mode when Vulkan device/queue is available.
3. **What remains diagnostics-only/deferred**
   - Fallback simulated-per-worker ownership when physical resources cannot be created (for example unavailable Vulkan device/queue), plus all multi-queue/parallel scope.
4. **Why submit remains serialized**
   - M14 is ownership hardening only; queue submit still passes through one serialized bridge and makes no hardware parallelism claim.
5. **How single-SGEMM behavior remains unchanged**
   - M14 modifies only batch-path worker command-resource ownership/diagnostics/failure handling; single-SGEMM path is untouched.

## Language/reference consistency note

This milestone changes native runtime/tests/docs only and does not change Oct language semantics. No `Language/reference` contract modifications were needed.
