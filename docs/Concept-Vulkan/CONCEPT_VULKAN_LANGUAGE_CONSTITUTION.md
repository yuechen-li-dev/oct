# Concept/Vulkan language constitution

Status: **normative M1 constitution; bounded kernel-54 compiler implemented; production remains handwritten**

Date: 2026-07-24

## 1. Identity and authority

The language is **Concept/Vulkan**. Its source extension is `.concept`, and a
source unit begins with:

```concept
profile Vulkan;
```

Concept/Vulkan is a real, imperative, C-family/Concept-like systems-language
profile for host-side Vulkan mechanisms. It is Concept-compatible in direction;
the current experimental Concept compiler is not its production dependency and
need not accept the M1 surface.

Concept/Vulkan is implemented through the working Go-based Oct/SDSL-V compiler
lineage. It initially supports only the surface proven necessary by current
Prometheus Vulkan mechanisms. It is not a general Concept revival, a declarative
reactor recipe, a Vulkan framework, or a policy language.

The permanent responsibility split is:

```text
Dominatus decides and coordinates.
Concept/Vulkan mechanisms execute committed work and report facts.
SDSL-V expresses shader-side computation.
Prometheus exposes semantic GPU capabilities and consumes generated C/H.
```

Generated C/H is the first backend and the native-build boundary. Prometheus
must not require a future general Concept compiler at downstream build time.

## 2. Governing rule

```text
Essential decisions remain explicit.
Mechanical consequences are generated.
```

An **essential decision** is a choice that can change correctness, ownership,
compatibility, admission, observable behavior, or the Vulkan contract. Current
examples are:

- the already-admitted package variant;
- logical resource role, byte range, usage, sharing, and required memory
  properties;
- whether a resource is imported, borrowed, or owned;
- descriptor binding contract and any binding not fixed by package metadata;
- dispatch dimensions when they are semantic inputs;
- declared reads, writes, host transfers, and required ordering;
- a non-derivable stage/access/layout override;
- the existing Prometheus error/detail mapping and observation boundary.

A **mechanical consequence** follows deterministically from those decisions and
repository-owned metadata. Current examples are:

- zero-initialized Vulkan create-info structures and `sType` fields;
- requirements queries, exact allocation-size propagation, zero-offset binding,
  and map/unmap calls through the Stage 5 helpers;
- descriptor pool sizing, allocation, and writes implied by typed bindings;
- a barrier whose stage/access pair is uniquely implied by adjacent declared
  accesses;
- command-buffer begin/end, pipeline/set binding, submission, fence wait, and
  reverse cleanup;
- failure branches that preserve an existing Prometheus error and cleanup order.

Generation must not erase an essential decision or invent policy.

## 2.1 M1 source naming

Concept/Vulkan follows the C++/Concept syntactic lineage. User-facing
functions, compiler-known operations, and type names use `PascalCase`; parameters
and locals use `camelCase`. Existing C ABI spelling is retained only at the
backend boundary, and MIR opcodes remain compiler snake_case. The canonical M1
source uses `Execute`, `CreateMappedEvidenceBuffer`, `BindDescriptor`,
`BeginCommands`, `DeclareAccess`, `Dispatch`, `SubmitAndWait`, and
`ReadObservation`. M1 enforces function/local naming in its bounded parser;
this is not a general style-lint subsystem.

## 2.2 M1 declaration grammar

Concept/Vulkan uses C++-shaped declarations, deliberately distinct from Oct,
SDSL-V, and Rust:

```concept
Result<ProbeEvidence, PrometheusError> Execute(
    borrow MechanismContext context,
    unsafe imported borrow AccelerationStructure admittedTlas)
{
    owned MappedEvidenceBuffer evidence =
        CreateMappedEvidenceBuffer(context)?;
}
```

Return types precede function names; parameter and local types precede their
names. `fn`, `name: Type`, `-> ReturnType`, `let`, and `var` are rejected;
there are no compatibility aliases. `borrow`, `owned`, `unsafe`, `imported`,
and `move` remain explicit ownership/boundary vocabulary, while `?` retains its
bounded fallibility meaning. `MappedEvidenceBuffer` is the narrow source
spelling of M1's existing mapped host-visible evidence-buffer capability;
`ComputePipeline`, `DescriptorSet`, `CommandRecording`, and `Submission` are
the existing M1 capability names, not new runtime abstractions.

## 3. Static and runtime facts

Compile time may consume only deterministic compiler-owned or
repository-owned inputs:

- source declarations, types, layouts, constants, and bounded pass structure;
- checked shader-package identity, variant metadata, descriptor count,
  entry-point name, workgroup size, push-constant size, and requirements;
- target Vulkan contract and native ABI declarations selected by the build;
- declared resource accesses and fixed kernel requirements.

Compile time must not query a live GPU, driver, queue, memory heap, environment,
clock, network, or ambient filesystem. M1 admits repository inputs by explicit
path/configuration supplied to the compiler; it has no general comptime I/O.

Live extensions, features, limits, queue families, devices, memory types, and
allocation results remain runtime admission or committed execution facts.
Package requirements may be checked statically for internal consistency, but
the current device still admits them at runtime. A memory-type index is a
runtime mechanism result selected from explicit required properties and an
already-committed placement rule; it is not a compile-time fact.

## 4. Values, ownership, and identity

### 4.1 Minimum value categories

M1 distinguishes:

- `borrow T`: non-owning use valid only for the lexical call/scope;
- `owned T`: affine, move-only ownership with exactly one live drop obligation;
- plain copy values: fixed scalars, handles explicitly declared as observations,
  and immutable package/static descriptors;
- `unsafe imported T`: a privileged borrowed Vulkan object admitted through a
  typed host boundary.

There is no implicit copy of `owned` values. `move` transfers the drop
obligation and use after move is rejected. M1 uses lexical local checking; it
does not promise universal Rust-style lifetime proof or infer safety across
arbitrary foreign storage.

### 4.2 Deterministic destruction

Owned locals drop in reverse successful-initialization order on success, early
return, propagated failure, and generated cleanup edges. Partially constructed
values drop only initialized members. Moved values are not dropped. Drop is
idempotent at the existing helper boundary where the production helper already
supports repeated cleanup.

Submission creates an in-flight ownership obligation. A resource used by an
in-flight command cannot be dropped, remapped, moved to an unrelated owner, or
rebound until the existing completion operation succeeds or the existing
failure path quarantines/retains it. Dependent pipeline, descriptor, command,
buffer, acceleration-structure, and scene objects drop before the borrowed
common runtime/device owner.

### 4.3 Identities are not interchangeable

The type system and MIR must not conflate:

- language ownership;
- logical tensor/resource identity;
- immutable content/weight identity and hash;
- mechanical Vulkan allocation identity;
- descriptor binding;
- committed execution facts;
- slot/generation or arena reuse epochs;
- in-flight submission ownership.

A `VkBuffer` allocation is not a tensor, a binding, content, or authorization.
A package variant is not a Dominatus decision. A descriptor does not own the
resource it names.

## 5. Fallibility

Fallible functions return `Result<T, PrometheusError>` (or the profile's
equivalent fallible return spelling) and use explicit propagation. `Result` is
must-use. M1 may use `?` as surface syntax, but its exact parser spelling is an
implementation detail until the M1 grammar is committed.

Lowering preserves existing C behavior:

- every failure maps to an existing `PROM_*` return, stage, and detail value;
- no public error, ABI code, or failure-ordering rule is added;
- the first current failure at a production call boundary remains the reported
  failure;
- cleanup runs in the same dependency-safe order before returning;
- Vulkan/package facts remain observations, not policy choices.

Generated helpers may use a single cleanup epilogue when that is behaviorally
equivalent and source-mapped. They may not collapse distinct existing failure
codes or turn a package/admission failure into a generic error.

## 6. M1 resource and pipeline types

M1 requires only:

- `borrow MechanismContext`: admitted runtime/device/queue/command-pool and
  package services;
- `borrow AccelerationStructure`: an already-created, already-admitted object
  with an explicit lifetime contract;
- `owned Buffer<HostVisibleCoherent, Storage, T>` for mapped observation;
- `PackageComputeEntry<Bindings, PushConstants>` checked against an exact
  package/variant identity;
- a typed `DescriptorSet<Bindings>`;
- `ComputePipeline<Entry>`;
- a lexical `CommandRecording`;
- an affine `Submission` completed by the existing synchronous wait.

Device-local buffers, transfer staging, push constants, and multiple storage
bindings are expected M1 types when required by the selected operation, but the
first conformance specimen does not need all of them. Images, layouts,
samplers, full ray-tracing pipelines/SBTs, multiple queues, and general Vulkan
object coverage are deferred until production evidence demands them.

Bindings are structural contracts with an exact set, binding number, descriptor
kind, access, and element/range type. Package metadata may prove entry point,
workgroup, descriptor count, push-constant byte count, and static requirements.
It does not currently encode every descriptor kind, so M1 source states the
kernel-54 binding kinds explicitly and validation cross-checks the package
count. Derivation is permitted only when the package becomes authoritative for
the missing fact.

## 7. Access and synchronization

M1 declares access at each operation boundary:

- `host_write`;
- `transfer_read` / `transfer_write`;
- `shader_read` / `shader_write`;
- `host_read`;
- `acceleration_structure_read`;
- `descriptor_read` as a binding property, not a memory barrier category.

The compiler constructs an ordered per-resource access chain. It may derive a
barrier only when the adjacent accesses, queue-family relationship, and layout
state select one safe current Vulkan mapping. Examples include host write to
transfer read, transfer write to shader read, shader write to transfer read,
and transfer write to host read as used by current Prometheus paths.

M1's mapped coherent kernel-54 observation uses submission/fence completion for
device-to-host availability, matching the current synchronous path. The source
still declares `shader_write -> host_read`; the MIR records how the existing
mechanism satisfies it.

Ambiguous cases require a visually explicit profile operation:

```concept
unsafe vulkan.sync_override(
    resource: evidence,
    src_stage: ComputeShader,
    src_access: ShaderWrite,
    dst_stage: Host,
    dst_access: HostRead,
);
```

An override is checked for resource ownership and scope, appears in MIR and
generated comments, and affects only the named transition. Images additionally
require explicit layouts until a later typed image model proves safe inference.
No barrier or layout transition is hidden behind an uninspectable default.

## 8. Effects decision

M1 does **not** introduce a general annotation-heavy effect system. The same
call-edge mistakes are caught more directly by typed capabilities and lexical
state:

- allocation requires `borrow MechanismContext` and returns `owned`;
- mapping requires the host-visible buffer capability;
- recording operations require `borrow mut CommandRecording`;
- submission consumes a finished recording and produces `Submission`;
- wait consumes or completes `Submission`;
- host observation requires completed device writes.

MIR records `allocate`, `map`, `record`, `submit`, `wait`, `observe`, and
`unsafe_vulkan` effects for audit and future checking. A general effect syntax
is deferred unless multiple real call sites demonstrate that capability/state
types do not make the constraint legible.

## 9. Static requirements, templates, and comptime

The three mechanisms remain separate:

- `concept` describes a static shape or requirement;
- templates provide bounded, compile-time generic reuse;
- `comptime` performs deterministic, bounded evaluation;
- runtime polymorphism is unrelated and absent from M1.

M1 needs static package/binding contract declarations and `static_assert`.
It does not require user-defined general-purpose templates. If one tiny
compiler-owned buffer/binding schema is parameterized, specialization is
finite and monomorphic before MIR.

M1 comptime supports integer/Boolean/string constants, fixed arrays/records,
field selection, bounded arithmetic/comparison, and static assertions over
repository-owned package metadata. It has a deterministic operation/fuel
limit and no recursion, arbitrary functions, generated identifiers, reflection,
live runtime inspection, or host side effects. SDSL-V's 256-statement
`comptime for` guard is useful precedent, not automatically the M1 limit.

## 10. Escape hatch

The only M1 escape is a typed `unsafe vulkan.<operation>` declaration or call
from a compiler-maintained allowlist. It must name:

- the exact Vulkan operation;
- every borrowed/owned handle and affected resource;
- declared pre/post access state;
- the existing failure mapping;
- why the profile cannot yet express the operation.

It is source-visible, MIR-visible, generated-comment-visible, and counted in a
machine-readable generation summary. It cannot embed arbitrary C, bypass drops,
manufacture ownership, weaken another resource's synchronization, query
Dominatus state, or call unlisted Vulkan symbols. M1 acceptance sets an explicit
maximum escape count for its specimen; the chosen capability probe requires no
escape for its normal dispatch path and one typed import boundary for the
prebuilt acceleration structure.

## 11. Diagnostics and source mapping

Every AST, typed node, MIR operation, cleanup edge, and generated helper retains
the `.concept` source path and span. Validation errors name the source
construct, package/variant fact, and conflicting production contract.

Generated C/H is deterministic and readable. It contains:

- a generated-file marker and source digest;
- stable helper/resource names derived from source declarations;
- comments with source path and line before operation groups;
- a sidecar source map from generated line ranges and MIR operation IDs to
  source spans;
- `#line` directives only where they improve compiler diagnostics without
  obscuring review.

C compiler failures are reported with both the generated location and mapped
Concept/Vulkan location. Generated code never claims that a Vulkan runtime
failure is a compile-time validation failure.

## 12. M1 semantic minimum

The first compiler milestone implements exactly one complete packaged
compute-operation conformance slice:

1. parse `profile Vulkan;` and one function;
2. load and strictly validate one repository-supplied shader package;
3. type a borrowed mechanism context and imported admitted acceleration
   structure;
4. select exact package variant `kernel-54-default`;
5. create one mapped host-visible coherent storage buffer;
6. validate typed bindings 0 (acceleration structure, read) and 1 (storage
   observation, write);
7. create package-backed module, layout, descriptor resources, and compute
   pipeline;
8. record one dispatch `(1, 1, 1)` with declared accesses;
9. submit and wait through the current synchronous mechanics;
10. read the mapped observation;
11. preserve all fallible exits and reverse cleanup;
12. emit deterministic checked-in C/H plus source map and generation manifest.

The specimen mirrors the bounded execution inside
`prom_ray_create_compute_resources` and
`prom_ray_query_triangle_scene_probe_impl` in
`reactor_vulkan_ray_query.c`. The acceleration structure is imported because
building BLAS/TLAS is a separate, substantially larger mechanism. Full
ray-query batch resource growth, descriptor rebinding, `ray_count` dispatch,
and result conversion remain the M2 equivalence target.

M1 does not require topology, Dominatus progression, adaptive choice, multiple
queues, scheduling, graph compilation, general templates, dynamic interfaces,
reflection, async, package management, or the full Concept language.

## 13. Profile MIR boundary

The M1 MIR is profile-specific and typed. Its operation vocabulary is:

| MIR operation | Current production lowering target |
| --- | --- |
| `borrow_context` | `prom_reactor_runtime_get_vk_services` / borrowed `prom_vk_runtime_services` |
| `import_acceleration_structure` | typed scene-owned TLAS handle at the existing probe boundary |
| `resolve_package_entry` | `prom_reactor_runtime_get_shader_package` plus `prom_shader_package_create_module` |
| `create_buffer` | `prom_vk_create_buffer` using Stage 5 mechanics |
| `map_observation` | mapped result returned by the existing buffer helper |
| `create_descriptor_layout` | kernel-54 `vkCreateDescriptorSetLayout` sequence |
| `create_pipeline_layout` | kernel-54 `vkCreatePipelineLayout` sequence |
| `allocate_descriptor_set` | descriptor-pool/create/allocate sequence |
| `bind_descriptor` | kernel-54 acceleration-structure and storage writes |
| `create_compute_pipeline` | package module plus `vkCreateComputePipelines` |
| `begin_recording` | `prom_ray_begin_command` |
| `declare_access` | typed validation input; no direct Vulkan call |
| `bind_compute` | `vkCmdBindPipeline` / `vkCmdBindDescriptorSets` |
| `dispatch` | `vkCmdDispatch(1, 1, 1)` |
| `end_submit_wait` | `prom_ray_end_submit_and_free` / `prom_ray_submit_command` |
| `observe_mapped` | existing post-wait `memcpy` from the evidence buffer |
| `drop` | `prom_vk_destroy_buffer`, Vulkan dependent-object destroys, module destroy |
| `fail_to_cleanup` | existing return/detail mapping plus reverse initialized drops |

MIR contains explicit ownership states, access chains, package identity,
source spans, and cleanup successors. It contains no policy, scoring,
Dominatus blackboard, model progress, scheduler, graph optimizer, pooling,
topology, or shader computation.

## 14. Generated authority

After M1, `.concept` source and its checked package/ABI inputs are semantic
authority. Generated C/H is a deterministic, checked-in native build input and
must not be hand edited.

Regeneration occurs in a temporary directory, formats output with the repository
chosen pinned formatter, byte-compares all C/H/map/manifest outputs, and fails
on drift. A generated manifest records compiler version, source digests,
package identity/variant/artifact digest, ABI digest, output digests, and escape
count. Existing shader-package identities and generated shader authorities
remain separate inputs; Concept/Vulkan does not regenerate shaders.

Review compares generated code beside the handwritten witness until equivalence
is proven. The native build consumes checked-in C/H without invoking the
compiler. Rollback switches the build back to the retained handwritten file and
reverts the generated/source set; public ABI, packages, and shaders are not
part of that rollback.

## 15. Equivalence and migration gates

- **M1:** compiler vertical slice and capability-probe conformance; parser,
  type/MIR/C determinism; failure/drop goldens; native compile and real-path
  validation.
- **M2:** generate a physical ray-query batch beside the handwritten path and
  compare allocation, descriptor bindings/rebinding, command sequence, access
  synchronization, `ray_count` dispatch, results, diagnostics, failure order,
  repeated lifecycle, and Vulkan validation.
- **M3:** migrate the production ray-query mechanism only after M2 equivalence.
- **M4:** express one Stage 4 SGEMM handoff without changing its committed
  variant, dimensions, bindings, handles, offsets, dispatch, synchronization,
  or execution state.
- **Prometheus Stage 7:** later establish the separate Dominatus model-operation
  authorization/observation seam. Concept/Vulkan never fakes that seam.

## 16. Explicit exclusions

M1 excludes Concept machines/transitions, `decide`, `yield`, dynamic interfaces,
vtables, runtime reflection, general heap/containers, general async,
exceptions, package-manager ambitions, DragonGod facilities, full RT
pipelines/SBT, shader mathematics, model topology, and lifecycle/progression
policy. PoC3 is design history; only its local ownership, explicit move,
deterministic drop, error-as-value, unsafe quarantine, C ABI, bounded comptime,
profiles, and MIR-first principles are adopted here.
