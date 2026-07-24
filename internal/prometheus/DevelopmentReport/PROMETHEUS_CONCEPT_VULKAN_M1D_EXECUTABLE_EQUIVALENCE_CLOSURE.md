# Prometheus Concept/Vulkan M1D — executable equivalence closure

Status: **MEANINGFUL PROGRESSION — one private executable now links and invokes
the real handwritten and generated kernel-54 paths against the same admitted
live runtime, proves success-path result/trace/cleanup correspondence, passes
with Vulkan validation enabled on this machine, and preserves production ABI;
remaining closure is blocked by the lack of a narrow handwritten creation-path
seam for stage-by-stage partial-construction failure equivalence.**

Starting checkpoint: `c7d36e672611bfb398159ac855368e237b3949ea`

Starting worktree: clean porcelain before edits.

Expected commit for this pass: `concept-vulkan: prove executable equivalence`

## 1. Starting checkpoint and worktree state

- `git rev-parse HEAD` before changes: `c7d36e672611bfb398159ac855368e237b3949ea`
- `git status --short` before changes: empty
- repository: canonical `oct` checkout
- date of this pass: 2026-07-24

## 2. Scope and non-goals

This pass adds:

- one private executable conformance harness;
- one private trace vocabulary;
- one private build helper;
- one private generated-authority validation helper;
- conformance-only trace emission from the actual handwritten and generated
  kernel-54 paths.

It does not add:

- public headers;
- public exports;
- production routing through generated code;
- new Concept syntax;
- shader/package changes;
- Dominatus or SDSL-V boundary changes;
- Stage 3/4/5/6 behavior changes;
- Stage 7 work.

## 3. M1C authority inspected

Inspected and treated as current authority before implementation:

- `PROMETHEUS_CONCEPT_VULKAN_M1C_NATIVE_CONFORMANCE_CLOSURE.md`
- `PROMETHEUS_CONCEPT_VULKAN_M1_KERNEL54_COMPILER_VERTICAL.md`
- `Examples/Concept-Vulkan/kernel54_probe.concept`
- generated `C/H/MIR/map/manifest`
- `internal/conceptvulkan/conceptvulkan.go`
- `internal/prometheus/native/reactor_vulkan_ray_query.c`
- `PROM_CONCEPT_VULKAN_CONFORMANCE` adapter introduced by M1C
- native manifests/build fragments
- Stage 0 authority and living status

## 4. Exact handwritten authority

The exact handwritten kernel-54 authority remains:

- `prom_ray_create_compute_resources(...)`
- `prom_ray_query_triangle_scene_probe_impl(...)`

in `internal/prometheus/native/reactor_vulkan_ray_query.c`.

The first owns kernel-54 descriptor/pipeline/evidence creation for the
persistent scene. The second owns the actual warm probe dispatch/readback path.

## 5. Exact generated authority

The exact generated authority remains:

- `prom_concept_vulkan_kernel54_execute(...)`

in `internal/prometheus/native/reactor_concept_vulkan_kernel54.generated.c`,
regenerated from `internal/conceptvulkan/conceptvulkan.go` and the canonical
`.concept` source.

## 6. One-harness link design

Added:

- `internal/prometheus/native/reactor_concept_vulkan_m1d_trace.h`
- `internal/prometheus/native/Marionette/concept_vulkan_m1d_conformance.c`
- `tools/concept_vulkan_m1d/build.ps1`
- `tools/concept_vulkan_m1d/validate_generation.ps1`

The harness:

- compiles the normal production native C set under
  `PROM_CONCEPT_VULKAN_CONFORMANCE`;
- compiles the checked-in generated kernel-54 C;
- links them into one private executable:
  `out/prometheus/native/concept_vulkan_m1d/concept_vulkan_m1d_conformance.exe`;
- creates one admitted runtime;
- creates equivalent triangle scenes sequentially on that runtime;
- invokes the real handwritten adapter and the real generated adapter;
- records each path independently;
- compares result bytes and ordered semantic traces;
- repeats the pair twice on the same admitted runtime.

## 7. Proof that both implementations are invoked

The executable invokes:

- `prom_concept_vulkan_kernel54_handwritten_adapter(...)`
- `prom_concept_vulkan_kernel54_generated_adapter(...)`

The conformance object audit confirms both symbols are present in the
conformance object file. The private harness returns failure if either path is
not actually called or if its result diverges.

Observed live output:

```text
PASS iteration=1 handwritten-events=12 generated-events=14 evidence=1
PASS iteration=2 handwritten-events=12 generated-events=14 evidence=1
```

## 8. Conformance adapter design

The adapter remains in `reactor_vulkan_ray_query.c` under
`#ifdef PROM_CONCEPT_VULKAN_CONFORMANCE`.

- handwritten adapter: calls the real
  `prom_ray_query_triangle_scene_probe_impl(...)`;
- generated adapter: resolves the same scene/package/services/TLAS and calls
  `prom_concept_vulkan_kernel54_execute(...)`.

This preserves the real static scene lookup and runtime/package access without
making any private helper public.

## 9. Production exclusion proof

- normal production source lists in `native_manifest.json`,
  `native_sources_windows.cmd`, and `native_sources_linux.sh` remain unchanged;
- `reactor_concept_vulkan_kernel54.generated.c` is still not a production
  source;
- `PROM_CONCEPT_VULKAN_CONFORMANCE` appears only in the generated private header
  and the handwritten translation unit;
- `git diff` against the starting checkpoint shows no changes to
  `internal/prometheus/native/include` or `reactor_api.h`;
- `dumpbin /exports out/prometheus/native/prometheus_reactor.dll` contains no
  `concept_vulkan` or conformance-only symbols.

## 10. Package and entry identity

Both paths continue to use:

- package identity: `prometheus.core@1`
- entry identity: `kernel-54-default`

No package manifest, lock, kernel ID, or shader object changed.

## 11. Descriptor and resource correspondence

Both paths preserve:

- binding `0`: admitted TLAS, acceleration-structure read
- binding `1`: four-byte coherent storage evidence buffer, shader write
- dispatch dimensions `(1,1,1)`

The semantic difference is ownership timing:

- handwritten path keeps descriptor/pipeline objects persistent on the scene;
- generated path creates and destroys descriptor/pipeline objects per execute.

This is classified as:

- `INTENTIONAL CONFORMANCE-ONLY DIFFERENCE`

because production authority is still the handwritten route and the comparison
target is semantic kernel-54 behavior, not identical internal object lifetime.

## 12. Access and synchronization correspondence

Both actual paths record the same semantic core:

- package resolved
- evidence buffer prepared
- command allocated
- command begun
- TLAS read declared
- evidence write declared
- dispatch `(1,1,1)`
- command end
- submit and wait
- mapped observation read
- cleanup

The generated path additionally records descriptor and pipeline construction
inside the execute path. The handwritten path instead records
`PERSISTENT_RESOURCES` because those objects were created during scene build.

## 13. Dispatch correspondence

Both paths issue exactly one:

```text
vkCmdDispatch(1, 1, 1)
```

No dispatch dimension changed.

## 14. Handwritten ordered trace

Observed handwritten ordered trace vocabulary:

1. `PACKAGE`
2. `PERSISTENT_RESOURCES`
3. `EVIDENCE`
4. `COMMAND_ALLOCATE`
5. `COMMAND_BEGIN`
6. `TLAS_READ`
7. `EVIDENCE_WRITE`
8. `DISPATCH`
9. `COMMAND_END`
10. `SUBMIT_WAIT`
11. `OBSERVE`
12. `CLEANUP`

## 15. Generated ordered trace

Observed generated ordered trace vocabulary:

1. `PACKAGE`
2. `EVIDENCE`
3. `DESCRIPTORS`
4. `PIPELINE`
5. `COMMAND_ALLOCATE`
6. `COMMAND_BEGIN`
7. `TLAS_READ`
8. `EVIDENCE_WRITE`
9. `DISPATCH`
10. `COMMAND_END`
11. `SUBMIT_WAIT`
12. `OBSERVE`
13. `CLEANUP` (execute-owned resources)
14. `CLEANUP` (scene destroy)

## 16. Trace normalization rules

Applied normalization rules:

1. stable event names are compared, not pointer values or Vulkan handles;
2. scene-persistent handwritten resources are represented as one
   `PERSISTENT_RESOURCES` event;
3. execute-local generated descriptor/pipeline creation remains explicit and is
   not collapsed away;
4. cleanup is compared semantically as reverse initialized destruction, not by
   raw handle identity.

## 17. Complete trace comparison

| Difference | Classification | Reason |
| --- | --- | --- |
| handwritten `PERSISTENT_RESOURCES` vs generated `DESCRIPTORS` + `PIPELINE` | INTENTIONAL CONFORMANCE-ONLY DIFFERENCE | handwritten scene owns persistent compute resources; generated execute owns them lexically |
| generated has two cleanup events | PROVEN EQUIVALENT | execute-local cleanup plus later scene cleanup; both are initialized-only and reverse ordered |
| core dispatch/readback sequence | PROVEN EQUIVALENT | same kernel-54 semantics, ordering, and result |

No generated semantic defect was observed on the admitted live route.

## 18. Success result and observation comparison

Compared live fields in `PrometheusRayQueryProbeResult` by raw struct bytes:

- hit bit
- triangle count
- BLAS built
- TLAS built
- vertex device address field
- BLAS device address field
- TLAS device address field

Observed result:

- handwritten result bytes == generated result bytes

## 19. Success cleanup comparison

Observed cleanup facts:

- generated path destroys only execute-owned buffer/descriptor/pipeline/command/fence objects;
- handwritten path keeps scene-owned kernel-54 resources alive for the warm
  probe and releases them on scene destroy;
- both paths preserve the borrowed runtime and borrowed TLAS ownership boundary;
- repeated same-runtime execution passes twice without stale-state failure.

## 20. Failure-injection inventory

Safely exercised in this pass:

- null runtime/invalid scene handle rejection
- null output-pointer rejection

Safely available but not yet equivalence-closed:

- `PROM_TESTCFG_FAIL_BUFFER_ALLOC`
- broader runtime flags such as queue-submit / command-end / download-style
  failures in other reactors

## 21. Per-stage failure results

Compared and passing:

| Lane | Handwritten | Generated |
| --- | --- | --- |
| null runtime / invalid handle | `PROM_INVALID_HANDLE` | `PROM_INVALID_HANDLE` |
| null output result | `PROM_ERROR` | `PROM_ERROR` |

Not closed:

- stage-by-stage partial-construction failure parity across the actual
  handwritten kernel-54 setup path and generated execute path.

## 22. Partial-construction cleanup

Generated execute continues to perform initialized-only reverse cleanup on
failure because its single-function cleanup epilogue is unchanged.

The missing proof is the handwritten create-time kernel-54 path under the same
stage-by-stage failure schedule. That path is split across scene construction
and warm probe entry, and no existing narrow handwritten seam exposes just that
create-time kernel-54 mechanism for equivalent conformance injection.

## 23. Reverse and exactly-once cleanup

Success-path proof:

- generated execute: reverse cleanup proved by code inspection plus successful
  repeated runs;
- handwritten scene/probe: repeated runs plus scene destroy prove no obvious
  double-destroy or stale reuse on the admitted route.

Failure-path exact-once parity remains part of the unresolved blocker in
Section 44.

## 24. Borrowed-resource preservation

The same admitted runtime is reused across:

1. handwritten run 1
2. generated run 1
3. handwritten run 2
4. generated run 2

After each path, the harness re-queries `prom_vk_runtime_services` and requires:

- same `ray_query_state`
- unchanged `validation_error_count`

This passes.

## 25. In-flight resource protection

Both paths submit synchronously and wait before observation or scene destroy.
No in-flight resource is destroyed before wait completion on the observed live
route.

## 26. Error identity and ordering

Compared rejection order for the available safe lanes:

- null output pointer is rejected before path work
- invalid runtime/scene is rejected as invalid handle

Observed classifications matched between handwritten and generated adapters.

## 27. Repeated lifecycle result

Result: **PASS**

The same admitted runtime survives two handwritten/generated pairs in sequence
with stable result bytes and stable validation-error count.

## 28. Vulkan validation result

Result: **PASS**

Validation layers were available on this machine on 2026-07-24. Running the
conformance executable with `PROMETHEUS_VK_VALIDATION=1` succeeded, and the
harness-required `validation_error_count` remained unchanged after each path.

## 29. Public header comparison

Result: **PASS**

`git diff` against the starting checkpoint showed no changes under:

- `internal/prometheus/native/include`
- `internal/prometheus/native/reactor_api.h`

## 30. Export inventory comparison

Result: **PASS**

`go run ./tools/prometheus_stage0 -check` still reports:

- exported symbols: `84`

`dumpbin /exports out/prometheus/native/prometheus_reactor.dll` contained no
`concept_vulkan` or conformance-only exports.

## 31. ABI digest comparison

Result: **PASS**

`go run ./tools/prometheus_stage0 -check` still reports:

- canonical function-signature digest:
  `89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262`

## 32. Conformance-symbol production exclusion

Result: **PASS**

- conformance object contains handwritten/generated adapter symbols;
- production DLL export inventory does not;
- public headers do not mention them;
- production build files do not define `PROM_CONCEPT_VULKAN_CONFORMANCE`.

## 33. Source-to-MIR-to-C-to-handwritten mapping

| Concept source / MIR | Generated C | Handwritten authority |
| --- | --- | --- |
| `CreateMappedEvidenceBuffer` / `create_buffer` | `prom_vk_create_buffer` | `prom_vk_create_buffer` inside `prom_ray_create_compute_resources` |
| `CreatePackagePipeline` / `create_pipeline` | package module + `vkCreateComputePipelines` | same package module + `vkCreateComputePipelines` in scene setup |
| `BindDescriptor` / `bind_descriptor` | layout/pool/set/update | same in scene setup |
| `BeginCommands` / `begin_recording` | allocate + `vkBeginCommandBuffer` | `prom_ray_begin_command` |
| `DeclareAccess` / read-write edges | trace semantics only | same semantic read/write edges |
| `Dispatch` / `dispatch` | `vkCmdDispatch(1,1,1)` | `vkCmdDispatch(1,1,1)` |
| `SubmitAndWait` / `submit_wait` | fence submit/wait | `prom_ray_end_submit_and_free` |
| `ReadObservation` / `observe` | `memcpy` from mapped evidence | same |

## 34. Deterministic generation result

Result: **PASS**

`tools/concept_vulkan_m1d/validate_generation.ps1` runs two temporary
generations and verifies byte-identical outputs.

## 35. Stale-output rejection

Result: **PASS**

`tools/concept_vulkan_m1d/validate_generation.ps1` mutates a temporary
generated C file and verifies that `concept-vulkan check` fails with `CV3001`.

## 36. Shader/package/manifest/lock/kernel preservation

Result: **PASS**

Preserved:

- package identity `prometheus.core@1`
- kernel entry `kernel-54-default`
- descriptor contract
- checked shader package
- model locks and projections
- production kernel inventory

Only the generated conformance C and its manifest digest changed, because the
generated file now includes conformance-only trace hooks under the private
macro.

## 37. Dominatus boundary

Preserved. No Dominatus policy, admission, judgment, or progression authority
moved into Concept/Vulkan or the conformance harness.

## 38. SDSL-V boundary

Preserved. No shader-source, SPIR-V, or SDSL-V language capability changed.

## 39. Stage 3/4/5/6 preservation

Preserved.

- Stage 3 common runtime ownership unchanged
- Stage 4 handoff unchanged
- Stage 5 buffer mechanics reused unchanged
- Stage 6 deferred-plan boundary unchanged

## 40. Stage 7 deferral

Preserved. No Stage 7 model-progress extraction work was started.

## 41. RQ-M1 paused-state preservation

Preserved. Physical batching and diagnostic-image authority remain paused while
Concept/Vulkan equivalence work continues.

## 42. Complete validation matrix

| Lane | Result |
| --- | --- |
| starting checkpoint / clean worktree verification | PASS |
| M1C diff/report inspection | PASS |
| Concept/Vulkan Go tests | PASS |
| `concept-vulkan check` on checked-in outputs | PASS |
| deterministic double generation | PASS |
| stale-output rejection | PASS |
| source-map / manifest JSON | PASS |
| private conformance build helper | PASS |
| one executable linking both actual paths | PASS |
| live handwritten invocation | PASS |
| live generated invocation | PASS |
| success-path probe equivalence | PASS |
| ordered semantic trace comparison | PASS |
| repeated same-runtime lifecycle (2 pairs) | PASS |
| borrowed-runtime preservation | PASS |
| validation layers enabled run | PASS |
| production Windows native build | PASS |
| Stage 0 export/digest audit | PASS |
| public-header diff against start checkpoint | PASS |
| production export absence of conformance symbols | PASS |
| null-handle rejection equivalence | PASS |
| null-output rejection equivalence | PASS |
| stage-by-stage partial-construction failure equivalence | NOT RUN — missing narrow handwritten creation-path seam |
| `git diff --check` | PASS |

## 43. Known limitations

- the conformance trace is intentionally private and build-gated;
- the handwritten path’s compute-resource construction is scene-persistent,
  while the generated path is execute-local;
- this pass does not create a second handwritten implementation or public test
  seam.

## 44. Unresolved blockers

The remaining blocker is precise and narrow:

The real handwritten kernel-54 mechanism is split between:

- scene creation (`prom_ray_create_compute_resources`)
- warm probe execution (`prom_ray_query_triangle_scene_probe_impl`)

The current conformance adapter exposes only the probe path as one clean,
private callable unit. Exhaustive stage-by-stage partial-construction failure
equivalence would require a similarly narrow private seam for the handwritten
kernel-54 create-time path. Adding a broad public seam, rerouting production,
or duplicating the implementation would violate the assignment boundary.

## 45. M1 closure assessment

Assessment: **not yet full M1D success**

M1D removed the largest remaining gap by proving:

- one executable
- both actual implementations linked
- both actually invoked
- live success equivalence on admitted hardware
- repeated lifecycle reuse
- validation-layer clean run
- production ABI/export preservation

But M1D does not yet close the handwritten create-time partial-failure
equivalence proof.

## 46. Exact proposed M2 assignment

Before Concept/Vulkan M2 on kernel-55 physical batching, add one further
bounded closure slice:

```text
Concept/Vulkan M1E — handwritten create-path failure equivalence seam
```

Scope:

- one private, non-public seam exposing the handwritten kernel-54
  create-time mechanism for conformance injection only;
- no public ABI changes;
- no production rerouting;
- no duplicate implementation.

Only after that seam exists can full per-stage failure equivalence close
cleanly. Physical batch equivalence remains the subsequent M2.

## 47. Rollback boundary

Rollback removes:

- the private M1D trace header;
- the private M1D harness source;
- the private M1D helper scripts;
- conformance-only trace hooks in generated and handwritten files;
- this report and related status updates.

Rollback does not alter:

- production handwritten routing;
- public ABI;
- package/shader identity;
- Stage 3/4/5/6 authority;
- Dominatus or SDSL-V boundaries.
