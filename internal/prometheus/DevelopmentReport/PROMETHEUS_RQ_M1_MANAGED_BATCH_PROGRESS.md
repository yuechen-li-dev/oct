# Prometheus RQ-M1 managed batch progress

Status: meaningful progression. The public semantic boundary, public-only
camera client, and admitted RTX execution are now present. The remaining work
is to replace the intentionally synchronous per-ray submission loop with one
bounded batch dispatch and to attach the camera specimen to an independent
full-image CPU authority and timing report.

## Inspected starting architecture

- Public and internal entry points were `prometheus_reactor_runtime_create`,
  `prometheus_reactor_runtime_ray_query_scene_create`, single-ray
  `...scene_trace`, and the legacy triangle probe API in `reactor_api.h/.c`
  and `reactor_vulkan_ray_query.c`.
- Triangles were copied to a device-addressable vertex buffer, used for one
  triangle BLAS, and also copied to a shader storage buffer for normals.
  Spheres were finite host records copied to one shader storage buffer and one
  procedural-AABB BLAS input. A TLAS held one identity instance per populated
  BLAS.
- Rays were previously one two-`float4` record dispatched once; raw hits were
  six `float4` lanes returned from a mapped buffer. The reactor owned every
  Vulkan object: buffers, BLAS/TLAS, scratch, descriptor set/pool/layout,
  pipeline layout/pipeline, command buffers, submit fence, and package module.
- Production shader identities remain package kernel 54
  `ray-query-capability-probe` and kernel 55 `ray-query-raw-hit`, with runtime
  variant `kernel-55-default`. The package remains digest-addressed and
  verified by `reactor_shader_package.c`; no caller SPIR-V or embedded fallback
  was added.

## RQ-M1 boundary

`include/prometheus_ray_query.h` is the Vulkan-free public contract:

1. `prometheus_ray_query_runtime_create` accepts only a package-root config.
2. Create an opaque empty scene.
3. Add copied triangle and analytic-sphere arrays.
4. Commit once; mutation after commit is rejected.
5. Submit a synchronous host-memory `PrometheusRayQueryBatchRequest`.
6. Destroy scene and runtime.

Rays contain origin, direction, closed `[t_min,t_max]`, and a visibility mask.
Zero-size batches are accepted after commit. Nonzero null arrays, undersized
strides, non-finite values, zero directions, inverted ranges, overflowed spans,
foreign/destroyed handles, and uncommitted scenes are rejected. Complete input
validation occurs before caller output is changed; successful batches replace
every output record with a fresh hit or explicit miss sentinel.

Hits return hit/miss, geometry kind, instance and primitive identities,
distance, barycentrics, position, normal, and analytic-sphere albedo/material.
Misses use zeroed fields except `instance_id`/`primitive_id` `UINT32_MAX`,
distance `-1`, and barycentrics `(-1,-1)`.

Input geometry is copied during add; callers may release it on return. After
commit the scene is immutable. The reactor remains the sole Vulkan owner.

## Evidence

On the admitted owner RTX route with `PROMETHEUS_VK_VALIDATION=1`:

- public-header layout/lifetime and mixed triangle+sphere batch tests passed;
- the raw mixed traversal test and double-precision numerical corpus passed;
- numerical maxima remained `t=9.22933504e-7`, position
  `9.22933504e-7`, normal `9.22933504e-7`, barycentric `1.3038516e-8`;
- SDSL-V check, lower/validator/emitter Go tests, DXC compilation, and
  `spirv-val` passed. Disassembly retained inline ray query initialization,
  progression, procedural intersection generation, and instance queries;
- the public-only camera client generated deterministic 64x64 PPM views for
  hit, distance, identity, geometry kind, normal, and barycentrics. A second
  identical execution retained all six SHA-256 hashes.

The client is `tools/prometheus_ray_query_camera_diagnostic/main.c`; it has no
Vulkan or private Prometheus include. PPM is intentionally used to avoid
embedding an image library in the foreign client.

## RQ-M1 owner acceptance and closure

PROMETHEUS RQ-M1 is owner-accepted and closed. The committed execution scene
retains paired mapped ray/raw-hit capacity, grows deterministically when needed,
rebinds descriptors 2 and 3 before retiring replaced buffers, and reuses that
capacity for later smaller batches. Each supported nonzero semantic batch records
one `vkCmdDispatch(ray_count,1,1)` and one synchronous submission; zero rays
remain a no-dispatch semantic no-op.

The reactor-private structural test proves that a 9-ray API batch adds exactly
one physical dispatch and submission, performs one paired buffer growth and one
descriptor rebind, and then reuses capacity on shrink. Kernel 55 remains
`kernel-55-default`, with package artifact
`917c37aadf40226fa0950003a25b0a449f9e6e05a614585ccb9aaa2f0d322012`.
SPIR-V inspection confirms dispatch-indexed ray/hit access together with the
accepted ray-query and procedural-intersection instructions.

Windows RTX validation and the Linux native build pass. The Linux strict-C
`strnlen` portability blocker was repaired by an equivalent bounded local scan.

The full 64x64 independent CPU image oracle and expanded cost-accounting
instrumentation are explicitly deferred integration-validation work for the
first substantial external Prometheus consumer, where end-to-end visual and
performance evidence will be stronger. They are not RQ-M1 blockers and should
not prompt further RQ-M1 implementation unless that consumer exposes a concrete
defect.
