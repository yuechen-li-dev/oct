# Prometheus Concept/Vulkan M1C — native and conformance closure

Status: **MEANINGFUL PROGRESSION — generated kernel-54 C now compiles to a real
native object and links against the real Prometheus/Vulkan contract; M1D has
since proven one admitted live handwritten/generated executable equivalence
path, but exhaustive handwritten create-path failure equivalence remains a
separate narrow closure seam.**

Starting checkpoint: `0ca7aa219e8a975020d57a2e5fc501e01f4fc0ad`; porcelain was
empty. M1C does not change Concept grammar, public API, production routing,
shader/package authority, Dominatus, SDSL-V, or Stages 3–6.

## Native design

The checked-in generated pair is `reactor_concept_vulkan_kernel54.generated.c/h`.
Its private `prom_concept_vulkan_kernel54_execute` now performs the bounded M1
lowering against real declarations: Stage-5 coherent storage buffer creation,
zero/map, descriptor layout/pool/set and bindings 0/1, package module for
`prometheus.core@1` / `kernel-54-default`, compute pipeline, one command,
`vkCmdDispatch(1,1,1)`, fence submit/wait, mapped readback, and reverse
initialized-only cleanup. It accepts borrowed services/TLAS/package and never
destroys them.

The legitimate toolchain was MSYS2 GCC 15.2 with the installed Vulkan SDK
`1.4.350.0` headers and import library. Generated C passed C11 syntax and
warning-clean object compilation. A conformance-only executable linked the
generated object with real `reactor_vulkan_common.c`,
`reactor_shader_package.c`, and `vulkan-1`; no fake declarations or SDK files
were added. Existing shader-package warnings require its inherited warning
policy during that link; generated C itself remains warning-clean.

## Private adapter and correspondence

Under `PROM_CONCEPT_VULKAN_CONFORMANCE` only,
`reactor_vulkan_ray_query.c` exposes two non-public adapters in its own
translation unit: one calls the actual handwritten
`prom_ray_query_triangle_scene_probe_impl`; the other obtains the same scene
services/package/TLAS and calls generated `Execute`. Thus the adapter has
access to the existing static scene lookup and helpers without removing
`static`, adding a public header, or altering the normal build. Production does
not define the macro or compile the generated C as a production source.

The actual handwritten authority remains
`prom_ray_create_compute_resources` and
`prom_ray_query_triangle_scene_probe_impl`: four-byte coherent evidence;
AS/read binding 0; storage/write binding 1; package entry `kernel-54-default`;
one synchronous `(1,1,1)` dispatch/readback. Generated code has the same
construction and cleanup sequence, represented by unchanged M1 MIR operations:
source -> create buffer/pipeline/descriptors/command -> access -> dispatch ->
submit/wait -> observe -> reverse drop. The conformance adapter object compiled
successfully with the real headers.

## Evidence classification

| Lane | Result |
| --- | --- |
| generated C syntax / C11 object | PASS |
| adapter object under conformance macro | PASS |
| conformance executable link | PASS |
| deterministic generation and stale-output test | PASS |
| source map/manifest JSON and native generated inventory | PASS |
| live handwritten/generated trace, success, cleanup, failure injection | NOT RUN — no admitted runtime exercised |
| Vulkan validation/repeated lifecycle | NOT RUN |
| production routing, public ABI/export, package/shader/lock preservation | PASS by isolated macro/private artifact review |

M1D now owns the live executable harness and validation-layer evidence. The
remaining closure is the handwritten create-path partial-failure seam needed
for stage-by-stage cleanup equivalence. M2 remains kernel-55 physical-batch
equivalence only after that closure. Rollback removes this generated lowering,
macro-gated adapter, and M1C evidence; handwritten production remains
authoritative.
