# Prometheus Concept/Vulkan M1 — kernel-54 compiler vertical

Status: **MEANINGFUL PROGRESSION — the hermetic compiler/artifact vertical is complete; private handwritten helper access prevents runtime-equivalent generated invocation without a production seam.**

Starting checkpoint: `be293f5f14fc1c0b324ce57ed44492395926474f`
(`concept-vulkan: establish M0 language constitution`), with empty porcelain.

## Scope and source

M1 adds `internal/conceptvulkan`, `cmd/concept-vulkan`, and canonical source
`Examples/Concept-Vulkan/kernel54_probe.concept`. Source naming is PascalCase
for `Execute` and operations and camelCase for parameters/locals. Its only
grammar is `profile Vulkan;`, the Prometheus import, one fixed typed borrowed
context/imported-TLAS function, owned locals, access declarations, fixed
dispatch, explicit command move, observation, and return. General Concept,
effects, async, policy, graphs, shader code, C embedding, and package
management are deferred.

The source grammar is C++-shaped: `ReturnType Function(Type parameter)` and
`owned LocalType local = expression;`. The earlier M1 `fn`, `name: Type`,
`-> ReturnType`, and untyped owned-local sketch was corrected after M1; it is
not accepted as compatibility syntax. `let` and `var` are likewise rejected.
The canonical local types are the existing M1 capabilities: concrete mapped
evidence buffer, `ComputePipeline`, `DescriptorSet`, `CommandRecording`, and
`Submission`; this correction introduces no runtime abstraction.

Compile-time facts are source plus fixed `prometheus.core@1` /
`kernel-54-default` and descriptor facts; no GPU, environment, network,
package-service, or absolute-path query occurs. Runtime facts remain admitted
context and TLAS.

## Type/MIR/outputs

Stable `CV1000`–`CV1202` diagnostics retain line/column spans. Validation
requires the kernel-54 order: evidence/pipeline/descriptors/command ownership;
TLAS read and evidence shader-write; `Dispatch(1,1,1)`; command move and
synchronous completion; observation; reverse cleanup. It rejects malformed
profiles/declarations, snake_case names, unsupported constructs, and altered
access/binding/dispatch/order.

The ten typed, source-spanned MIR operations are `create_buffer`,
`create_pipeline`, `bind_descriptor`, `begin_recording`, two `declare_access`,
`dispatch`, `submit_wait`, `observe`, and `return`. They map respectively to
Stage-5 `prom_vk_create_buffer`, package/module/pipeline creation, descriptor
writes 0/1, command begin, semantic access/completion, `vkCmdDispatch(1,1,1)`,
synchronous submit/wait, mapped `memcpy`, and reverse initialized cleanup.
Ownership and fallibility are explicit per operation; borrowed context/TLAS are
never dropped.

Checked-in C/H, MIR, source map, and manifest live in
`internal/prometheus/native/` with the deterministic
`reactor_concept_vulkan_kernel54` prefix. They are deterministic and
repository-relative. The manifest includes source/output SHA-256, compiler,
package/entry/descriptor identity and options; it has no timestamp or host
data. `concept-vulkan check` byte-compares all outputs and reports `CV3001` for
missing, stale, or hand-edited output. C/H is private conformance-only and adds
no public API.

## Handwritten witness and blocker

The exact authority is `prom_ray_create_compute_resources` and
`prom_ray_query_triangle_scene_probe_impl` in `reactor_vulkan_ray_query.c`:
`kernel-54-default`; AS/read binding 0; storage/write binding 1; four-byte
coherent mapped evidence; zero/write; one compute dispatch; synchronous
completion; readback. MIR/comments provide static correspondence.

The required `prom_ray_query_scene`, command begin, and submit/free helpers are
`static` in that C translation unit. No existing private adapter accepts the M1
admitted context/TLAS while exposing package/pipeline/descriptor/command state
and failure seams. Duplicating that state or adding a production helper is out
of M1 scope. Native compilation is **NOT RUN** because this checkout lacks
Vulkan headers; runtime success/failure/repeat/validation equivalence is also
**NOT RUN** and is not claimed.

## Preservation, validation, and M2

No production source/header, public ABI/export, shader/SPIR-V/package,
manifest/lock/kernel, Dominatus authority, SDSL-V behavior, Stage 3/4/5/6
contract, Stage 7 state, or ray-batch/image state changed. The 84-export/ABI
baseline is preserved because no public symbol is added.

| Lane | Result |
| --- | --- |
| parser/span/naming/ownership/MIR/determinism tests | PASS |
| two generations, output check, stale rejection | PASS |
| map/manifest JSON parse | PASS |
| native syntax compile | NOT RUN — Vulkan SDK headers unavailable |
| static witness correspondence | PASS — MIR/witness review |
| runtime success/failure/cleanup/validation | NOT RUN — private adapter absent |

M1C now owns the private kernel-54 adapter and native conformance evidence.
M2 begins only after M1C runs it against the same admitted scene/TLAS for
result, cleanup, failure-injection, and validation equivalence; M2 then assesses
kernel-55 physical-batch equivalence.
Rollback removes these additive compiler/source/generated/report/evidence files.
