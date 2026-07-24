# Prometheus Stage 5 — mechanical allocation and cleanup substrate

## 1. Starting checkpoint and scope

Stage 5 began from the clean Stage 4 checkpoint
`93e568d0aecfa97d3e6e97c87ed520751e5c1bae`
(`prometheus: clarify resource state and execution handoff`). `HEAD` resolved
to that exact commit, `origin/main` contains it and the prior Stage 4
checkpoint, and the worktree was clean before edits.

This is a deliberately small extraction of repeated Vulkan **buffer**
construction mechanics. It neither creates a resource architecture nor changes
an allocation decision.

## 2. Architectural constitution

**Dominatus decides and coordinates. Vulkan mechanisms execute and report
facts.** Dominatus remains the only authority for admission, path/compute,
layout/precision, transfer, buffering, lease, and commitment decisions. The
new private functions contain no Dominatus input, semantic transition, policy,
blackboard, allocation reuse, residency, or lifecycle state.

## 3. Deferred-live boundary

`G4E2B_CHECKPOINT_ROOT` is not configured. Required-live Gemma, fresh
checkpoint-backed allocation/teardown, and checkpoint-dependent Z-Image were
not rerun. This ordinary local-payload setup limitation is not an
architectural conclusion. Linux remains unclaimed.

## 4. Pre-change allocation inventory

All production Vulkan buffer allocation routes already entered
`native/reactor_vulkan_common.c`; no image route was proven equivalent.

| Constructor / consumers | Semantic owner and role | Selection and mapping | Cleanup |
| --- | --- | --- | --- |
| `prom_vk_create_buffer`; SGEMM, fused reduction/transformer, FFT, batch, model block, ray query | Each consumer's typed input/output, staging/readback, temporary, activation, or arena-backed record | caller supplies required flags; requirement bits choose a type; offset zero; caller selects mapped/unmapped | caller-owned reverse cleanup through `prom_vk_destroy_buffer` |
| `prom_vk_create_buffer_for_placement`; SGEMM direct/staged resources | SGEMM A/B/C and upload/readback typed arenas | existing placement policy chooses type; offset zero; mapping stays explicit at the call | existing helper cleanup on every post-create failure |
| `prom_vk_create_buffer_shared_between_families`; model-block M2 windows | model-block weight windows, not SGEMM weights | existing concurrent two-family sharing and required flags; offset zero; existing mapping input | existing helper cleanup on every post-create failure |
| `prom_vk_create_device_address_buffer`; ray-query AS storage/build inputs/scratch | ray-scene and acceleration-structure ownership | existing required flags plus the existing device-address allocation `pNext`; offset zero; caller-selected mapping | existing helper cleanup on every post-create failure |

`prom_vk_buffer` continues to record only physical mechanics: `VkBuffer`,
`VkDeviceMemory`, requested size, usage/sharing facts, requirement alignment,
selected memory type/property facts, zero binding offset, and an optional
mapped pointer. No logical identity, content hash, generation, binding,
residency, lease, descriptor, or role is stored there.

## 5. Proven duplication and non-duplication

The four constructors repeated the same concrete sequence: initialize a
`prom_vk_buffer`; populate `VkBufferCreateInfo`; call `vkCreateBuffer`; query
requirements; set alignment; allocate exactly `requirements.size`; bind at
offset zero; and optionally map at offset zero for the requested buffer size.
They all use `prom_vk_destroy_buffer`, whose order is unmap, destroy buffer,
free memory, nulling each handle.

They are not policy-equivalent in memory type selection: ordinary/device-address
constructors use caller-supplied required flags; SGEMM placement uses its
placement selector; device-address allocation adds the device-address `pNext`;
and M2 weight windows use concurrent queue-family sharing. Those distinctions
remain visible in the named wrappers and were not normalized.

## 6. Classification

| Category | Stage 5 treatment |
| --- | --- |
| Common Vulkan mechanics | extracted: create, requirements discovery, allocation, zero-offset bind, optional mapping |
| Allocation policy | retained: required flags, placement selection, test-force rules, queue sharing, device-address allocation flags, mapping request, cleanup-on-failure choice |
| Semantic resource state | retained outside: SGEMM typed roles/arenas, model weights/windows, activation roles, generations, hashes, bindings, leases, descriptors, and execution handoff |

## 7. Extraction decision and concrete substrate

The evidence proves a shared, bounded substrate. Two private concrete
functions in `reactor_vulkan_common.c` now own it:

- `prom_vk_buffer_create_mechanics`: initializes the physical record, creates
  the buffer, discovers requirements, and records alignment.
- `prom_vk_buffer_allocate_bind_map_mechanics`: allocates using the caller's
  already-selected type and optional `pNext`, binds at zero, and maps only when
  the caller supplies the existing mapped request.

The existing public-internal constructor names remain the production call
sites. There is no interface, registry, allocator policy, suballocation,
pooling, cache, or new ownership state machine.

## 8. Exact migrated and deliberately retained paths

Migrated constructors are `prom_vk_create_buffer`,
`prom_vk_create_device_address_buffer`,
`prom_vk_create_buffer_shared_between_families`, and
`prom_vk_create_buffer_for_placement`. Therefore their existing SGEMM, batch,
FFT, fused-reduction/transformer, model-block, and ray-query consumers now use
the substrate through their existing typed wrappers.

Not migrated: image allocation (no equivalent current production evidence),
acceleration-structure objects (their backing buffers only use the
device-address wrapper), descriptors/pipelines/command buffers/fences,
submission/readback, and arena semantic records. These have different
mechanics or are outside allocation scope.

## 9. Allocation, mapping, and cleanup preservation

For every migrated wrapper, requested size, Vulkan requirement size and
alignment, requirement memory-type bits, selection function and input flags,
selected type index, property flags, allocation size, and binding offset remain
the same expressions as before. Allocation size remains
`requirements.size`; binding remains `0u`; no alignment or memory flag changed.

Mapping remains `vkMapMemory(memory, 0u, requested_size, 0u, ...)` only when
the pre-existing `map_memory` input is nonzero. No flush or invalidate was
present before or after this pass. Coherent/cached/device-local choices stay
with the caller's existing selection rule.

The ordinary wrapper deliberately retains its prior caller-owned partial
cleanup behavior. Device-address, concurrent-family, and placement wrappers
still call `prom_vk_destroy_buffer` after every post-create failure. Final
cleanup remains unmap, destroy buffer, free memory; successful cleanup nulls
the mapped pointer and both handles, so repeated cleanup is safe.

## 10. Dominatus, Stage 3, and Stage 4 preservation

The functions have no decision inputs beyond already-resolved mechanical Vulkan
arguments. They neither observe nor mutate Dominatus. Allocation success or
failure continues to flow through existing callers as observed Vulkan facts.

`prom_vk_runtime` still owns the instance, device, queues, command pools, and
common destruction. SGEMM still destroys its buffers and execution resources
before runtime cleanup; SGEMM retains command buffers, descriptors, pipelines,
fences, timing, submission, synchronization, dispatch, and readback.

The private Stage 4 `prom_sgemm_execution_handoff` was not changed. Its
committed visible Dominatus snapshot remains the bounded execution authority;
this stage does not reopen or mutate it. SGEMM A/B/C/upload arena meanings,
descriptor bindings, variants, and slots remain typed and local.

## 11. ABI, generated, shader, package, and numerical preservation

No exported ABI declaration, layout, detail code, calling convention, shader,
SPIR-V, generated projection, manifest, lock, package membership, or kernel
identity changed. Post-check values remain 84 exported symbols, signature
digest `89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262`,
69 public structs, package `prometheus.core@1`, kernel 68
`kernel-68-default`, and kernel 69 `kernel-69-default`.

The preserved SHA-256 snapshots are native shader manifest
`8EC65F4E3D81C52EFFC94826DD6460A9B240416E711EB2738F414063376F3AD8`,
generated shader-ID projection
`CDBF2DF77306C82CDC1705B9E73992D8249294435F6625CF6C2DAB86BD5C9D3C`,
and model lock
`71EF202B4E34B562BD0D8526D1E0C674640CBBA02FB7C484D8DADF981C8B226E`.
Their paths are absent from the Stage 5 diff. The established staged package
manifest reference remains
`A110CEBC3ABC737BB450C53D5F2A5ED46CDD7C48DFD300688A0AB567A64EF19C`.

The canonical M34b result is preserved as an expected inherited failure:
`m=3`, `n=17`, `k=7`, selected variant 4, final column of all three rows,
expected `1.6458333730697632`, observed `0` (three assertions). The related
A2x4 footprint witness still has the five expected failures: expected
`1/2/4/16/32`, observed `2/4/16/32/128`.

## 12. Tests and validation

| Lane | Result |
| --- | --- |
| checkpoint, clean worktree, remote checkpoints | PASS before edits |
| repository authority and Stage 0 static authority | PASS |
| required-live skip-detection self-test | PASS |
| Windows native build | PASS (pre-existing compiler warnings only) |
| `PrometheusStage5MechanicalBufferAllocationAndCleanupPreserveMappedState` | PASS on real Vulkan: mapped/unmapped, alignment, flags, zero offset, repeated cleanup |
| runtime partial-failure/idempotent cleanup | PASS |
| memory placement selection | PASS |
| Stage 4 execution-handoff preservation | PASS |
| ABI/detail/layout snapshot | PASS |
| M34b production variants | EXPECTED INHERITED FAIL; exact fingerprint preserved |
| A2x4 footprint witness | EXPECTED INHERITED FAIL; exact fingerprint preserved |
| live Gemma, checkpoint Z-Image, fresh payload teardown | SKIP / NOT RUN; payload unavailable |
| Linux | NOT RUN / unclaimed |

The focused Stage 5 witness uses the existing real runtime, not a mock Vulkan
universe. It observes requested size, requirement alignment, selected required
flags, zero binding offset, mapped/unmapped state, and repeated final cleanup.
It cannot observe driver callbacks or the unstored runtime requirement size
without invasive instrumentation; static expression equivalence and Vulkan
validation are the honest remaining evidence.

## 13. Files, rollback, remaining coupling, and Stage 6 boundary

Production changed: `internal/prometheus/native/reactor_vulkan_common.c`.
Test changed: `internal/prometheus/native/Marionette/reactor_shader_registry_tests.cpp`.
This report, the evidence index, and reviewer handoff are the only documentation
changes. No file was deleted.

Reverting the one Stage 5 commit restores four local mechanical sequences and
removes the focused witness; it does not touch public ABI, generated bytes,
shaders, packages, payloads, runtime ownership, Dominatus, or execution
semantics.

Remaining intentional duplication/coupling is policy and semantic: the four
named selection/cleanup wrappers, SGEMM placement/arena logic, model weight
windows, ray AS object lifetime, and all operation-specific execution
resources. The exact Stage 6 candidate boundary is model execution planning:
a closed Z-Image stage-order plan and Gemma orchestration extraction, preserving
generated identities, dispatch order, residency, allocation ceilings, and
numerical authorities. Stage 6 is not begun here.

## Result

PROMETHEUS STAGE 5: SUCCESS
