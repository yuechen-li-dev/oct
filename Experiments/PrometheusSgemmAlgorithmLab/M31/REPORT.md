# Prometheus SGEMM Algorithm Lab — M31 (Native Upload-Only Dedicated Transfer Queue)

## 1) M30 handoff summary

M30 required the native path to:

- enable dedicated transfer queue usage only when independently useful,
- keep upload-only as the first shipped policy,
- enforce explicit transfer/compute synchronization and queue-family ownership,
- keep fixed-double slot ownership authoritative,
- and provide explicit fallback reason diagnostics.

## 2) Upload-only policy implemented

M31 implements **upload-only** transfer queue usage for staged A/B uploads:

- transfer queue path is considered only for `PROM_VK_PATH_STAGED_UPLOAD`,
- C readback stays on existing compute/readback path and is not moved to transfer queue,
- path selection remains in the judgment engine with explicit fallback reasons.

## 3) Fixed-double slot interaction

M31 preserves M29 slot authority:

- uploads/compute still flow through slot HFSM transitions and slot ownership checks,
- transfer-submit failure marks the active slot failed with per-slot diagnostics,
- async completion still finalizes slot `IN_FLIGHT -> CONSUMED -> EMPTY`,
- bounded WIP (`<= 2`) is unchanged by transfer-queue usage.

## 4) Synchronization and ownership strategy

For dedicated transfer upload-only path:

1. host writes to staged upload buffers are made visible to transfer copy (`HOST -> TRANSFER` barrier),
2. transfer copies A/B into device-local staged buffers on transfer queue,
3. transfer signals semaphore and releases queue-family ownership when families differ,
4. compute queue waits semaphore, acquires ownership, then dispatches SGEMM.

Async readiness now includes transfer completion via transfer-fence tracking (`m31_async_transfer_complete`) before reporting ready.

## 5) Queue detection logic

Initialization now classifies transfer queue support into:

- no dedicated transfer queue,
- dedicated transfer queue (transfer-capable, non-compute family),
- pseudo/shared transfer queue (test-path simulation with same family).

Diagnostics expose:

- transfer and compute family indices,
- whether families differ,
- dedicated availability,
- whether upload-only transfer path was actually used,
- reason-coded fallback when not used.

## 6) Diagnostics added

`PrometheusSgemmPolicyDiagnostics` now includes M31 fields for:

- transfer usage/policy selection,
- queue-family topology and handoff/wait counters,
- transfer fallback reason,
- transfer failure slot/reason,
- upload-only policy marker,
- async transfer completion state.

## 7) Tests added

Marionette M31 tests cover:

- no dedicated transfer queue fallback reason,
- pseudo/shared queue fallback reason,
- dedicated transfer queue enablement for qualifying upload-only staged shapes,
- async readiness requiring transfer completion when transfer queue path is active,
- transfer-submit failure surfacing slot-aware diagnostics.

## 8) Deferred scope (intentional)

M31 intentionally defers:

- transfer-queue C readback,
- upload+readback transfer policy,
- generalized multi-queue scheduler,
- triple buffering, allocator redesign, and broad async architecture changes.

## 9) Language/reference consistency note

This milestone modified native Vulkan/C code and diagnostics only; no Oct syntax or `Language/reference` contract inconsistencies were introduced in this pass.
