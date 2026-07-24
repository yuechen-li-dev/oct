# Prometheus Stage 3 — Vulkan runtime ownership extraction

## 1. Starting checkpoint and scope

Stage 3 started from 1f2fc2a5c95cbbefcb3dcc0b76751b752d5fed46
("prometheus: consolidate ABI and lifecycle vocabulary"). The starting branch was
main, tracking origin/main, with a clean worktree and the checkpoint present on
the expected remote branch.

This pass is a conservative ownership extraction. It establishes one concrete
internal owner for the already-proven common Vulkan lifetime and changes SGEMM to use
that owner through a narrow internal contract. It does not add a reactor framework,
change ABI, or redesign allocation, dispatch, synchronization, weights, slots, or
model behavior.

The external Gemma checkpoint remains unavailable. No live Gemma equivalence claim is
made.

## 2. Deferred-live boundary

The required-live Gemma wrapper self-test passes its skip detection, but the actual
checkpoint-backed lanes were not run because G4E2B_CHECKPOINT_ROOT is unavailable.
The same-session -7406 boundary, repeated MainTransformer1 topology, and all
payload-dependent Z-Image and Gemma claims remain exactly deferred. No payload,
checkpoint, binary, cache, image, or machine-specific path was added.

## 3. Pre-change ownership map

At the checkpoint, reactor_vulkan_sgemm.c contained the complete Vulkan
initialization and teardown path:

| Resource | Checkpoint owner and creation | Consumers | Destruction order |
| --- | --- | --- | --- |
| Instance, validation layer, debug messenger | SGEMM vk_runtime_init | SGEMM and exported diagnostics | Debug messenger, then instance |
| Physical device and properties | SGEMM runtime struct | SGEMM, buffer placement, diagnostics | Borrowed until device teardown |
| Logical device and queue-family selection | SGEMM vk_runtime_init | All Vulkan execution paths | After command pools |
| Compute/transfer queues | SGEMM runtime struct | SGEMM upload and dispatch | Implicit with device |
| Compute/transfer command pools | SGEMM vk_runtime_init | SGEMM command buffers; shared-service projections | Before device |
| Device properties, limits, memory properties, capability buckets | SGEMM runtime struct | SGEMM selectors and diagnostics | Struct storage; Vulkan parent lifetime |
| Ray-query device PFNs and cooperative/subgroup capability state | SGEMM runtime struct | Public diagnostics and reactor consumers | Struct storage; Vulkan parent lifetime |
| Shader package root/package | SGEMM creation and cleanup path | Package-backed SGEMM module creation and package API | Package before runtime allocation is freed |
| Query pool, descriptors, pipelines, command buffers, fences, semaphores, buffers | SGEMM | SGEMM only | Before common device teardown |

The architecture audit identified the instance, selected physical device, logical
device, queues, command pools, capabilities, and package as common; it identified
buffers, command buffers, descriptors, pipelines, and staging/readback objects as
SGEMM execution state. This classification is the basis for the extraction.

## 4. Extracted common resources

prom_vk_runtime in native/reactor_vulkan_runtime.h/.c now owns:

- instance creation and destruction;
- validation-layer request/availability state and debug messenger callback state;
- deterministic physical-device selection;
- queue-family selection, compute/transfer queue handles, and dedicated-transfer policy;
- required Vulkan 1.4 capability and extension validation;
- logical-device feature and extension enablement;
- compute and transfer command pools;
- device properties, limits, memory properties, timestamp capability, occupancy classes,
  cooperative-matrix state, subgroup capability state, and ray-query PFNs;
- the package root/package lifetime fields used by the existing package-backed path;
- idempotent common cleanup and package cleanup helpers.

The owner is concrete, internal, and explicitly constructed. No interface, global
service locator, registry, provider, factory, slot, session, lease, plan, or reactor
instance was introduced.

## 5. Resources deliberately left with SGEMM

SGEMM still owns all operation-specific execution resources and behavior:

- descriptor set layout, descriptor pool, descriptor sets, and ring descriptor sets;
- pipeline layout, shader modules, pipelines, and mutable pipeline instances;
- primary and ring command buffers;
- timestamp query pool and SGEMM timing state;
- submit/transfer fences and semaphore;
- direct and staged input/output buffers, arenas, placement state, and readback;
- async tasks, submission ring state, reduction state, controller state, diagnostics,
  and SGEMM policy/selector state;
- SGEMM command recording, barriers, dispatch geometry, submission, and readback.

Package path discovery/open remains coordinated by the SGEMM creation path because it
is still coupled to the public configuration-size gate and existing package failure
mapping. The package object and root storage are nevertheless owned and destroyed by
the common owner. This is a deliberate narrow retention, not a new abstraction.

## 6. Construction flow before and after

Before, SGEMM performed one monolithic construction sequence: instance, physical
device, capabilities, logical device, queues, command pools, validation messenger,
timestamp pool, descriptors, pipelines, and SGEMM command resources.

After:

1. SGEMM resolves the existing package configuration and initializes the concrete
   prom_vk_runtime owner.
2. The owner performs the unchanged Vulkan loader, instance, physical-device,
   queue-family, feature/extension, device, queue, and command-pool sequence.
3. SGEMM creates its timestamp query pool, validation debug messenger boundary,
   descriptor resources, pipelines, command buffers, fences, semaphore, and
   execution resources exactly as before.
4. The owner is exposed internally through typed fields, not through an exported ABI.

The SGEMM wrapper is only a compatibility coordinator for the existing construction
path; ownership of common Vulkan handles is in prom_vk_runtime.

## 7. Destruction and partial-failure flow

The after-flow is explicit and reverse-ordered:

1. SGEMM waits for device idle through the owner.
2. SGEMM retires/cleans async and reduction state.
3. SGEMM destroys buffers, pipelines, shader modules, pipeline layout, descriptors,
   query pool, command buffers, fences, and semaphore.
4. The owner destroys transfer and compute command pools.
5. The owner destroys the logical device.
6. The owner destroys the debug messenger and instance.
7. The owner destroys the package and frees its root storage.

Every owner cleanup operation checks its handle, nulls it after destruction, and is
safe to call again. If instance, device, or command-pool construction fails partway
through, prom_vk_runtime_cleanup releases only the handles successfully acquired
and preserves the existing failure result mapping. SGEMM-specific failure paths
retain their existing cleanup through the same outer cleanup sequence.

The focused partial-failure test exercises injected device-creation failure over
repeated cycles and directly checks idempotent owner/package cleanup. Existing
validation counters remain the available exactly-once observation seam; no telemetry
framework was added.

## 8. Exact production files changed

Production source files changed:

- internal/prometheus/native/reactor_vulkan_runtime.c — added common owner
  implementation.
- internal/prometheus/native/reactor_vulkan_runtime.h — added common owner contract.
- internal/prometheus/native/reactor_vulkan_sgemm.c — mechanically migrated common
  handle/capability use and made SGEMM teardown borrow the owner.
- internal/prometheus/native/reactor_vulkan_sgemm_internal.h — separated owner state
  from SGEMM state.
- internal/prometheus/native/reactor_batch.c — migrated device/physical-device/test
  flag access to the owner.
- internal/prometheus/native/native_manifest.json — registered the new production
  source.

Generated native source inventories were regenerated:

- internal/prometheus/native/native_sources_windows.cmd
- internal/prometheus/native/native_sources_linux.sh

Focused internal tests were updated only for owner field paths, explicit staged package
roots, and lifecycle seams:

- internal/prometheus/native/Marionette/reactor_prometheus_audit_tests.cpp
- internal/prometheus/native/Marionette/reactor_shader_registry_tests.cpp
- internal/prometheus/native/Marionette/reactor_stub_tests.cpp

No files were deleted.

## 9. Internal contract introduced

    typedef struct prom_vk_runtime prom_vk_runtime;

    VkResult prom_vk_runtime_init(prom_vk_runtime* runtime, uint32_t test_flags);
    VkResult prom_vk_runtime_enable_validation(prom_vk_runtime* runtime);
    void prom_vk_runtime_wait_idle(prom_vk_runtime* runtime);
    void prom_vk_runtime_cleanup(prom_vk_runtime* runtime);
    void prom_vk_runtime_destroy_package(prom_vk_runtime* runtime);

SGEMM receives typed access to the concrete owner through its private runtime
structure. The public headers, exported functions, bridge projections, and calling
conventions are unchanged.

## 10. Proof that device and queue selection are unchanged

The owner was extracted mechanically from the checkpoint initializer. The following
rules remain unchanged:

- loader and physical-device Vulkan API validation require Vulkan 1.4;
- physical devices are enumerated in the same order and the first device with a
  compute-capable family is selected;
- the first compute-capable family remains the compute family;
- a transfer-only family remains preferred when available, with the same forced
  shared/disabled test configurations;
- queue index zero is used for compute and transfer queues;
- the same required extensions, optional cooperative-matrix and ray-query extensions,
  feature chains, and device creation rules are used;
- the same queue-family indices are placed into the device queue-create infos;
- the same reset-capable command-pool flags and family indices are used.

The live Vulkan preflight preserved device identity and reported the RTX 3070 device,
subgroup size 32, and the same compute/arithmetic/basic/shuffle admission facts.

## 11. Proof that allocation and execution semantics are unchanged

No buffer creation call, usage flag, memory-property request, allocation size/range,
residency decision, placement key, staging policy, descriptor binding, pipeline
layout, shader module identity, dispatch geometry, barrier, queue submission, fence
wait, or readback operation was intentionally changed. The SGEMM code only changed
common handle access from the outer runtime to runtime->vulkan.

The live direct-path diagnostic produced computed values through the extracted
runtime, with the observed variant-4 final-column mismatch described in the
validation section. No shader source or SPIR-V was modified to mask that result.

## 12. ABI, generated, shader, and package preservation

The canonical authority check remained PASS:

- 84 header declarations matched 84 DLL exports;
- no missing or extra exports;
- signature digest remained
  89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262;
- 69 public structs and layouts remained unchanged;
- detail codes and constants remained unchanged;
- bridge projections remained derived from prometheus_bridge_abi.h;
- reactor_api.h and reactor_api.c remained canonical;
- package identity remained prometheus.core@1;
- kernel 68 remained kernel-68-default;
- kernel 69 remained kernel-69-default;
- -7406 evidence and the same-session characterization remained unchanged.

The native manifest check passed. No authoritative generated bytes, shader bytes,
hashes, package membership, or kernel identities changed.

## 13. Lifecycle tests and validation performed

PASS:

- canonical prometheus_authority.ps1;
- Stage 0 static authority check;
- required-live skip-detection self-test;
- native manifest parity check;
- focused Go Prometheus, Gemma wrapper, shader-package, native-manifest, and compiled
  model-lock tests;
- Windows native build, including reactor DLL, Marionette tests/benchmarks, and SDSL-V
  host;
- create/destroy lifecycle;
- explicit initialization failure-stage reporting;
- runtime failure-stage reporting;
- owner partial-construction/idempotent-cleanup test;
- package-root ownership/config-size gate test;
- Stage 0 ABI/detail snapshot test;
- Vulkan preflight on available NVIDIA hardware;
- git diff --check (whitespace clean).

FAIL:

- PrometheusM34bValidationEnabledProductionVariants exposed a package-backed
  variant-4 final-column result of zero where the CPU oracle expects 1.645833...
  The other tested production variants execute through the direct path, and no
  shader, pipeline, allocation, or numerical workaround was introduced. The
  failure is left visible rather than weakening the assertion.

SKIP:

- required-live Gemma lanes because the external checkpoint is unavailable.

NOT RUN:

- live Gemma equivalence;
- checkpoint-dependent Z-Image;
- fresh payload-backed allocation/teardown;
- Linux validation;
- broad slow wrapper lanes not required for this ownership-only change.

## 14. Observation limitations

The existing production validation surface exposes validation counters and return
status, but it does not provide a production object-destruction counter. The focused
owner seam therefore proves handle nulling, repeated cleanup safety, failure cleanup,
and SGEMM-before-owner teardown ordering structurally; it cannot independently count
driver-level destruction callbacks without invasive instrumentation. No invasive
instrumentation was added.

The current native architecture note contains an inconsistent historical statement
that the M40a instance request is Vulkan 1.3, while the current platform contract,
checkpoint code, and extracted owner require Vulkan 1.4. The note was not rewritten in
this stage; the inconsistency is surfaced for intentional documentation repair.

## 15. Live validation not performed

No live Gemma checkpoint-backed validation was performed. No fresh payload-backed
allocation/teardown or checkpoint-dependent Z-Image validation was performed. The
non-payload Windows Vulkan preflight used the staged canonical shader package and
reported the available NVIDIA device; that result is not a Gemma equivalence claim.

## 16. Rollback boundary

The rollback boundary is the single Stage 3 commit. Reverting it restores the
checkpoint's monolithic SGEMM Vulkan initializer, original private runtime layout,
original native source inventory, and original focused test setup. No public header,
generated authority, shader, package, payload, or external checkpoint needs rollback.

## 17. Remaining SGEMM coupling

SGEMM still coordinates package configuration/open, owns all execution resources,
creates pipelines and command buffers, records and submits work, owns readback and
timing, and exposes the existing public runtime entry points. These are intentional
SGEMM couplings. The common owner has no model, weight, slot, session, lease, plan,
registry, or reactor ownership.

The focused M34b live mismatch also remains coupled to the existing package-backed
variant route. It was not reinterpreted as a runtime extraction opportunity.

## 18. Exact Stage 4 candidate boundary

Stage 4 may examine only the proven SGEMM-to-reactor execution handoff: slot ownership,
weight preparation and required generation/hash propagation, and a bounded snapshot
contract if a second reactor genuinely consumes it. It must begin from fresh evidence
and must not introduce speculative slots, sessions, leases, weight registries, binding
snapshots, execution plans, reactor registries, factories, providers, or plugins in
this Stage 3 commit.

## Result

The common Vulkan runtime/device lifetime now has one concrete internal home and SGEMM
uses it through an explicit private contract. Because the available package-backed
M34b lane remains a visible FAIL and package configuration/open remains coordinated
by SGEMM, this checkpoint is reported as:

PROMETHEUS STAGE 3: MEANINGFUL PROGRESSION

## 19. M34b regression-classification addendum

This addendum classifies the visible M34b witness before any Stage 4 work.  No
production source, shader, generated authority, package, or test expectation was
changed during this pass.

### Exact comparison

The canonical command was:

```powershell
& .\out\prometheus\native\marionette_tests.exe PrometheusM34bValidationEnabledProductionVariants
```

The test itself requests existing Vulkan validation with
`PROMETHEUS_VK_VALIDATION=1`.  It uses `m=3`, `n=17`, and `k=7`, and exercises
production variants 3 (SRT), 4 (B2x2), and 5 (A2x4).  Each checkpoint was built
from its own worktree and package output.  The Stage 2 package was rebuilt at
`out/prometheus/native/shaders`; the Stage 3 package was rebuilt at
`out/prometheus/native/SerialCanonical/shaders`.  Their `manifest.json` SHA-256
values were identical:
`A110CEBC3ABC737BB450C53D5F2A5ED46CDD7C48DFD300688A0AB567A64EF19C`.

| checkpoint | repetitions | result | variant / cells | expected | observed |
| --- | ---: | --- | --- | ---: | ---: |
| Stage 2 `1f2fc2a5c95cbbefcb3dcc0b76751b752d5fed46` | 3 | FAIL | 4 / final column of all three rows | 1.6458333730697632 | 0 |
| Stage 3 `2bb06095a1755a30f5d5ab8f140838f59ee51be4` | 3 | FAIL | 4 / final column of all three rows | 1.6458333730697632 | 0 |

Variants 3 and 5 completed through the direct path.  Every run produced exactly
three assertion failures at the CPU-oracle comparison and no Vulkan validation
warning or error was reported by the exercised test surface.  The result is
therefore **INHERITED DETERMINISTIC FAILURE**, not a Stage 3 regression.  The
related `PrometheusM34bA2x4UsesCanonicalDispatchFootprint` witness is also
inherited: both checkpoints report the same five geometry assertions (expected
Z/row/column coverage `1/2/4/16/32`, observed `2/4/16/32/128`).  These are
unresolved behavioral authorities and were deliberately not repaired here.

### Command-pool extraction audit

The compute and optional transfer pools remain proven device mechanisms.  The
extracted owner preserves the selected compute and transfer queue-family indices,
gets queue index zero from those same families, and creates each pool with exactly
`VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT`.  SGEMM, FFT, and fused
reduction continue to allocate and free their own command buffers from the
borrowed pool handles; SGEMM retains all command-buffer reset/reuse, submission
ordering, fence association, external synchronization assumptions, and execution
state.  No thread ownership or scheduling policy moved into `prom_vk_runtime`.

SGEMM first waits idle, then destroys its pipelines, descriptors, query pool,
fences, semaphores, buffers, and command buffers before calling owner cleanup.
The owner then destroys transfer pool, compute pool, device, debug messenger, and
instance, and nulls every handle.  The focused
`PrometheusVulkanRuntimeOwnerPartialFailureAndIdempotentCleanup` test passed.
Partial creation failure therefore follows the same reverse cleanup path.  The
pools remain in `prom_vk_runtime`: their creation is device/queue-family-bound,
their allocation clients are already multiple mechanisms, and this extraction did
not change their concurrency, reset, reuse, or lifetime semantics.

### Dominatus boundary

`reactor_vulkan_runtime.[ch]` contains only Vulkan construction facts, immutable
capabilities, handles, dispatch loading, validation counters, and deterministic
cleanup bookkeeping.  It adds no semantic lifecycle state machine, shadow
blackboard, utility judgment, adaptive policy, admission decision, hysteresis,
or scheduler.  Capability-state fields report Vulkan facts only; policy remains
in the established Dominatus and SGEMM paths.

**Dominatus decides and coordinates. Vulkan mechanisms execute and report facts.**

### Validation after classification

PASS: detached Stage 2 and current Stage 3 Windows native builds; three M34b
repetitions at each checkpoint; focused lifecycle, preflight, package-root, and
runtime-owner cleanup tests (six of seven combined focused tests); canonical
repository authority; Stage 0 static authority; required-live skip-detection
self-test; `go test ./internal/prometheus/...`; and `git diff --check`.

FAIL (preserved, inherited): M34b production-variant witness and its narrower
A2x4 footprint witness as described above.  SKIP: required-live Gemma because
the checkpoint is unavailable.  NOT RUN: live Gemma equivalence,
checkpoint-dependent Z-Image, fresh payload-backed allocation/teardown, and
Linux validation.  Stage 4 has not begun.
