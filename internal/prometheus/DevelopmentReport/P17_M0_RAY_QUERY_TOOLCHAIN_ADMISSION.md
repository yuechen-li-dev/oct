# P17 M0 — Ray-query toolchain and runtime admission

Status: meaningful progression; this is not an M1 renderer qualification.

## Repository naming note

`PROMETHEUS_P14_PLUS_ROADMAP.md` already uses **P17** for PVT evidence tuning.
The requested ray-query work uses the same identifier.  This report preserves
the requested `P17 M0` label so its evidence is discoverable, but does not
silently rewrite that separate roadmap or claim that its PVT work is complete.
Future planning should assign one of these tracks a distinct program identifier
before treating either document as the single current-work pointer.

## Purpose

P17's first bounded requirement was to prove that Prometheus can represent and
compile a Vulkan ray-query compute workload without an RT-pipeline or shader
binding table.  The previous tree contained compute-only Vulkan reactors and
no acceleration-structure resource in SDSL-V.

This milestone adds a compiler-owned, deliberately narrow surface:

- `readonly acceleration_structure` is a first-class SDSL-V shader resource;
- `RayQueryAny(tlas, origin, direction, tMin, tMax)` is a compute-only
  intrinsic with an exact typed signature;
- the HLSL emitter owns `RayDesc` and inline `RayQuery` construction, traversal,
  and committed-hit conversion; source shaders cannot inject raw HLSL;
- the toolchain selects `cs_6_5` and records the ray-query capability plus
  `VK_KHR_acceleration_structure` and `VK_KHR_ray_query` requirements.

The production asset is
`shaders/sdslv/production/rayquery/ray_query_capability_probe.sdslv`.  Its
checked-in canonical module header is
`reactor_vulkan_ray_query_capability_probe_spirv.h`; it is registered as
production shader asset 54 and is covered by the native manifest.

## Verified compiler evidence

The following completed on 2026-07-22 with the installed Vulkan SDK DXC and
SPIR-V Tools:

```text
go run ./cmd/oct sdslv check internal/prometheus/shaders/sdslv/production/rayquery/ray_query_capability_probe.sdslv
go run ./cmd/oct sdslv compile internal/prometheus/shaders/sdslv/production/rayquery/ray_query_capability_probe.sdslv --validate --require-spirv-val
```

The canonical module SHA-256 is
`904e4820ad6cad5b7c12b63364f559b7f61e60be350b624b549959eb121cacd7`.
Disassembly contains the expected hardware ray-query instructions and types:

```text
OpCapability RayQueryKHR
OpTypeAccelerationStructureKHR
OpTypeRayQueryKHR
OpRayQueryInitializeKHR
OpRayQueryProceedKHR
OpRayQueryGetIntersectionTypeKHR
```

This is a real Vulkan ray-query module, not a CPU traversal, an RT-pipeline
module, or a text-only capability declaration.

## Runtime admission boundary

The shared Vulkan runtime now probes and conditionally enables:

- `VK_KHR_acceleration_structure`;
- `VK_KHR_ray_query`;
- `VK_KHR_deferred_host_operations`;
- buffer-device-address, acceleration-structure, and ray-query feature bits.

It loads only the acceleration-structure entry points required for the next
scene-building milestone.  It exposes an explicit state:
`unsupported`, `extension-missing`, `feature-missing`, `entry-point-missing`,
or `device-feature-enabled`.  Standard compute initialization remains usable
when ray-query admission is unavailable.  The native admission test passes
both normally and with `PROMETHEUS_VK_VALIDATION=1`.

`vulkaninfo --summary` on the qualification host reports an NVIDIA GeForce RTX
3070 (driver 596.36.0.0) and a Vulkan 1.4.329 device.  That establishes the
available hardware context, but it is not presented as BLAS/TLAS execution
proof because this milestone intentionally has not built or dispatched a
scene yet.

## Deliberate M1 boundary

M1 remains open.  There is no public scene handle, BLAS/TLAS build/update,
device-address geometry buffer ownership, descriptor-AS update, rendered
image, analytic-sphere candidate/commit shader path, shadow query, CPU oracle,
or benchmark report in this change.  Consequently there is no claim of a
hardware-rendered 10-sphere result, image comparison, no-fallback behavior, or
GPU timing.

The concrete next implementation block is now isolated: add the owned native
scene lifecycle and a minimal triangle BLAS/TLAS build, bind that TLAS to this
already-validated canonical module, and make the capability probe dispatch
under validation.  Once that real path exists, expand the compiler-owned
ray-query surface from the M0 boolean probe to the candidate/commit operations
needed for analytic procedural spheres and only then qualify the full M1
renderer against its CPU oracle.

## Checks run

```text
go test ./internal/sdslv/...
go test ./internal/prometheus/...
go run ./tools/prometheus_native_manifest -check
go run ./tools/sdslv_workspace_check -inventory
cmd /c internal\prometheus\native\build_windows.cmd
out\prometheus\native\marionette_tests.exe PrometheusReactor_RayQuery
PROMETHEUS_VK_VALIDATION=1 out\prometheus\native\marionette_tests.exe PrometheusReactor_RayQuery
git diff --check
```

All listed checks passed.  No commit, tag, push, or pull request was created.
