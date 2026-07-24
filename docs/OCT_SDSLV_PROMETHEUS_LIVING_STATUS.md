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
assignment. The active assignment is **Concept/Vulkan EVT1 M1A**: payload
enums and exhaustive `match`. Production remains handwritten.

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

## Next bounded slice

Concept/Vulkan EVT1 M1A adds:

- one real payload-enum and exhaustive-`match` language vertical from
  `.concept` source through typed MIR, deterministic C11, native compilation,
  and executable behavior;
- mixed unit, single-payload, multi-payload, and enum-payload variants;
- qualified variant construction only;
- expression-form and statement-form `match`;
- explicit tag-plus-union C11 lowering with single-evaluation guarantees;
- focused hardware-independent and Vulkan-shaped specimens.

The owner-preserved paused states are unchanged:

- Prometheus RQ-M1 physical batching is preserved but paused;
- DVT-2 optimization is preserved but paused;
- EVT1 M1B (concepts-first templates and compile-time evaluation) is deferred
  until after M1A;
- the previously proposed M1E failure-equivalence seam is not the current
  assignment.

## Provenance note

The M0 assignment referred to a canonical July 23 living-status text with
different wording and a system-responsibility table. No tracked file containing
that text exists at checkpoint
`457671c3094ead30019b4a96407430fe010d6998`. This document establishes the
in-repository current briefing and makes that authority gap explicit rather
than silently attributing edits to a missing file.
