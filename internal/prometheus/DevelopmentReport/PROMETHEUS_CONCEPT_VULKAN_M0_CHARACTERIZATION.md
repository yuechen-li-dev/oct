# Prometheus Concept/Vulkan M0 — production characterization and implementation boundary

Status: **SUCCESS — constitution and M1 implementation boundary established**

Date: 2026-07-24

## 1. Starting checkpoint and repository state

M0 started at exactly:

```text
457671c3094ead30019b4a96407430fe010d6998
prometheus: classify closed model plan boundary
```

`git status --porcelain=v1 --untracked-files=all` was empty and
`main...origin/main` had no reported divergence. Stage 6 was the HEAD commit.
Its report, evidence-index entry, and reviewer-handoff entry were present.
Ignored machine-local build/cache files exist in the checkout but are not
tracked repository authority; M0 neither deletes nor adds them. No tracked
payload, build output, binary, cache, or machine-specific path was present or
absorbed.

This milestone changes documentation and authority indexing only. It does not
change production C/C++/Go, generated artifacts, `.concept` production source,
runtime behavior, or build generation.

## 2. Scope, owner decision, and non-goals

The owner decision is:

> Build the low-level language machinery Prometheus needs now through the
> working Oct/SDSL-V compiler lineage. Preserve compatible proven Concept ideas.
> Backport the production-proven design into a future feature-complete,
> self-hosted Concept.

The language is **Concept/Vulkan**, a real imperative host-mechanism profile,
not a general Concept revival, reactor DSL, policy engine, scheduler, graph IR,
or Vulkan framework. The governing rule is:

```text
Essential decisions remain explicit.
Mechanical consequences are generated.
```

M0 characterizes and specifies. It adds no parser, lexer, type checker, MIR,
C backend, build integration, generated C/H, runtime change, ray-query change,
SGEMM rewrite, model-plan extraction, or Stage 7 seam.

## 3. Sources inspected

Current Prometheus authorities inspected:

- `PROMETHEUS_STAGE0_CHARACTERIZATION_AND_VOCABULARY.md`;
- `PROMETHEUS_STAGE1_REPOSITORY_AND_GENERATED_AUTHORITY.md`;
- `PROMETHEUS_STAGE2_ABI_AND_VOCABULARY_CONSOLIDATION.md`;
- `PROMETHEUS_STAGE3_VULKAN_RUNTIME_OWNERSHIP_EXTRACTION.md`;
- `PROMETHEUS_STAGE4_RESOURCE_STATE_AND_EXECUTION_HANDOFF.md`;
- `PROMETHEUS_STAGE5_MECHANICAL_ALLOCATION_AND_CLEANUP_SUBSTRATE.md`;
- `PROMETHEUS_STAGE6_CLOSED_MODEL_EXECUTION_PLAN.md`;
- `PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`;
- `PROMETHEUS_DEVELOPMENT_EVIDENCE_INDEX.json`;
- the full architecture audit and ray-query M0/M1/RQ-M1 reports.

Production and contract paths inspected:

- `native/reactor_vulkan_runtime.[ch]` and
  `reactor_vulkan_runtime_internal.h`;
- `native/reactor_vulkan_common.c`;
- `native/reactor_vulkan_sgemm.c` and
  `reactor_vulkan_sgemm_internal.h`;
- `native/reactor_shader_package.[ch]` and Go
  `internal/prometheus/shaderpackage/package.go`;
- `native/reactor_vulkan_ray_query.c`,
  `native/include/prometheus_ray_query.h`, and ray-query Marionette tests;
- `native/reactor_vulkan_fused_reduction.c`,
  `reactor_vulkan_fft.c`, model-block/transformer paths, public API, native
  manifest, shader manifest, model lock, generated projections, and build
  scripts;
- Dominatus blackboard, judgment, SGEMM adapter, slot adapter, and tests.

Compiler/reference paths inspected:

- `Language/reference/language/18-concepts.md`;
- SDSL-V M5/M6/M13/M14/M14a/M16 documents and language fixtures;
- `internal/concept`, Oct parser/typechecker/build MIR paths;
- `internal/sdslv/{lex,token,parse,ast,validate,consteval,lower,vdmir,emit,toolchain}`;
- `internal/source` and diagnostic conventions;
- local Concept history at
  `C:\Users\yuech\source\repos\Concept\docs\Concept-PoC3.md`.

The requested older compute-reactor seed note was searched by name and phrase.
No distinct tracked note was found. The deferred compute-reactor experiment
register in `PROMETHEUS_DVT2_MX5_VULKAN14_MIGRATION.md` is historical
motivation only; it is not promoted to architecture.

## 4. Baseline authority

`go run ./tools/prometheus_stage0 -check` passed at the starting checkpoint:

| Authority | Baseline |
| --- | --- |
| public exports | 84 |
| ABI function-signature digest | `89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262` |
| public structs | 69 |
| package | `prometheus.core@1` |
| package kernels / variants / artifacts | 69 / 69 / 68 |
| source assets / implementations | 69 / 18 |
| kernel 68 | `kernel-68-default` |
| kernel 69 | `kernel-69-default` |
| native shader manifest SHA-256 | `8ec65f4e3d81c52effc94826dd6460a9b240416e711eb2738f414063376f3ad8` |
| generated shader-ID SHA-256 | `cdbf2df77306c82cdc1705b9e73992d8249294435f6625cf6c2dab86bd5c9d3c` |
| model lock SHA-256 | `71ef202b4e34b562bd0d8526d1e0c674640cbba02fb7c484d8dadf981c8b226e` |
| resolved descriptor SHA-256 | `1888f67f755596ad789c5cd22ea5cdf3da9402eb84187e980408cfce271094ce` |
| resolved audit schedule SHA-256 | `7c00bf7c32719f6ea446d006e52bff12d037e7e649603c5af5fd386fee8b41c0` |
| native manifest SHA-256 | `1908f39b5656c6ca3cb87ba91101a052209e14cf3703c3a08834b7c87577985d` |

The Stage 1 staged package-manifest reference remains
`a110cebc3abc737bb450c53d5f2a5ed46cdd7c48dfd300688a0ab567a64ef19c`.
The package/static/generated count differences and 29 repeated
`MainTransformer1` successors remain descriptive/disputed, not repaired.

Prometheus production C builds as C11 on Windows and Linux. Windows tests use
the current MSVC C++ mode (`/std:c++latest`); Linux Marionette uses C++23.
Native source membership is manifest-generated. Oct/SDSL-V is implemented in
Go; its relevant compiler packages are listed in section 15.

## 5. Architectural constitution

Dominatus is the Prometheus control kernel. Semantic lifecycle, state
transitions, admission, coordination, blackboard state, judgment, variant and
policy selection, retry/hysteresis/commitment, authorization, progress,
completion, and success/failure interpretation live in or pass through
Dominatus.

Concept/Vulkan is subordinate:

```text
Dominatus decides and coordinates.
Vulkan mechanisms execute and report facts.
```

SDSL-V remains shader-side. Concept/Vulkan does not contain shader mathematics.
Prometheus remains the public semantic capability and C/H consumer; it does not
depend on the future general Concept compiler. Generated C/H remains the
auditable native implementation boundary.

## 6. Current Vulkan authoring inventory

### 6.1 Common runtime creation and ownership

`prom_vk_runtime_init` in `reactor_vulkan_runtime.c` creates the instance,
chooses the physical device, admits features/extensions, selects queue
families, creates the device/queues/command pools, loads extension functions,
and opens the shader package. `prom_vk_runtime_cleanup` waits idle and destroys
owned objects in dependency order. SGEMM borrows services; ray scenes copy a
borrowed service record and are destroyed before runtime cleanup.

Essential facts: requested feature families, admitted runtime configuration,
selected queue roles where committed, and ownership/borrowing.

Mechanical consequences: zeroed create infos, extension arrays, handle
creation, function loading, partial-failure cleanup, and reverse teardown.

Live capabilities are runtime observations supplied to Dominatus/host. They are
never compile-time Concept/Vulkan queries.

### 6.2 Stage 5 allocation and cleanup

`reactor_vulkan_common.c` centralizes buffer creation, requirements discovery,
allocation, zero-offset binding, optional mapping, and unmap/destroy/free.
Named wrappers retain required property flags, SGEMM placement selection,
concurrent queue sharing, device-address `pNext`, map choice, and current
partial-cleanup policy.

Memory type is not a source-chosen literal in general. Source states the
required memory properties and usage; a runtime helper selects a compatible
type from live facts. When Dominatus has committed placement, the mechanism
consumes it. Allocation size remains the exact Vulkan requirement size and bind
offset remains zero.

### 6.3 Bounded SGEMM and Stage 4 handoff

After Dominatus commits path/compute, layout/precision, transfer, buffering,
and lease facts, `prom_sgemm_execution_handoff` carries M/N/logical-K/compute-K,
selected path/mode/variant, slot, wait dependency, pipeline, descriptor set,
three descriptor ranges, and dispatch geometry. Command binding, push
constants, dispatch, submission/wait, and readback consume it.

Concept/Vulkan may eventually express these committed mechanics. It must not
select the variant, change dimensions, reinterpret roles, acquire authority,
or advance SGEMM state.

### 6.4 Package verification and pipeline construction

The Go package builder and native loader validate
`prometheus.shader-package.v1`, package/runtime ABI, content-addressed SPIR-V,
digest/size/alignment/magic, entry point, local size, variants, requirements,
and implementation references. `prom_shader_package_create_module` rechecks
artifact bytes and creates a module. Current mechanism paths then state
descriptor layouts and create pipeline layouts/pipelines.

Package metadata can statically supply variant identity, entry, workgroup,
descriptor count, push-constant bytes, and requirements. It does not currently
encode all descriptor kinds/accesses, so source must state those and the
compiler cross-checks the count. Shader/package authority is input, not
generated Concept/Vulkan authority.

### 6.5 Ray-query scene construction

`reactor_vulkan_ray_query.c` validates/copies triangles and spheres, creates
host-visible/device-address buffers, builds separate triangle/procedural BLAS,
creates an identity-instance TLAS, allocates scratch for completed build
submissions, and creates package-backed kernel-54/55 compute resources.

Essential: geometry representation, AS kind, device-address usage, memory
properties, build ordering, binding kinds, package variants, ranges, and scene
immutability after commit.

Mechanical: create-info population, requirements queries, scratch lifetime,
descriptor pool/write ceremony, module/pipeline construction, and reverse
failure cleanup.

### 6.6 Ray-query recording, batch dispatch, completion, and readback

RQ-M1 is owner-accepted and closed. A supported nonzero public batch validates
all input before output mutation, ensures paired mapped ray/hit capacity,
rebinds descriptor bindings 2/3 before retiring replaced buffers, records one
`vkCmdDispatch(ray_count, 1, 1)`, performs one synchronous submission/wait,
converts all raw hits, then atomically publishes caller output. A zero batch is
a semantic no-op.

The physical implementation is real production authority, but work on expanded
image authority/cost accounting is paused for this detour. It is neither
abandoned nor falsely marked complete.

### 6.7 Partial failure, repeated cleanup, and destruction

Ray scene creation uses a single failure path through
`prom_ray_scene_destroy`; AS scratch is released after completed build;
pipelines/layouts/pools/buffers/AS objects are conditionally destroyed.
Stage 5 buffer cleanup nulls handles and supports repeated cleanup. Runtime
destruction takes all remaining scenes before common device teardown.

The generator must track successful initialization, emit cleanup on every
exit, drop in reverse dependency order, skip moved/uninitialized values, and
preserve existing return/detail ordering. It must not assume that every helper
has identical partial-failure ownership: the ordinary buffer wrapper retains a
caller-owned distinction recorded by Stage 5.

## 7. Fact classification for representative operations

| Operation/fact | Classification and owner |
| --- | --- |
| Dominatus path/variant/lease/model authorization | semantic policy/control fact; Dominatus |
| Stage 4 SGEMM handoff fields | committed execution facts consumed by mechanism |
| buffer usage, required properties, sharing, mapping intent | essential Vulkan decisions |
| requirement query/allocation/bind/create-info/drop sequence | generated mechanical consequences |
| allocation result, completion, hit/result/detail | runtime observations |
| package identity/digest/entry/workgroup and lock/kernel identities | generated/package authority facts |
| 84 exports, public layouts/errors/failure ordering | public ABI/compatibility outside experimentation |
| repeated descriptor/create-info/cleanup blocks | accidental ceremony suitable for generation |

Buffer usage remains explicit because it controls validity and synchronization.
Ownership may be inferred only from construction/import syntax and lexical
movement; descriptor membership never implies ownership. Cleanup order may be
generated from the ownership/dependency graph.

Dispatch dimensions are semantic inputs when supplied by a committed request
(`ray_count`, SGEMM geometry). They are derived mechanics only when package
metadata and a declared logical extent uniquely determine them. Kernel-54's
`(1,1,1)` is a fixed mechanism fact.

## 8. Highest-risk handwritten failure classes

Production evidence supports these risks:

1. partial-construction leaks/double destruction across many Vulkan handles;
2. dependent-object destruction after the common device owner;
3. descriptor declaration/write drift from package/kernel contracts;
4. buffer replacement before descriptor rebinding or in-flight completion;
5. missing/incorrect stage-access ordering across host, transfer, compute, and
   readback;
6. conflation of logical identity/generation with mechanical allocation,
   binding, slot, or ownership;
7. dispatch geometry/push constants diverging from a committed handoff;
8. live feature/limit/memory facts being mistaken for static metadata;
9. failure-code/order collapse while consolidating cleanup.

These are exhibited by the existing ceremony and preservation tests. M0 does
not invent a generic resource-pooling or graph problem absent from production.

## 9. Stage 3–6 preservation

- **Stage 3:** `prom_vk_runtime` remains the common instance/device/queue/
  command-pool/package owner. Operation resources remain typed local owners.
- **Stage 4:** `prom_sgemm_execution_handoff` remains the immutable-by-
  convention committed mechanism boundary. No field or call order changes.
- **Stage 5:** policy-free allocation/bind/map/cleanup helpers remain the
  lowering targets. No placement policy is moved into language machinery.
- **Stage 6:** the finite 34-position Z-Image sequence remains implicit and
  unextracted. `retarget_position` and model progression remain Vulkan-session
  owned because no Dominatus authorization/completion seam exists.

Concept/Vulkan does not add an `authorized` keyword, shadow scheduler, plan,
topology, or progression owner. Stage 7 remains deferred until after the
initial Concept/Vulkan proof.

## 10. Static/runtime boundary

Static inputs: source/types/layouts; checked package metadata; descriptor
contracts stated in source; fixed kernel requirements; target Vulkan/ABI
contract; declared accesses; bounded constants and fixed pass structure.

Runtime inputs: device/features/extensions/limits; queue families; memory
types/heaps; allocation results; admitted/committed Dominatus facts; external
handles; dispatch dimensions supplied by work; fence/completion; readback and
errors.

The compiler may prove that a package requires ray query. It cannot prove the
current GPU admits ray query. The host/runtime must supply an admitted
mechanism context.

## 11. Ownership, cleanup, and identity model

M1 uses affine owned values, explicit `move`, lexical borrows, typed unsafe
imports, initialized-state tracking, and deterministic reverse drop. It does
not promise global lifetime proof.

An in-flight submission holds use obligations over all referenced resources.
Drop/rebind/map conflicts are rejected until wait completes. Partial
construction drops only initialized members. Existing idempotent helpers remain
idempotent. Dependent resources drop before borrowed runtime teardown.

Language ownership, tensor/resource identity, weight/content identity,
allocation identity, descriptor binding, committed execution fact, and
submission ownership are distinct types/relations.

## 12. Fallibility

Fallible source operations use must-use results and explicit propagation. MIR
has success and failure successors and explicit cleanup. C lowering preserves
the existing `PROM_*` status/stage/detail values and first-failure order; it
adds no public error. Package errors, Vulkan failures, invalid handles, and
admission failures are not collapsed into a new Concept/Vulkan code.

## 13. Resources, bindings, access, synchronization, and effects

The M1 minimum is a borrowed mechanism context, imported admitted acceleration
structure, mapped host-visible coherent storage buffer, exact package compute
entry, typed descriptor set/pipeline, command-recording scope, and affine
submission.

Access categories are host write/read, transfer read/write, shader read/write,
and acceleration-structure read. The compiler derives a barrier only when
adjacent declared accesses and queue/layout facts make the current Vulkan
mapping unique. Ambiguity requires a resource-local typed unsafe override.
Synchronization remains visible in MIR/generated comments.

M1 does not add a surface effect system. Typed capabilities and command/
submission state catch allocation, mapping, recording, submission, wait, and
observation call-edge errors with less annotation. MIR still records those
effects for audit.

## 14. Metaprogramming minimum and escape policy

`concept`, template specialization, and `comptime` remain distinct. M1 needs
static package/binding requirements and assertions, not general user templates.
Comptime is deterministic, bounded, hermetic, and limited to fixed value
operations over explicit repository inputs. There is no live GPU query,
reflection, recursion, arbitrary host function, or ambient I/O.

The escape hatch is an allowlisted typed `unsafe vulkan.<operation>` with
explicit handles, resource pre/post state, existing error mapping, source/MIR/
generated visibility, and a measured count. It cannot embed C, manufacture
ownership, bypass cleanup, or weaken unrelated synchronization.

## 15. Oct/SDSL-V compiler inventory and reuse decision

### 15.1 Observed machinery

| Machinery | Implementation evidence | Reuse result |
| --- | --- | --- |
| source files/spans | `internal/source`; spans on SDSL-V AST/MIR | directly reusable |
| SDSL-V tokens/lexer | `internal/sdslv/token`, `lex` | reusable after narrow profile extension; current lexer is shader-specific and rejects `?` |
| SDSL-V parser/AST | `parse`, `ast` | conceptually useful, implementation-incompatible; shader declarations dominate |
| diagnostics | source spans plus validator diagnostic conventions | directly reusable after assigning Concept/Vulkan codes |
| `concept` | Oct `internal/concept` value shapes/refinements; SDSL-V static config schemas | semantic precedent; neither implementation directly models host resources/ownership |
| templates | SDSL-V single config parameter and pre-MIR monomorphization | reusable after narrow extraction only if M1 proves a user template need; unnecessary for first slice |
| bounded comptime | SDSL-V `consteval` and pre-VDMIR expansion; 256 statement guard | evaluator is AST-coupled; approach reusable, code reusable after narrow extraction |
| static assertions | Oct `Require`; SDSL-V `require`/`static assert` | semantics directly reusable; host types need new checker path |
| types | Oct typechecker and SDSL-V AST/VDMIR types | conceptually reusable, implementation-incompatible with affine Vulkan resources |
| generic specialization | SDSL-V `compile Template<Config>` | unnecessary for first slice |
| MIR | Oct general MIR and shader-specific VD-MIR | MIR-first architecture reusable; neither IR can honestly encode host Vulkan ownership |
| deterministic artifacts | SDSL-V VDMIR/HLSL/SPIR-V/header and golden tests | toolchain/testing pattern directly reusable |
| package integration | `internal/prometheus/shaderpackage` strict manifest validation | directly reusable as compiler input validation |
| Go/C output | Oct emits Go; SDSL-V emits HLSL | implementation-incompatible; M1 needs new deterministic C/H emitter |

Oct `concept` is package-local value/refinement machinery and erases through the
ordinary type/MIR path. SDSL-V `concept` is a compile-time config schema;
templates monomorphize and bounded comptime expands before VD-MIR. Shared syntax
does not imply a shared host-language implementation.

### 15.2 Smallest honest relationship

Choose **a narrow sibling compiler reusing selected packages**:

```text
internal/source + diagnostics/token conventions
  -> Concept/Vulkan profile lexer/parser/typed AST
  -> profile ownership/access validation
  -> Concept/Vulkan MIR
  -> deterministic C/H + map/manifest
```

M1 should extend/extract the lexer only as needed, use the strict Go shader-
package validator as an input authority, and reuse deterministic/golden
toolchain patterns. It should not force host Vulkan semantics into SDSL-V's
shader AST/VD-MIR or Oct's broad runtime compiler. This remains in the working
Go Oct/SDSL-V lineage without creating a dependency on the experimental
Concept repository.

No compiler-organization question blocks M1.

## 16. Concept PoC3 classification

Adopt:

- essential difficulty remains visible; accidental ceremony is generated;
- local affine ownership, explicit moves, initialized-state tracking, and
  deterministic reverse drop;
- borrow/reference versus owned/raw distinction without a universal lifetime
  proof;
- errors as must-use values;
- unsafe as an auditable invariant assertion, not disabled type checking;
- C ABI-first interop and MIR-first semantics;
- deterministic bounded comptime and profile-specific defaults.

Modify:

- PoC3's broad effects become M1 typed capabilities/state with MIR effects;
- general concepts/templates become only static package/binding requirements;
- C ABI is internal generated integration, not a new public ABI;
- address spaces become concrete Vulkan buffer property/usage types only where
  current mechanisms require them.

Reject/defer beyond M1:

- machines/transitions, `decide`, `yield`, lifecycle/state authority;
- dynamic interfaces/vtables and runtime polymorphism;
- reflection/macros/generated declarations beyond the fixed compiler backend;
- general heap, allocator/container/arena APIs;
- general async, exceptions/panic machinery, package manager;
- C++ interop, LLVM/native backend ambitions;
- DragonGod-specific memory/mind/automata/events/replay facilities.

The current local PoC3 implementation history proves that several ideas can be
implemented, but it is not current production authority and no code is copied.

## 17. Normative language constitution

The concise normative authority is:

`docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md`.

It fixes identity, syntax direction (`profile Vulkan;`, `.concept`), governing
rule, static/runtime boundary, affine ownership/drop, fallibility, typed
resources/bindings, access/synchronization, effects decision, metaprogramming,
escape hatch, diagnostics/source maps, M1 minimum, MIR, generated authority,
and migration gates.

## 18. M1 specimen choice

M1 uses the existing package-backed **ray-query capability probe mechanism**
(`kernel-54-default`) with a borrowed, already-admitted TLAS. This is the
smallest current complete package-to-observation witness:

- exact package-backed module;
- two typed bindings;
- one owned mapped evidence buffer;
- descriptor/pipeline creation;
- one command scope and `(1,1,1)` dispatch;
- synchronous submit/wait;
- host observation;
- complete fallible construction and reverse cleanup.

Importing the admitted TLAS keeps BLAS/TLAS construction out of the compiler
conformance slice. The full physical kernel-55 ray batch is a clean production
authority but includes capacity growth, descriptor rebind, public batch
atomicity, and multiple geometry buffers; it remains the stronger M2
equivalence target. SGEMM is intentionally not M1 because it carries the
scarred Dominatus/slot/placement/variant handoff.

## 19. Specimen A — M1 vertical operation

This is normative design syntax, not implemented code. Labels are part of the
M0 classification. A label on a declaration or control-scope line applies to
its nested field/call lines unless a nested line carries a different label.

```concept
profile Vulkan;                                      // REQUIRED FOR M1
import Prometheus.Vulkan;                            // REQUIRED FOR M1

concept CapabilityProbeBindings {                   // REQUIRED FOR M1
    binding 0: borrow AccelerationStructure read;
    binding 1: Buffer<HostVisibleCoherent, Storage,
                      ProbeEvidence> write;
}

package compute CapabilityProbe =                   // REQUIRED FOR M1
    "prometheus.core@1"::"kernel-54-default"
    bindings CapabilityProbeBindings
    push_constants 0;

fn probe(
    borrow context: MechanismContext,
    unsafe imported borrow scene_tlas: AccelerationStructure
) -> Result<ProbeEvidence, PrometheusError> {         // REQUIRED FOR M1
    owned evidence = Buffer<HostVisibleCoherent,
                            Storage, ProbeEvidence>
        ::create(context, count: 1)?;                 // REQUIRED FOR M1
    evidence.map()?.write(ProbeEvidence::zero());     // REQUIRED FOR M1

    owned pipeline = ComputePipeline::create(
        context, CapabilityProbe)?;                   // REQUIRED FOR M1
    owned descriptors = CapabilityProbeBindings::bind(
        context, scene_tlas, evidence)?;              // REQUIRED FOR M1

    owned command = context.record()?;                // REQUIRED FOR M1
    with recording borrow mut command {               // REQUIRED FOR M1
        command.bind(pipeline, descriptors);
        command.access(scene_tlas, acceleration_structure_read);
        command.access(evidence, shader_write);
        command.dispatch(1, 1, 1);
    }

    owned submission = context.submit(move command)?; // REQUIRED FOR M1
    context.wait(move submission)?;                   // REQUIRED FOR M1
    access(evidence, host_read after shader_write);   // REQUIRED FOR M1
    return Ok(evidence.map()?.read(0));
} // reverse drop: descriptors/pipeline/evidence; context/TLAS remain borrowed
```

The exact spelling of `unsafe imported`, `with command`, and `access` is
illustrative syntax still requiring grammar evidence. Their semantics are
required for M1. General user-defined `concept` syntax is deferred if a
compiler-owned binding declaration expresses the same contract more narrowly.

## 20. Specimen B — SGEMM pressure test

This is deferred-but-expected language pressure, not M1 or a migration plan.
Labels on declaration/control-scope lines apply to their nested lines.

```concept
profile Vulkan;                                      // REQUIRED FOR M1 identity
import Prometheus.Vulkan;                            // REQUIRED FOR M1
import Prometheus.SgemmMechanism;                    // DEFERRED, EXPECTED

fn execute_committed_sgemm(
    borrow context: MechanismContext,
    borrow committed: SgemmExecutionHandoff,
    borrow a: BufferRange<Storage, read>,
    borrow b: BufferRange<Storage, read>,
    borrow c: BufferRange<Storage, write>
) -> Result<SgemmObservation, PrometheusError> {      // DEFERRED, EXPECTED
    static_assert(committed.bindings == [0, 1, 2]);  // ILLUSTRATIVE

    owned command = context.record()?;
    with recording borrow mut command {
        command.bind(committed.pipeline,
                     committed.descriptor_set);
        command.push(SgemmPush {
            m: committed.m,
            n: committed.n,
            k: committed.compute_k,
        });
        command.access(a, shader_read);
        command.access(b, shader_read);
        command.access(c, shader_write);
        command.dispatch(committed.dispatch_geometry);
    }
    if committed.wait_for_transfer {                  // DEFERRED, EXPECTED
        context.consume_committed_transfer_wait();
    }
    return context.submit_wait_observe(move command, c)?;
}
```

`SgemmExecutionHandoff` is supplied after Dominatus commitment. This code cannot
score/select a variant, change dimensions/bindings/offsets/handles/dispatch,
authorize work, mutate execution state, or advance lifecycle. Exact adapters
and observation types require later implementation evidence.

## 21. Concept/Vulkan MIR and C mapping

| MIR operation | Existing mechanism |
| --- | --- |
| `borrow_context` | `prom_reactor_runtime_get_vk_services` |
| `import_acceleration_structure` | scene-owned `tlas.handle` at probe boundary |
| `resolve_package_entry` | `prom_reactor_runtime_get_shader_package`; `prom_shader_package_create_module` |
| `create_buffer` / `map` | `prom_vk_create_buffer` and Stage 5 mechanics |
| `create_descriptor_layout` | kernel-54 descriptor layout sequence |
| `create_pipeline_layout` | `vkCreatePipelineLayout` sequence |
| `allocate_descriptor_set` | descriptor pool/create/allocate sequence |
| `bind_descriptor` | acceleration-structure/storage descriptor writes |
| `create_compute_pipeline` | `vkCreateComputePipelines` |
| `begin_recording` | `prom_ray_begin_command` |
| `declare_access` | compile-time/MIR validation; mapped to a barrier or completion rule |
| `bind_compute` | `vkCmdBindPipeline`, `vkCmdBindDescriptorSets` |
| `dispatch` | current `vkCmdDispatch(1,1,1)` |
| `submit` / `wait` | `prom_ray_end_submit_and_free`, `prom_ray_submit_command` |
| `observe_mapped` | post-wait evidence `memcpy` |
| `drop` | existing Vulkan destroys and `prom_vk_destroy_buffer` |
| `fail_to_cleanup` | current `PROM_*` return/detail plus reverse initialized cleanup |

The MIR includes typed ownership state, in-flight obligations, access chains,
package identity, source span, and cleanup successors. It excludes policy,
Dominatus state, model progress, scheduling, graph optimization, pooling,
topology, and shader computation.

## 22. Generated authority and build integration

M1 `.concept` plus checked package/ABI inputs become semantic source. Generated
C/H, source map, and generation manifest are checked-in deterministic build
inputs marked no-hand-edit. A temporary regeneration must byte-compare output.
A pinned formatter and stable names are required. The manifest records compiler
version, source/package/artifact/ABI/output digests and escape count.

Shader source/SPIR-V/package identity remains separately authoritative.
Concept/Vulkan never regenerates it. Native downstream builds consume checked-
in C/H without invoking the compiler. During equivalence, generated and
handwritten implementations coexist behind an internal test/build selection;
rollback retains the handwritten path and reverts only the generated/source
set.

## 23. Equivalence strategy and roadmap

- **M1:** implement the bounded capability-probe compiler slice, deterministic
  parser/type/MIR/C generation, golden output, cleanup/failure tests, native
  compile, and real-path result comparison.
- **M2:** generate kernel-55 physical batch beside handwritten code; compare
  allocation/capacity growth, descriptor bindings/rebind-before-retire, command
  sequence, barriers, `ray_count` dispatch, results/diagnostics, failure order,
  repeat cleanup, and Vulkan validation.
- **M3:** migrate production ray query only after M2 equivalence.
- **M4:** express the immutable Stage 4 SGEMM handoff without taking policy.
- **Stage 7:** after the mechanism layer stabilizes, separately establish one
  Dominatus model-operation authorization/observation seam before any plan
  extraction.

## 24. Living-status reconciliation

No tracked file at the starting checkpoint contains the assignment's quoted
July 23 wording (“optional adaptive policy and higher-level workload control”)
or its described system-responsibility table. Repository-wide tracked-path and
content searches confirmed the absence. M0 therefore does not pretend to edit a
missing document. It creates the concise in-repository canonical briefing at
`docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`, records the provenance gap there,
and indexes it here.

That briefing:

- names Dominatus as control kernel;
- pauses ray-query image/cost work at the accepted physical-batch boundary;
- makes Concept/Vulkan M0/M1 the focus;
- keeps SDSL-V shader-side and generated C/H as the native boundary;
- defers Stage 7;
- records raw Vulkan C as no longer the intended new-reactor authoring surface.

## 25. Exact production files deliberately unchanged

All files under `internal/prometheus/native/`, all Go compiler/runtime code, all
Oct/SDSL-V source and fixtures, all shaders/SPIR-V/generated headers, package
contents, manifests, locks, kernels, model projections, public headers, bridge
ABI, Stage 3/4/5/6 reports, and runtime tests are deliberately unchanged.

In particular: ray-query execution/batching; SGEMM/M34b/A2x4; M46/M49 and
`-7406`; Gemma/Z-Image topology/weights/bindings/numerics; Stage 4 handoff;
Stage 5 allocation substrate; and Vulkan model-session progression.

## 26. Validation performed

| Lane | Result | Evidence |
| --- | --- | --- |
| required checkpoint / clean tracked worktree | PASS | exact SHA and empty porcelain before edits |
| Stage 0 authority | PASS | 84 exports, ABI/package/kernel facts above |
| required-live skip self-test | PASS | canonical authority script validation |
| JSON evidence index | PASS | strict JSON parse after update |
| native manifest / generated authority | PASS | canonical authority script |
| compiled model lock | PASS | canonical authority script |
| shader-package authority | PASS | canonical authority temporary package check |
| manifest/lock preservation | PASS | hashes and no changed authority paths |
| focused Go tests | PASS | Prometheus/shaderpackage/native-manifest/lock lanes |
| documentation path/link verification | PASS | referenced repository paths checked |
| Windows native/Vulkan | NOT RUN | documentation-only; no Concept/Vulkan implementation exists |
| live Gemma/Z-Image payload | NOT RUN | no claim; not required for M0 |
| Linux live Vulkan | NOT RUN | unclaimed |
| inherited M34b/A2x4 | NOT RUN | unchanged documented authority; no runtime claim |
| `git diff --check` | PASS | final diff |

Any Windows native tests run by the canonical authority path are preservation
evidence only, not Concept/Vulkan validation.

## 27. Open and deferred questions

No question blocks M1. M1 must settle exact token spellings for borrow/import/
result/access scopes, a stable diagnostic prefix, the fixed comptime fuel
value, formatter selection, and generated file locations as part of its
vertical implementation. These are bounded implementation choices under this
constitution.

Deferred because they do not block M1:

- general templates/concepts/effects;
- images and layout typing;
- multi-queue ownership transfers and async submission;
- full AS construction in language;
- general reflection/package management;
- Stage 7 authorization and closed model plan;
- M46/M49, repeated topology, and generated/static registry disputes.

## 28. Rollback boundary

Revert the single M0 documentation commit. That removes the report,
constitution, in-repository living status, evidence-index entries, and reviewer
detour note. It changes no production/generated byte and requires no runtime
rollback.

## 29. Exact M1 assignment

Implement one `profile Vulkan;` compiler vertical under a narrow Go sibling
package in the Oct/SDSL-V lineage. Parse/type/lower the capability-probe
specimen; strictly load `prometheus.core@1` variant `kernel-54-default`; model
borrowed context/imported TLAS, one owned mapped evidence buffer, typed
bindings, one command/dispatch/submission/wait/readback, affine cleanup and
existing error mapping; emit deterministic checked-in C/H plus source map and
manifest; compile beside the handwritten probe; prove success/failure/drop and
output equivalence without replacing production.

Do not add general Concept, policy, Stage 7, a scheduler/graph, shader
computation, ray batch migration, or SGEMM migration.

## Result

CONCEPT/VULKAN M0: SUCCESS
