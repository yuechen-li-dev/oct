# P8a Report — Vulkan Reactor Reuse Protocol Port

## Scope completed

P8a ports the M7 reuse protocol into the Vulkan C SGEMM execution path by moving SGEMM execution buffers from per-call allocation/destruction into runtime-owned persistent state and enforcing explicit reuse rules.

## Runtime-owned objects now reused

The SGEMM path now keeps these objects in runtime state across calls:

- `reusable_a`, `reusable_b`, `reusable_c` (`VkBuffer` + `VkDeviceMemory` + mapped pointers)
- descriptor bindings for those buffers (updated only when buffer identity changes)
- command recording validity (record once per stable shape/binding regime, then resubmit)
- submission in-flight state (`in_flight_submit`), guarded by fence status before reuse

Persistent objects already owned by runtime before P8a and retained:

- descriptor pool / descriptor set
- command buffer
- fence

## M7 invariants extracted and Vulkan mapping

1. **No reuse while in flight**
   - M7: disallow logical in-flight reuse.
   - Vulkan mapping: `in_flight_submit` + `vkGetFenceStatus(...)` gate; SGEMM rejects reuse with `PROM_DETAIL_REUSE_IN_FLIGHT` if completion precondition is not met.

2. **Full reset before persistent command reuse**
   - M7: persistent reuse requires full reset.
   - Vulkan mapping: `vkResetCommandBuffer(...)` occurs whenever command recording is invalidated; no partial-recording reuse path exists.

3. **Refresh bindings when buffer identity changes**
   - M7: same-shape but changed buffer identity still requires update.
   - Vulkan mapping: shape-triggered buffer recreation invalidates `descriptor_bindings_valid`, forcing `vkUpdateDescriptorSets(...)` before submit.

4. **Shape changes invalidate command + binding assumptions**
   - M7: shape change invalidates both command recording assumptions and binding metadata.
   - Vulkan mapping: shape mismatch destroys/recreates reusable buffers and clears both `descriptor_bindings_valid` and `command_recording_valid`.

5. **Reject illegal reuse when preconditions are unmet**
   - M7: reject/surface unsafe reuse.
   - Vulkan mapping: in-flight precondition failure returns `PROM_ERROR` with explicit stage/detail diagnostics.

6. **Make stale-state causes observable**
   - M7: diagnostics must be explicit.
   - Vulkan mapping: new detail code `PROM_DETAIL_REUSE_IN_FLIGHT` and stage-preserving failure reporting.

## Tests added/updated

- Extended runtime failure matrix with `PROM_TESTCFG_SKIP_SUBMIT_WAIT` to prove explicit in-flight reuse rejection.
- Added SGEMM protocol regression test covering:
  - repeated same-shape execution
  - same-shape execution with different host buffer identities
  - shape-change execution with correct invalidation/rebuild

## Deferred to later P8 milestones

- device-local staging path integration
- tiled/blocked SGEMM strategy work
- asynchronous/non-blocking submission design
- broader reactor architecture changes beyond this reuse protocol port

## Inconsistency surfaced

M7 models generic protocol semantics (pure Oct state machine), while this implementation maps those invariants onto concrete Vulkan resource lifetime and synchronization behavior. This remains an intentional abstraction difference, not a protocol contradiction.
