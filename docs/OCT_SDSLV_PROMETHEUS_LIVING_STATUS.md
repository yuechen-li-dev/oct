# Oct / SDSL-V / Prometheus living project status

Date: 2026-07-24

Status: current in-repository briefing

## Current direction

Oct is the scientific language/toolchain. SDSL-V is its typed shader-side
language and compiler. Prometheus is the native Vulkan execution runtime and
public semantic GPU-capability boundary. Dominatus is the Prometheus control
kernel.

The immediate project is **Concept/Vulkan**: a bounded host-side language
profile for expressing Prometheus Vulkan mechanisms without continuing to
author the repetitive bulk of new mechanisms directly in raw Vulkan C.
Concept/Vulkan M0 fixed the language constitution and implementation boundary.
The original kernel-54 proof vertical is now accepted as sufficient proof of
compiler/native viability. M1D remains honestly classified as
`MEANINGFUL PROGRESSION`: one private executable linked and invoked the real
handwritten and generated kernel-54 paths on an admitted live runtime, but the
proposed M1E handwritten create-path failure seam is not the active
assignment. EVT1 M1A is accepted and closed. **Concept/Vulkan EVT1 M1B-A** is
accepted and closed. **Concept/Vulkan EVT1 M1B-B** is now complete:
constrained one-parameter free-function templates, explicit concrete
invocation, symbolic body checking against named concept closures, and
deterministic private C11 monomorphization all run through the EVT1 typed MIR
without introducing runtime generic machinery. **Concept/Vulkan EVT1 M1B-C**
is accepted and closed. **Concept/Vulkan EVT1 M1B-D** is now complete:
fixed-size compile-time arrays, typed array literals, deterministic indexing,
exact `Len(...)`, array-valued compile-time declarations/functions/locals,
structural equality over valid element domains, bounded `while` traversal, and
finite structural validation all run through parsing, validation, typed MIR,
generated C, checked outputs, and native specimens without introducing runtime
collections or changing production authority. Production remains handwritten.

The owner direction is:

```text
Essential Vulkan decisions remain explicit.
Mechanical consequences are generated.
```

Concept/Vulkan is production-driven and Concept-compatible in direction. The
experimental general Concept project remains paused as a production
dependency. If it later becomes feature-complete and self-hosted, the proven
profile design may be backported.

## System responsibilities

| System | Current responsibility | Explicit non-responsibility |
| --- | --- | --- |
| Oct | scientific language, tests, artifacts, packages, native integration, and the host compiler/tooling lineage | Vulkan mechanism policy or shader computation |
| SDSL-V | shader-side computation, typed shader resources, validation, VD-MIR, HLSL/SPIR-V artifacts, and package metadata | host runtime mechanisms, lifecycle, or scheduling |
| Dominatus | semantic lifecycle, admission, coordination, blackboard state, judgment, policy/variant choice, authorization, progress, commitment, and completion | Vulkan command/resource ceremony |
| Concept/Vulkan | host-side Vulkan resources, bindings, pipelines, command recording, declared access/synchronization, dispatch, submission, observation, and deterministic cleanup for already-committed work | policy, scheduling, lifecycle/progression authority, model topology, or shader mathematics |
| Prometheus | reusable semantic GPU capabilities and consumption of generated/native C/H | dependence on the future general Concept compiler |
| generated C/H | auditable checked-in native build inputs and existing Prometheus integration boundary | semantic authority independent of `.concept`, packages, ABI inputs, or generation manifest |

The permanent boundary is:

```text
Dominatus decides and coordinates.
Vulkan mechanisms execute and report facts.
```

## Accepted current Prometheus boundary

Stages 3–5 remain production authority:

- `prom_vk_runtime` owns the common Vulkan instance/device/queues/command pools,
  capability observations, package, and reverse cleanup;
- the Stage 4 SGEMM handoff contains already-committed mechanism facts after
  Dominatus selection/admission;
- Stage 5 owns policy-free buffer creation, requirements, allocation, binding,
  optional mapping, and cleanup mechanics.

Stage 6 established that Z-Image's 34-position retarget sequence is finite but
not safely extractable as a closed execution plan. Model-operation
authorization and `retarget_position` progression remain Vulkan-session owned,
and no Dominatus authorization/completion seam exists. Prometheus Stage 7 is
deferred until after the initial Concept/Vulkan proof. Concept/Vulkan does not
conceal or solve that missing seam.

The accepted ray-query implementation includes the owner-accepted physical
batch path: one supported nonzero semantic batch produces one
`vkCmdDispatch(ray_count,1,1)` and one synchronous submission, with paired
mapped capacity, descriptor rebind-before-retire, reuse on shrink, public
contract validation, and prior RTX/Linux build evidence. Expanded independent
full-image authority and cost accounting are paused at that boundary for the
Concept/Vulkan detour; they are not completed or abandoned.

## Authority and build boundary

Generated C/H will initially be deterministic, readable, checked in, and
reviewed beside handwritten production mechanisms. Downstream native builds
continue to work without compiler regeneration. Shader source, SPIR-V,
shader-package identity, public ABI, model lock/projections, and generated C/H
each retain distinct authority.

Raw Vulkan C is no longer the intended authoring surface for new Prometheus
reactors. Existing handwritten mechanisms remain production authority until a
generated path proves behavioral and failure-path equivalence.

## Current bounded slice

Concept/Vulkan EVT1 now includes:

- ordinary mutable `struct` declaration, positional construction, field read,
  field mutation, value-copy, nested-field access, and deterministic transparent
  C11 representation;
- bounded `immovable struct` semantics: mutable final-storage construction,
  borrow-based mutation, and explicit rejection of copy, whole-value
  assignment, by-value passing/return, embedding, and enum payload use;
- named one-parameter `concept` declarations, free-function operation
  requirements, prerequisite concepts, cycle rejection, and exact-signature
  checking;
- declaration-level concrete satisfaction assertions that remain
  compile-time-only and emit no runtime tables, vtables, witness objects, or
  public symbols;
- constrained free-function templates with exactly one type parameter and
  exactly one named concept constraint over that same parameter;
- explicit `TemplateName<ConcreteType>(...)` invocation with no deduction;
- symbolic dependent-call binding to ordered concept requirements and
  deterministic concrete monomorphization to one private C11 helper per unique
  `(template, concrete type)` key;
- top-level and local `comptime` declarations plus top-level `comptime`
  free functions;
- compile-time-only `static_assert`, expression-valued `if`, ordinary runtime
  `while`, and compile-time `while` gated by explicit `bounded(limit)`;
- deterministic bounded compile-time evaluation over `int`, `bool`, `string`,
  enums, structs, and fixed arrays composed from accepted compile-time values;
- exact fixed-array literals, indexing, and `Len(...)` in compile-time
  contexts only, with complete erasure before runtime C11 lowering;
- focused hardware-independent and Vulkan-shaped specimens proving structs,
  immovability, concept satisfaction, explicit-only template instantiation,
  deterministic instance reuse, bounded compile-time evaluation, finite
  structural validation over ordered arrays, and preserved M1A enum/`match`
  behavior.

The owner-preserved paused states are unchanged:

- Prometheus RQ-M1 physical batching is preserved but paused;
- DVT-2 optimization is preserved but paused;
- EVT1 M1B-D now closes the fixed-size compile-time array gap and completes the
  bounded EVT1 compile-time substrate needed for the first DragonGod vertical;
- the historical kernel-54 milestone named `M1C` remains distinct from the
  later planned EVT1 milestone named `M1B-C`;
- DragonGod lifecycle automata remain deferred, but they are now the intended
  first serious post-substrate direction rather than waiting on another EVT1
  compile-time language blocker;
- the first production mechanism reconstruction remains deferred to EVT1 M1C;
- backporting accepted EVT1 semantics into the broader Zig Concept bootstrap
  remains outside this assignment and another reviewer’s boundary;
- the previously proposed M1E failure-equivalence seam is not the current
  assignment.

## Provenance note

The M0 assignment referred to a canonical July 23 living-status text with
different wording and a system-responsibility table. No tracked file containing
that text exists at checkpoint
`457671c3094ead30019b4a96407430fe010d6998`. This document establishes the
in-repository current briefing and makes that authority gap explicit rather
than silently attributing edits to a missing file.
